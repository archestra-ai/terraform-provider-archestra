package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestAccParentPermissions(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-parent-sharing")
	config := func(fields string) string {
		return fmt.Sprintf(`resource "archestra_agent" "test" {
name=%q
 %s
 }`, name, fields)
	}
	var id string
	var api *client.ClientWithResponses
	check := func(count string) resource.TestCheckFunc {
		return resource.ComposeTestCheckFunc(func(s *terraform.State) error {
			next := s.RootModule().Resources["archestra_agent.test"].Primary.ID
			if id != "" && id != next {
				return fmt.Errorf("permissions update replaced agent")
			}
			id = next
			return nil
		}, resource.TestCheckResourceAttr("archestra_agent.test", "permissions.#", count))
	}
	granted := `permissions=[{subject_type="organization",subject_id="*",actions=["read","use"]}]`
	updated := `permissions=[{subject_type="organization",subject_id="*",actions=["read"]}]`
	drift := func() {
		r, err := api.GetResourcePermissionsWithResponse(t.Context(), "agent", id)
		if err != nil || r.JSON200 == nil {
			t.Fatal("read owned permission fixture", err)
		}
		body := fmt.Sprintf(`{"revision":%d,"grants":[]}`, r.JSON200.Revision)
		changed, err := api.UpdateResourcePermissionsWithBodyWithResponse(t.Context(), "agent", id, "application/json", strings.NewReader(body))
		if err != nil || changed.StatusCode() != 200 {
			t.Fatal("inject owned sharing drift", err)
		}
	}
	resource.Test(t, resource.TestCase{PreCheck: func() {
		testAccPreCheck(t)
		var err error
		api, err = client.NewClientWithResponses(os.Getenv("ARCHESTRA_BASE_URL"), client.WithRequestEditorFn(func(_ context.Context, r *http.Request) error {
			r.Header.Set("Authorization", os.Getenv("ARCHESTRA_API_KEY"))
			return nil
		}))
		if err != nil {
			t.Fatal(err)
		}
	}, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{
		{Config: config(granted), Check: check("1")},
		{ResourceName: "archestra_agent.test", ImportState: true, ImportStateVerify: true},
		{Config: config(updated), Check: check("1"), ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction("archestra_agent.test", plancheck.ResourceActionUpdate)}}},
		{Config: config(updated), ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
		{PreConfig: drift, Config: config(updated), Check: check("1")},
		{Config: config(`description="Sharing remains unmanaged when omitted."`), Check: check("1")},
		{Config: config(`permissions=[]`), Check: check("0")},
		{Config: config(`permissions=[]`), ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
	}})
}

// Exercise organization-owned objects using an actual short-lived service-account
// credential. The ordinary human-owner case above must not hide this creator grant.
func TestAccParentPermissionsServiceAccount(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("acceptance tests are disabled")
	}
	testAccPreCheck(t)
	name := acctest.RandomWithPrefix("tf-service-sharing")
	original := os.Getenv("ARCHESTRA_API_KEY")
	admin, err := client.NewClientWithResponses(os.Getenv("ARCHESTRA_BASE_URL"), client.WithRequestEditorFn(func(_ context.Context, r *http.Request) error { r.Header.Set("Authorization", original); return nil }))
	if err != nil {
		t.Fatal(err)
	}
	body := fmt.Sprintf(`{"name":%q,"role":"admin"}`, name)
	account, err := admin.CreateServiceAccountWithBodyWithResponse(t.Context(), "application/json", strings.NewReader(body))
	if err != nil || account.JSON200 == nil {
		t.Fatal("create isolated service-account fixture", err)
	}
	var secret string
	t.Cleanup(func() {
		deleted, err := admin.DeleteServiceAccountWithResponse(context.Background(), account.JSON200.Id)
		if err != nil || (deleted.StatusCode() != 200 && deleted.StatusCode() != 404) {
			t.Error("delete service-account fixture", err)
			return
		}
		if secret != "" {
			denied, err := admin.GetServiceAccountWithResponse(context.Background(), account.JSON200.Id, func(_ context.Context, r *http.Request) error { r.Header.Set("Authorization", secret); return nil })
			if err != nil || denied.StatusCode() != 401 {
				t.Error("short-lived fixture credential was not revoked", err)
			}
		}
	})
	token, err := admin.CreateServiceAccountTokenWithBodyWithResponse(t.Context(), account.JSON200.Id, "application/json", strings.NewReader(`{"name":"Synthetic short-lived sharing fixture","expiresIn":600}`))
	if err != nil || token.JSON200 == nil {
		t.Fatal("create short-lived fixture credential", err)
	}
	var credentials struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(token.Body, &credentials); err != nil || credentials.Token == "" {
		t.Fatal("invalid fixture credential response", err)
	}
	secret = credentials.Token
	t.Setenv("ARCHESTRA_API_KEY", secret)
	config := func(fields string) string {
		return fmt.Sprintf(`resource "archestra_agent" "test" {
 name=%q
 %s
 }`, name+"-agent", fields)
	}
	var id string
	identity := func(s *terraform.State) error {
		row := s.RootModule().Resources["archestra_agent.test"].Primary
		if id != "" && row.ID != id {
			return fmt.Errorf("service-account sharing changed parent identity")
		}
		id = row.ID
		return nil
	}
	granted := `permissions=[{subject_type="role",subject_id="admin",actions=["read","use"]}]`
	resource.Test(t, resource.TestCase{PreCheck: func() { testAccPreCheck(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{
		{Config: config(granted), Check: resource.ComposeTestCheckFunc(identity, resource.TestCheckResourceAttr("archestra_agent.test", "permissions.#", "1"))},
		{Config: config(`description="Unmanaged sharing is preserved."`), Check: resource.ComposeTestCheckFunc(identity, resource.TestCheckResourceAttr("archestra_agent.test", "permissions.#", "1"))},
		{Config: config(`permissions=[]`), Check: resource.ComposeTestCheckFunc(identity, resource.TestCheckResourceAttr("archestra_agent.test", "permissions.#", "0"))},
		{Config: config(`permissions=[]`), ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
	}})
}

func TestAccEnvironmentInlinePermissions(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-environment-sharing")
	config := func(grants string) string {
		return fmt.Sprintf(`resource "archestra_environment" "test" {
 name=%q
 permissions=%s
 }`, name, grants)
	}
	var identity string
	check := func(count string) resource.TestCheckFunc {
		return resource.ComposeTestCheckFunc(func(s *terraform.State) error {
			id := s.RootModule().Resources["archestra_environment.test"].Primary.ID
			if identity != "" && identity != id {
				return fmt.Errorf("sharing changed environment identity")
			}
			identity = id
			return nil
		}, resource.TestCheckResourceAttr("archestra_environment.test", "permissions.#", count))
	}
	resource.Test(t, resource.TestCase{PreCheck: func() { testAccPreCheck(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{
		{Config: config(`[{subject_type="organization",subject_id="*",actions=["read","use"]}]`), Check: check("1")},
		{ResourceName: "archestra_environment.test", ImportState: true, ImportStateVerify: true},
		{Config: config(`[{subject_type="organization",subject_id="*",actions=["read"]}]`), Check: check("1")},
		{Config: config(`[]`), Check: check("0")},
		{Config: config(`[]`), ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
	}})
}
