package provider

import (
	"context"
	"fmt"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestAccTeamInlineBindings(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-team-bindings")
	var id string
	config := func(fields string) string {
		return fmt.Sprintf(`resource "archestra_team" "test" {
name=%q
%s
}`, name, fields) + `
data "archestra_team" "lookup" {id=archestra_team.test.id}
`
	}
	initial := config(`external_group_ids=["engineering","support"]
vault_folder="secret/data/test/engineering"`)
	updated := config(`external_group_ids=["operations"]
vault_folder="secret/data/test/operations"`)
	api := func() *client.ClientWithResponses {
		c, err := client.NewClientWithResponses(os.Getenv("ARCHESTRA_BASE_URL"), client.WithRequestEditorFn(func(_ context.Context, r *http.Request) error {
			r.Header.Set("Authorization", os.Getenv("ARCHESTRA_API_KEY"))
			return nil
		}))
		if err != nil {
			t.Fatal(err)
		}
		return c
	}
	same := func(s *terraform.State) error {
		if s.RootModule().Resources["archestra_team.test"].Primary.ID != id {
			return fmt.Errorf("binding change replaced the team")
		}
		return nil
	}
	resource.Test(t, resource.TestCase{PreCheck: func() { testAccPreCheck(t); testAccRequireByosEnabled(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{
		{Config: initial, Check: resource.ComposeTestCheckFunc(resource.TestCheckResourceAttr("archestra_team.test", "external_group_ids.#", "2"), resource.TestCheckResourceAttrPair("data.archestra_team.lookup", "external_group_ids.#", "archestra_team.test", "external_group_ids.#"), resource.TestCheckResourceAttrPair("data.archestra_team.lookup", "vault_folder", "archestra_team.test", "vault_folder"), resource.TestCheckResourceAttr("archestra_team.test", "vault_folder", "secret/data/test/engineering"), func(s *terraform.State) error {
			id = s.RootModule().Resources["archestra_team.test"].Primary.ID
			return nil
		})},
		{ResourceName: "archestra_team.test", ImportState: true, ImportStateVerify: true},
		{Config: initial, PlanOnly: true},
		{Config: updated, Check: resource.ComposeTestCheckFunc(same, resource.TestCheckResourceAttr("archestra_team.test", "external_group_ids.#", "1"), resource.TestCheckResourceAttr("archestra_team.test", "vault_folder", "secret/data/test/operations"))},
		{Config: updated, PlanOnly: true},
		{PreConfig: func() {
			c := api()
			groups, err := c.GetTeamExternalGroupsWithResponse(t.Context(), id)
			if err != nil || groups.JSON200 == nil || len(*groups.JSON200) != 1 {
				t.Fatal("read owned SSO mapping", err)
			}
			removed, err := c.RemoveTeamExternalGroupWithResponse(t.Context(), id, (*groups.JSON200)[0].Id)
			if err != nil || removed.StatusCode() != 200 {
				t.Fatal("modify owned SSO mapping", err)
			}
			folder, err := c.SetTeamVaultFolderWithResponse(t.Context(), id, client.SetTeamVaultFolderJSONRequestBody{VaultPath: "secret/data/test/drift"})
			if err != nil || folder.JSON200 == nil {
				t.Fatal("modify owned Vault mapping", err)
			}
		}, Config: updated, PlanOnly: true, ExpectNonEmptyPlan: true},
		{Config: updated, Check: same},
		{Config: config(""), Check: resource.ComposeTestCheckFunc(same, resource.TestCheckResourceAttr("archestra_team.test", "external_group_ids.#", "1"), resource.TestCheckResourceAttr("archestra_team.test", "vault_folder", "secret/data/test/operations"))},
		{Config: config(`external_group_ids=[]
vault_folder=""`), Check: resource.ComposeTestCheckFunc(same, resource.TestCheckResourceAttr("archestra_team.test", "external_group_ids.#", "0"), resource.TestCheckResourceAttr("archestra_team.test", "vault_folder", ""))},
		{Config: config(`external_group_ids=[]
vault_folder=""`), PlanOnly: true},
	}})
}

func TestTeamBindingsRespectLicensedReadAccess(t *testing.T) {
	for _, managed := range []string{"", "groups", "vault"} {
		t.Run(managed, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if managed == "vault" && strings.HasSuffix(r.URL.Path, "/external-groups") {
					fmt.Fprint(w, `[]`)
					return
				}
				w.WriteHeader(http.StatusForbidden)
				fmt.Fprint(w, `{"error":{"message":"Not available","type":"forbidden"}}`)
			}))
			defer server.Close()
			c, err := client.NewClientWithResponses(server.URL)
			if err != nil {
				t.Fatal(err)
			}
			data := TeamResourceModel{ID: types.StringValue("team-a"), ExternalGroupIDs: types.SetNull(types.StringType), VaultFolder: types.StringNull()}
			var d diag.Diagnostics
			if managed == "groups" {
				data.ExternalGroupIDs = agentSet(t.Context(), []string{"existing"}, &d)
			}
			if managed == "vault" {
				data.VaultFolder = types.StringValue("secret/data/existing")
			}
			previousGroups, previousFolder := data.ExternalGroupIDs, data.VaultFolder
			(&TeamResource{client: c}).readBindings(t.Context(), &data, &d)
			if d.HasError() != (managed != "") {
				t.Fatalf("managed=%s diagnostics=%v", managed, d)
			}
			if managed == "groups" && !data.ExternalGroupIDs.Equal(previousGroups) {
				t.Fatal("forbidden read replaced managed groups")
			}
			if managed == "vault" && !data.VaultFolder.Equal(previousFolder) {
				t.Fatal("forbidden read replaced managed Vault path")
			}
			if managed == "" && (!data.ExternalGroupIDs.IsNull() || !data.VaultFolder.IsNull()) {
				t.Fatal("unavailable unmanaged settings were reported as empty")
			}
		})
	}
}
func TestTeamGroupAdditionFailureRetainsExtras(t *testing.T) {
	deletes := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.Method {
		case http.MethodGet:
			fmt.Fprint(w, `[{"id":"old-mapping","teamId":"team-a","groupIdentifier":"existing"}]`)
		case http.MethodPost:
			w.WriteHeader(http.StatusForbidden)
			fmt.Fprint(w, `{"error":{"message":"Denied","type":"forbidden"}}`)
		default:
			deletes++
			w.WriteHeader(500)
		}
	}))
	defer server.Close()
	c, err := client.NewClientWithResponses(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	var d diag.Diagnostics
	data := TeamResourceModel{ID: types.StringValue("team-a"), ExternalGroupIDs: agentSet(t.Context(), []string{"new"}, &d)}
	(&TeamResource{client: c}).replaceExternalGroups(t.Context(), &data, &d)
	if !d.HasError() || deletes != 0 {
		t.Fatal("failed desired assignment removed an existing mapping", d, deletes)
	}
}

func TestTeamBindingReadErrorsRetainInputs(t *testing.T) {
	for _, status := range []int{404, 500} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(status)
				fmt.Fprint(w, `{"error":{"message":"Not available","type":"error"}}`)
			}))
			defer server.Close()
			c, err := client.NewClientWithResponses(server.URL)
			if err != nil {
				t.Fatal(err)
			}
			var d diag.Diagnostics
			before := agentSet(t.Context(), []string{"existing"}, &d)
			data := TeamResourceModel{ID: types.StringValue("team-a"), ExternalGroupIDs: before, VaultFolder: types.StringValue("secret/data/team")}
			(&TeamResource{client: c}).readBindings(t.Context(), &data, &d)
			if !d.HasError() || !data.ExternalGroupIDs.Equal(before) || data.ID.ValueString() != "team-a" {
				t.Fatal("failed child read changed the parent or its mappings", d)
			}
		})
	}
}
