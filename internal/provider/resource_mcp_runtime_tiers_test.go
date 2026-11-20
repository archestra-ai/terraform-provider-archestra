package provider

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

// The tiers are organization-wide; the test keeps the default names, so other
// tests on the same stack see S, M and L throughout. Destroy leaves XL behind:
// the deleted registry entry still names it.
func TestAccMcpRuntimeTiersResource(t *testing.T) {
	if os.Getenv("TF_ACC") != "" && !testAccAPIVersionAtLeast(t, "1.5.0-rc.41") {
		t.Skip("MCP runtime tiers need API 1.5.0-rc.41 or later")
	}
	const name = "archestra_mcp_runtime_tiers.test"
	config := func(xl string) string {
		return fmt.Sprintf(`resource "archestra_mcp_runtime_tiers" "test" {
  tiers = [
    { name = "S", cpu_request = 0.1 },
    { name = "M", cpu_request = 0.5, memory_request_mib = 512, memory_limit_mib = 1024 },
    { name = "L", cpu_request = 1, memory_request_mib = 1024, memory_limit_mib = 2048 },
    %s
  ]
}

resource "archestra_mcp_registry_entry" "tiered" {
  name = "tf-acc-tiered-server"
  local_config = {
    command      = "npx"
    arguments    = ["-y", "@modelcontextprotocol/server-filesystem", "/tmp"]
    runtime_tier = lower(archestra_mcp_runtime_tiers.test.tiers[3].name)
  }
}`, xl)
	}
	resource.Test(t, resource.TestCase{PreCheck: func() { testAccPreCheck(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{
		{Config: config(`{ name = "XL", cpu_request = 2, cpu_limit = 4 }`), Check: resource.ComposeTestCheckFunc(
			resource.TestCheckResourceAttr(name, "tiers.#", "4"),
			resource.TestCheckResourceAttr(name, "tiers.0.name", "S"),
			resource.TestCheckResourceAttr(name, "tiers.3.name", "XL"),
			resource.TestCheckNoResourceAttr(name, "tiers.3.memory_request_mib"),
			resource.TestCheckResourceAttrSet(name, "tiers.3.effective_memory_request_mib"),
			resource.TestCheckResourceAttr(name, "tiers.1.effective_memory_limit_mib", "1024"),
			resource.TestCheckResourceAttr("archestra_mcp_registry_entry.tiered", "local_config.runtime_tier", "xl"),
		)},
		{Config: config(`{ name = "XL", cpu_request = 2, cpu_limit = 4 }`), ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
		{ResourceName: name, ImportState: true, ImportStateVerify: true},
		// Resizing a tier in use updates it in place.
		{Config: config(`{ name = "XL", cpu_request = 2, memory_limit_mib = 8192 }`), ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction(name, plancheck.ResourceActionUpdate)}}, Check: resource.ComposeTestCheckFunc(
			resource.TestCheckNoResourceAttr(name, "tiers.3.cpu_limit"),
			resource.TestCheckResourceAttr(name, "tiers.3.memory_limit_mib", "8192"),
			resource.TestCheckResourceAttr(name, "tiers.3.effective_memory_limit_mib", "8192"),
		)},
		{Config: config(`{ name = "XL", cpu_request = 2, memory_limit_mib = 8192 }`), ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
	}})
}
