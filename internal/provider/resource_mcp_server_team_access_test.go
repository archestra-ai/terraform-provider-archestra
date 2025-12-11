package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccMcpServerTeamAccess_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				// Config: Create Team, Server, and Link them
				Config: `
					resource "archestra_team" "test" {
						name = "tf-acc-test-team"
					}
					resource "archestra_mcp_server" "test" {
						name = "tf-acc-test-server"
						url  = "http://localhost:8080"
					}
					resource "archestra_mcp_server_team_access" "test" {
						mcp_server_id = archestra_mcp_server.test.id
						team_id       = archestra_team.test.id
					}
				`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("archestra_mcp_server_team_access.test", "mcp_server_id"),
					resource.TestCheckResourceAttrSet("archestra_mcp_server_team_access.test", "team_id"),
				),
			},
		},
	})
}