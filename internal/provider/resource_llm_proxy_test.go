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
