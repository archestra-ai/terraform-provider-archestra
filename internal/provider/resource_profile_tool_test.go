package provider

import (
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccProfileToolResource(t *testing.T) {
	t.Skip("Skipping - requires unassigned tool from MCP server catalog (tools not discoverable until server is running)")
}

func TestAccProfileToolResourceSanitize(t *testing.T) {
	t.Skip("Skipping - requires unassigned tool from MCP server catalog (tools not discoverable until server is running)")
}

func TestAccProfileToolResourceInvalidTreatment(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccProfileToolResourceConfigInvalidTreatment(),
				ExpectError: regexp.MustCompile(`value must be one of`),
			},
		},
	})
}

func testAccProfileToolResourceConfigInvalidTreatment() string {
	return `
resource "archestra_agent" "test" {
  name = "profile-tool-invalid-test"
}

resource "archestra_profile_tool" "test" {
  profile_id            = archestra_agent.test.id
  tool_id               = "00000000-0000-0000-0000-000000000000"
  tool_result_treatment = "invalid_treatment_value"
}
`
}
