package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccCurrentAgentControls(t *testing.T) {
	initial := `resource "archestra_agent" "test" {
name="tf-current-agent-controls"
access_all_subagents=false
tool_exposure_mode="search_and_run_only"
}`
	updated := `resource "archestra_agent" "test" {
name="tf-current-agent-controls"
access_all_subagents=true
tool_exposure_mode="full"
}`
	resource.Test(t, resource.TestCase{PreCheck: func() { testAccPreCheck(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{
		{Config: initial, Check: resource.TestCheckResourceAttr("archestra_agent.test", "access_all_subagents", "false")},
		{ResourceName: "archestra_agent.test", ImportState: true, ImportStateVerify: true},
		{Config: initial, PlanOnly: true},
		{Config: updated, Check: resource.TestCheckResourceAttr("archestra_agent.test", "tool_exposure_mode", "full")},
		{Config: updated, PlanOnly: true},
	}})
}
func TestAccCurrentGatewayControls(t *testing.T) {
	base := `resource "archestra_environment" "test" {
name="tf-gateway-environment"
namespace="default"
}
resource "archestra_mcp_gateway" "test" {
name="tf-current-gateway-controls"
environment_id=archestra_environment.test.id
`
	initial := base + `access_all_tools=true
missing_credential_behavior="block"
}`
	updated := base + `access_all_tools=false
missing_credential_behavior="warn"
}`
	resource.Test(t, resource.TestCase{PreCheck: func() { testAccPreCheck(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{
		{Config: initial, Check: resource.TestCheckResourceAttr("archestra_mcp_gateway.test", "access_all_tools", "true")},
		{ResourceName: "archestra_mcp_gateway.test", ImportState: true, ImportStateVerify: true},
		{Config: initial, PlanOnly: true},
		{Config: updated, Check: resource.TestCheckResourceAttr("archestra_mcp_gateway.test", "missing_credential_behavior", "warn")},
		{Config: updated, PlanOnly: true},
	}})
}
