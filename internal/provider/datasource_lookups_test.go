package provider

import (
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// A team, a service account and an environment are found by exact name, and
// the lookup returns the ID the resource was created with.
func TestAccLookupDataSourcesByName(t *testing.T) {
	const created = `
resource "archestra_team" "test" {
  name        = "tf-acc-lookup-team"
  description = "Found by name"
}

resource "archestra_service_account" "test" {
  name = "tf-acc-lookup-service-account"
  role = "member"
}

resource "archestra_environment" "test" {
  name      = "tf-acc-lookup-environment"
  namespace = "default"
}
`
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: created},
			{
				Config: created + `
data "archestra_team" "test" {
  name = archestra_team.test.name
}

data "archestra_service_account" "test" {
  name = archestra_service_account.test.name
}

data "archestra_environment" "test" {
  name = archestra_environment.test.name
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair("data.archestra_team.test", "id", "archestra_team.test", "id"),
					resource.TestCheckResourceAttr("data.archestra_team.test", "description", "Found by name"),
					resource.TestCheckResourceAttrPair("data.archestra_service_account.test", "id", "archestra_service_account.test", "id"),
					resource.TestCheckResourceAttr("data.archestra_service_account.test", "role", "member"),
					resource.TestCheckResourceAttrPair("data.archestra_environment.test", "id", "archestra_environment.test", "id"),
				),
			},
			// The API's team name filter matches part of a name; the lookup does not.
			{
				Config:      created + `data "archestra_team" "partial" { name = "tf-acc-lookup" }`,
				ExpectError: regexp.MustCompile(`Team Not Found`),
			},
			{
				Config:      created + `data "archestra_team" "empty" {}`,
				ExpectError: regexp.MustCompile(`Missing Lookup`),
			},
			{
				Config:      created + `data "archestra_service_account" "missing" { name = "tf-acc-no-such-account" }`,
				ExpectError: regexp.MustCompile(`Service Account Not Found`),
			},
			{
				Config:      created + `data "archestra_environment" "missing" { name = "tf-acc-no-such-environment" }`,
				ExpectError: regexp.MustCompile(`Environment Not Found`),
			},
			{
				Config:      created + `data "archestra_mcp_server" "missing" { name = "tf-acc-no-such-installation" }`,
				ExpectError: regexp.MustCompile(`MCP Server Not Found`),
			},
			{
				Config:      created + `data "archestra_mcp_server" "empty" {}`,
				ExpectError: regexp.MustCompile(`Missing Lookup`),
			},
			{
				Config:      created + `data "archestra_user" "missing" { email = "tf-acc-nobody@example.invalid" }`,
				ExpectError: regexp.MustCompile(`User Not Found`),
			},
		},
	})
}
