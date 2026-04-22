package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccProfileToolResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){
			"archestra": providerserver.NewProtocol6WithError(New("test")()),
		},
		Steps: []resource.TestStep{
			// Create and Read
			{
				Config: testAccProfileToolResourceConfig("archestra-test-profile"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair("archestra_profile_tool.test", "profile_id", "archestra_profile.test", "id"),
					resource.TestCheckResourceAttrSet("archestra_profile_tool.test", "id"),
					resource.TestCheckResourceAttrSet("archestra_profile_tool.test", "credential_resolution_mode"),
				),
			},
			// Import
			{
				ResourceName:      "archestra_profile_tool.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			// Delete checks are implicit
		},
	})
}

func testAccProfileToolResourceConfig(profileName string) string {
	return fmt.Sprintf(`
resource "archestra_profile" "test" {
  name = "%[1]s"
}

resource "archestra_mcp_registry_catalog_item" "test" {
  name = "test-server"
  local_config = {
    command = "npx"
    arguments = ["-y", "@modelcontextprotocol/server-filesystem", "./"]
  }
}

resource "archestra_mcp_server_installation" "test" {
  name          = "test-server-inst"
  catalog_id = archestra_mcp_registry_catalog_item.test.id
}

data "archestra_mcp_server_tool" "test" {
  mcp_server_id = archestra_mcp_server_installation.test.id
  name          = "test-server__read_file"
  depends_on    = [archestra_mcp_server_installation.test]
}

resource "archestra_profile_tool" "test" {
  profile_id    = archestra_profile.test.id
  tool_id       = data.archestra_mcp_server_tool.test.id
  mcp_server_id = archestra_mcp_server_installation.test.id
}
`, profileName)
}
