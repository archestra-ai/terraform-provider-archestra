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

// TestAccTeamVaultFolderResource exercises Create / Read / Update / Import.
// Gated on EE + BYOS — the POST/DELETE routes call `assertByosEnabled()`
// and 403 with "Readonly Vault is not enabled" without it.
func TestAccTeamVaultFolderResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
			testAccRequireEnterprise(t)
			testAccRequireByosEnabled(t)
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccTeamVaultFolderResourceConfig("team-vault-folder-test", "secret/data/test/initial"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("archestra_team_vault_folder.test", "vault_path", "secret/data/test/initial"),
					resource.TestCheckResourceAttrPair(
						"archestra_team_vault_folder.test", "team_id",
						"archestra_team.test", "id",
					),
					resource.TestCheckResourceAttrSet("archestra_team_vault_folder.test", "id"),
					resource.TestCheckResourceAttrSet("archestra_team_vault_folder.test", "created_at"),
				),
			},
			// Idempotency pin: catches Read-side state-drift bugs.
			{
				Config: testAccTeamVaultFolderResourceConfig("team-vault-folder-test", "secret/data/test/initial"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
			{
				ResourceName:      "archestra_team_vault_folder.test",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					rs, ok := s.RootModule().Resources["archestra_team_vault_folder.test"]
					if !ok {
						return "", fmt.Errorf("resource not found")
					}
					return rs.Primary.Attributes["team_id"], nil
				},
				ImportStateVerifyIgnore: []string{"updated_at"},
			},
			{
				Config: testAccTeamVaultFolderResourceConfig("team-vault-folder-test", "secret/data/test/updated"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("archestra_team_vault_folder.test", "vault_path", "secret/data/test/updated"),
				),
			},
		},
	})
}

// TestAccTeamVaultFolderResource_InvalidPath pins the plan-time
// validator that rejects `..`, leading `/`, and trailing `/` — mirrors
// the backend's `Invalid Vault path` 400.
func TestAccTeamVaultFolderResource_InvalidPath(t *testing.T) {
	cases := []struct {
		name    string
		path    string
		errFrag string
	}{
		{"dot-dot", "secret/../leaked", "must not contain '..'"},
		{"leading-slash", "/secret/data/x", "must not start with '/'"},
		{"trailing-slash", "secret/data/x/", "must not end with '/'"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resource.Test(t, resource.TestCase{
				PreCheck:                 func() { testAccPreCheck(t) },
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
				Steps: []resource.TestStep{
					{
						Config: fmt.Sprintf(`
resource "archestra_team_vault_folder" "test" {
  team_id    = "11111111-1111-1111-1111-111111111111"
  vault_path = %q
}
`, tc.path),
						ExpectError: regexp.MustCompile(regexp.QuoteMeta(tc.errFrag)),
					},
				},
			})
		})
	}
}

// TestAccTeamVaultFolderResource_RecoversFromBackendDelete pins the
// Read-404 recovery contract: backend delete out-of-band → next refresh
// drops from state → plan recreates.
func TestAccTeamVaultFolderResource_RecoversFromBackendDelete(t *testing.T) {
	var capturedTeamID string

	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
			testAccRequireEnterprise(t)
			testAccRequireByosEnabled(t)
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccTeamVaultFolderResourceConfig("team-vault-folder-recovery", "secret/data/test/recovery"),
				Check: func(s *terraform.State) error {
					rs, ok := s.RootModule().Resources["archestra_team_vault_folder.test"]
					if !ok {
						return fmt.Errorf("archestra_team_vault_folder.test not in state")
					}
					capturedTeamID = rs.Primary.Attributes["team_id"]
					if capturedTeamID == "" {
						return fmt.Errorf("captured team_id is empty")
					}
					return nil
				},
			},
			{
				PreConfig: func() {
					if capturedTeamID == "" {
						t.Fatal("OOB delete: capturedTeamID empty (step 1 didn't run?)")
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
					delResp, err := c.DeleteTeamVaultFolderWithResponse(t.Context(), capturedTeamID)
					if err != nil {
						t.Fatalf("OOB delete: %s", err)
					}
					if delResp.StatusCode() != 200 && delResp.StatusCode() != 204 && delResp.StatusCode() != 404 {
						t.Fatalf("OOB delete: unexpected status %d: %s", delResp.StatusCode(), string(delResp.Body))
					}
				},
				Config: testAccTeamVaultFolderResourceConfig("team-vault-folder-recovery", "secret/data/test/recovery"),
				Check: func(s *terraform.State) error {
					rs, ok := s.RootModule().Resources["archestra_team_vault_folder.test"]
					if !ok {
						return fmt.Errorf("archestra_team_vault_folder.test not in state after recovery apply")
					}
					if rs.Primary.Attributes["team_id"] != capturedTeamID {
						return fmt.Errorf("team_id changed unexpectedly (got %q, captured %q)", rs.Primary.Attributes["team_id"], capturedTeamID)
					}
					return nil
				},
			},
		},
	})
}

func testAccTeamVaultFolderResourceConfig(teamName, vaultPath string) string {
	return fmt.Sprintf(`
resource "archestra_team" "test" {
  name = %[1]q
}

resource "archestra_team_vault_folder" "test" {
  team_id    = archestra_team.test.id
  vault_path = %[2]q
}
`, teamName, vaultPath)
}
