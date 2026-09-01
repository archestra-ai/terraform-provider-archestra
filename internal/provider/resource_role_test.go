package provider

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"testing"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// TestAccRoleResource exercises Create / Read / Update / Import.
// Gated on the EE license being active; without it `POST /api/roles`
// is unavailable and Create returns 404.
func TestAccRoleResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t); testAccRequireEnterprise(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccRoleResourceConfig("Acceptance Test Role", `description = "initial"`, `agent = ["read"]`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("archestra_role.test", "name", "Acceptance Test Role"),
					resource.TestCheckResourceAttr("archestra_role.test", "description", "initial"),
					resource.TestCheckResourceAttr("archestra_role.test", "permission.agent.#", "1"),
					resource.TestCheckResourceAttr("archestra_role.test", "permission.agent.0", "read"),
					resource.TestCheckResourceAttr("archestra_role.test", "predefined", "false"),
					resource.TestCheckResourceAttrSet("archestra_role.test", "id"),
					resource.TestCheckResourceAttrSet("archestra_role.test", "role"),
					resource.TestCheckResourceAttrSet("archestra_role.test", "created_at"),
				),
			},
			// Idempotency pin: catches Read-side state-drift bugs.
			{
				Config: testAccRoleResourceConfig("Acceptance Test Role", `description = "initial"`, `agent = ["read"]`),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
			{
				ResourceName:            "archestra_role.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"updated_at"},
			},
			{
				Config: testAccRoleResourceConfig(
					"Acceptance Test Role",
					`description = "updated"`,
					`agent = ["read", "create", "update"]
    mcpGateway = ["read"]`,
				),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("archestra_role.test", "description", "updated"),
					resource.TestCheckResourceAttr("archestra_role.test", "permission.agent.#", "3"),
					resource.TestCheckResourceAttr("archestra_role.test", "permission.mcpGateway.#", "1"),
				),
			},
		},
	})
}

// TestAccRoleResource_InvalidActionRejected pins the plan-time
// validator on unknown action verbs. Doesn't need EE.
func TestAccRoleResource_InvalidActionRejected(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "archestra_role" "test" {
  name        = "Bad Action"
  permission  = {
    agent = ["bogus-action"]
  }
}
`,
				ExpectError: regexp.MustCompile(`Action "bogus-action" is not one of`),
			},
		},
	})
}

// TestAccRoleResource_RecoversFromBackendDelete pins the Read-404
// recovery contract: backend delete out-of-band → next refresh drops
// from state → plan recreates.
func TestAccRoleResource_RecoversFromBackendDelete(t *testing.T) {
	var capturedID string

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t); testAccRequireEnterprise(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccRoleResourceConfig("Recovers After OOB Delete", `description = "captured"`, `agent = ["read"]`),
				Check: func(s *terraform.State) error {
					rs, ok := s.RootModule().Resources["archestra_role.test"]
					if !ok {
						return fmt.Errorf("archestra_role.test not in state")
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
					delResp, err := c.DeleteRoleWithResponse(t.Context(), capturedID)
					if err != nil {
						t.Fatalf("OOB delete: %s", err)
					}
					if delResp.StatusCode() != 200 && delResp.StatusCode() != 204 && delResp.StatusCode() != 404 {
						t.Fatalf("OOB delete: unexpected status %d: %s", delResp.StatusCode(), string(delResp.Body))
					}
				},
				Config: testAccRoleResourceConfig("Recovers After OOB Delete", `description = "captured"`, `agent = ["read"]`),
				Check: func(s *terraform.State) error {
					rs, ok := s.RootModule().Resources["archestra_role.test"]
					if !ok {
						return fmt.Errorf("archestra_role.test not in state after recovery apply")
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

func testAccRoleResourceConfig(name, descAttr, permBody string) string {
	return fmt.Sprintf(`
resource "archestra_role" "test" {
  name = %[1]q
  %[2]s

  permission = {
    %[3]s
  }
}
`, name, descAttr, permBody)
}
