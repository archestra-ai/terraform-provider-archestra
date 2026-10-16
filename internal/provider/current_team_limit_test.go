package provider

import (
	"fmt"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"testing"
)

func TestAccCurrentTeamHierarchy(t *testing.T) {
	base := `resource "archestra_team" "parent" {name="tf-hierarchy-parent"}
 resource "archestra_team" "test" {
 name="tf-hierarchy-child"
 `
	initial := base + `parent_id=archestra_team.parent.id
 labels=[{key="stage",value="initial"}]
 }`
	updated := base + `parent_id=archestra_team.parent.id
 labels=[{key="stage",value="updated"}]
 }`
	cleared := base + `labels=[]
 }`
	resource.Test(t, resource.TestCase{PreCheck: func() { testAccPreCheck(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{
		{Config: initial, Check: resource.ComposeAggregateTestCheckFunc(resource.TestCheckResourceAttrPair("archestra_team.test", "parent_id", "archestra_team.parent", "id"), resource.TestCheckResourceAttr("archestra_team.test", "labels.0.value", "initial"))},
		{ResourceName: "archestra_team.test", ImportState: true, ImportStateVerify: true},
		{Config: initial, PlanOnly: true},
		{Config: updated, Check: resource.TestCheckResourceAttr("archestra_team.test", "labels.0.value", "updated")},
		{Config: updated, PlanOnly: true},
		{Config: cleared, Check: resource.TestCheckNoResourceAttr("archestra_team.test", "parent_id")},
		{Config: cleared, PlanOnly: true},
	}})
}
func TestAccCurrentLimitFields(t *testing.T) {
	orgID := testAccOrganizationID(t)
	config := func(interval, labels string) string {
		return fmt.Sprintf(`resource "archestra_limit" "test" {
 entity_id=%q
 entity_type="organization"
 limit_type="token_cost"
 limit_value=100000
 model=["provider-test-model"]
 cleanup_interval=%q
 labels=%s
 }`, orgID, interval, labels)
	}
	initial := config("calendar_day", `[{key="stage",value="initial"}]`)
	updated := config("calendar_month", `[{key="stage",value="updated"}]`)
	cleared := config("24h", `[]`)
	resource.Test(t, resource.TestCase{PreCheck: func() { testAccPreCheck(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{
		{Config: initial, Check: resource.TestCheckResourceAttr("archestra_limit.test", "cleanup_interval", "calendar_day")},
		{ResourceName: "archestra_limit.test", ImportState: true, ImportStateVerify: true},
		{Config: initial, PlanOnly: true},
		{Config: updated, Check: resource.ComposeAggregateTestCheckFunc(resource.TestCheckResourceAttr("archestra_limit.test", "cleanup_interval", "calendar_month"), resource.TestCheckResourceAttr("archestra_limit.test", "labels.0.value", "updated"))},
		{Config: updated, PlanOnly: true},
		{Config: cleared, Check: resource.TestCheckResourceAttr("archestra_limit.test", "labels.#", "0")},
		{Config: cleared, PlanOnly: true},
	}})
}
