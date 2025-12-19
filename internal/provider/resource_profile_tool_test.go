package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccProfileToolResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){
			"archestra": providerserver.NewProtocol6WithError(New("test")()),
		},
		Steps: []resource.TestStep{
			// Create and Read
			{
				Config: testAccProfileToolResourceConfig("archestra-test-profile"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair("archestra_profile_tool.test", "profile_id", "archestra_profile.test", "id"),
					resource.TestCheckResourceAttr("archestra_profile_tool.test", "tool_result_treatment", "untrusted"),
					resource.TestCheckResourceAttr("archestra_profile_tool.test", "use_dynamic_team_credential", "true"),
					resource.TestCheckResourceAttr("archestra_profile_tool.test", "response_modifier_template", "Hello {{.Result}}"),
					resource.TestCheckResourceAttrPair("archestra_profile_tool.test", "credential_source_mcp_server_id", "archestra_mcp_server_installation.test", "id"),
					resource.TestCheckResourceAttrSet("archestra_profile_tool.test", "id"),
				),
			},
			// Update
			{
				Config: testAccProfileToolResourceConfigUpdate("archestra-test-profile"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("archestra_profile_tool.test", "tool_result_treatment", "trusted"),
					resource.TestCheckResourceAttr("archestra_profile_tool.test", "allow_usage_when_untrusted_data_is_present", "true"),
					resource.TestCheckResourceAttr("archestra_profile_tool.test", "use_dynamic_team_credential", "false"),
					resource.TestCheckResourceAttr("archestra_profile_tool.test", "response_modifier_template", "Modified {{.Result}}"),
					resource.TestCheckResourceAttrPair("archestra_profile_tool.test", "credential_source_mcp_server_id", "archestra_mcp_server_installation.test", "id"),
					resource.TestCheckResourceAttrPair("archestra_profile_tool.test", "execution_source_mcp_server_id", "archestra_mcp_server_installation.test", "id"),
				),
			},
			// Unset optional fields
			{
				Config: testAccProfileToolResourceConfigUnset("archestra-test-profile"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("archestra_profile_tool.test", "tool_result_treatment", "trusted"),
					resource.TestCheckResourceAttr("archestra_profile_tool.test", "allow_usage_when_untrusted_data_is_present", "true"),
					// Verify optional fields are unset (null) or defaulted
					resource.TestCheckResourceAttr("archestra_profile_tool.test", "use_dynamic_team_credential", "false"),
					resource.TestCheckNoResourceAttr("archestra_profile_tool.test", "credential_source_mcp_server_id"),
					resource.TestCheckNoResourceAttr("archestra_profile_tool.test", "execution_source_mcp_server_id"),
					resource.TestCheckNoResourceAttr("archestra_profile_tool.test", "response_modifier_template"),
				),
			},
			// Import
			{
				ResourceName:      "archestra_profile_tool.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			// Delete checks are implicit
		},
	})
}

func testAccProfileToolResourceConfig(profileName string) string {
	return fmt.Sprintf(`
resource "archestra_profile" "test" {
  name = "%[1]s"
}

resource "archestra_mcp_registry_catalog_item" "test" {
  name = "test-server"
  local_config = {
    command = "npx"
    arguments = ["-y", "@modelcontextprotocol/server-filesystem", "./"]
  }
}

resource "archestra_mcp_server_installation" "test" {
  name          = "test-server-inst"
  mcp_server_id = archestra_mcp_registry_catalog_item.test.id
}

data "archestra_mcp_server_tool" "test" {
  mcp_server_id = archestra_mcp_server_installation.test.id
  name          = "test-server__read_file"
  depends_on    = [archestra_mcp_server_installation.test]
}

resource "archestra_profile_tool" "test" {
  profile_id = archestra_profile.test.id
  tool_id    = data.archestra_mcp_server_tool.test.id
  
  # Set a specific treatment to verify it is applied
  tool_result_treatment = "untrusted"
  use_dynamic_team_credential = true
  response_modifier_template  = "Hello {{.Result}}"
  credential_source_mcp_server_id = archestra_mcp_server_installation.test.id
  execution_source_mcp_server_id  = archestra_mcp_server_installation.test.id
}
`, profileName)
}

func testAccProfileToolResourceConfigUpdate(profileName string) string {
	return fmt.Sprintf(`
resource "archestra_profile" "test" {
  name = "%[1]s"
}

resource "archestra_mcp_registry_catalog_item" "test" {
  name = "test-server"
  local_config = {
    command = "npx"
    arguments = ["-y", "@modelcontextprotocol/server-filesystem", "./"]
  }
}

resource "archestra_mcp_server_installation" "test" {
  name          = "test-server-inst"
  mcp_server_id = archestra_mcp_registry_catalog_item.test.id
}

data "archestra_mcp_server_tool" "test" {
  mcp_server_id = archestra_mcp_server_installation.test.id
  name          = "test-server__read_file"
  depends_on    = [archestra_mcp_server_installation.test]
}

resource "archestra_profile_tool" "test" {
  profile_id = archestra_profile.test.id
  tool_id    = data.archestra_mcp_server_tool.test.id
  
  tool_result_treatment = "trusted"
  allow_usage_when_untrusted_data_is_present = true
  use_dynamic_team_credential = false
  response_modifier_template  = "Modified {{.Result}}"
  credential_source_mcp_server_id = archestra_mcp_server_installation.test.id
  execution_source_mcp_server_id  = archestra_mcp_server_installation.test.id
}
`, profileName)
}

func testAccProfileToolResourceConfigUnset(profileName string) string {
	return fmt.Sprintf(`
resource "archestra_profile" "test" {
  name = "%[1]s"
}

resource "archestra_mcp_registry_catalog_item" "test" {
  name = "test-server"
  local_config = {
    command = "npx"
    arguments = ["-y", "@modelcontextprotocol/server-filesystem", "./"]
  }
}

resource "archestra_mcp_server_installation" "test" {
  name          = "test-server-inst"
  mcp_server_id = archestra_mcp_registry_catalog_item.test.id
}

data "archestra_mcp_server_tool" "test" {
  mcp_server_id = archestra_mcp_server_installation.test.id
  name          = "test-server__read_file"
  depends_on    = [archestra_mcp_server_installation.test]
}

resource "archestra_profile_tool" "test" {
  profile_id = archestra_profile.test.id
  tool_id    = data.archestra_mcp_server_tool.test.id
  
  // Keep required/other fields
  tool_result_treatment = "trusted"
  allow_usage_when_untrusted_data_is_present = true
  
  // Explicitly removed:
  // use_dynamic_team_credential
  // credential_source_mcp_server_id
  // execution_source_mcp_server_id
  // response_modifier_template
}
`, profileName)
}
