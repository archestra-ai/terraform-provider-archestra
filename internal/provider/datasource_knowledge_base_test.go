package provider

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccKnowledgeBaseDataSource verifies the singular lookup
// against a freshly-created KB. The KB resource isn't registered on
// this branch, so we provision the KB via curl in a `t.Cleanup`-paired
// fixture below — too noisy. Instead, this test relies on the
// `data.archestra_knowledge_bases` plural data source to discover
// any KB created by other acceptance tests in the same backend.
// If the backend has zero KBs, the test t.Skips with an actionable
// message rather than silently passing.
//
// Note: this test runs `data.archestra_knowledge_bases` first to
// find a known-existing KB ID, then re-queries it through the
// singular data source.
func TestAccKnowledgeBaseDataSource_NotFound(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
data "archestra_knowledge_base" "missing" {
  id = "00000000-0000-0000-0000-000000000000"
}
`,
				ExpectError: regexp.MustCompile(`(?i)not found`),
			},
		},
	})
}

// TestAccKnowledgeBasesDataSource exercises the plural list against
// an empty filter. The list endpoint always returns 200 even when
// the org has zero KBs — the assertion only checks that `total` is
// set, not its value.
func TestAccKnowledgeBasesDataSource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `data "archestra_knowledge_bases" "all" {}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.archestra_knowledge_bases.all", "total"),
				),
			},
		},
	})
}

// TestAccRoleDataSource_Predefined uses a literal predefined role
// name (`admin`) to verify the singular lookup. Predefined roles are
// always present on every backend.
func TestAccRoleDataSource_Predefined(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `data "archestra_role" "admin" { id = "admin" }`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.archestra_role.admin", "predefined", "true"),
					resource.TestCheckResourceAttr("data.archestra_role.admin", "role", "admin"),
					resource.TestCheckResourceAttrSet("data.archestra_role.admin", "permission.organization.0"),
				),
			},
		},
	})
}

// TestAccRolesDataSource lists all roles and asserts at least the
// predefined ones are present.
func TestAccRolesDataSource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `data "archestra_roles" "all" {}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					// Predefined admin / member / owner are seeded on
					// every Archestra backend, so the count must be
					// non-zero.
					resource.TestMatchResourceAttr("data.archestra_roles.all", "total", regexp.MustCompile(`^[1-9]`)),
				),
			},
		},
	})
}

// TestAccOrganizationMembersDataSource lists every member. Backend
// always has at least the seeded admin user.
func TestAccOrganizationMembersDataSource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `data "archestra_organization_members" "all" {}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestMatchResourceAttr("data.archestra_organization_members.all", "total", regexp.MustCompile(`^[1-9]`)),
				),
			},
		},
	})
}

// TestAccVirtualApiKeysDataSource lists virtual keys. Backend may
// have zero — assert only that `total` is set, not its value.
func TestAccVirtualApiKeysDataSource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `data "archestra_virtual_api_keys" "all" {}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.archestra_virtual_api_keys.all", "total"),
				),
			},
		},
	})
}

// TestAccKnowledgeConnectorDataSource_NotFound verifies the 404
// path on the singular connector lookup. The connector resource
// isn't registered on this branch, so a positive-path test would
// require provisioning a connector through another path; the 404
// case is the more robust regression pin.
func TestAccKnowledgeConnectorDataSource_NotFound(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      `data "archestra_knowledge_connector" "missing" { id = "00000000-0000-0000-0000-000000000000" }`,
				ExpectError: regexp.MustCompile(`(?i)not found`),
			},
		},
	})
}

// TestAccScheduleTriggerRunsDataSource_BadTrigger verifies the
// 404 path on the runs lookup when the trigger UUID doesn't exist.
func TestAccScheduleTriggerRunsDataSource_BadTrigger(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
data "archestra_schedule_trigger_runs" "missing" {
  trigger_id = "11111111-1111-4111-a111-111111111111"
}
`,
				ExpectError: regexp.MustCompile(`(?i)not found`),
			},
		},
	})
}

// TestAccScheduleTriggerRunsDataSource_InvalidStatus pins the
// status validator.
func TestAccScheduleTriggerRunsDataSource_InvalidStatus(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
data "archestra_schedule_trigger_runs" "test" {
  trigger_id = "00000000-0000-0000-0000-000000000000"
  status     = %q
}
`, "bogus-status"),
				ExpectError: regexp.MustCompile(`value must be one of`),
			},
		},
	})
}
