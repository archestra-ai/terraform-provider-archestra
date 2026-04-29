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

func TestAccMCPServerResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccMCPServerResourceConfig("test-mcp-server"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"archestra_mcp_registry_catalog_item.test",
						tfjsonpath.New("name"),
						knownvalue.StringExact("test-mcp-server"),
					),
					statecheck.ExpectKnownValue(
						"archestra_mcp_registry_catalog_item.test",
						tfjsonpath.New("description"),
						knownvalue.StringExact("Test MCP server for acceptance testing"),
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
				Config: testAccMCPServerResourceConfigUpdated("test-mcp-server-updated"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"archestra_mcp_registry_catalog_item.test",
						tfjsonpath.New("name"),
						knownvalue.StringExact("test-mcp-server-updated"),
					),
					statecheck.ExpectKnownValue(
						"archestra_mcp_registry_catalog_item.test",
						tfjsonpath.New("description"),
						knownvalue.StringExact("Updated test MCP server"),
					),
				},
			},
			// Delete testing automatically occurs in TestCase
		},
	})
}

func TestAccMCPServerInstallationResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccMCPServerInstallationResourceConfig("test-installation"),
				ConfigStateChecks: []statecheck.StateCheck{
					// name should match the user's configured value
					statecheck.ExpectKnownValue(
						"archestra_mcp_server_installation.test",
						tfjsonpath.New("name"),
						knownvalue.StringExact("test-installation"),
					),
					// display_name is the actual name from the API (may have suffix)
					statecheck.ExpectKnownValue(
						"archestra_mcp_server_installation.test",
						tfjsonpath.New("display_name"),
						knownvalue.NotNull(),
					),
					// tools is populated post-install. The filesystem MCP
					// server advertises read_file/write_file/etc.; we don't
					// pin specific names or counts because the upstream
					// server is free to add/remove. Indexing AtSliceIndex(0)
					// implicitly asserts the list has at least one element.
					statecheck.ExpectKnownValue(
						"archestra_mcp_server_installation.test",
						tfjsonpath.New("tools"),
						knownvalue.NotNull(),
					),
					statecheck.ExpectKnownValue(
						"archestra_mcp_server_installation.test",
						tfjsonpath.New("tools").AtSliceIndex(0).AtMapKey("id"),
						knownvalue.NotNull(),
					),
					statecheck.ExpectKnownValue(
						"archestra_mcp_server_installation.test",
						tfjsonpath.New("tools").AtSliceIndex(0).AtMapKey("name"),
						knownvalue.NotNull(),
					),
					// parameters is a JSON-encoded JSON Schema string. The
					// filesystem MCP server's tools all advertise parameters,
					// so this must be non-null and shaped like a JSON object
					// (starts with "{" and ends with "}").
					statecheck.ExpectKnownValue(
						"archestra_mcp_server_installation.test",
						tfjsonpath.New("tools").AtSliceIndex(0).AtMapKey("parameters"),
						knownvalue.NotNull(),
					),
					statecheck.ExpectKnownValue(
						"archestra_mcp_server_installation.test",
						tfjsonpath.New("tools").AtSliceIndex(0).AtMapKey("parameters"),
						knownvalue.StringRegexp(regexp.MustCompile(`^\{.*\}$`)),
					),
					// Fresh install: nobody has assigned this tool to an agent yet.
					statecheck.ExpectKnownValue(
						"archestra_mcp_server_installation.test",
						tfjsonpath.New("tools").AtSliceIndex(0).AtMapKey("assigned_agent_count"),
						knownvalue.Int64Exact(0),
					),
					statecheck.ExpectKnownValue(
						"archestra_mcp_server_installation.test",
						tfjsonpath.New("tools").AtSliceIndex(0).AtMapKey("assigned_agents"),
						knownvalue.ListSizeExact(0),
					),
					// created_at is RFC 3339; assert the date+time prefix.
					statecheck.ExpectKnownValue(
						"archestra_mcp_server_installation.test",
						tfjsonpath.New("tools").AtSliceIndex(0).AtMapKey("created_at"),
						knownvalue.StringRegexp(regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}`)),
					),
				},
			},
			// ImportState testing - skip verify on `name` (import doesn't restore
			// the user's configured name) and on `tools` (server-managed list
			// whose ordering and exact contents depend on when the read fires
			// relative to the MCP server's tool-discovery cycle).
			{
				ResourceName:            "archestra_mcp_server_installation.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"name", "tools"},
			},
			// Delete testing automatically occurs in TestCase
			// Note: Update test removed since name change triggers replacement
		},
	})
}

func testAccMCPServerResourceConfig(name string) string {
	return fmt.Sprintf(`
resource "archestra_mcp_registry_catalog_item" "test" {
  name        = %[1]q
  description = "Test MCP server for acceptance testing"
  docs_url    = "https://github.com/example/test-server"

  local_config = {
    command   = "npx"
    arguments = ["-y", "@modelcontextprotocol/server-filesystem", "/tmp"]
  }
}
`, name)
}

func testAccMCPServerResourceConfigUpdated(name string) string {
	return fmt.Sprintf(`
resource "archestra_mcp_registry_catalog_item" "test" {
  name        = %[1]q
  description = "Updated test MCP server"
  docs_url    = "https://github.com/example/test-server-updated"

  local_config = {
    command   = "npx"
    arguments = ["-y", "@modelcontextprotocol/server-filesystem", "/tmp"]
  }
}
`, name)
}

func testAccMCPServerInstallationResourceConfig(name string) string {
	return fmt.Sprintf(`
# First create an MCP server in the registry
resource "archestra_mcp_registry_catalog_item" "dependency" {
  name        = "test-dependency-server"
  description = "Dependency server for installation test"
  docs_url    = "https://github.com/example/dependency-server"

  local_config = {
    command   = "npx"
    arguments = ["-y", "@modelcontextprotocol/server-filesystem", "/tmp"]
  }
}

# Then create an installation of it
resource "archestra_mcp_server_installation" "test" {
  name          = %[1]q
  catalog_id = archestra_mcp_registry_catalog_item.dependency.id
}
`, name)
}
