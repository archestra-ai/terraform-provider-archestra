package provider

import (
	"fmt"
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
					statecheck.ExpectKnownValue("archestra_mcp_gateway.test", tfjsonpath.New("scope"), knownvalue.StringExact("org")),
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

// TestAccMcpGatewayResource_TeamsRemoveCycle pins RemoveOnConfigNullList on `teams`.
func TestAccMcpGatewayResource_TeamsRemoveCycle(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccMcpGatewayResourceConfigTeamScoped("tf-acc-mcp-gw-cycle"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("archestra_mcp_gateway.cycle", tfjsonpath.New("scope"), knownvalue.StringExact("team")),
					statecheck.ExpectKnownValue("archestra_mcp_gateway.cycle", tfjsonpath.New("teams"), knownvalue.ListSizeExact(1)),
				},
			},
			{
				Config: testAccMcpGatewayResourceConfigOrgScoped("tf-acc-mcp-gw-cycle"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("archestra_mcp_gateway.cycle", tfjsonpath.New("scope"), knownvalue.StringExact("org")),
					statecheck.ExpectKnownValue("archestra_mcp_gateway.cycle", tfjsonpath.New("teams"), knownvalue.ListSizeExact(0)),
				},
			},
		},
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
  scope = "team"
  teams = [archestra_team.cycle.id]
}
`, name)
}

func testAccMcpGatewayResourceConfigOrgScoped(name string) string {
	return fmt.Sprintf(`
resource "archestra_team" "cycle" {
  name        = %[1]q
  description = "remove-cycle test team"
}

resource "archestra_mcp_gateway" "cycle" {
  name = %[1]q
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
