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
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// TestAccKnowledgeConnectorResource exercises Create / Read / Update
// / Import on the simplest connector type (notion — no required
// config keys). `enabled = false` avoids waiting on the post-create
// sync; the backend still enqueues the sync task but our test
// doesn't assert on sync_status, so the test stays decoupled from
// the upstream Notion API.
func TestAccKnowledgeConnectorResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccKnowledgeConnectorResourceConfig("Initial Notion", `description = "initial"`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("archestra_knowledge_connector.test", "name", "Initial Notion"),
					resource.TestCheckResourceAttr("archestra_knowledge_connector.test", "connector_type", "notion"),
					resource.TestCheckResourceAttr("archestra_knowledge_connector.test", "visibility", "org-wide"),
					resource.TestCheckResourceAttr("archestra_knowledge_connector.test", "enabled", "false"),
					resource.TestCheckResourceAttr("archestra_knowledge_connector.test", "credentials.api_token", "fake-token-for-test"),
					resource.TestCheckResourceAttrSet("archestra_knowledge_connector.test", "id"),
					resource.TestCheckResourceAttrSet("archestra_knowledge_connector.test", "secret_id"),
					resource.TestCheckResourceAttrSet("archestra_knowledge_connector.test", "created_at"),
				),
			},
			{
				ResourceName:      "archestra_knowledge_connector.test",
				ImportState:       true,
				ImportStateVerify: true,
				// credentials.api_token isn't echoed by the backend
				// (write-only); last_sync_at/last_sync_status mutate
				// after the post-create sync runs; updated_at can be
				// fractionally newer on the import refresh.
				ImportStateVerifyIgnore: []string{
					"credentials",
					"last_sync_at",
					"last_sync_status",
					"updated_at",
				},
			},
			{
				Config: testAccKnowledgeConnectorResourceConfig("Renamed Notion", `description = "updated"`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("archestra_knowledge_connector.test", "name", "Renamed Notion"),
					resource.TestCheckResourceAttr("archestra_knowledge_connector.test", "description", "updated"),
					// api_token must survive the Update — write-only
					// field preserved from prior state through every
					// Read/Update cycle.
					resource.TestCheckResourceAttr("archestra_knowledge_connector.test", "credentials.api_token", "fake-token-for-test"),
				),
			},
		},
	})
}

// TestAccKnowledgeConnectorResource_MissingRequiredConfigKey pins the
// per-type required-field plan-time validator. `jira` requires
// `jiraBaseUrl` and `isCloud`; the config below omits both.
func TestAccKnowledgeConnectorResource_MissingRequiredConfigKey(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "archestra_knowledge_connector" "test" {
  name           = "Bad Jira"
  connector_type = "jira"
  config         = jsonencode({ projectKey = "ENG" })
  credentials    = { api_token = "x" }
  enabled        = false
}
`,
				ExpectError: regexp.MustCompile(`connector_type "jira" requires.*jiraBaseUrl`),
			},
		},
	})
}

// TestAccKnowledgeConnectorResource_ReservedTypeKey verifies that
// users can't pass `type` in `config` — the provider injects it from
// `connector_type` and a user-passed `type` would silently shadow
// the discriminator.
func TestAccKnowledgeConnectorResource_ReservedTypeKey(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "archestra_knowledge_connector" "test" {
  name           = "With Type"
  connector_type = "notion"
  config         = jsonencode({ type = "notion" })
  credentials    = { api_token = "x" }
  enabled        = false
}
`,
				ExpectError: regexp.MustCompile(`type. is set automatically`),
			},
		},
	})
}

// TestAccKnowledgeConnectorResource_TeamScopeMissingTeams pins the
// visibility/team_ids cross-field rule on the team-scoped branch.
func TestAccKnowledgeConnectorResource_TeamScopeMissingTeams(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "archestra_knowledge_connector" "test" {
  name           = "Team No Teams"
  connector_type = "notion"
  visibility     = "team-scoped"
  config         = jsonencode({})
  credentials    = { api_token = "x" }
  enabled        = false
}
`,
				ExpectError: regexp.MustCompile(`team_ids must contain at least one team ID`),
			},
		},
	})
}

// TestAccKnowledgeConnectorResource_OrgScopeWithTeams pins the
// inverse — org-wide must not have team_ids.
func TestAccKnowledgeConnectorResource_OrgScopeWithTeams(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "archestra_knowledge_connector" "test" {
  name           = "Org With Teams"
  connector_type = "notion"
  visibility     = "org-wide"
  team_ids       = ["00000000-0000-0000-0000-000000000000"]
  config         = jsonencode({})
  credentials    = { api_token = "x" }
  enabled        = false
}
`,
				ExpectError: regexp.MustCompile(`team_ids must be empty`),
			},
		},
	})
}

// TestAccKnowledgeConnectorResource_TeamScopedHappyPath pins the
// other side of the visibility/team_ids cross-field validator: a
// real archestra_team referenced from a team-scoped connector must
// apply cleanly. Catches a regression where the inverse branch's
// string-literal mismatch ("team" vs "team-scoped") made the happy
// path unreachable.
func TestAccKnowledgeConnectorResource_TeamScopedHappyPath(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "archestra_team" "test" {
  name = "knowledge-connector-team-scoped-test"
}

resource "archestra_knowledge_connector" "test" {
  name           = "Team Scoped Notion"
  connector_type = "notion"
  visibility     = "team-scoped"
  team_ids       = [archestra_team.test.id]
  config         = jsonencode({})
  credentials    = { api_token = "fake-token-for-test" }
  enabled        = false
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("archestra_knowledge_connector.test", "visibility", "team-scoped"),
					resource.TestCheckResourceAttr("archestra_knowledge_connector.test", "team_ids.#", "1"),
					resource.TestCheckResourceAttrPair(
						"archestra_knowledge_connector.test", "team_ids.0",
						"archestra_team.test", "id",
					),
				),
			},
		},
	})
}

// TestAccKnowledgeConnectorResource_ConfigUpdate pins the config-
// only Update path. Catches the regression where `config` AttrSpec
// was Synthetic and MergePatch never detected config-only diffs —
// users editing only `config` produced no wire body and the
// connector kept its old settings silently.
func TestAccKnowledgeConnectorResource_ConfigUpdate(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccKnowledgeConnectorConfigUpdateConfig(`jsonencode({ pageIds = ["page-a"] })`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("archestra_knowledge_connector.test", "config", `{"pageIds":["page-a"]}`),
				),
			},
			{
				Config: testAccKnowledgeConnectorConfigUpdateConfig(`jsonencode({ pageIds = ["page-a", "page-b"] })`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("archestra_knowledge_connector.test", "config", `{"pageIds":["page-a","page-b"]}`),
				),
			},
		},
	})
}

func testAccKnowledgeConnectorConfigUpdateConfig(configExpr string) string {
	return fmt.Sprintf(`
resource "archestra_knowledge_connector" "test" {
  name           = "Config Update Test"
  connector_type = "notion"
  config         = %s
  credentials    = { api_token = "fake-token-for-test" }
  enabled        = false
}
`, configExpr)
}

// TestAccKnowledgeConnectorResource_InvalidConnectorType verifies
// the OneOf validator on connector_type. Catches typos at plan.
func TestAccKnowledgeConnectorResource_InvalidConnectorType(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "archestra_knowledge_connector" "test" {
  name           = "Bad Type"
  connector_type = "bitbucket"
  config         = jsonencode({})
  credentials    = { api_token = "x" }
  enabled        = false
}
`,
				ExpectError: regexp.MustCompile(`value must be one of`),
			},
		},
	})
}

// TestAccKnowledgeConnectorResource_RecoversFromBackendDelete pins
// the Read-404 recovery contract: backend delete out-of-band → next
// refresh drops from state → plan recreates.
func TestAccKnowledgeConnectorResource_RecoversFromBackendDelete(t *testing.T) {
	var capturedID string

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccKnowledgeConnectorResourceConfig("Recovery Connector", `description = "captured"`),
				Check: func(s *terraform.State) error {
					rs, ok := s.RootModule().Resources["archestra_knowledge_connector.test"]
					if !ok {
						return fmt.Errorf("archestra_knowledge_connector.test not in state")
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
					delResp, err := c.DeleteConnectorWithResponse(t.Context(), capturedID)
					if err != nil {
						t.Fatalf("OOB delete: %s", err)
					}
					if delResp.StatusCode() != 200 && delResp.StatusCode() != 204 && delResp.StatusCode() != 404 {
						t.Fatalf("OOB delete: unexpected status %d: %s", delResp.StatusCode(), string(delResp.Body))
					}
				},
				Config: testAccKnowledgeConnectorResourceConfig("Recovery Connector", `description = "captured"`),
				Check: func(s *terraform.State) error {
					rs, ok := s.RootModule().Resources["archestra_knowledge_connector.test"]
					if !ok {
						return fmt.Errorf("archestra_knowledge_connector.test not in state after recovery apply")
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

// KB reconcile coverage is deferred to a combined-branch integration
// test — this worktree doesn't have the `archestra_knowledge_base`
// resource registered (it lives on its own feature branch), so HCL
// can't declare KBs to pass into knowledge_base_ids here. The
// reconcile code path is exercised end-to-end once both branches
// land on main; until then, see the resource source's
// `reconcileKnowledgeBaseIDs` doc comment for the contract.

func testAccKnowledgeConnectorResourceConfig(name, descAttr string) string {
	return fmt.Sprintf(`
resource "archestra_knowledge_connector" "test" {
  name           = %[1]q
  %[2]s
  connector_type = "notion"
  config         = jsonencode({})

  credentials = {
    api_token = "fake-token-for-test"
  }

  enabled = false
}
`, name, descAttr)
}
