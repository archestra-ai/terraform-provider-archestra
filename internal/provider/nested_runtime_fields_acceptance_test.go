package provider

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

func TestAccAgentNetworkPolicy(t *testing.T) {
	if os.Getenv("TF_ACC") != "" && !testAccAPIVersionAtLeast(t, "1.5.0-rc.20") {
		t.Skip("Agent network policies require API 1.5.0-rc.20")
	}
	name := acctest.RandomWithPrefix("tf-egress")
	config := func(policy string) string {
		return fmt.Sprintf(`resource "archestra_agent" "test" {
  name          = %q
  system_prompt = "You are a helpful agent."
  %s
}`, name, policy)
	}
	partial := config(`network_policy = { allowed_domains = ["api.example.com"] }`)
	off := config(`network_policy = { egress_mode = "off" }`)
	inherit := config("")
	address := "archestra_agent.test"
	noChanges := resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}
	resource.Test(t, resource.TestCase{PreCheck: func() { testAccPreCheck(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{
		{Config: partial, Check: resource.ComposeAggregateTestCheckFunc(
			resource.TestCheckResourceAttr(address, "network_policy.egress_mode", "restricted"),
			resource.TestCheckResourceAttr(address, "network_policy.domain_preset", "none"),
			resource.TestCheckResourceAttr(address, "network_policy.allowed_domains.0", "api.example.com"),
			resource.TestCheckResourceAttr(address, "network_policy.allowed_cidrs.#", "0"),
		)},
		// The API's normalization reads back as the defaults filled the plan.
		{Config: partial, ConfigPlanChecks: noChanges},
		{ResourceName: address, ImportState: true, ImportStateVerify: true},
		{Config: off, Check: resource.TestCheckResourceAttr(address, "network_policy.egress_mode", "off")},
		{Config: off, ConfigPlanChecks: noChanges},
		{Config: inherit, Check: resource.TestCheckNoResourceAttr(address, "network_policy.egress_mode")},
		{Config: inherit, ConfigPlanChecks: noChanges},
	}})
}

func TestAccGatewaySubagentLoadBalancing(t *testing.T) {
	if os.Getenv("TF_ACC") != "" && !testAccAPIVersionAtLeast(t, "1.5.0-rc.23") {
		t.Skip("Subagent load balancing requires API 1.5.0-rc.23")
	}
	name := acctest.RandomWithPrefix("tf-balance")
	config := func(setting string) string {
		return fmt.Sprintf(`resource "archestra_mcp_gateway" "test" {
  name = %q
  %s
}`, name, setting)
	}
	partial := config(`subagent_load_balancing = { enabled = true }`)
	weighted := config(`subagent_load_balancing = {
    enabled   = true
    algorithm = "weighted"
    weights   = { "11111111-1111-4111-8111-111111111111" = 5 }
    retries   = 0
  }`)
	cleared := config("")
	address := "archestra_mcp_gateway.test"
	noChanges := resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}
	resource.Test(t, resource.TestCase{PreCheck: func() { testAccPreCheck(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{
		{Config: partial, Check: resource.ComposeAggregateTestCheckFunc(
			resource.TestCheckResourceAttr(address, "subagent_load_balancing.enabled", "true"),
			resource.TestCheckResourceAttr(address, "subagent_load_balancing.algorithm", "round_robin"),
			resource.TestCheckResourceAttr(address, "subagent_load_balancing.retries", "1"),
			resource.TestCheckResourceAttr(address, "subagent_load_balancing.fallback_enabled", "true"),
		)},
		{Config: partial, ConfigPlanChecks: noChanges},
		{ResourceName: address, ImportState: true, ImportStateVerify: true},
		{Config: weighted, Check: resource.TestCheckResourceAttr(address, "subagent_load_balancing.weights.11111111-1111-4111-8111-111111111111", "5")},
		{Config: weighted, ConfigPlanChecks: noChanges},
		{Config: cleared, Check: resource.TestCheckNoResourceAttr(address, "subagent_load_balancing.enabled")},
		{Config: cleared, ConfigPlanChecks: noChanges},
	}})
}

// TestAccRegistryEntryDesktopConfig needs a stack with ARCHESTRA_BETA=true,
// which desktop MCP servers require; set ARCHESTRA_TEST_DESKTOP_MCP=true.
func TestAccRegistryEntryDesktopConfig(t *testing.T) {
	if os.Getenv("TF_ACC") != "" {
		if !testAccAPIVersionAtLeast(t, "1.5.0-rc.32") {
			t.Skip("Desktop MCP registry entries require API 1.5.0-rc.32")
		}
		if os.Getenv("ARCHESTRA_TEST_DESKTOP_MCP") != "true" {
			t.Skip("set ARCHESTRA_TEST_DESKTOP_MCP=true on a stack with ARCHESTRA_BETA=true")
		}
	}
	name := acctest.RandomWithPrefix("tf-desktop")
	config := func(desktop string) string {
		return fmt.Sprintf(`resource "archestra_mcp_registry_entry" "test" {
  name           = %q
  desktop_config = %s
}`, name, desktop)
	}
	command := config(`{ command = "npx -y @modelcontextprotocol/server-everything" }`)
	url := config(`{ url = "http://localhost:3333/mcp" }`)
	address := "archestra_mcp_registry_entry.test"
	resource.Test(t, resource.TestCase{PreCheck: func() { testAccPreCheck(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{
		{Config: command, Check: resource.TestCheckResourceAttr(address, "desktop_config.command", "npx -y @modelcontextprotocol/server-everything")},
		{Config: command, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
		{Config: url, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction(address, plancheck.ResourceActionUpdate)}},
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr(address, "desktop_config.url", "http://localhost:3333/mcp"),
				resource.TestCheckNoResourceAttr(address, "desktop_config.command"),
			)},
	}})
}
