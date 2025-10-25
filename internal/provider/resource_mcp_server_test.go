package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccMCPServerResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccMCPServerResourceConfig("test-mcp-server"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"archestra_mcp_server_installation.test",
						tfjsonpath.New("name"),
						knownvalue.StringExact("test-mcp-server"),
					),
				},
			},
			// ImportState testing
			{
				ResourceName:      "archestra_mcp_server_installation.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			// Update testing - MCP servers require replacement on update
			// Delete testing automatically occurs in TestCase
		},
	})
}

func testAccMCPServerResourceConfig(name string) string {
	return fmt.Sprintf(`
resource "archestra_mcp_server_installation" "test" {
  name = %[1]q
}
`, name)
}
