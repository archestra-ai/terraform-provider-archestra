package provider

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccMcpGatewayResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccMcpGatewayResourceConfig("tf-acc-mcp-gw"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("archestra_mcp_gateway.test", tfjsonpath.New("name"), knownvalue.StringExact("tf-acc-mcp-gw")),
				},
			},
			{
				ResourceName:      "archestra_mcp_gateway.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: testAccMcpGatewayResourceConfigUpdated("tf-acc-mcp-gw-renamed"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("archestra_mcp_gateway.test", tfjsonpath.New("name"), knownvalue.StringExact("tf-acc-mcp-gw-renamed")),
					statecheck.ExpectKnownValue("archestra_mcp_gateway.test", tfjsonpath.New("description"), knownvalue.StringExact("updated")),
				},
			},
		},
	})
}

// Retired sharing inputs must fail before any remote object is created.
func TestAccMcpGatewayResource_RejectsRetiredSharing(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck: func() { testAccPreCheck(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{{Config: testAccMcpGatewayResourceConfigTeamScoped("tf-retired-sharing"), ExpectError: regexp.MustCompile("Unsupported argument")}},
	})
}

func testAccMcpGatewayResourceConfigTeamScoped(name string) string {
	return fmt.Sprintf(`
resource "archestra_team" "cycle" {
  name        = %[1]q
  description = "remove-cycle test team"
}

resource "archestra_mcp_gateway" "cycle" {
  name  = %[1]q

  teams = [archestra_team.cycle.id]
}
`, name)
}

func testAccMcpGatewayResourceConfig(name string) string {
	return fmt.Sprintf(`
resource "archestra_mcp_gateway" "test" {
  name = %q
  labels = [
    { key = "tier", value = "shared" }
  ]
}
`, name)
}

func testAccMcpGatewayResourceConfigUpdated(name string) string {
	return fmt.Sprintf(`
resource "archestra_mcp_gateway" "test" {
  name        = %q
  description = "updated"
  labels = [
    { key = "tier", value = "shared" }
  ]
}
`, name)
}
