package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

// Each integration is saved turned off (enabling one checks its credentials
// with the chat service), updated, and imported. Secrets are write-only, so
// an import cannot recover them.
func TestAccChatOpsResources(t *testing.T) {
	config := func(appID string) string {
		return `
resource "archestra_chatops_slack" "test" {
  enabled         = false
  connection_mode = "webhook"
  app_id          = "` + appID + `"
  bot_token       = "xoxb-tf-acc-not-a-real-token"
  signing_secret  = "tf-acc-signing-secret"
}

resource "archestra_chatops_ms_teams" "test" {
  enabled    = false
  app_id     = "` + appID + `"
  tenant_id  = "00000000-0000-0000-0000-000000000000"
  app_secret = "tf-acc-app-secret"
}

resource "archestra_chatops_telegram" "test" {
  enabled   = false
  bot_token = "123456:tf-acc-not-a-real-token"
}
`
	}
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config("A0TFACC0001"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("archestra_chatops_slack.test", "id", "slack"),
					resource.TestCheckResourceAttr("archestra_chatops_slack.test", "enabled", "false"),
					resource.TestCheckResourceAttr("archestra_chatops_slack.test", "app_id", "A0TFACC0001"),
					resource.TestCheckResourceAttr("archestra_chatops_slack.test", "connection_mode", "webhook"),
					resource.TestCheckResourceAttr("archestra_chatops_ms_teams.test", "app_id", "A0TFACC0001"),
					resource.TestCheckResourceAttr("archestra_chatops_ms_teams.test", "tenant_id", "00000000-0000-0000-0000-000000000000"),
					resource.TestCheckResourceAttr("archestra_chatops_telegram.test", "enabled", "false"),
				),
			},
			{Config: config("A0TFACC0001"), ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
			{
				Config: config("A0TFACC0002"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("archestra_chatops_slack.test", "app_id", "A0TFACC0002"),
					resource.TestCheckResourceAttr("archestra_chatops_ms_teams.test", "app_id", "A0TFACC0002"),
				),
			},
			{ResourceName: "archestra_chatops_slack.test", ImportState: true, ImportStateVerify: true, ImportStateVerifyIgnore: []string{"bot_token", "signing_secret"}},
			{ResourceName: "archestra_chatops_ms_teams.test", ImportState: true, ImportStateVerify: true, ImportStateVerifyIgnore: []string{"app_secret"}},
			{ResourceName: "archestra_chatops_telegram.test", ImportState: true, ImportStateVerify: true, ImportStateVerifyIgnore: []string{"bot_token"}},
		},
	})
}
