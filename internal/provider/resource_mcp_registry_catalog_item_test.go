package provider

import (
	"fmt"
	"os"
	"regexp"
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
						knownvalue.SetExact([]knownvalue.Check{
							knownvalue.ObjectPartial(map[string]knownvalue.Check{
								"key":   knownvalue.StringExact("API_URL"),
								"type":  knownvalue.StringExact("plain_text"),
								"value": knownvalue.StringExact("{{API_URL}}"),
							}),
							knownvalue.ObjectPartial(map[string]knownvalue.Check{
								"key":   knownvalue.StringExact("API_TOKEN"),
								"type":  knownvalue.StringExact("plain_text"),
								"value": knownvalue.StringExact("{{API_TOKEN}}"),
							}),
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
    environment = [
      { key = "API_URL",   type = "plain_text", value = "{{API_URL}}" },
      { key = "API_TOKEN", type = "plain_text", value = "{{API_TOKEN}}" },
    ]
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
						knownvalue.SetExact([]knownvalue.Check{
							knownvalue.ObjectPartial(map[string]knownvalue.Check{
								"key":   knownvalue.StringExact("GRAFANA_URL"),
								"type":  knownvalue.StringExact("plain_text"),
								"value": knownvalue.StringExact("{{GRAFANA_URL}}"),
							}),
							knownvalue.ObjectPartial(map[string]knownvalue.Check{
								"key":   knownvalue.StringExact("GRAFANA_SERVICE_ACCOUNT_TOKEN"),
								"type":  knownvalue.StringExact("secret"),
								"value": knownvalue.StringExact("{{GRAFANA_SERVICE_ACCOUNT_TOKEN}}"),
							}),
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
    environment = [
      { key = "GRAFANA_URL",                   type = "plain_text", value = "{{GRAFANA_URL}}" },
      { key = "GRAFANA_SERVICE_ACCOUNT_TOKEN", type = "secret",     value = "{{GRAFANA_SERVICE_ACCOUNT_TOKEN}}" },
    ]
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

// TestAccMcpRegistryCatalogItemResourceWithEnvDefaults exercises the
// `local_config.environment[].default` round-trip across all three scalar
// types (plain_text, number, boolean). Regression coverage for the codegen
// patcher fix in c62a2f1: before the patcher rewrote `envVar.Default` to
// `interface{}`, the inline `z.union([string, number, boolean])` produced a
// broken Go union with an unexported field, so json.Marshal returned `"{}"`
// and Catalog Read wrote `"{}"` into state — causing permanent plan diffs
// for any env var with a default. The second step re-applies the same
// Config so the framework's plan-no-drift assertion catches any state
// divergence between Create and Read.
func TestAccMcpRegistryCatalogItemResourceWithEnvDefaults(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create + Read: assert each default round-trips to its bare
			// HCL form, NOT the broken `"{}"` value.
			{
				Config: testAccMcpRegistryCatalogItemResourceConfigWithEnvDefaults("test-env-defaults-item"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"archestra_mcp_registry_catalog_item.test_env_defaults",
						tfjsonpath.New("name"),
						knownvalue.StringExact("test-env-defaults-item"),
					),
					statecheck.ExpectKnownValue(
						"archestra_mcp_registry_catalog_item.test_env_defaults",
						tfjsonpath.New("local_config").AtMapKey("environment"),
						knownvalue.SetExact([]knownvalue.Check{
							knownvalue.ObjectPartial(map[string]knownvalue.Check{
								"key":     knownvalue.StringExact("GREETING"),
								"type":    knownvalue.StringExact("plain_text"),
								"default": knownvalue.StringExact("hello"),
							}),
							knownvalue.ObjectPartial(map[string]knownvalue.Check{
								"key":     knownvalue.StringExact("MAX_RETRIES"),
								"type":    knownvalue.StringExact("number"),
								"default": knownvalue.StringExact("42"),
							}),
							knownvalue.ObjectPartial(map[string]knownvalue.Check{
								"key":     knownvalue.StringExact("ENABLE_CACHE"),
								"type":    knownvalue.StringExact("boolean"),
								"default": knownvalue.StringExact("true"),
							}),
						}),
					),
				},
			},
			// Re-apply identical Config: the framework runs an internal
			// `terraform plan` and fails if state drifted between Create
			// and Read (which is exactly what the `"{}"` bug produced).
			{
				Config: testAccMcpRegistryCatalogItemResourceConfigWithEnvDefaults("test-env-defaults-item"),
			},
			// ImportState testing
			{
				ResourceName:      "archestra_mcp_registry_catalog_item.test_env_defaults",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccMcpRegistryCatalogItemResourceConfigWithEnvDefaults(name string) string {
	return fmt.Sprintf(`
resource "archestra_mcp_registry_catalog_item" "test_env_defaults" {
  name        = %[1]q
  description = "Test MCP server with polymorphic environment defaults"

  local_config = {
    docker_image = "mcp/grafana"
    arguments    = ["-t", "stdio"]
    environment = [
      { key = "GREETING",     type = "plain_text", default = "hello" },
      { key = "MAX_RETRIES",  type = "number",     default = "42" },
      { key = "ENABLE_CACHE", type = "boolean",    default = "true" },
    ]
  }
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

// TestAccMcpRegistryCatalogItemResourceAdvancedOAuth exercises the OAuth fields
// added in Archestra v1.2.20: grant_type, audience, endpoint overrides,
// default_scopes, discovery URL, provider metadata, and assorted flags.
func TestAccMcpRegistryCatalogItemResourceAdvancedOAuth(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccMcpRegistryCatalogItemResourceConfigAdvancedOAuth("oauth-advanced-" + acctest.RandString(6)),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"archestra_mcp_registry_catalog_item.oauth_advanced",
						tfjsonpath.New("remote_config").AtMapKey("oauth_config").AtMapKey("grant_type"),
						knownvalue.StringExact("authorization_code"),
					),
					statecheck.ExpectKnownValue(
						"archestra_mcp_registry_catalog_item.oauth_advanced",
						tfjsonpath.New("remote_config").AtMapKey("oauth_config").AtMapKey("audience"),
						knownvalue.StringExact("https://api.example.com"),
					),
					statecheck.ExpectKnownValue(
						"archestra_mcp_registry_catalog_item.oauth_advanced",
						tfjsonpath.New("remote_config").AtMapKey("oauth_config").AtMapKey("authorization_endpoint"),
						knownvalue.StringExact("https://auth.example.com/oauth/authorize"),
					),
					statecheck.ExpectKnownValue(
						"archestra_mcp_registry_catalog_item.oauth_advanced",
						tfjsonpath.New("remote_config").AtMapKey("oauth_config").AtMapKey("token_endpoint"),
						knownvalue.StringExact("https://auth.example.com/oauth/token"),
					),
					statecheck.ExpectKnownValue(
						"archestra_mcp_registry_catalog_item.oauth_advanced",
						tfjsonpath.New("remote_config").AtMapKey("oauth_config").AtMapKey("well_known_url"),
						knownvalue.StringExact("https://auth.example.com/.well-known/oauth-authorization-server"),
					),
					statecheck.ExpectKnownValue(
						"archestra_mcp_registry_catalog_item.oauth_advanced",
						tfjsonpath.New("remote_config").AtMapKey("oauth_config").AtMapKey("default_scopes"),
						knownvalue.ListExact([]knownvalue.Check{knownvalue.StringExact("openid")}),
					),
					statecheck.ExpectKnownValue(
						"archestra_mcp_registry_catalog_item.oauth_advanced",
						tfjsonpath.New("remote_config").AtMapKey("oauth_config").AtMapKey("provider_name"),
						knownvalue.StringExact("Example OAuth"),
					),
					statecheck.ExpectKnownValue(
						"archestra_mcp_registry_catalog_item.oauth_advanced",
						tfjsonpath.New("remote_config").AtMapKey("oauth_config").AtMapKey("browser_auth"),
						knownvalue.Bool(true),
					),
					statecheck.ExpectKnownValue(
						"archestra_mcp_registry_catalog_item.oauth_advanced",
						tfjsonpath.New("remote_config").AtMapKey("oauth_config").AtMapKey("generic_oauth"),
						knownvalue.Bool(false),
					),
				},
			},
		},
	})
}

func testAccMcpRegistryCatalogItemResourceConfigAdvancedOAuth(name string) string {
	return fmt.Sprintf(`
resource "archestra_mcp_registry_catalog_item" "oauth_advanced" {
  name        = %[1]q
  description = "OAuth advanced config for acceptance testing"

  remote_config = {
    url = "https://api.example.com/mcp/"
    oauth_config = {
      client_id              = "acc-client"
      client_secret          = "acc-secret"
      grant_type             = "authorization_code"
      redirect_uris          = ["https://app.example.com/callback"]
      scopes                 = ["read", "write"]
      default_scopes         = ["openid"]
      audience               = "https://api.example.com"
      authorization_endpoint = "https://auth.example.com/oauth/authorize"
      token_endpoint         = "https://auth.example.com/oauth/token"
      well_known_url         = "https://auth.example.com/.well-known/oauth-authorization-server"
      provider_name          = "Example OAuth"
      browser_auth           = true
      generic_oauth          = false
    }
  }
}
`, name)
}

// TestAccMcpRegistryCatalogItemResourceImagePullCredentials exercises the
// `credentials` variant of image_pull_secrets added in Archestra v1.2.20.
func TestAccMcpRegistryCatalogItemResourceImagePullCredentials(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccMcpRegistryCatalogItemResourceConfigImagePullCredentials("ipsc-" + acctest.RandString(6)),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"archestra_mcp_registry_catalog_item.ipsc",
						tfjsonpath.New("local_config").AtMapKey("image_pull_secrets").AtSliceIndex(0).AtMapKey("source"),
						knownvalue.StringExact("credentials"),
					),
					statecheck.ExpectKnownValue(
						"archestra_mcp_registry_catalog_item.ipsc",
						tfjsonpath.New("local_config").AtMapKey("image_pull_secrets").AtSliceIndex(0).AtMapKey("server"),
						knownvalue.StringExact("registry.example.com"),
					),
					statecheck.ExpectKnownValue(
						"archestra_mcp_registry_catalog_item.ipsc",
						tfjsonpath.New("local_config").AtMapKey("image_pull_secrets").AtSliceIndex(0).AtMapKey("username"),
						knownvalue.StringExact("deploy-bot"),
					),
				},
			},
		},
	})
}

func testAccMcpRegistryCatalogItemResourceConfigImagePullCredentials(name string) string {
	return fmt.Sprintf(`
resource "archestra_mcp_registry_catalog_item" "ipsc" {
  name        = %[1]q
  description = "Catalog item pulling from a private registry with inline credentials"

  local_config = {
    command      = "/usr/local/bin/mcp-server"
    docker_image = "registry.example.com/team/mcp-server:1.0.0"
    image_pull_secrets = [
      {
        source   = "credentials"
        server   = "registry.example.com"
        username = "deploy-bot"
        password = "super-secret"
        email    = "devops@example.com"
      }
    ]
  }
}
`, name)
}

func TestAccMcpRegistryCatalogItemResourceWithVaultRefs(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
			testAccRequireByosEnabled(t)
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccMcpRegistryCatalogItemResourceConfigWithVaultRefs("vault-refs-item"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"archestra_mcp_registry_catalog_item.vault_refs",
						tfjsonpath.New("local_config_vault_path"),
						knownvalue.StringExact("secret/data/mcp/local"),
					),
					statecheck.ExpectKnownValue(
						"archestra_mcp_registry_catalog_item.vault_refs",
						tfjsonpath.New("local_config_vault_key"),
						knownvalue.StringExact("env"),
					),
				},
			},
		},
	})
}

func testAccMcpRegistryCatalogItemResourceConfigWithVaultRefs(name string) string {
	return fmt.Sprintf(`
resource "archestra_mcp_registry_catalog_item" "vault_refs" {
  name        = %[1]q
  description = "Catalog item referencing BYOS vault paths"

  local_config = {
    command   = "npx"
    arguments = ["-y", "@modelcontextprotocol/server-filesystem", "/tmp"]
  }

  local_config_vault_path = "secret/data/mcp/local"
  local_config_vault_key  = "env"
}
`, name)
}

func TestAccMcpRegistryCatalogItemResourceWithUserConfig(t *testing.T) {
	name := "user-config-" + acctest.RandString(6)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccMcpRegistryCatalogItemResourceConfigWithUserConfig(name),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"archestra_mcp_registry_catalog_item.user_config",
						tfjsonpath.New("user_config").AtMapKey("workspace").AtMapKey("title"),
						knownvalue.StringExact("Workspace Path"),
					),
					statecheck.ExpectKnownValue(
						"archestra_mcp_registry_catalog_item.user_config",
						tfjsonpath.New("user_config").AtMapKey("workspace").AtMapKey("type"),
						knownvalue.StringExact("directory"),
					),
					statecheck.ExpectKnownValue(
						"archestra_mcp_registry_catalog_item.user_config",
						tfjsonpath.New("user_config").AtMapKey("workspace").AtMapKey("required"),
						knownvalue.Bool(true),
					),
					statecheck.ExpectKnownValue(
						"archestra_mcp_registry_catalog_item.user_config",
						tfjsonpath.New("user_config").AtMapKey("max_results").AtMapKey("default"),
						knownvalue.StringExact("50"),
					),
					statecheck.ExpectKnownValue(
						"archestra_mcp_registry_catalog_item.user_config",
						tfjsonpath.New("user_config").AtMapKey("enable_cache").AtMapKey("default"),
						knownvalue.StringExact("true"),
					),
				},
			},
		},
	})
}

func testAccMcpRegistryCatalogItemResourceConfigWithUserConfig(name string) string {
	return fmt.Sprintf(`
resource "archestra_mcp_registry_catalog_item" "user_config" {
  name        = %[1]q
  description = "Catalog item exposing installer-configurable fields"

  local_config = {
    command   = "npx"
    arguments = ["-y", "@modelcontextprotocol/server-filesystem", "/tmp"]
  }

  user_config = {
    workspace = {
      title       = "Workspace Path"
      description = "Absolute path to the workspace root"
      type        = "directory"
      required    = true
    }
    max_results = {
      title       = "Max Results"
      description = "Maximum number of records to return"
      type        = "number"
      default     = jsonencode(50)
      min         = 1
      max         = 500
    }
    enable_cache = {
      title       = "Enable Cache"
      description = "Whether to cache API responses"
      type        = "boolean"
      default     = jsonencode(true)
    }
  }
}
`, name)
}

// TestAccMcpRegistryCatalogItemResourceWithEnterpriseManagedConfig round-trips an EE catalog item.
// Requires a pre-existing identity provider UUID; set ARCHESTRA_TEST_IDP_ID to opt in.
func TestAccMcpRegistryCatalogItemResourceWithEnterpriseManagedConfig(t *testing.T) {
	idpID := os.Getenv("ARCHESTRA_TEST_IDP_ID")
	// Gate the t.Fatal on TF_ACC so plain `go test` doesn't bomb out before
	// resource.Test can apply its own TF_ACC skip. With TF_ACC set, missing
	// IDP env is a setup defect, not a test skip.
	if idpID == "" && os.Getenv("TF_ACC") != "" {
		t.Fatal("ARCHESTRA_TEST_IDP_ID must be set; provision a throwaway IdP with scripts/bootstrap-test-idp.sh and export the resulting UUID")
	}
	name := "emc-" + acctest.RandString(6)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccMcpRegistryCatalogItemResourceConfigWithEMC(name, idpID),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"archestra_mcp_registry_catalog_item.emc",
						tfjsonpath.New("enterprise_managed_config").AtMapKey("identity_provider_id"),
						knownvalue.StringExact(idpID),
					),
					statecheck.ExpectKnownValue(
						"archestra_mcp_registry_catalog_item.emc",
						tfjsonpath.New("enterprise_managed_config").AtMapKey("token_injection_mode"),
						knownvalue.StringExact("header"),
					),
					statecheck.ExpectKnownValue(
						"archestra_mcp_registry_catalog_item.emc",
						tfjsonpath.New("enterprise_managed_config").AtMapKey("assertion_mode"),
						knownvalue.StringExact("exchange"),
					),
				},
			},
		},
	})
}

func testAccMcpRegistryCatalogItemResourceConfigWithEMC(name, idpID string) string {
	return fmt.Sprintf(`
resource "archestra_mcp_registry_catalog_item" "emc" {
  name        = %[1]q
  description = "Catalog item with enterprise-managed credentials"

  remote_config = {
    url = "https://mcp.example.com/sse"
  }

  enterprise_managed_config = {
    identity_provider_id      = %[2]q
    resource_type             = "mcp"
    resource_identifier       = "mcp://example.com/resource"
    requested_credential_type = "bearer_token"
    scopes                    = ["read", "write"]
    audience                  = "mcp.example.com"
    token_injection_mode      = "header"
    header_name               = "Authorization"
    fallback_mode             = "fail_closed"
    cache_ttl_seconds         = 300
    assertion_mode            = "exchange"
  }
}
`, name, idpID)
}

// TestAccMcpRegistryCatalogItemResource_BothConfigsSet pins the
// ValidateConfig XOR — local_config and remote_config must be mutually
// exclusive at plan time.
func TestAccMcpRegistryCatalogItemResource_BothConfigsSet(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "archestra_mcp_registry_catalog_item" "test" {
  name        = "tf-acc-catalog-both"
  description = "x"

  local_config = {
    command   = "node"
    arguments = ["server.js"]
  }

  remote_config = {
    url = "https://example.com/mcp"
  }
}
`,
				ExpectError: regexp.MustCompile(`only one of local_config or remote_config`),
			},
		},
	})
}

// TestAccMcpRegistryCatalogItemResource_NeitherConfigSet pins the other
// arm — at least one of local_config or remote_config must be present.
func TestAccMcpRegistryCatalogItemResource_NeitherConfigSet(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "archestra_mcp_registry_catalog_item" "test" {
  name        = "tf-acc-catalog-neither"
  description = "x"
}
`,
				ExpectError: regexp.MustCompile(`exactly one of local_config or remote_config`),
			},
		},
	})
}

// TestAccMcpRegistryCatalogItemResource_LocalConfigEmpty pins the
// inner XOR — local_config must contain command or docker_image.
func TestAccMcpRegistryCatalogItemResource_LocalConfigEmpty(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "archestra_mcp_registry_catalog_item" "test" {
  name        = "tf-acc-catalog-empty-local"
  description = "x"

  local_config = {
    arguments = ["x"]
  }
}
`,
				ExpectError: regexp.MustCompile(`either command or docker_image must be set`),
			},
		},
	})
}
