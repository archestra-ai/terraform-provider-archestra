package provider

import (
	"fmt"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

func TestAccDefaultUserLimitResource(t *testing.T) {
	config := func(amount int, models string) string {
		return fmt.Sprintf(`
 resource "archestra_environment" "limit_parent" {name="tf-default-limit-parent"}
 resource "archestra_default_user_limit" "test" {
 environment_id=archestra_environment.limit_parent.id
 limit_value=%d
 cleanup_interval="calendar_month"
 model=%s
 }`, amount, models)
	}
	resource.Test(t, resource.TestCase{PreCheck: func() { testAccPreCheck(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{
		{Config: config(100, `["test-model"]`), Check: resource.TestCheckResourceAttr("archestra_default_user_limit.test", "limit_value", "100")},
		{ResourceName: "archestra_default_user_limit.test", ImportState: true, ImportStateVerify: true},
		{Config: config(250, `[]`), ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction("archestra_default_user_limit.test", plancheck.ResourceActionUpdate)}}, Check: resource.ComposeTestCheckFunc(resource.TestCheckResourceAttr("archestra_default_user_limit.test", "limit_value", "250"), resource.TestCheckResourceAttr("archestra_default_user_limit.test", "model.#", "0"))},
		{Config: config(250, `[]`), ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
	}})
}

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
		{Config: config("disabled"), Check: resource.TestCheckResourceAttr("archestra_mcp_server_installation.test", "hibernation_mode", "disabled")},
		{Config: config("enabled"), ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction("archestra_mcp_server_installation.test", plancheck.ResourceActionUpdate)}}, Check: resource.TestCheckResourceAttr("archestra_mcp_server_installation.test", "hibernation_mode", "enabled")},
		{Config: config("enabled"), ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
	}})
}

func TestAccMCPServerInstallationPersonalUser(t *testing.T) {
	config := testAccMCPServerInstallationResourceConfig("tf-personal-reference") + testAccMCPServerResourceConfig("tf-personal-target-catalog") + `
resource "archestra_mcp_server_installation" "personal_user" {
name="tf-personal-target"
catalog_id=archestra_mcp_registry_catalog_item.test.id
scope="personal"
user_id=archestra_mcp_server_installation.test.user_id
}`

	resource.Test(t, resource.TestCase{PreCheck: func() { testAccPreCheck(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{
		{Config: config, Check: resource.TestCheckResourceAttrPair("archestra_mcp_server_installation.personal_user", "user_id", "archestra_mcp_server_installation.test", "user_id")},
		{Config: config, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
	}})
}
