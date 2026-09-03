package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccAgentToolExclusionsResource(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-excl")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t); testAccRequireAgentRuntimeEnabled(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccAgentToolExclusionsConfig(rName, true),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("archestra_agent_tool_exclusions.test", tfjsonpath.New("excluded_tool_ids"), knownvalue.SetSizeExact(1)),
				},
			},
			{
				ResourceName:      "archestra_agent_tool_exclusions.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			// Full replace down to the empty set — the resource owns the
			// whole list, so this clears every exclusion without destroy.
			{
				Config: testAccAgentToolExclusionsConfig(rName, false),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("archestra_agent_tool_exclusions.test", tfjsonpath.New("excluded_tool_ids"), knownvalue.SetSizeExact(0)),
				},
			},
		},
	})
}

func testAccAgentToolExclusionsConfig(name string, excludeTool bool) string {
	excluded := "[]"
	if excludeTool {
		excluded = fmt.Sprintf(`[archestra_mcp_server_installation.test.tool_id_by_name["%s-server__read_text_file"]]`, name)
	}
	return fmt.Sprintf(`
resource "archestra_agent" "test" {
  name             = %q
  system_prompt    = "You run delegated repository work."
  access_all_tools = true
}

resource "archestra_mcp_registry_catalog_item" "test" {
  name        = "%s-server"
  description = "Test MCP server"
  docs_url    = "https://github.com/example/test"

  local_config = {
    command   = "npx"
    arguments = ["-y", "@modelcontextprotocol/server-filesystem", "/tmp"]
  }
}

resource "archestra_mcp_server_installation" "test" {
  name       = "%s-install"
  catalog_id = archestra_mcp_registry_catalog_item.test.id
}

resource "archestra_agent_tool_exclusions" "test" {
  agent_id          = archestra_agent.test.id
  excluded_tool_ids = %s
}
`, name, name, name, excluded)
}
