package provider

import (
	"fmt"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"testing"
)

func TestAccCatalogCurrentEnvironmentAndClone(t *testing.T) {
	config := func(environment string) string {
		return fmt.Sprintf(`
 resource "archestra_environment" "first" {
 name="tf-catalog-first"
 namespace="default"
 }
 resource "archestra_environment" "second" {
 name="tf-catalog-second"
 namespace="default"
 }
 resource "archestra_mcp_registry_catalog_item" "source" {
 name="tf-catalog-clone-source"
 remote_config={url="http://127.0.0.1:19999/mcp"}
 }
 resource "archestra_mcp_registry_catalog_item" "test" {
 name="tf-catalog-current-fields"
 cloned_from=archestra_mcp_registry_catalog_item.source.id
 environment_id=archestra_environment.%s.id
 multitenant=false
 remote_config={url="http://127.0.0.1:19999/mcp"}
 }
 `, environment)
	}
	initial := config("first")
	updated := config("second")
	resource.Test(t, resource.TestCase{PreCheck: func() { testAccPreCheck(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{
		{Config: initial, Check: resource.ComposeAggregateTestCheckFunc(resource.TestCheckResourceAttrPair("archestra_mcp_registry_catalog_item.test", "cloned_from", "archestra_mcp_registry_catalog_item.source", "id"), resource.TestCheckResourceAttrPair("archestra_mcp_registry_catalog_item.test", "environment_id", "archestra_environment.first", "id"), resource.TestCheckResourceAttr("archestra_mcp_registry_catalog_item.test", "multitenant", "false"))},
		{ResourceName: "archestra_mcp_registry_catalog_item.test", ImportState: true, ImportStateVerify: true},
		{Config: initial, PlanOnly: true},
		{Config: updated, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction("archestra_mcp_registry_catalog_item.test", plancheck.ResourceActionUpdate)}}, Check: resource.TestCheckResourceAttrPair("archestra_mcp_registry_catalog_item.test", "environment_id", "archestra_environment.second", "id")},
		{Config: updated, PlanOnly: true},
	}})
}
