package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

// The organization's one notification is created, edited in place, cleared of
// its expiry, imported and removed.
func TestAccSiteNotificationResource(t *testing.T) {
	const name = "archestra_site_notification.test"
	created := `
resource "archestra_site_notification" "test" {
  content    = "tf-acc: maintenance on Saturday"
  expires_at = "2099-01-01T00:00:00Z"
  is_active  = false
}
`
	updated := `
resource "archestra_site_notification" "test" {
  content     = "tf-acc: maintenance moved to Sunday"
  is_active   = false
  dismissible = false
}
`
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: created,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(name, "id"),
					resource.TestCheckResourceAttr(name, "expires_at", "2099-01-01T00:00:00Z"),
					resource.TestCheckResourceAttr(name, "is_active", "false"),
					resource.TestCheckResourceAttr(name, "dismissible", "true"),
				),
			},
			{Config: created, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
			{
				Config: updated,
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction(name, plancheck.ResourceActionUpdate),
				}},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(name, "content", "tf-acc: maintenance moved to Sunday"),
					resource.TestCheckNoResourceAttr(name, "expires_at"),
					resource.TestCheckResourceAttr(name, "dismissible", "false"),
				),
			},
			{ResourceName: name, ImportState: true, ImportStateVerify: true},
		},
	})
}
