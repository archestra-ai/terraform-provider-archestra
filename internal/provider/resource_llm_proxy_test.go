package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccLlmProxyResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccLlmProxyResourceConfig("tf-acc-llm-proxy"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("archestra_llm_proxy.test", tfjsonpath.New("name"), knownvalue.StringExact("tf-acc-llm-proxy")),
					statecheck.ExpectKnownValue("archestra_llm_proxy.test", tfjsonpath.New("scope"), knownvalue.StringExact("org")),
				},
			},
			{
				ResourceName:      "archestra_llm_proxy.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: testAccLlmProxyResourceConfigUpdated("tf-acc-llm-proxy-renamed"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("archestra_llm_proxy.test", tfjsonpath.New("name"), knownvalue.StringExact("tf-acc-llm-proxy-renamed")),
					statecheck.ExpectKnownValue("archestra_llm_proxy.test", tfjsonpath.New("description"), knownvalue.StringExact("updated")),
				},
			},
		},
	})
}

// TestAccLlmProxyResource_TeamsRemoveCycle pins RemoveOnConfigNullList on `teams`.
func TestAccLlmProxyResource_TeamsRemoveCycle(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccLlmProxyResourceConfigTeamScoped("tf-acc-llm-proxy-cycle"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("archestra_llm_proxy.cycle", tfjsonpath.New("scope"), knownvalue.StringExact("team")),
					statecheck.ExpectKnownValue("archestra_llm_proxy.cycle", tfjsonpath.New("teams"), knownvalue.ListSizeExact(1)),
				},
			},
			{
				Config: testAccLlmProxyResourceConfigOrgScoped("tf-acc-llm-proxy-cycle"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("archestra_llm_proxy.cycle", tfjsonpath.New("scope"), knownvalue.StringExact("org")),
					statecheck.ExpectKnownValue("archestra_llm_proxy.cycle", tfjsonpath.New("teams"), knownvalue.ListSizeExact(0)),
				},
			},
		},
	})
}

func testAccLlmProxyResourceConfigTeamScoped(name string) string {
	return fmt.Sprintf(`
resource "archestra_team" "cycle" {
  name        = %[1]q
  description = "remove-cycle test team"
}

resource "archestra_llm_proxy" "cycle" {
  name  = %[1]q
  scope = "team"
  teams = [archestra_team.cycle.id]
}
`, name)
}

func testAccLlmProxyResourceConfigOrgScoped(name string) string {
	return fmt.Sprintf(`
resource "archestra_team" "cycle" {
  name        = %[1]q
  description = "remove-cycle test team"
}

resource "archestra_llm_proxy" "cycle" {
  name = %[1]q
}
`, name)
}

func testAccLlmProxyResourceConfig(name string) string {
	return fmt.Sprintf(`
resource "archestra_llm_proxy" "test" {
  name = %q
}
`, name)
}

func testAccLlmProxyResourceConfigUpdated(name string) string {
	return fmt.Sprintf(`
resource "archestra_llm_proxy" "test" {
  name        = %q
  description = "updated"
}
`, name)
}
