package provider

import (
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// Each singular data source ships with a 404 regression pin. Positive
// paths are covered by the corresponding plural data source tests
// (data.archestra_schedule_trigger_runs, data.archestra_organization_members)
// or the resource happy-path tests — building a fresh row inside the
// data-source test and tearing it down adds setup noise without
// improving regression coverage of this specific lookup-by-id flow.

func TestAccAgentDataSource_NotFound(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      `data "archestra_agent" "missing" { id = "11111111-1111-4111-a111-111111111111" }`,
				ExpectError: regexp.MustCompile(`(?i)not found`),
			},
		},
	})
}

func TestAccScheduleTriggerDataSource_NotFound(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      `data "archestra_schedule_trigger" "missing" { id = "11111111-1111-4111-a111-111111111111" }`,
				ExpectError: regexp.MustCompile(`(?i)not found`),
			},
		},
	})
}

func TestAccIdentityProviderDataSource_NotFound(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      `data "archestra_identity_provider" "missing" { id = "does-not-exist" }`,
				ExpectError: regexp.MustCompile(`(?i)not found`),
			},
		},
	})
}

func TestAccApiKeyDataSource_NotFound(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      `data "archestra_api_key" "missing" { id = "definitely-not-real-id" }`,
				ExpectError: regexp.MustCompile(`(?i)not found`),
			},
		},
	})
}
