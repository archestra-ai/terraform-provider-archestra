package provider

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// TestAccVirtualApiKeyResource exercises Create / Read / Update / Import
// against a vault-backed parent LLM provider api key (BYOS gate).
func TestAccVirtualApiKeyResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t); testAccRequireByosEnabled(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccVirtualApiKeyResourceConfig("Initial Virtual Key", "org"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("archestra_virtual_api_key.test", "name", "Initial Virtual Key"),
					resource.TestCheckResourceAttr("archestra_virtual_api_key.test", "scope", "org"),
					resource.TestCheckResourceAttrSet("archestra_virtual_api_key.test", "id"),
					resource.TestCheckResourceAttrSet("archestra_virtual_api_key.test", "value"),
					resource.TestCheckResourceAttrSet("archestra_virtual_api_key.test", "secret_id"),
					resource.TestCheckResourceAttrSet("archestra_virtual_api_key.test", "created_at"),
				),
			},
			{
				ResourceName:      "archestra_virtual_api_key.test",
				ImportState:       true,
				ImportStateVerify: true,
				// value is one-shot at Create and never echoed by Read,
				// so import populates a null `value` instead of the
				// original token. Also drop author_name (display string
				// the platform may localise) and last_used_at (mutates
				// after the resource has been used).
				ImportStateVerifyIgnore: []string{"value", "author_name", "last_used_at"},
				ImportStateIdFunc:       testAccVirtualApiKeyImportStateIdFunc("archestra_virtual_api_key.test"),
			},
			{
				Config: testAccVirtualApiKeyResourceConfig("Renamed Virtual Key", "org"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("archestra_virtual_api_key.test", "name", "Renamed Virtual Key"),
					resource.TestCheckResourceAttr("archestra_virtual_api_key.test", "scope", "org"),
				),
			},
		},
	})
}

// TestAccVirtualApiKeyResource_TeamScope covers scope = "team" with the
// teams list resolved from a real archestra_team resource.
func TestAccVirtualApiKeyResource_TeamScope(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t); testAccRequireByosEnabled(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccVirtualApiKeyResourceConfigTeamScope(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("archestra_virtual_api_key.team_scoped", "scope", "team"),
					resource.TestCheckResourceAttr("archestra_virtual_api_key.team_scoped", "teams.#", "1"),
					resource.TestCheckResourceAttrPair(
						"archestra_virtual_api_key.team_scoped", "teams.0",
						"archestra_team.test", "id",
					),
				),
			},
		},
	})
}

// TestAccVirtualApiKeyResource_TeamScopeMissingTeams verifies the
// ValidateConfig plan-time error when scope = "team" but no teams set.
func TestAccVirtualApiKeyResource_TeamScopeMissingTeams(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "archestra_virtual_api_key" "test" {
  llm_provider_api_key_id = "11111111-1111-1111-1111-111111111111"
  name                    = "missing teams"
  scope                   = "team"
}
`,
				ExpectError: regexp.MustCompile(`teams must contain at least one team ID`),
			},
		},
	})
}

// TestAccVirtualApiKeyResource_OrgScopeWithTeams verifies the
// ValidateConfig plan-time error when scope != "team" but teams is set.
func TestAccVirtualApiKeyResource_OrgScopeWithTeams(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "archestra_virtual_api_key" "test" {
  llm_provider_api_key_id = "11111111-1111-1111-1111-111111111111"
  name                    = "org with teams"
  scope                   = "org"
  teams                   = ["22222222-2222-2222-2222-222222222222"]
}
`,
				ExpectError: regexp.MustCompile(`teams must be empty when scope`),
			},
		},
	})
}

// TestAccVirtualApiKeyResource_InvalidExpiresAt verifies the RFC 3339
// validator catches malformed timestamps at plan time.
func TestAccVirtualApiKeyResource_InvalidExpiresAt(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "archestra_virtual_api_key" "test" {
  llm_provider_api_key_id = "11111111-1111-1111-1111-111111111111"
  name                    = "bad timestamp"
  expires_at              = "not-a-real-timestamp"
}
`,
				ExpectError: regexp.MustCompile(`Invalid timestamp`),
			},
		},
	})
}

// TestAccVirtualApiKeyResource_BareUUIDImportRejected pins the composite
// import contract: a bare UUID gets a "Invalid Import ID" error rather
// than a partial import that breaks on the next refresh.
func TestAccVirtualApiKeyResource_BareUUIDImportRejected(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t); testAccRequireByosEnabled(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccVirtualApiKeyResourceConfig("Imported Virtual Key", "org"),
			},
			{
				ResourceName:  "archestra_virtual_api_key.test",
				ImportState:   true,
				ImportStateId: "11111111-1111-1111-1111-111111111111",
				ExpectError:   regexp.MustCompile(`Invalid Import ID`),
			},
		},
	})
}

func testAccVirtualApiKeyResourceConfig(name, scope string) string {
	return fmt.Sprintf(`
resource "archestra_llm_provider_api_key" "parent" {
  name              = "Virtual Key Parent"
  llm_provider      = "ollama"
  vault_secret_path = "secret/data/test/ollama"
  vault_secret_key  = "api_key"
}

resource "archestra_virtual_api_key" "test" {
  llm_provider_api_key_id = archestra_llm_provider_api_key.parent.id
  name                    = %[1]q
  scope                   = %[2]q
}
`, name, scope)
}

func testAccVirtualApiKeyResourceConfigTeamScope() string {
	return `
resource "archestra_llm_provider_api_key" "parent" {
  name              = "Virtual Key Team Parent"
  llm_provider      = "ollama"
  vault_secret_path = "secret/data/test/ollama"
  vault_secret_key  = "api_key"
}

resource "archestra_team" "test" {
  name = "virtual-key-team-test"
}

resource "archestra_virtual_api_key" "team_scoped" {
  llm_provider_api_key_id = archestra_llm_provider_api_key.parent.id
  name                    = "Team Scoped Virtual Key"
  scope                   = "team"
  teams                   = [archestra_team.test.id]
}
`
}

// testAccVirtualApiKeyImportStateIdFunc returns a composite
// `<parent_id>:<id>` for the import test step.
func testAccVirtualApiKeyImportStateIdFunc(resourceName string) resource.ImportStateIdFunc {
	return func(s *terraform.State) (string, error) {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return "", fmt.Errorf("resource %s not found", resourceName)
		}
		parent := rs.Primary.Attributes["llm_provider_api_key_id"]
		id := rs.Primary.Attributes["id"]
		if parent == "" || id == "" {
			return "", fmt.Errorf("resource %s has empty parent (%q) or id (%q)", resourceName, parent, id)
		}
		return parent + ":" + id, nil
	}
}
