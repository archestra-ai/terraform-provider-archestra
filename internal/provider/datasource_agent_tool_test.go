package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccAgentToolDataSource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Read testing
			{
				Config: testAccAgentToolDataSourceConfig("agent-id-here", "write_file"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"data.archestra_agent_tool.test",
						tfjsonpath.New("agent_id"),
						knownvalue.StringExact("agent-id-here"),
					),
					statecheck.ExpectKnownValue(
						"data.archestra_agent_tool.test",
						tfjsonpath.New("tool_name"),
						knownvalue.StringExact("write_file"),
					),
				},
			},
		},
	})
}

func testAccAgentToolDataSourceConfig(agentID, toolName string) string {
	return fmt.Sprintf(`
data "archestra_agent_tool" "test" {
  agent_id  = %[1]q
  tool_name = %[2]q
}
`, agentID, toolName)
}
