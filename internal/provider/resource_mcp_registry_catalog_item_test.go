package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccMcpRegistryCatalogItemResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccMcpRegistryCatalogItemResourceConfig("test-item", "Test Description"),
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
				Config: testAccMcpRegistryCatalogItemResourceConfig("test-item-updated", "Updated Description"),
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
				},
			},
			// Delete testing automatically occurs in TestCase
		},
	})
}

func testAccMcpRegistryCatalogItemResourceConfig(name, description string) string {
	return fmt.Sprintf(`
resource "archestra_mcp_registry_catalog_item" "test" {
  name        = %[1]q
  description = %[2]q

  local_config = {
    command   = "npx"
    arguments = ["-y", "@modelcontextprotocol/server-filesystem", "/tmp"]
  }
}
`, name, description)
}

func TestAccMcpRegistryCatalogItemResourceWithMetadata(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccMcpRegistryCatalogItemResourceConfigWithMetadata("test-metadata-item"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"archestra_mcp_registry_catalog_item.test_metadata",
						tfjsonpath.New("name"),
						knownvalue.StringExact("test-metadata-item"),
					),
					statecheck.ExpectKnownValue(
						"archestra_mcp_registry_catalog_item.test_metadata",
						tfjsonpath.New("version"),
						knownvalue.StringExact("1.0.0"),
					),
					statecheck.ExpectKnownValue(
						"archestra_mcp_registry_catalog_item.test_metadata",
						tfjsonpath.New("repository"),
						knownvalue.StringExact("https://github.com/example/mcp-server"),
					),
					statecheck.ExpectKnownValue(
						"archestra_mcp_registry_catalog_item.test_metadata",
						tfjsonpath.New("instructions"),
						knownvalue.StringExact("Run npm install"),
					),
					statecheck.ExpectKnownValue(
						"archestra_mcp_registry_catalog_item.test_metadata",
						tfjsonpath.New("icon"),
						knownvalue.StringExact("\U0001f527"),
					),
				},
			},
			// ImportState testing
			{
				ResourceName:      "archestra_mcp_registry_catalog_item.test_metadata",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccMcpRegistryCatalogItemResourceConfigWithMetadata(name string) string {
	return fmt.Sprintf(`
resource "archestra_mcp_registry_catalog_item" "test_metadata" {
  name         = %[1]q
  description  = "Test MCP server with metadata fields"
  version      = "1.0.0"
  repository   = "https://github.com/example/mcp-server"
  instructions = "Run npm install"
  icon         = "\U0001f527"

  local_config = {
    command   = "npx"
    arguments = ["-y", "@modelcontextprotocol/server-filesystem", "/tmp"]
  }
}
`, name)
}

func TestAccMcpRegistryCatalogItemResourceRemote(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccMcpRegistryCatalogItemResourceConfigRemote("test-remote-mcp-server"),
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

func TestAccMcpRegistryCatalogItemResourceRemoteWithOAuth(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccMcpRegistryCatalogItemResourceConfigRemoteWithOAuth("test-remote-oauth-server"),
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

func testAccMcpRegistryCatalogItemResourceConfigRemote(name string) string {
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

func testAccMcpRegistryCatalogItemResourceConfigRemoteWithOAuth(name string) string {
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

func TestAccMcpRegistryCatalogItemResourceDockerImageWithoutCommand(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create with docker_image only (no command)
			{
				Config: testAccMcpRegistryCatalogItemResourceConfigDockerImage("test-docker-item"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"archestra_mcp_registry_catalog_item.test_docker",
						tfjsonpath.New("name"),
						knownvalue.StringExact("test-docker-item"),
					),
					statecheck.ExpectKnownValue(
						"archestra_mcp_registry_catalog_item.test_docker",
						tfjsonpath.New("local_config").AtMapKey("docker_image"),
						knownvalue.StringExact("mcp/grafana"),
					),
				},
			},
			// ImportState testing
			{
				ResourceName:      "archestra_mcp_registry_catalog_item.test_docker",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccMcpRegistryCatalogItemResourceConfigDockerImage(name string) string {
	return fmt.Sprintf(`
resource "archestra_mcp_registry_catalog_item" "test_docker" {
  name        = %[1]q
  description = "Test MCP server with docker image only"

  local_config = {
    docker_image = "mcp/grafana"
    arguments    = ["-t", "stdio"]
  }
}
`, name)
}

func TestAccMcpRegistryCatalogItemResourceWithEnvironmentVariables(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create with environment variables
			{
				Config: testAccMcpRegistryCatalogItemResourceConfigWithEnv("test-env-item"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"archestra_mcp_registry_catalog_item.test_env",
						tfjsonpath.New("name"),
						knownvalue.StringExact("test-env-item"),
					),
					statecheck.ExpectKnownValue(
						"archestra_mcp_registry_catalog_item.test_env",
						tfjsonpath.New("local_config").AtMapKey("environment"),
						knownvalue.MapExact(map[string]knownvalue.Check{
							"API_URL":   knownvalue.StringExact("{{API_URL}}"),
							"API_TOKEN": knownvalue.StringExact("{{API_TOKEN}}"),
						}),
					),
				},
			},
			// ImportState testing
			{
				ResourceName:      "archestra_mcp_registry_catalog_item.test_env",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccMcpRegistryCatalogItemResourceConfigWithEnv(name string) string {
	return fmt.Sprintf(`
resource "archestra_mcp_registry_catalog_item" "test_env" {
  name        = %[1]q
  description = "Test MCP server with environment variables"

  local_config = {
    command   = "npx"
    arguments = ["-y", "@example/mcp-server"]
    environment = {
      API_URL   = "{{API_URL}}"
      API_TOKEN = "{{API_TOKEN}}"
    }
  }

  auth_fields = [
    {
      name        = "API_URL"
      label       = "API URL"
      type        = "text"
      required    = true
      description = "The API URL"
    },
    {
      name        = "API_TOKEN"
      label       = "API Token"
      type        = "password"
      required    = true
      description = "The API authentication token"
    }
  ]
}
`, name)
}

func TestAccMcpRegistryCatalogItemResourceDockerImageWithEnv(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create with docker_image and environment variables (no command)
			{
				Config: testAccMcpRegistryCatalogItemResourceConfigDockerImageWithEnv("test-docker-env-item"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"archestra_mcp_registry_catalog_item.test_docker_env",
						tfjsonpath.New("name"),
						knownvalue.StringExact("test-docker-env-item"),
					),
					statecheck.ExpectKnownValue(
						"archestra_mcp_registry_catalog_item.test_docker_env",
						tfjsonpath.New("local_config").AtMapKey("docker_image"),
						knownvalue.StringExact("mcp/grafana"),
					),
					statecheck.ExpectKnownValue(
						"archestra_mcp_registry_catalog_item.test_docker_env",
						tfjsonpath.New("local_config").AtMapKey("environment"),
						knownvalue.MapExact(map[string]knownvalue.Check{
							"GRAFANA_URL":                   knownvalue.StringExact("{{GRAFANA_URL}}"),
							"GRAFANA_SERVICE_ACCOUNT_TOKEN": knownvalue.StringExact("{{GRAFANA_SERVICE_ACCOUNT_TOKEN}}"),
						}),
					),
				},
			},
			// ImportState testing
			{
				ResourceName:      "archestra_mcp_registry_catalog_item.test_docker_env",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccMcpRegistryCatalogItemResourceConfigDockerImageWithEnv(name string) string {
	return fmt.Sprintf(`
resource "archestra_mcp_registry_catalog_item" "test_docker_env" {
  name        = %[1]q
  description = "Test MCP server with docker image and environment variables"

  local_config = {
    docker_image = "mcp/grafana"
    arguments    = ["-t", "stdio"]
    environment = {
      GRAFANA_URL                   = "{{GRAFANA_URL}}"
      GRAFANA_SERVICE_ACCOUNT_TOKEN = "{{GRAFANA_SERVICE_ACCOUNT_TOKEN}}"
    }
  }

  auth_fields = [
    {
      name        = "GRAFANA_URL"
      label       = "Grafana URL"
      type        = "text"
      required    = true
      description = "The URL of your Grafana instance"
    },
    {
      name        = "GRAFANA_SERVICE_ACCOUNT_TOKEN"
      label       = "Grafana Service Account Token"
      type        = "password"
      required    = true
      description = "Service account token for authenticating with Grafana"
    }
  ]
}
`, name)
}

func TestAccMcpRegistryCatalogItemResourceWithLabelsAndEnvFrom(t *testing.T) {
	rName := acctest.RandStringFromCharSet(8, acctest.CharSetAlphaNum)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccMcpRegistryCatalogItemResourceConfigWithLabelsAndEnvFrom(rName),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"archestra_mcp_registry_catalog_item.test_labels_envfrom",
						tfjsonpath.New("name"),
						knownvalue.StringExact(fmt.Sprintf("labels-envfrom-test-%s", rName)),
					),
					statecheck.ExpectKnownValue(
						"archestra_mcp_registry_catalog_item.test_labels_envfrom",
						tfjsonpath.New("labels").AtSliceIndex(0).AtMapKey("key"),
						knownvalue.StringExact("env"),
					),
					statecheck.ExpectKnownValue(
						"archestra_mcp_registry_catalog_item.test_labels_envfrom",
						tfjsonpath.New("labels").AtSliceIndex(0).AtMapKey("value"),
						knownvalue.StringExact("test"),
					),
					statecheck.ExpectKnownValue(
						"archestra_mcp_registry_catalog_item.test_labels_envfrom",
						tfjsonpath.New("local_config").AtMapKey("env_from"),
						knownvalue.ListExact([]knownvalue.Check{
							knownvalue.ObjectExact(map[string]knownvalue.Check{
								"type":   knownvalue.StringExact("configMap"),
								"name":   knownvalue.StringExact("test-config"),
								"prefix": knownvalue.Null(),
							}),
						}),
					),
				},
			},
			// ImportState testing
			{
				ResourceName:      "archestra_mcp_registry_catalog_item.test_labels_envfrom",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccMcpRegistryCatalogItemResourceConfigWithLabelsAndEnvFrom(rName string) string {
	return fmt.Sprintf(`
resource "archestra_mcp_registry_catalog_item" "test_labels_envfrom" {
  name        = "labels-envfrom-test-%[1]s"
  description = "Test MCP server with labels and env_from"

  labels = [{
    key   = "env"
    value = "test"
  }]

  local_config = {
    docker_image = "alpine:latest"
    env_from = [{
      type = "configMap"
      name = "test-config"
    }]
  }
}
`, rName)
}

func TestAccMcpRegistryCatalogItemResourceRemoteWithPAT(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create remote server with PAT authentication
			{
				Config: testAccMcpRegistryCatalogItemResourceConfigRemoteWithPAT("test-remote-pat-server"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"archestra_mcp_registry_catalog_item.test_remote_pat",
						tfjsonpath.New("name"),
						knownvalue.StringExact("test-remote-pat-server"),
					),
					statecheck.ExpectKnownValue(
						"archestra_mcp_registry_catalog_item.test_remote_pat",
						tfjsonpath.New("remote_config").AtMapKey("url"),
						knownvalue.StringExact("https://api.githubcopilot.com/mcp/"),
					),
					statecheck.ExpectKnownValue(
						"archestra_mcp_registry_catalog_item.test_remote_pat",
						tfjsonpath.New("auth_fields"),
						knownvalue.ListExact([]knownvalue.Check{
							knownvalue.ObjectExact(map[string]knownvalue.Check{
								"name":        knownvalue.StringExact("GITHUB_TOKEN"),
								"label":       knownvalue.StringExact("GitHub Personal Access Token"),
								"type":        knownvalue.StringExact("password"),
								"required":    knownvalue.Bool(true),
								"description": knownvalue.StringExact("A GitHub PAT with appropriate permissions for the MCP server"),
							}),
						}),
					),
				},
			},
			// ImportState testing
			{
				ResourceName:      "archestra_mcp_registry_catalog_item.test_remote_pat",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccMcpRegistryCatalogItemResourceConfigRemoteWithPAT(name string) string {
	return fmt.Sprintf(`
resource "archestra_mcp_registry_catalog_item" "test_remote_pat" {
  name        = %[1]q
  description = "Test remote MCP server with Personal Access Token authentication"
  docs_url    = "https://github.com/github/github-mcp-server"

  remote_config = {
    url = "https://api.githubcopilot.com/mcp/"
  }

  auth_fields = [
    {
      name        = "GITHUB_TOKEN"
      label       = "GitHub Personal Access Token"
      type        = "password"
      required    = true
      description = "A GitHub PAT with appropriate permissions for the MCP server"
    }
  ]
}
`, name)
}
