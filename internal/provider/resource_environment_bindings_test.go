package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAccTeamVaultFolderResource(t *testing.T) {
	config := func(path string) string {
		return fmt.Sprintf(`
 resource "archestra_team" "vault_parent" {name="tf-vault-folder-parent"}
 resource "archestra_team_vault_folder" "test" {
 team_id=archestra_team.vault_parent.id
 vault_path=%q
 }`, path)
	}
	resource.Test(t, resource.TestCase{PreCheck: func() { testAccPreCheck(t); testAccRequireByosEnabled(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{
		{Config: config("secret/data/test"), Check: resource.TestCheckResourceAttrPair("archestra_team_vault_folder.test", "id", "archestra_team.vault_parent", "id")},
		{ResourceName: "archestra_team_vault_folder.test", ImportState: true, ImportStateVerify: true},
		{Config: config("secret/data/test/other"), ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction("archestra_team_vault_folder.test", plancheck.ResourceActionUpdate)}}, Check: resource.TestCheckResourceAttr("archestra_team_vault_folder.test", "vault_path", "secret/data/test/other")},
		{Config: config("secret/data/test/other"), ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
	}})
}
func TestAccEnvironmentDefaultResource(t *testing.T) {
	config := func(selected string) string {
		return fmt.Sprintf(`
 resource "archestra_environment" "first" {name="tf-resource-default-first"}
 resource "archestra_environment" "second" {name="tf-resource-default-second"}
 resource "archestra_environment_default" "other" {
 resource_type="knowledgeSource"
 environment_id=archestra_environment.second.id
 }
 resource "archestra_environment_default" "test" {
 resource_type="app"
 environment_id=%s
 }`, selected)
	}
	resource.Test(t, resource.TestCase{PreCheck: func() { testAccPreCheck(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{
		{Config: config("archestra_environment.first.id"), Check: resource.TestCheckResourceAttrPair("archestra_environment_default.test", "environment_id", "archestra_environment.first", "id")},
		{ResourceName: "archestra_environment_default.test", ImportState: true, ImportStateVerify: true},
		{Config: config("archestra_environment.second.id"), ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction("archestra_environment_default.test", plancheck.ResourceActionUpdate)}}, Check: resource.ComposeTestCheckFunc(resource.TestCheckResourceAttrPair("archestra_environment_default.test", "environment_id", "archestra_environment.second", "id"), resource.TestCheckResourceAttrPair("archestra_environment_default.other", "environment_id", "archestra_environment.second", "id"))},
		{Config: config("null"), Check: resource.ComposeTestCheckFunc(resource.TestCheckNoResourceAttr("archestra_environment_default.test", "environment_id"), resource.TestCheckResourceAttrPair("archestra_environment_default.other", "environment_id", "archestra_environment.second", "id"))},
		{Config: config("null"), ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
	}})
}
func TestEnvironmentDefaultChangesOnlySelectedKind(t *testing.T) {
	defaults := map[string]*string{"app": nil, "agent": nil, "mcpGateway": nil, "mcpRegistry": nil, "knowledgeSource": nil}
	other := "22222222-2222-4222-8222-222222222222"
	defaults["agent"] = &other
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PATCH" || r.URL.Path != "/api/environments/defaults" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		var body map[string]*string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		if len(body) != 1 {
			t.Errorf("default update must own one slot, got %v", body)
		}
		for k, v := range body {
			if k != "app" {
				t.Errorf("changed unrelated kind %s", k)
			}
			defaults[k] = v
		}
		calls++
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(defaults); err != nil {
			t.Error(err)
		}
	}))
	defer server.Close()
	api, err := client.NewClientWithResponses(server.URL, client.WithHTTPClient(server.Client()))
	if err != nil {
		t.Fatal(err)
	}
	r := &EnvironmentDefaultResource{client: api}
	data := EnvironmentDefaultResourceModel{ResourceType: types.StringValue("app"), EnvironmentID: types.StringValue("11111111-1111-4111-8111-111111111111")}
	var diagnostics diag.Diagnostics
	if !r.set(context.Background(), &data, false, &diagnostics) {
		t.Fatal(diagnostics)
	}
	if data.ID.ValueString() != "app" || data.EnvironmentID.ValueString() != "11111111-1111-4111-8111-111111111111" {
		t.Fatal("did not persist selected default")
	}
	if !r.set(context.Background(), &data, true, &diagnostics) {
		t.Fatal(diagnostics)
	}
	if calls != 2 || defaults["app"] != nil || defaults["agent"] == nil || *defaults["agent"] != other {
		t.Fatal("clearing the managed slot changed another kind")
	}
}
