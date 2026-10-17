package provider

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestAccAdditionalParentPermissions(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("acceptance tests are disabled")
	}
	testAccPreCheck(t)
	name := acctest.RandomWithPrefix("tf-sharing-parents")
	modelID := testAccGetFirstModelID(t)
	api, err := client.NewClientWithResponses(os.Getenv("ARCHESTRA_BASE_URL"), client.WithRequestEditorFn(func(_ context.Context, r *http.Request) error {
		r.Header.Set("Authorization", os.Getenv("ARCHESTRA_API_KEY"))
		return nil
	}))
	if err != nil {
		t.Fatal(err)
	}
	models, err := api.GetModelsWithApiKeysWithResponse(t.Context(), nil)
	if err != nil || models.JSON200 == nil {
		t.Fatal("read isolated model fixture", err)
	}
	var modelUUID string
	for _, model := range *models.JSON200 {
		if model.ModelId == modelID {
			if modelUUID != "" {
				t.Fatal("ambiguous isolated model fixture")
			}
			modelUUID = model.Id.String()
		}
	}
	if modelUUID == "" {
		t.Fatal("isolated model disappeared")
	}
	helper := ResourcePermissionPolicyResource{client: api}
	policyState := parentPermissionState("llmModel", modelUUID)
	var d diag.Diagnostics
	original, status := helper.readPolicy(t.Context(), &policyState, &d)
	if status != 200 || d.HasError() {
		t.Fatal("read model sharing baseline", d)
	}
	originalSet := parentPermissionGrants(t.Context(), &policyState, original, &d)
	if d.HasError() {
		t.Fatal(d)
	}
	t.Cleanup(func() {
		var d diag.Diagnostics
		current, status := helper.readPolicy(context.Background(), &policyState, &d)
		if status != 200 || d.HasError() {
			t.Error("read model sharing cleanup", d)
			return
		}
		observed := parentPermissionGrants(context.Background(), &policyState, current, &d)
		if !observed.Equal(originalSet) {
			for _, grant := range current.Grants {
				if grant.Subject.Type != "organization" || grant.Subject.ID != "*" || len(grant.Actions) != 1 || grant.Actions[0] != "read" {
					t.Error("Unexpected concurrent model-sharing edit; cleanup refused")
					return
				}
			}
			if helper.writePolicy(context.Background(), &policyState, current.Revision, original.Grants, &d) == nil {
				t.Error("restore model sharing", d)
			}
		}
	})
	expiry := time.Now().UTC().Add(10 * time.Minute).Format(time.RFC3339)
	config := func(grants string) string {
		return fmt.Sprintf(`locals {grants=%s}
resource "archestra_service_account" "sharing" {
 name=%q
 role="member"
 permissions=local.grants
}
resource "archestra_remote_agent" "sharing" {
 source=%q
 permissions=local.grants
}
resource "archestra_virtual_key" "sharing" {
 name=%q
 key_type="standard"
 include_future_providers=true
 expires_at=%q
 permissions=local.grants
}
resource "archestra_oauth_client" "sharing" {
 name=%q
 scopes=["llm:proxy"]
 include_future_providers=true
 service_account_id=archestra_service_account.sharing.id
 permissions=local.grants
}
resource "archestra_plugin" "manual" {
 display_name=%q
 client_type="claude-code"
 supported_platforms=["posix"]
 files={".claude-plugin/plugin.json"={content=jsonencode({name="synthetic-sharing",version="1.0.0"})}}
 permissions=local.grants
}
resource "archestra_plugin" "git" {
 display_name=%q
 client_type="claude-code"
 supported_platforms=["posix"]
 git_source={repo="archestra-ai/terraform-provider-archestra",commit_sha=%q,subdir=%q,exclude=[]}
 permissions=local.grants
}
resource "archestra_model" "sharing" {
 model_id=%q
 permissions=local.grants
}
`, grants, name+"-identity", testRemoteCard(name+"-remote"), name+"-virtual", expiry, name+"-oauth", name+"-manual", name+"-git", gitPluginFixtureSHA, gitPluginFixtureSubdir, modelID)
	}
	identities, secrets := map[string]string{}, map[string]string{}
	addresses := []string{"archestra_service_account.sharing", "archestra_remote_agent.sharing", "archestra_virtual_key.sharing", "archestra_oauth_client.sharing", "archestra_plugin.manual", "archestra_plugin.git", "archestra_model.sharing"}
	check := func(count string) resource.TestCheckFunc {
		return func(s *terraform.State) error {
			for _, address := range addresses {
				row := s.RootModule().Resources[address].Primary
				if old := identities[address]; old != "" && old != row.ID {
					return fmt.Errorf("sharing changed %s identity", address)
				}
				identities[address] = row.ID
				if row.Attributes["permissions.#"] != count {
					return fmt.Errorf("%s additional grants count is %s, want %s", address, row.Attributes["permissions.#"], count)
				}
				for _, field := range []string{"value", "client_secret"} {
					if value := row.Attributes[field]; value != "" {
						key := address + "." + field
						if old := secrets[key]; old != "" && old != value {
							return fmt.Errorf("sharing rotated %s", key)
						}
						secrets[key] = value
					}
				}
			}
			return nil
		}
	}
	granted := `[{subject_type="organization",subject_id="*",actions=["read"]}]`
	resource.Test(t, resource.TestCase{ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{
		{Config: config(granted), Check: check("1")},
		{Config: config(granted), ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
		{Config: strings.ReplaceAll(config(granted), "permissions=local.grants", ""), Check: check("1")},
		{Config: config(`[]`), Check: check("0")},
		{Config: config(`[]`), ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
	}})
}
