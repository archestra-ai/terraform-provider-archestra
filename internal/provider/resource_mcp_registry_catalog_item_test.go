package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccMCPRegistryCatalogItemResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccMCPRegistryCatalogItemResourceConfig("test-item", "Test Description"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"archestra_mcp_registry_catalog_item.test",
						tfjsonpath.New("name"),
						knownvalue.StringExact("test-item"),
					),
					statecheck.ExpectKnownValue(
						"archestra_mcp_registry_catalog_item.test",
						tfjsonpath.New("description"),
						knownvalue.StringExact("Test Description"),
					),
					statecheck.ExpectKnownValue(
						"archestra_mcp_registry_catalog_item.test",
						tfjsonpath.New("docs_url"),
						knownvalue.StringExact("https://github.com/example/test-server"),
					),
				},
			},
			// ImportState testing
			{
				ResourceName:      "archestra_mcp_registry_catalog_item.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			// Update and Read testing
			{
				Config: testAccMCPRegistryCatalogItemResourceConfigUpdated("test-item-updated", "Updated Description"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"archestra_mcp_registry_catalog_item.test",
						tfjsonpath.New("name"),
						knownvalue.StringExact("test-item-updated"),
					),
					statecheck.ExpectKnownValue(
						"archestra_mcp_registry_catalog_item.test",
						tfjsonpath.New("description"),
						knownvalue.StringExact("Updated Description"),
					),
					statecheck.ExpectKnownValue(
						"archestra_mcp_registry_catalog_item.test",
						tfjsonpath.New("docs_url"),
						knownvalue.StringExact("https://github.com/example/test-server-updated"),
					),
				},
			},
			// Delete testing automatically occurs in TestCase
		},
	})
}

func testAccMCPRegistryCatalogItemResourceConfig(name, description string) string {
	return fmt.Sprintf(`
resource "archestra_mcp_registry_catalog_item" "test" {
  name        = %[1]q
  description = %[2]q
  docs_url    = "https://github.com/example/test-server"

  local_config = {
    command   = "npx"
    arguments = ["-y", "@modelcontextprotocol/server-filesystem", "/tmp"]
  }
}
`, name, description)
}

func testAccMCPRegistryCatalogItemResourceConfigUpdated(name, description string) string {
	return fmt.Sprintf(`
resource "archestra_mcp_registry_catalog_item" "test" {
  name        = %[1]q
  description = %[2]q
  docs_url    = "https://github.com/example/test-server-updated"

  local_config = {
    command   = "npx"
    arguments = ["-y", "@modelcontextprotocol/server-filesystem", "/tmp"]
  }
}
`, name, description)
}
