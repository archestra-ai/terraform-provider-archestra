package provider

import (
	"fmt"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

func TestAccKnowledgeDirectoryResource(t *testing.T) {
	config := func(name string) string {
		return fmt.Sprintf(`resource "archestra_knowledge_directory" "test" {name=%q}`, name)
	}
	resource.Test(t, resource.TestCase{PreCheck: func() { testAccPreCheck(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{
		{Config: config("tf-directory-initial"), Check: resource.TestCheckResourceAttr("archestra_knowledge_directory.test", "file_count", "0")},
		{ResourceName: "archestra_knowledge_directory.test", ImportState: true, ImportStateVerify: true},
		{Config: config("tf-directory-updated"), ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction("archestra_knowledge_directory.test", plancheck.ResourceActionUpdate)}}, Check: resource.TestCheckResourceAttr("archestra_knowledge_directory.test", "name", "tf-directory-updated")},
		{Config: config("tf-directory-updated"), ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
	}})
}

func TestAccMCPServerInstallationHibernation(t *testing.T) {
	config := func(mode string) string {
		return strings.TrimSuffix(strings.TrimSpace(testAccMCPServerInstallationResourceConfig("tf-hibernation")), "}") + fmt.Sprintf("hibernation_mode=%q\n}", mode)
	}
	resource.Test(t, resource.TestCase{PreCheck: func() { testAccPreCheck(t); testAccRequireEnterprise(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{
		{Config: config("disabled"), Check: resource.TestCheckResourceAttr("archestra_mcp_server.test", "hibernation_mode", "disabled")},
		{Config: config("enabled"), ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction("archestra_mcp_server.test", plancheck.ResourceActionUpdate)}}, Check: resource.TestCheckResourceAttr("archestra_mcp_server.test", "hibernation_mode", "enabled")},
		{Config: config("enabled"), ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
	}})
}

func TestAccMCPServerInstallationPersonalUser(t *testing.T) {
	config := testAccMCPServerInstallationResourceConfig("tf-personal-reference") + testAccMCPServerResourceConfig("tf-personal-target-catalog") + `
resource "archestra_mcp_server" "personal_user" {
name="tf-personal-target"
catalog_id=archestra_mcp_registry_entry.test.id
scope="personal"
user_id=archestra_mcp_server.test.user_id
}`

	resource.Test(t, resource.TestCase{PreCheck: func() { testAccPreCheck(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{
		{Config: config, Check: resource.TestCheckResourceAttrPair("archestra_mcp_server.personal_user", "user_id", "archestra_mcp_server.test", "user_id")},
		{Config: config, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
	}})
}
