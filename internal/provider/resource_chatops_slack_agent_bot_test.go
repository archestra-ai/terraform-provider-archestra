package provider

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// A bot is connected to an agent turned off (enabling it checks the token
// with Slack), changed in place, imported by agent ID and disconnected.
func TestAccChatOpsSlackAgentBotResource(t *testing.T) {
	if os.Getenv("TF_ACC") != "" && !testAccAPIVersionAtLeast(t, chatOpsMinimumAPIVersion) {
		t.Skipf("the ChatOps configuration API needs Archestra %s or later", chatOpsMinimumAPIVersion)
	}
	suffix := strings.ToUpper(acctest.RandString(8))
	config := func(mode string) string {
		return fmt.Sprintf(`
resource "archestra_agent" "test" {
  name = "tf-acc-slack-bot-%[1]s"
}

resource "archestra_chatops_slack_agent_bot" "test" {
  agent_id        = archestra_agent.test.id
  enabled         = false
  app_id          = "ATFACC%[1]s"
  connection_mode = %[2]q
  bot_token       = "xoxb-tf-acc-not-a-real-token"
  signing_secret  = "tf-acc-signing-secret"
  app_level_token = "xapp-tf-acc-not-a-real-token"
}
`, suffix, mode)
	}
	const name = "archestra_chatops_slack_agent_bot.test"
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckSlackAgentBotsGone,
		Steps: []resource.TestStep{
			{
				Config: config("webhook"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair(name, "id", "archestra_agent.test", "id"),
					resource.TestCheckResourceAttr(name, "enabled", "false"),
					resource.TestCheckResourceAttr(name, "connected", "false"),
					resource.TestCheckResourceAttr(name, "app_id", "ATFACC"+suffix),
					resource.TestCheckResourceAttr(name, "connection_mode", "webhook"),
				),
			},
			{Config: config("webhook"), ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
			{
				Config: config("socket"),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction(name, plancheck.ResourceActionUpdate),
				}},
				Check: resource.TestCheckResourceAttr(name, "connection_mode", "socket"),
			},
			{
				ResourceName:            name,
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"bot_token", "signing_secret", "app_level_token"},
			},
		},
	})
}

// Destroy disconnects the bot: the agent is no longer listed with one.
func testAccCheckSlackAgentBotsGone(s *terraform.State) error {
	api, err := client.NewClientWithResponses(os.Getenv("ARCHESTRA_BASE_URL"), client.WithRequestEditorFn(func(_ context.Context, r *http.Request) error {
		r.Header.Set("Authorization", os.Getenv("ARCHESTRA_API_KEY"))
		return nil
	}))
	if err != nil {
		return err
	}
	result, err := api.ListSlackAgentBotsWithResponse(context.Background())
	if err != nil {
		return err
	}
	if result.JSON200 == nil {
		return fmt.Errorf("list Slack agent bots: status %d", result.StatusCode())
	}
	for _, rs := range s.RootModule().Resources {
		if rs.Type != "archestra_chatops_slack_agent_bot" {
			continue
		}
		for _, bot := range result.JSON200.Bots {
			if bot.AgentId == rs.Primary.ID {
				return fmt.Errorf("agent %s still has a Slack bot", rs.Primary.ID)
			}
		}
	}
	return nil
}
