package provider

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
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
					// secret_id must settle to a known value after Create —
					// without an explicit assignment in Create, the planned
					// Unknown leaks into state and Plugin Framework rejects
					// with "provider still indicated an unknown value".
					// Bug 7's UseStateForUnknown only fires on Update, so
					// Create needs to mirror Read's null-or-value handling.
					statecheck.ExpectKnownValue(
						"archestra_mcp_server_installation.test",
						tfjsonpath.New("secret_id"),
						knownvalue.Null(),
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
					// tool_id_by_name carries the same data indexed for
					// the lookup-by-name flow. Asserting it's non-null is
					// enough — the per-key lookup is exercised by examples
					// and the bulk-resource acceptance tests via
					// `archestra_mcp_server_installation.<n>.tool_id_by_name[...]`.
					statecheck.ExpectKnownValue(
						"archestra_mcp_server_installation.test",
						tfjsonpath.New("tool_id_by_name"),
						knownvalue.NotNull(),
					),
				},
			},
			// Re-apply with identical config — pins the `tools` list-ordering
			// stability invariant. Without the sort in projectMcpServerTools,
			// the backend's non-deterministic order surfaces here as a
			// spurious positional-list diff.
			{
				Config: testAccMCPServerInstallationResourceConfig("test-installation"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
			// ImportState testing — composite `<uuid>:<name>` round-trips
			// fully (Bug 11 fix). Backend's `name` column stores the
			// constructed `<baseName>-<ownerId|teamId>` for local installs,
			// so the user-configured base name can't be recovered from the
			// API response — composite carries it through import.
			{
				ResourceName:      "archestra_mcp_server_installation.test",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					rs, ok := s.RootModule().Resources["archestra_mcp_server_installation.test"]
					if !ok {
						return "", fmt.Errorf("resource not found in state")
					}
					return rs.Primary.Attributes["id"] + ":" + rs.Primary.Attributes["name"], nil
				},
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

// TestAccMCPServerInstallationResource_UserConfigValuesIdempotent pins the
// secret_id-roundtrip invariant. When user_config_values is set without an
// explicit secret_id, the backend auto-creates a secret and returns its
// UUID; previously the schema declared secret_id as Optional-only (not
// Computed), so the next plan diffed config-null vs state-non-null and
// triggered a spurious destroy+recreate. Optional+Computed+UseStateForUnknown
// preserves the backend-assigned UUID across plans.
func TestAccMCPServerInstallationResource_UserConfigValuesIdempotent(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccMCPServerInstallationUserConfigConfig("tf-acc-msi-uc"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"archestra_mcp_server_installation.test",
						tfjsonpath.New("secret_id"),
						knownvalue.NotNull(),
					),
				},
			},
			// Re-apply identical config — must be a no-op. Without the
			// Computed flag on secret_id, this would diff `secret_id =
			// "<uuid>" -> null # forces replacement`.
			{
				Config: testAccMCPServerInstallationUserConfigConfig("tf-acc-msi-uc"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccMCPServerInstallationUserConfigConfig(name string) string {
	return fmt.Sprintf(`
resource "archestra_mcp_registry_catalog_item" "test" {
  name        = "%[1]s-cat"
  description = "user_config catalog for secret_id idempotency test"

  local_config = {
    command   = "npx"
    arguments = ["-y", "@modelcontextprotocol/server-filesystem"]
  }

  user_config = {
    workspace = {
      title       = "Workspace path"
      description = "Absolute path the server is allowed to read."
      type        = "string"
      required    = true
    }
  }
}

resource "archestra_mcp_server_installation" "test" {
  name       = "%[1]s-install"
  catalog_id = archestra_mcp_registry_catalog_item.test.id

  user_config_values = {
    workspace = jsonencode("/tmp")
  }
}
`, name)
}
