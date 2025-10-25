package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccMCPServerToolDataSource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Read testing
			{
				Config: testAccMCPServerToolDataSourceConfig("mcp-server-id-here", "read_file"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"data.archestra_mcp_server_tool.test",
						tfjsonpath.New("mcp_server_id"),
						knownvalue.StringExact("mcp-server-id-here"),
					),
					statecheck.ExpectKnownValue(
						"data.archestra_mcp_server_tool.test",
						tfjsonpath.New("name"),
						knownvalue.StringExact("read_file"),
					),
				},
			},
		},
	})
}

func testAccMCPServerToolDataSourceConfig(mcpServerID, toolName string) string {
	return fmt.Sprintf(`
data "archestra_mcp_server_tool" "test" {
  mcp_server_id = %[1]q
  name          = %[2]q
}
`, mcpServerID, toolName)
}
