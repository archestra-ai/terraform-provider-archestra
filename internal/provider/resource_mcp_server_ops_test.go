package provider

import (
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

// TestAccMCPServerReinstallResource happy-path against the filesystem
// MCP fixture. Reinstall doesn't need real credentials (it just
// re-runs the K8s deploy), so this covers the structural shape that
// `reauthenticate` shares (same Create/Read/Update/Delete pattern,
// same `trigger`/`executed_at` mechanism). A direct reauth happy-path
// requires an OAuth-bearing MCP server (GitHub/Slack/etc.) the local
// stack doesn't provision; the reauth path is pinned plan-time via
// TestAccMCPServerReauthenticateResource_NoCredentialFields.
func TestAccMCPServerReinstallResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccMCPServerReinstallConfig("trigger-1"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"archestra_mcp_server_reinstall.test",
						tfjsonpath.New("executed_at"),
						knownvalue.StringRegexp(regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}`)),
					),
				},
			},
			{
				Config: testAccMCPServerReinstallConfig("trigger-1"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

// TestAccMCPServerReauthenticateResource_NoCredentialFields pins
// the ValidateConfig that catches the backend's 400 at plan time.
func TestAccMCPServerReauthenticateResource_NoCredentialFields(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "archestra_mcp_server_reauthenticate" "bad" {
  mcp_server_id = "00000000-0000-0000-0000-000000000000"
  trigger       = "anything"
}
`,
				ExpectError: regexp.MustCompile(`At least one credential field is required`),
			},
		},
	})
}

func testAccMCPServerReinstallConfig(trigger string) string {
	return `
resource "archestra_mcp_registry_catalog_item" "reinstall_dep" {
  name        = "tf-acc-reinstall-dep"
  description = "Dependency server for reinstall test"
  docs_url    = "https://github.com/example/reinstall"

  local_config = {
    command   = "npx"
    arguments = ["-y", "@modelcontextprotocol/server-filesystem", "/tmp"]
  }
}

resource "archestra_mcp_server_installation" "reinstall_target" {
  name       = "tf-acc-reinstall-target"
  catalog_id = archestra_mcp_registry_catalog_item.reinstall_dep.id
}

resource "archestra_mcp_server_reinstall" "test" {
  mcp_server_id = archestra_mcp_server_installation.reinstall_target.id
  trigger       = "` + trigger + `"
}
`
}
