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
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccTeamResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccTeamResourceConfig("test-team", "Test Description"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"archestra_team.test",
						tfjsonpath.New("name"),
						knownvalue.StringExact("test-team"),
					),
					statecheck.ExpectKnownValue(
						"archestra_team.test",
						tfjsonpath.New("description"),
						knownvalue.StringExact("Test Description"),
					),
				},
			},
			// ImportState testing
			{
				ResourceName:      "archestra_team.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			// Update and Read testing
			{
				Config: testAccTeamResourceConfig("test-team-updated", "Updated Description"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"archestra_team.test",
						tfjsonpath.New("name"),
						knownvalue.StringExact("test-team-updated"),
					),
					statecheck.ExpectKnownValue(
						"archestra_team.test",
						tfjsonpath.New("description"),
						knownvalue.StringExact("Updated Description"),
					),
				},
			},
			// Delete testing automatically occurs in TestCase
		},
	})
}

func TestAccTeamResourceWithToonCompression(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Step 1: Apply org-level scope = "team" first (pre-condition
			// for the team-level TOON flag to be honored — the provider's
			// ModifyPlan pre-flight checks the live backend value).
			{
				Config: testAccTeamResourceConfigOrgTeamScope("test-team-toon", "Team for toon compression test"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"archestra_team.test",
						tfjsonpath.New("name"),
						knownvalue.StringExact("test-team-toon"),
					),
				},
			},
			// Step 2: Now that org scope is "team", enabling the team-level
			// TOON flag is allowed.
			{
				Config: testAccTeamResourceConfigWithToon("test-team-toon", "Team for toon compression test", true),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"archestra_team.test",
						tfjsonpath.New("name"),
						knownvalue.StringExact("test-team-toon"),
					),
					statecheck.ExpectKnownValue(
						"archestra_team.test",
						tfjsonpath.New("convert_tool_results_to_toon"),
						knownvalue.Bool(true),
					),
				},
			},
		},
	})
}

// TestAccTeamResource_CreateWithToonTrue pins the create-time TOON
// path. Backend's `CreateTeamBodySchema` is missing
// `convertToolResultsToToon` (only `UpdateTeamBodySchema` has it), so
// the field used to be silently stripped from the Create body and the
// response echoed `false` — surfacing as "Provider produced
// inconsistent result after apply". Provider workaround: post-Create
// follow-up Update. This test covers Create-with-TOON-true in a single
// step; the pre-existing TestAccTeamResourceWithToonCompression only
// exercised Update.
func TestAccTeamResource_CreateWithToonTrue(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccTeamResourceConfigWithToon("test-team-create-toon", "Create-time TOON regression pin", true),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"archestra_team.test",
						tfjsonpath.New("convert_tool_results_to_toon"),
						knownvalue.Bool(true),
					),
				},
			},
		},
	})
}

func testAccTeamResourceConfigOrgTeamScope(name, description string) string {
	return fmt.Sprintf(`
resource "archestra_organization_settings" "test" {
  compression_scope = "team"
}

resource "archestra_team" "test" {
  name        = %[1]q
  description = %[2]q
  depends_on  = [archestra_organization_settings.test]
}
`, name, description)
}

func testAccTeamResourceConfigWithToon(name, description string, convertToToon bool) string {
	// Setting team-level convert_tool_results_to_toon requires the org to
	// be in `compression_scope = "team"`, otherwise the backend silently
	// ignores the team flag and the provider's pre-flight blocks the apply.
	return fmt.Sprintf(`
resource "archestra_organization_settings" "test" {
  compression_scope = "team"
}

resource "archestra_team" "test" {
  name                         = %[1]q
  description                  = %[2]q
  convert_tool_results_to_toon = %[3]t
  depends_on                   = [archestra_organization_settings.test]
}
`, name, description, convertToToon)
}

func testAccTeamResourceConfig(name, description string) string {
	return fmt.Sprintf(`
resource "archestra_team" "test" {
  name        = %[1]q
  description = %[2]q
}
`, name, description)
}

// TestAccTeamResource_RecoversFromBackendDelete pins the contract that
// out-of-band deletion of a managed resource doesn't break the next apply.
// Read sees 404 → drops state → plan shows "1 to add" → apply creates fresh.
// Same pattern (`IsNotFound → RemoveResource`) is used on every Read and
// Update; this test on `archestra_team` is the representative regression
// pin so a future revert anywhere in the sweep gets caught.
func TestAccTeamResource_RecoversFromBackendDelete(t *testing.T) {
	var capturedID string

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccTeamResourceConfig("tf-acc-team-oob-delete", "Captures id for OOB delete"),
				Check: func(s *terraform.State) error {
					rs, ok := s.RootModule().Resources["archestra_team.test"]
					if !ok {
						return fmt.Errorf("archestra_team.test not in state")
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
					delResp, err := c.DeleteTeamWithResponse(t.Context(), capturedID)
					if err != nil {
						t.Fatalf("OOB delete: %s", err)
					}
					if delResp.StatusCode() != 200 && delResp.StatusCode() != 204 && delResp.StatusCode() != 404 {
						t.Fatalf("OOB delete: unexpected status %d: %s", delResp.StatusCode(), string(delResp.Body))
					}
				},
				Config: testAccTeamResourceConfig("tf-acc-team-oob-delete", "Captures id for OOB delete"),
				Check: func(s *terraform.State) error {
					rs, ok := s.RootModule().Resources["archestra_team.test"]
					if !ok {
						return fmt.Errorf("archestra_team.test not in state after recovery apply")
					}
					if rs.Primary.Attributes["id"] == "" || rs.Primary.Attributes["id"] == capturedID {
						return fmt.Errorf("id %q is empty or unchanged from %q — should have been recreated", rs.Primary.Attributes["id"], capturedID)
					}
					return nil
				},
			},
		},
	})
}

// TestAccTeamResource_ToonPreflightFailsWithoutTeamScope pins the
// ModifyPlan pre-flight on `convert_tool_results_to_toon`. The backend
// silently ignores the team-level flag when
// `archestra_organization_settings.compression_scope != "team"`, which
// previously surfaced as Terraform's "Provider produced inconsistent
// result after apply" mid-apply, leaving partial state. The pre-flight
// catches it at plan-time with an actionable error before any resource
// is created.
//
// Step 1 explicitly applies `compression_scope = "organization"`
// (resetting any leftover singleton state from prior tests in the
// run). Step 2 then declares a team with TOON=true; the pre-flight
// reads the now-known scope and refuses the plan.
func TestAccTeamResource_ToonPreflightFailsWithoutTeamScope(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "archestra_organization_settings" "test" {
  compression_scope = "organization"
}
`,
			},
			{
				Config: `
resource "archestra_organization_settings" "test" {
  compression_scope = "organization"
}

resource "archestra_team" "preflight" {
  name                         = "tf-acc-team-toon-preflight"
  description                  = "Should fail at plan because org scope is not 'team'"
  convert_tool_results_to_toon = true
  depends_on                   = [archestra_organization_settings.test]
}
`,
				ExpectError: regexp.MustCompile(`compression_scope`),
			},
		},
	})
}
