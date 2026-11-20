package provider

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// Each object is created, then found by its exact name, and the lookup
// returns the ID the resource was created with. A partial name finds nothing.
func TestAccNamedLookupDataSources(t *testing.T) {
	suffix := acctest.RandString(8)
	created := fmt.Sprintf(`
resource "archestra_role" "test" {
  name       = "tf-acc-lookup-role-%[1]s"
  permission = { agent = ["read"] }
}

resource "archestra_project" "test" {
  name        = "tf-acc-lookup-project-%[1]s"
  description = "Found by name"
}

resource "archestra_knowledge_base" "test" {
  name = "tf-acc-lookup-kb-%[1]s"
}

resource "archestra_service_account" "test" {
  name = "tf-acc-lookup-oauth-sa-%[1]s"
  role = "member"
}

resource "archestra_oauth_client" "test" {
  name                     = "tf-acc-lookup-oauth-%[1]s"
  scopes                   = ["llm:proxy"]
  include_future_providers = true
  service_account_id       = archestra_service_account.test.id
}

resource "archestra_mcp_registry_entry" "test" {
  name = "tf-acc-lookup-mcp-%[1]s"
  local_config = {
    command   = "npx"
    arguments = ["-y", "@modelcontextprotocol/server-filesystem", "/tmp"]
  }
}

resource "archestra_identity_provider" "test" {
  provider_id = "tf-acc-lookup-idp-%[1]s"
  domain      = "lookup-%[1]s.example.com"
  issuer      = "https://accounts.google.com"

  oidc_config {
    issuer             = "https://accounts.google.com"
    discovery_endpoint = "https://accounts.google.com/.well-known/openid-configuration"
    client_id          = "terraform-client"
    client_secret      = "terraform-secret"
    scopes             = ["openid", "email", "profile"]

    mapping {
      email = "email"
      name  = "name"
    }
  }

  role_mapping {
    default_role = "member"
  }
}
`, suffix)
	lookups := `
data "archestra_role" "test" {
  name = archestra_role.test.name
}

data "archestra_project" "test" {
  name = archestra_project.test.name
}

data "archestra_knowledge_base" "test" {
  name = archestra_knowledge_base.test.name
}

data "archestra_oauth_client" "test" {
  name = archestra_oauth_client.test.name
}

data "archestra_mcp_registry_entry" "test" {
  name = archestra_mcp_registry_entry.test.name
}

data "archestra_identity_provider" "test" {
  provider_id = archestra_identity_provider.test.provider_id
}
`
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t); testAccRequireEnterprise(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: created},
			{
				Config: created + lookups,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair("data.archestra_role.test", "id", "archestra_role.test", "id"),
					resource.TestCheckResourceAttrPair("data.archestra_role.test", "role", "archestra_role.test", "role"),
					resource.TestCheckResourceAttr("data.archestra_role.test", "predefined", "false"),
					resource.TestCheckResourceAttrPair("data.archestra_project.test", "id", "archestra_project.test", "id"),
					resource.TestCheckResourceAttr("data.archestra_project.test", "description", "Found by name"),
					resource.TestCheckResourceAttrPair("data.archestra_knowledge_base.test", "id", "archestra_knowledge_base.test", "id"),
					resource.TestCheckResourceAttrPair("data.archestra_oauth_client.test", "id", "archestra_oauth_client.test", "id"),
					resource.TestCheckResourceAttrPair("data.archestra_oauth_client.test", "service_account_id", "archestra_service_account.test", "id"),
					resource.TestCheckResourceAttrPair("data.archestra_mcp_registry_entry.test", "id", "archestra_mcp_registry_entry.test", "id"),
					resource.TestCheckResourceAttrPair("data.archestra_identity_provider.test", "id", "archestra_identity_provider.test", "id"),
					resource.TestCheckResourceAttr("data.archestra_identity_provider.test", "issuer", "https://accounts.google.com"),
				),
			},
			{
				Config:      created + `data "archestra_project" "partial" { name = "tf-acc-lookup-project" }`,
				ExpectError: regexp.MustCompile(`Project Not Found`),
			},
			{
				Config:      created + `data "archestra_mcp_registry_entry" "missing" { name = "tf-acc-no-such-entry" }`,
				ExpectError: regexp.MustCompile(`MCP Registry Entry Not Found`),
			},
		},
	})
}
