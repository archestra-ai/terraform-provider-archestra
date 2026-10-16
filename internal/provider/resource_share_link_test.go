package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	fwresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

const shareSkillFixtures = `resource "archestra_skill" "first" {
 name="tf-share-first"
 description="Synthetic link fixture"
 content="Synthetic first skill."
 files={}
 labels=[]
 environment_ids=[]
 }
 resource "archestra_skill" "second" {
 name="tf-share-second"
 description="Synthetic link fixture"
 content="Synthetic second skill."
 files={}
 labels=[]
 environment_ids=[]
 }`

func shareLinkAPI(method, path string) (int, []byte, error) {
	// #nosec G704 -- The runner explicitly chooses its isolated API; callers supply only fixed routes and owned UUIDs.
	req, err := http.NewRequestWithContext(context.Background(), method, os.Getenv("ARCHESTRA_BASE_URL")+path, nil)
	if err != nil {
		return 0, nil, fmt.Errorf("invalid fixture request")
	}
	req.Header.Set("Authorization", os.Getenv("ARCHESTRA_API_KEY"))
	// #nosec G704 -- This is the explicitly configured acceptance API.
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, nil, fmt.Errorf("fixture API request failed")
	}
	defer func() { _ = response.Body.Close() }()
	body, err := io.ReadAll(response.Body)
	return response.StatusCode, body, err
}
func shareLinkAnonymousStatus(cloneURL string) (int, error) {
	target, err := url.Parse(cloneURL)
	if err != nil {
		return 0, fmt.Errorf("invalid clone URL")
	}
	base, err := url.Parse(os.Getenv("ARCHESTRA_BASE_URL"))
	if err != nil {
		return 0, fmt.Errorf("invalid isolated API URL")
	}
	target.Scheme = base.Scheme
	target.Host = base.Host
	target.Path += "/info/refs"
	target.RawQuery = "service=git-upload-pack"
	// #nosec G704 -- Clone paths are exercised against the explicitly selected acceptance API, without authorization headers.
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, target.String(), nil)
	if err != nil {
		return 0, fmt.Errorf("invalid anonymous fixture request")
	}
	c := &http.Client{Timeout: 30 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	// #nosec G704 -- The request uses the runner's own API host; errors deliberately omit its secret-bearing URL.
	response, err := c.Do(req)
	if err != nil {
		return 0, fmt.Errorf("anonymous fixture request failed")
	}
	defer func() { _ = response.Body.Close() }()
	_, err = io.Copy(io.Discard, response.Body)
	return response.StatusCode, err
}
func TestAccMarketplaceShareLinkSkills(t *testing.T) {
	config := func(nonce, skill string) string {
		return shareSkillFixtures + fmt.Sprintf(`
 resource "archestra_share_link" "test" {
 name="tf-skill-share"
 skill_ids=[archestra_skill.%s.id]
 plugin_ids=[]
 rotation_nonce=%q
 }`, skill, nonce)
	}
	var id, raw, clone string
	capture := func(s *terraform.State) error {
		v := s.RootModule().Resources["archestra_share_link.test"].Primary
		id = v.ID
		raw = v.Attributes["raw_token"]
		clone = v.Attributes["clone_url"]
		if !strings.HasPrefix(raw, "archestra_skl_") || !strings.Contains(clone, raw) {
			return fmt.Errorf("secret-once token and clone URL were not published")
		}
		return nil
	}
	preserve := func(s *terraform.State) error {
		v := s.RootModule().Resources["archestra_share_link.test"].Primary
		if v.ID != id || v.Attributes["raw_token"] != raw || v.Attributes["clone_url"] != clone {
			return fmt.Errorf("refresh lost secret-once delivery state")
		}
		return shareLinkReadable(clone)
	}
	replacement := func(s *terraform.State) error {
		v := s.RootModule().Resources["archestra_share_link.test"].Primary
		if v.ID == id || v.Attributes["raw_token"] == raw {
			return fmt.Errorf("explicit link replacement retained its old identity or token")
		}
		code, err := shareLinkAnonymousStatus(clone)
		if err != nil {
			return err
		}
		if code != http.StatusNotFound {
			return fmt.Errorf("replaced token still serves public clone access: status%d", code)
		}
		if err := capture(s); err != nil {
			return err
		}
		return shareLinkReadable(clone)
	}
	resource.Test(t, resource.TestCase{PreCheck: func() { testAccPreCheck(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, CheckDestroy: func(*terraform.State) error {
		code, err := shareLinkAnonymousStatus(clone)
		if err != nil {
			return err
		}
		if code != http.StatusNotFound {
			return fmt.Errorf("destroy did not revoke the final clone URL: status%d", code)
		}
		rows, err := shareLinkRows()
		if err != nil {
			return err
		}
		for _, row := range rows {
			if row["id"] == id {
				if row["status"] != "revoked" {
					return fmt.Errorf("deleted link was not marked revoked")
				}
				if _, present := row["rawToken"]; present {
					return fmt.Errorf("list unexpectedly disclosed the raw token")
				}
				return nil
			}
		}
		return fmt.Errorf("revoked link metadata disappeared")
	}, Steps: []resource.TestStep{
		{Config: config("1", "first"), Check: capture},
		{ResourceName: "archestra_share_link.test", ImportState: true, ImportStateVerify: true, ImportStateVerifyIgnore: []string{"raw_token", "clone_url", "rotation_nonce"}},
		{Config: config("1", "first"), Check: preserve, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
		{Config: config("2", "first"), Check: replacement, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction("archestra_share_link.test", plancheck.ResourceActionDestroyBeforeCreate)}}},
		{PreConfig: func() {
			code, _, err := shareLinkAPI(http.MethodDelete, "/api/skill-share-links/"+id)
			if err != nil || code != http.StatusOK {
				t.Fatal("unable to revoke owned link fixture")
			}
		}, Config: config("2", "first"), Check: replacement},
		{Config: config("2", "second"), Check: replacement, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction("archestra_share_link.test", plancheck.ResourceActionDestroyBeforeCreate)}}},
	}})
}
func TestAccMarketplaceShareLinkPluginsAndExpiry(t *testing.T) {
	config := func(expiry string, mixed bool) string {
		skills := `[]`
		if mixed {
			skills = `[archestra_skill.first.id]`
		}
		return shareSkillFixtures + "\n" + pluginFixtureConfig("tf-shared-plugin", "Synthetic package", true, false) + fmt.Sprintf(`
 resource "archestra_share_link" "test" {
 name="tf-plugin-share"
 skill_ids=%s
 plugin_ids=[archestra_plugin.test.id]
 plugin_platform="posix"
 expires_at=%q
 }`, skills, expiry)
	}
	var id, clone string
	capture := func(s *terraform.State) error {
		v := s.RootModule().Resources["archestra_share_link.test"].Primary
		id = v.ID
		clone = v.Attributes["clone_url"]
		return nil
	}
	expired := func(s *terraform.State) error {
		v := s.RootModule().Resources["archestra_share_link.test"].Primary
		if v.ID == id {
			return fmt.Errorf("expiry change did not replace link")
		}
		code, err := shareLinkAnonymousStatus(clone)
		if err != nil {
			return err
		}
		if code != http.StatusNotFound {
			return fmt.Errorf("old executable-link token remained readable")
		}
		clone = v.Attributes["clone_url"]
		code, err = shareLinkAnonymousStatus(clone)
		if err != nil {
			return err
		}
		if code != http.StatusNotFound {
			return fmt.Errorf("expired executable-link token remained readable")
		}
		return nil
	}
	resource.Test(t, resource.TestCase{PreCheck: func() { testAccPreCheck(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{
		{Config: config("2040-01-01T00:00:00Z", false), Check: resource.ComposeTestCheckFunc(capture, resource.TestCheckResourceAttr("archestra_share_link.test", "skill_ids.#", "0"), resource.TestCheckResourceAttr("archestra_share_link.test", "plugin_client_type", "claude-code"))},
		{ResourceName: "archestra_share_link.test", ImportState: true, ImportStateVerify: true, ImportStateVerifyIgnore: []string{"raw_token", "clone_url"}},
		// Clone access updates last_used_at, so verify import before invoking it.
		{Config: config("2040-01-01T00:00:00Z", false), Check: func(*terraform.State) error { return shareLinkReadable(clone) }},
		{Config: config("2040-01-01T00:00:00Z", true), Check: resource.ComposeTestCheckFunc(capture, func(*terraform.State) error { return shareLinkReadable(clone) }, resource.TestCheckResourceAttr("archestra_share_link.test", "skill_ids.#", "1"))},
		{Config: config("2001-01-01T00:00:00Z", true), Check: resource.ComposeTestCheckFunc(expired, resource.TestCheckResourceAttr("archestra_share_link.test", "status", "expired"))},
		{Config: config("2001-01-01T00:00:00Z", true), ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
	}})
}
func TestAccMarketplaceShareLinkRejectsInvalidInputs(t *testing.T) {
	for _, tc := range []struct{ name, fields, match string }{
		{"UUID case", `skill_ids=["11111111-ABCD-4111-8111-111111111111"]
 plugin_ids=[]`, "lowercase canonical skill UUID"},
		{"empty", `skill_ids=[]
 plugin_ids=[]`, "Empty marketplace membership"},
		{"platform", `skill_ids=[]
 plugin_ids=["11111111-1111-4111-8111-111111111111"]
 expires_at="2040-01-01T00:00:00Z"`, "Missing plugin platform"},
		{"expiry", `skill_ids=[]
 plugin_ids=["11111111-1111-4111-8111-111111111111"]
 plugin_platform="posix"`, "Missing plugin expiration"},
		{"timestamp", `skill_ids=["11111111-1111-4111-8111-111111111111"]
 plugin_ids=[]
 expires_at="2040-01-01T00:00:00.1234Z"`, "Noncanonical share-link timestamp"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resource.Test(t, resource.TestCase{PreCheck: func() { testAccPreCheck(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{{Config: `resource "archestra_share_link" "test" {
` + tc.fields + `
}`, ExpectError: regexp.MustCompile(tc.match)}}})
		})
	}
}
func TestShareLinkTimestampPrecision(t *testing.T) {
	for _, tc := range []struct {
		value string
		valid bool
	}{{"2040-01-01T00:00:00Z", true}, {"2040-01-01T00:00:00.123Z", true}, {"2040-01-01T00:00:00.1Z", true}, {"2040-01-01T00:00:00.1234Z", false}, {"2040-01-01T00:00:00.100Z", false}, {"2040-01-01T00:00:00+00:00", false}, {"not a date", false}} {
		t.Run(tc.value, func(t *testing.T) {
			resp := validator.StringResponse{}
			shareLinkTimestampValidator{}.ValidateString(t.Context(), validator.StringRequest{ConfigValue: types.StringValue(tc.value)}, &resp)
			if resp.Diagnostics.HasError() == tc.valid {
				t.Fatalf("unexpected timestamp validation: %v", resp.Diagnostics)
			}
		})
	}
}

func shareLinkRows() ([]map[string]any, error) {
	code, body, err := shareLinkAPI(http.MethodGet, "/api/skill-share-links")
	if err != nil {
		return nil, err
	}
	if code != http.StatusOK {
		return nil, fmt.Errorf("share-link list returned status%d", code)
	}
	var value struct {
		Links []map[string]any `json:"links"`
	}
	err = json.Unmarshal(body, &value)
	return value.Links, err
}

func TestMarketplaceShareLinkReadsPreserveSecretStateAndFailures(t *testing.T) {
	body := []byte(`{"id":"11111111-1111-4111-8111-111111111111","name":null,"organizationId":"org","createdByUserId":"user","tokenStart":"prefix","marketplaceName":"fixture","pluginClientType":null,"pluginPlatform":null,"expiresAt":null,"revokedAt":null,"lastUsedAt":null,"createdAt":"2026-01-01T00:00:00Z","updatedAt":"2026-01-01T00:00:00Z","status":"active","skills":[{"id":"22222222-2222-4222-8222-222222222222","name":"synthetic","description":""}],"plugins":[]}`)
	for _, status := range []int{http.StatusForbidden, http.StatusInternalServerError} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { http.Error(w, "synthetic read failure", status) }))
			defer server.Close()
			apiClient, err := client.NewClientWithResponses(server.URL)
			if err != nil {
				t.Fatal(err)
			}
			r := &MarketplaceShareLinkResource{client: apiClient}
			data := MarketplaceShareLinkResourceModel{}
			var d diag.Diagnostics
			r.flatten(t.Context(), &data, body, &d)
			if d.HasError() {
				t.Fatal(d)
			}
			if !data.RawToken.IsNull() || !data.CloneURL.IsNull() {
				t.Fatal("metadata import fabricated a secret")
			}
			data.RawToken = types.StringValue("synthetic-secret-once")
			data.CloneURL = types.StringValue("synthetic-clone-url")
			r.flatten(t.Context(), &data, body, &d)
			if data.RawToken.ValueString() != "synthetic-secret-once" || data.CloneURL.ValueString() != "synthetic-clone-url" {
				t.Fatal("metadata refresh erased the secret-once state")
			}
			var schemaResponse fwresource.SchemaResponse
			r.Schema(t.Context(), fwresource.SchemaRequest{}, &schemaResponse)
			state := tfsdk.State{Schema: schemaResponse.Schema}
			d.Append(state.Set(t.Context(), &data)...)
			if d.HasError() {
				t.Fatal(d)
			}
			response := fwresource.ReadResponse{State: state}
			r.Read(t.Context(), fwresource.ReadRequest{State: state}, &response)
			if !response.Diagnostics.HasError() || !response.State.Raw.Equal(state.Raw) {
				t.Fatal("failed list read did not preserve state")
			}
		})
	}
}

func shareLinkReadable(clone string) error {
	code, err := shareLinkAnonymousStatus(clone)
	if err != nil {
		return err
	}
	if code != http.StatusOK {
		return fmt.Errorf("new share does not grant read-only clone access: status%d", code)
	}
	return nil
}
