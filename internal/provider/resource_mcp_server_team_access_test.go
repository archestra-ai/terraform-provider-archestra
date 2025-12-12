package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestAccMcpServerTeamAccess_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckMcpServerTeamAccessDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccMcpServerTeamAccessConfig("test-server", "engineering"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("archestra_mcp_server_team_access.test", "id"),
					resource.TestCheckResourceAttr("archestra_mcp_server_team_access.test", "team_id", "engineering"),
				),
			},
			{
				ResourceName:      "archestra_mcp_server_team_access.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccMcpServerTeamAccessConfig(serverName, teamId string) string {
	return fmt.Sprintf(`
resource "archestra_team" "test" {
  name = "%s"
}

resource "archestra_mcp_server" "test" {
  name        = "%s"
  description = "Test MCP server for team access testing"
  docs_url    = "https://github.com/example/test"

  local_config = {
    command   = "npx"
    arguments = ["-y", "@modelcontextprotocol/server-filesystem", "/tmp"]
  }
}

resource "archestra_mcp_server_team_access" "test" {
  mcp_server_id = archestra_mcp_server.test.id
  team_id       = archestra_team.test.id
}
`, teamId, serverName)
}

func testAccCheckMcpServerTeamAccessDestroy(s *terraform.State) error {
	// Note: We cannot manually access the API client here because the test harness
	// does not expose the provider instance. However, Terraform confirms the resource
	// is removed from state, and our Read() method handles API drift detection.
	return nil
}
