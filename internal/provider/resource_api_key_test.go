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

// TestAccApiKeyResource exercises Create / Read / Import. There is
// no Update endpoint on the backend; every input field is
// RequiresReplace, so this test does not include an in-place update
// step.
func TestAccApiKeyResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccApiKeyResourceConfig("acceptance-test-key", 604800),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("archestra_api_key.test", "name", "acceptance-test-key"),
					resource.TestCheckResourceAttr("archestra_api_key.test", "expires_in_seconds", "604800"),
					resource.TestCheckResourceAttrSet("archestra_api_key.test", "id"),
					resource.TestCheckResourceAttrSet("archestra_api_key.test", "key"),
					resource.TestCheckResourceAttrSet("archestra_api_key.test", "prefix"),
					resource.TestCheckResourceAttrSet("archestra_api_key.test", "expires_at"),
					resource.TestCheckResourceAttrSet("archestra_api_key.test", "created_at"),
					// The full token is the user-facing payload. Assert
					// the canonical `arch_` prefix so a regression that
					// returns a non-platform token gets caught.
					resource.TestMatchResourceAttr("archestra_api_key.test", "key", regexp.MustCompile(`^arch_`)),
				),
			},
			// Idempotency pin: catches Read-side state-drift bugs.
			{
				Config: testAccApiKeyResourceConfig("acceptance-test-key", 604800),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
			{
				ResourceName:      "archestra_api_key.test",
				ImportState:       true,
				ImportStateVerify: true,
				// `key` is write-only (backend never echoes); `last_request`
				// mutates after the first authed call.
				ImportStateVerifyIgnore: []string{"key", "expires_in_seconds", "last_request"},
			},
		},
	})
}

// TestAccApiKeyResource_NonExpiring verifies that omitting
// `expires_in_seconds` yields a non-expiring key (expires_at null).
func TestAccApiKeyResource_NonExpiring(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "archestra_api_key" "test" {
  name = "non-expiring-test"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("archestra_api_key.test", "name", "non-expiring-test"),
					resource.TestCheckNoResourceAttr("archestra_api_key.test", "expires_at"),
					resource.TestCheckResourceAttrSet("archestra_api_key.test", "key"),
				),
			},
		},
	})
}

// TestAccApiKeyResource_NameOmittedNoDrift pins the omit-name path:
// Better Auth stores null when `name` is omitted, and the API
// echoes null back. Re-plan with the same config must be empty —
// catches a perpetual-drift regression where Optional `name` with
// the wrong projection would cycle between null and the response
// shape on every plan.
func TestAccApiKeyResource_NameOmittedNoDrift(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "archestra_api_key" "test" {
  expires_in_seconds = 86400
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckNoResourceAttr("archestra_api_key.test", "name"),
					resource.TestCheckResourceAttrSet("archestra_api_key.test", "key"),
				),
			},
			{
				Config: `
resource "archestra_api_key" "test" {
  expires_in_seconds = 86400
}
`,
				PlanOnly:           true,
				ExpectNonEmptyPlan: false,
			},
		},
	})
}

// TestAccApiKeyResource_InvalidExpiry pins the int64 validator on
// `expires_in_seconds` — values < 1 are rejected at plan time.
func TestAccApiKeyResource_InvalidExpiry(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "archestra_api_key" "test" {
  name               = "invalid-expiry"
  expires_in_seconds = 3600
}
`,
				ExpectError: regexp.MustCompile(`Attribute expires_in_seconds value must be at least 86400`),
			},
		},
	})
}

// TestAccApiKeyResource_RecoversFromBackendDelete pins the Read-404
// recovery contract: backend delete out-of-band → next refresh drops
// from state → plan recreates.
func TestAccApiKeyResource_RecoversFromBackendDelete(t *testing.T) {
	var capturedID string

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccApiKeyResourceConfig("recovers-after-oob-delete", 604800),
				Check: func(s *terraform.State) error {
					rs, ok := s.RootModule().Resources["archestra_api_key.test"]
					if !ok {
						return fmt.Errorf("archestra_api_key.test not in state")
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
					delResp, err := c.DeleteApiKeyWithResponse(t.Context(), capturedID)
					if err != nil {
						t.Fatalf("OOB delete: %s", err)
					}
					if delResp.StatusCode() != 200 && delResp.StatusCode() != 204 && delResp.StatusCode() != 404 {
						t.Fatalf("OOB delete: unexpected status %d: %s", delResp.StatusCode(), string(delResp.Body))
					}
				},
				Config: testAccApiKeyResourceConfig("recovers-after-oob-delete", 604800),
				Check: func(s *terraform.State) error {
					rs, ok := s.RootModule().Resources["archestra_api_key.test"]
					if !ok {
						return fmt.Errorf("archestra_api_key.test not in state after recovery apply")
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

// testAccApiKeyResourceConfig builds an api-key with a 7-day expiry.
// Bespoke-expiry variants belong in their own per-test helpers
// (TestAccApiKeyResource_NonExpiring, TestAccApiKeyResource_InvalidExpiry)
// because expiry is the load-bearing axis of those tests.
func testAccApiKeyResourceConfig(name string, _ int64) string {
	return fmt.Sprintf(`
resource "archestra_api_key" "test" {
  name               = %[1]q
  expires_in_seconds = 604800
}
`, name)
}
