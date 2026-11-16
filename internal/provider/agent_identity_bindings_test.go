package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

// A platform without the identity-bindings route (1.4) answers a bare 404;
// the platform's own 404s carry an error message.
const (
	routeMissingBody  = `{"message":"Route GET:/api/agents/x/identity-bindings not found","error":"Not Found","statusCode":404}`
	platformErrorBody = `{"error":{"message":"Identity provider not found","type":"api_not_found_error"}}`
)

func identityBindingsServer(t *testing.T, status int, body string) (*AgentResource, *[]string) {
	t.Helper()
	var seen []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		seen = append(seen, req.Method)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		fmt.Fprint(w, body)
	}))
	t.Cleanup(server.Close)
	c, err := client.NewClientWithResponses(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	return &AgentResource{client: c}, &seen
}

func TestReadIdentityBindings(t *testing.T) {
	id := uuid.MustParse("11111111-1111-4111-8111-111111111111")
	for name, tc := range map[string]struct {
		status  int
		body    string
		wantErr bool
		want    int // bindings; -1 = null
	}{
		"bindings":                 {200, `{"bindings":[{"identityProviderId":"okta","externalId":"wlp1"}]}`, false, 1},
		"older platform is null":   {404, routeMissingBody, false, -1},
		"platform 404 is an error": {404, platformErrorBody, true, 0},
		"forbidden is an error":    {403, `{"error":{"message":"forbidden","type":"api_authorization_error"}}`, true, 0},
	} {
		t.Run(name, func(t *testing.T) {
			r, _ := identityBindingsServer(t, tc.status, tc.body)
			data := AgentResourceModel{IdentityBindings: types.SetUnknown(agentIdentityBindingType)}
			var diags diag.Diagnostics
			r.readIdentityBindings(t.Context(), id, &data, &diags)
			if diags.HasError() != tc.wantErr {
				t.Fatalf("diags = %v", diags)
			}
			if tc.wantErr {
				return
			}
			if tc.want < 0 {
				if !data.IdentityBindings.IsNull() {
					t.Fatalf("got %v, want null", data.IdentityBindings)
				}
				return
			}
			if got := len(data.IdentityBindings.Elements()); got != tc.want {
				t.Fatalf("got %d bindings, want %d", got, tc.want)
			}
		})
	}
}

func TestWriteIdentityBindingsOnAnOlderPlatformNamesTheRelease(t *testing.T) {
	id := uuid.MustParse("11111111-1111-4111-8111-111111111111")
	bindings, d := types.SetValueFrom(t.Context(), agentIdentityBindingType, []agentIdentityBinding{
		{IdentityProviderID: types.StringValue("okta"), ExternalID: types.StringValue("wlp1")},
	})
	if d.HasError() {
		t.Fatal(d)
	}
	for body, want := range map[string]string{routeMissingBody: identityBindingsMinimumAPIVersion, platformErrorBody: "Identity provider not found"} {
		r, seen := identityBindingsServer(t, 404, body)
		var diags diag.Diagnostics
		if r.writeIdentityBindings(t.Context(), id, &AgentResourceModel{IdentityBindings: bindings}, &diags) {
			t.Fatal("a 404 wrote the bindings")
		}
		if len(*seen) != 1 || (*seen)[0] != http.MethodPut {
			t.Fatalf("requests = %v", *seen)
		}
		if detail := diags.Errors()[0].Detail(); !strings.Contains(detail, want) {
			t.Errorf("error %q does not mention %q", detail, want)
		}
	}
}

func TestAccAgentIdentityBindings(t *testing.T) {
	if os.Getenv("TF_ACC") != "" && !testAccAPIVersionAtLeast(t, identityBindingsMinimumAPIVersion) {
		t.Skip("Identity bindings require API " + identityBindingsMinimumAPIVersion)
	}
	if os.Getenv("TF_ACC") == "" {
		return
	}
	testAccPreCheck(t)
	suffix := acctest.RandString(8)
	idpID := testAccAgentIdentityProvider(t, suffix)
	first, second := uuid.NewString(), uuid.NewString()
	const name = "archestra_agent.test"
	config := func(bindings string) string {
		return fmt.Sprintf(`
resource "archestra_agent" "test" {
  name          = "tf-acc-identity-%[1]s"
  system_prompt = "You are a helpful agent."
  %[2]s
}
`, suffix, bindings)
	}
	binding := func(externalID string) string {
		return fmt.Sprintf(`identity_bindings = [{ identity_provider_id = %q, external_id = %q }]`, idpID, externalID)
	}
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config(binding(first)),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(name, tfjsonpath.New("identity_bindings"), knownvalue.SetExact([]knownvalue.Check{
						knownvalue.ObjectExact(map[string]knownvalue.Check{
							"identity_provider_id": knownvalue.StringExact(idpID),
							"external_id":          knownvalue.StringExact(first),
						}),
					})),
				},
			},
			{
				Config: config(binding(second)),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(name, tfjsonpath.New("identity_bindings"), knownvalue.SetExact([]knownvalue.Check{
						knownvalue.ObjectPartial(map[string]knownvalue.Check{"external_id": knownvalue.StringExact(second)}),
					})),
				},
			},
			{
				// Omitting the attribute leaves the binding alone.
				Config: config(""),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(name, tfjsonpath.New("identity_bindings"), knownvalue.SetSizeExact(1)),
				},
			},
			{
				// An empty set clears it.
				Config: config("identity_bindings = []"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(name, tfjsonpath.New("identity_bindings"), knownvalue.SetSizeExact(0)),
				},
			},
		},
	})
}

// testAccAgentIdentityProvider creates a throwaway Microsoft Entra
// On-Behalf-Of identity provider with an Agent ID blueprint, which agents
// can be bound to by their Entra agent identity's app ID (a GUID), and
// deletes it after the test.
func testAccAgentIdentityProvider(t *testing.T, suffix string) string {
	t.Helper()
	issuer := "https://login.microsoftonline.com/" + uuid.NewString() + "/v2.0"
	payload := fmt.Sprintf(`{
		"providerId": "tf-acc-agent-identity-%[1]s",
		"domain": "%[1]s.agent-identity.test",
		"issuer": %[2]q,
		"oidcConfig": {
			"issuer": %[2]q,
			"discoveryEndpoint": %[3]q,
			"clientId": %[4]q,
			"clientSecret": "tf-acc-secret",
			"pkce": false,
			"skipDiscovery": true,
			"enterpriseManagedCredentials": {
				"exchangeStrategy": "entra_obo",
				"entraAgentId": {"blueprintClientId": %[5]q}
			}
		}
	}`, strings.ToLower(suffix), issuer, issuer+"/.well-known/openid-configuration", uuid.NewString(), uuid.NewString())
	request := func(ctx context.Context, method, path string, body string) (int, []byte) {
		req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(os.Getenv("ARCHESTRA_BASE_URL"), "/")+path, strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Authorization", os.Getenv("ARCHESTRA_API_KEY"))
		if body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		response, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		data, _ := io.ReadAll(response.Body)
		return response.StatusCode, data
	}
	status, body := request(t.Context(), http.MethodPost, "/api/identity-providers", payload)
	var created struct {
		ID string `json:"id"`
	}
	if status != http.StatusOK || json.Unmarshal(body, &created) != nil || created.ID == "" {
		t.Fatalf("create the agent identity provider: status %d: %s", status, body)
	}
	t.Cleanup(func() {
		// t.Context() is already canceled when cleanups run.
		if status, body := request(context.Background(), http.MethodDelete, "/api/identity-providers/"+created.ID, ""); status != http.StatusOK {
			t.Errorf("delete the agent identity provider %s: status %d: %s", created.ID, status, body)
		}
	})
	return created.ID
}
