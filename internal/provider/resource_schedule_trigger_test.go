package provider

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"testing"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// TestAccScheduleTriggerResource exercises Create / Read / Update / Import.
func TestAccScheduleTriggerResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccScheduleTriggerResourceConfig("Initial Trigger", "0 9 * * 1-5", "America/New_York", true),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("archestra_schedule_trigger.test", "name", "Initial Trigger"),
					resource.TestCheckResourceAttr("archestra_schedule_trigger.test", "cron_expression", "0 9 * * 1-5"),
					resource.TestCheckResourceAttr("archestra_schedule_trigger.test", "timezone", "America/New_York"),
					resource.TestCheckResourceAttr("archestra_schedule_trigger.test", "enabled", "true"),
					resource.TestCheckResourceAttrSet("archestra_schedule_trigger.test", "id"),
					resource.TestCheckResourceAttrSet("archestra_schedule_trigger.test", "actor_user_id"),
					resource.TestCheckResourceAttrSet("archestra_schedule_trigger.test", "created_at"),
				),
			},
			// Idempotency pin: catches Read-side state-drift bugs.
			{
				Config: testAccScheduleTriggerResourceConfig("Initial Trigger", "0 9 * * 1-5", "America/New_York", true),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
			{
				ResourceName:            "archestra_schedule_trigger.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"last_executed_at"},
			},
			{
				Config: testAccScheduleTriggerResourceConfig("Renamed Trigger", "*/30 * * * *", "UTC", false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("archestra_schedule_trigger.test", "name", "Renamed Trigger"),
					resource.TestCheckResourceAttr("archestra_schedule_trigger.test", "cron_expression", "*/30 * * * *"),
					resource.TestCheckResourceAttr("archestra_schedule_trigger.test", "timezone", "UTC"),
					resource.TestCheckResourceAttr("archestra_schedule_trigger.test", "enabled", "false"),
				),
			},
		},
	})
}

// TestAccScheduleTriggerResource_InvalidCronRejected verifies the
// backend rejects malformed cron expressions at apply.
func TestAccScheduleTriggerResource_InvalidCronRejected(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccScheduleTriggerResourceConfig("Bad Cron", "not-a-cron", "UTC", true),
				ExpectError: regexp.MustCompile(`(?i)cron`),
			},
		},
	})
}

// TestAccScheduleTriggerResource_InvalidTimezoneRejected verifies the
// backend rejects non-IANA timezones at apply.
func TestAccScheduleTriggerResource_InvalidTimezoneRejected(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccScheduleTriggerResourceConfig("Bad TZ", "0 9 * * *", "Mars/Olympus_Mons", true),
				ExpectError: regexp.MustCompile(`(?i)timezone`),
			},
		},
	})
}

// TestAccScheduleTriggerResource_RecoversFromBackendDelete pins the
// Read-404 recovery contract: backend delete out-of-band → next refresh
// drops from state → plan recreates.
func TestAccScheduleTriggerResource_RecoversFromBackendDelete(t *testing.T) {
	var capturedID string

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccScheduleTriggerResourceConfig("Recovery Trigger", "0 9 * * 1-5", "UTC", true),
				Check: func(s *terraform.State) error {
					rs, ok := s.RootModule().Resources["archestra_schedule_trigger.test"]
					if !ok {
						return fmt.Errorf("archestra_schedule_trigger.test not in state")
					}
					capturedID = rs.Primary.Attributes["id"]
					if capturedID == "" {
						return fmt.Errorf("captured id is empty")
					}
					return nil
				},
			},
			{
				PreConfig: func() {
					if capturedID == "" {
						t.Fatal("OOB delete: capturedID empty (step 1 didn't run?)")
					}
					c, err := client.NewClientWithResponses(
						os.Getenv("ARCHESTRA_BASE_URL"),
						client.WithRequestEditorFn(func(_ context.Context, req *http.Request) error {
							req.Header.Set("Authorization", os.Getenv("ARCHESTRA_API_KEY"))
							return nil
						}),
					)
					if err != nil {
						t.Fatalf("OOB delete: build client: %s", err)
					}
					id, err := uuid.Parse(capturedID)
					if err != nil {
						t.Fatalf("OOB delete: parse id: %s", err)
					}
					delResp, err := c.DeleteScheduleTriggerWithResponse(t.Context(), id)
					if err != nil {
						t.Fatalf("OOB delete: %s", err)
					}
					if delResp.StatusCode() != 200 && delResp.StatusCode() != 204 && delResp.StatusCode() != 404 {
						t.Fatalf("OOB delete: unexpected status %d: %s", delResp.StatusCode(), string(delResp.Body))
					}
				},
				Config: testAccScheduleTriggerResourceConfig("Recovery Trigger", "0 9 * * 1-5", "UTC", true),
				Check: func(s *terraform.State) error {
					rs, ok := s.RootModule().Resources["archestra_schedule_trigger.test"]
					if !ok {
						return fmt.Errorf("archestra_schedule_trigger.test not in state after recovery apply")
					}
					if rs.Primary.Attributes["id"] == "" || rs.Primary.Attributes["id"] == capturedID {
						return fmt.Errorf("recovery did not produce a new id (got %q, captured %q)", rs.Primary.Attributes["id"], capturedID)
					}
					return nil
				},
			},
		},
	})
}

func testAccScheduleTriggerResourceConfig(name, cron, tz string, enabled bool) string {
	return fmt.Sprintf(`
resource "archestra_agent" "test" {
  name = "schedule-trigger-test-agent"
}

resource "archestra_schedule_trigger" "test" {
  name             = %[1]q
  agent_id         = archestra_agent.test.id
  message_template = "Generate the report."
  cron_expression  = %[2]q
  timezone         = %[3]q
  enabled          = %[4]t
}
`, name, cron, tz, enabled)
}
