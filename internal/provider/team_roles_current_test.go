package provider

import (
	"context"
	"fmt"
	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

func TestAccTeamRoleAssignments(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-role-team")
	prefix := fmt.Sprintf(`
resource "archestra_role" "team" {
 name = %q
 permission = {agent = ["read"]}
}
`, name)
	config := func(roles string) string {
		return prefix + fmt.Sprintf(`
resource "archestra_team" "test" {
 name = %q
 %s
}
`, name, roles)
	}
	var teamID string
	assigned := config("roles = [archestra_role.team.role]")
	omitted := config("")
	cleared := config("roles = []")
	resource.Test(t, resource.TestCase{PreCheck: func() { testAccPreCheck(t); testAccRequireEnterprise(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{
		{Config: assigned, Check: resource.ComposeAggregateTestCheckFunc(resource.TestCheckResourceAttrPair("archestra_team.test", "roles.0", "archestra_role.team", "role"), func(s *terraform.State) error {
			teamID = s.RootModule().Resources["archestra_team.test"].Primary.ID
			return nil
		})},
		{ResourceName: "archestra_team.test", ImportState: true, ImportStateVerify: true},
		{Config: assigned, PlanOnly: true},
		{Config: assigned, PlanOnly: true, ExpectNonEmptyPlan: true, PreConfig: func() {
			c, err := client.NewClientWithResponses(os.Getenv("ARCHESTRA_BASE_URL"), client.WithHTTPClient(&http.Client{Timeout: 15 * time.Second}), client.WithRequestEditorFn(func(_ context.Context, req *http.Request) error {
				req.Header.Set("Authorization", os.Getenv("ARCHESTRA_API_KEY"))
				return nil
			}))
			if err != nil {
				t.Fatal(err)
			}
			response, err := c.UpdateTeamWithBodyWithResponse(t.Context(), teamID, "application/json", strings.NewReader(`{"roles":[]}`))
			if err != nil || response.JSON200 == nil {
				t.Fatal("Unable to create isolated team role drift", err)
			}
		}},
		{Config: assigned, Check: resource.TestCheckResourceAttr("archestra_team.test", "roles.#", "1")},
		{Config: omitted, Check: resource.TestCheckResourceAttr("archestra_team.test", "roles.#", "1")},
		{Config: omitted, PlanOnly: true},
		{Config: cleared, Check: resource.TestCheckResourceAttr("archestra_team.test", "roles.#", "0")},
		{Config: cleared, PlanOnly: true},
	}})
}
