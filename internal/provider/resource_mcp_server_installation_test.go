package provider

import (
	"fmt"
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
				},
			},
			// ImportState testing - skip verify since import doesn't restore the user's name
			{
				ResourceName:            "archestra_mcp_server_installation.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"name"},
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
  mcp_server_id = archestra_mcp_registry_catalog_item.dependency.id
}
`, name)
}

func TestAccMCPServerResourceRemote(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccMCPServerResourceConfigRemote("test-remote-mcp-server"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"archestra_mcp_registry_catalog_item.test_remote",
						tfjsonpath.New("name"),
						knownvalue.StringExact("test-remote-mcp-server"),
					),
				},
			},
		},
	})
}

func TestAccMCPServerResourceRemoteWithOAuth(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccMCPServerResourceConfigRemoteWithOAuth("test-remote-oauth-server"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"archestra_mcp_registry_catalog_item.test_remote_oauth",
						tfjsonpath.New("name"),
						knownvalue.StringExact("test-remote-oauth-server"),
					),
				},
			},
		},
	})
}

func testAccMCPServerResourceConfigRemote(name string) string {
	return fmt.Sprintf(`
resource "archestra_mcp_registry_catalog_item" "test_remote" {
  name        = %[1]q
  description = "Test remote MCP server"
  docs_url    = "https://github.com/github/github-mcp-server"

  remote_config = {
    url = "https://api.githubcopilot.com/mcp/"
  }

  auth_fields = [
    {
      name        = "GITHUB_TOKEN"
      label       = "GitHub Token"
      type        = "password"
      required    = true
      description = "GitHub Personal Access Token"
    }
  ]
}
`, name)
}

func testAccMCPServerResourceConfigRemoteWithOAuth(name string) string {
	return fmt.Sprintf(`
resource "archestra_mcp_registry_catalog_item" "test_remote_oauth" {
  name        = %[1]q
  description = "Test remote MCP server with OAuth"
  docs_url    = "https://github.com/example/mcp-server"

  remote_config = {
    url = "https://api.example.com/mcp/"
    oauth_config = {
      client_id                  = "my-client-id"
      redirect_uris              = ["https://frontend.archestra.dev/oauth-callback"]
      scopes                     = ["read", "write"]
      supports_resource_metadata = true
    }
  }
}
`, name)
}
