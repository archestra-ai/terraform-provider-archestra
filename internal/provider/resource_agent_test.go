package provider

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccAgentResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccAgentResourceConfig("tf-acc-agent"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("archestra_agent.test", tfjsonpath.New("name"), knownvalue.StringExact("tf-acc-agent")),
					statecheck.ExpectKnownValue("archestra_agent.test", tfjsonpath.New("scope"), knownvalue.StringExact("org")),
					statecheck.ExpectKnownValue("archestra_agent.test", tfjsonpath.New("teams"), knownvalue.ListSizeExact(0)),
					statecheck.ExpectKnownValue("archestra_agent.test", tfjsonpath.New("system_prompt"), knownvalue.StringExact("You are a helpful agent.")),
				},
			},
			{
				ResourceName:      "archestra_agent.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: testAccAgentResourceConfigUpdated("tf-acc-agent-renamed"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("archestra_agent.test", tfjsonpath.New("name"), knownvalue.StringExact("tf-acc-agent-renamed")),
					statecheck.ExpectKnownValue("archestra_agent.test", tfjsonpath.New("description"), knownvalue.StringExact("updated")),
				},
			},
		},
	})
}

func TestAccAgentResource_WithEmailConfig(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccAgentResourceConfigWithEmail("tf-acc-agent-email", true, "internal"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("archestra_agent.email_test", tfjsonpath.New("incoming_email_enabled"), knownvalue.Bool(true)),
					statecheck.ExpectKnownValue("archestra_agent.email_test", tfjsonpath.New("incoming_email_security_mode"), knownvalue.StringExact("internal")),
					statecheck.ExpectKnownValue("archestra_agent.email_test", tfjsonpath.New("incoming_email_allowed_domain"), knownvalue.StringExact("example.com")),
				},
			},
		},
	})
}

func TestAccAgentResource_WithBuiltInAgentConfig(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-agent-builtin")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccAgentResourceConfigBuiltInDualLlmMain(rName, 5),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("archestra_agent.builtin", tfjsonpath.New("built_in_agent_config").AtMapKey("name"), knownvalue.StringExact("dual-llm-main-agent")),
					statecheck.ExpectKnownValue("archestra_agent.builtin", tfjsonpath.New("built_in_agent_config").AtMapKey("max_rounds"), knownvalue.Int64Exact(5)),
				},
			},
			{
				Config: testAccAgentResourceConfigBuiltInDualLlmMain(rName, 8),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("archestra_agent.builtin", tfjsonpath.New("built_in_agent_config").AtMapKey("max_rounds"), knownvalue.Int64Exact(8)),
				},
			},
			// Clear built_in_agent_config so the destroy phase can delete the
			// agent. The backend rejects deletion while the agent is marked
			// built-in, so the resource needs to revert to a regular agent
			// first.
			{
				Config: testAccAgentResourceConfigPlain(rName),
			},
		},
	})
}

func testAccAgentResourceConfigPlain(name string) string {
	return fmt.Sprintf(`
resource "archestra_agent" "builtin" {
  name = %q
}
`, name)
}

func testAccAgentResourceConfig(name string) string {
	return fmt.Sprintf(`
resource "archestra_agent" "test" {
  name          = %q
  system_prompt = "You are a helpful agent."
  labels = [
    { key = "team",        value = "engineering" },
    { key = "environment", value = "production"  }
  ]
}
`, name)
}

func testAccAgentResourceConfigUpdated(name string) string {
	return fmt.Sprintf(`
resource "archestra_agent" "test" {
  name          = %q
  description   = "updated"
  system_prompt = "You are a helpful agent."
  labels = [
    { key = "team",        value = "engineering" },
    { key = "environment", value = "staging" }
  ]
}
`, name)
}

func testAccAgentResourceConfigWithEmail(name string, emailEnabled bool, securityMode string) string {
	return fmt.Sprintf(`
resource "archestra_agent" "email_test" {
  name                          = %q
  system_prompt                 = "You handle inbound mail."
  incoming_email_enabled        = %t
  incoming_email_security_mode  = %q
  incoming_email_allowed_domain = "example.com"
}
`, name, emailEnabled, securityMode)
}

func testAccAgentResourceConfigBuiltInDualLlmMain(name string, maxRounds int) string {
	return fmt.Sprintf(`
resource "archestra_agent" "builtin" {
  name = %q

  built_in_agent_config {
    name       = "dual-llm-main-agent"
    max_rounds = %d
  }
}
`, name, maxRounds)
}

// TestAccAgentResource_TeamScopeMissingTeams pins the cross-field
// validator from ValidateConfig: scope = "team" without a teams list
// must error at plan time, not 400 at apply.
func TestAccAgentResource_TeamScopeMissingTeams(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "archestra_agent" "test" {
  name          = "tf-acc-agent-team-scope-missing"
  system_prompt = "You are a helpful agent."
  scope         = "team"
}
`,
				ExpectError: regexp.MustCompile(`teams must be set`),
			},
		},
	})
}

// TestAccAgentResource_TeamsRemoveCycle pins RemoveOnConfigNullList on `teams`.
// Note the second step also flips scope back to "org" because the cross-field
// validator requires teams when scope = "team"; the modifier still has to
// emit null for teams independently of that change.
func TestAccAgentResource_TeamsRemoveCycle(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccAgentResourceConfigTeamScoped("tf-acc-agent-team-cycle"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("archestra_agent.cycle", tfjsonpath.New("scope"), knownvalue.StringExact("team")),
					statecheck.ExpectKnownValue("archestra_agent.cycle", tfjsonpath.New("teams"), knownvalue.ListSizeExact(1)),
				},
			},
			{
				Config: testAccAgentResourceConfigOrgScoped("tf-acc-agent-team-cycle"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("archestra_agent.cycle", tfjsonpath.New("scope"), knownvalue.StringExact("org")),
					// HCL omits `teams`. EmptyListOnConfigNull() turns that
					// into an explicit empty list on the wire (the backend
					// rejects null but accepts []). The previous bug left
					// the prior team in state (size 1), so size 0 catches
					// the regression.
					statecheck.ExpectKnownValue("archestra_agent.cycle", tfjsonpath.New("teams"), knownvalue.ListSizeExact(0)),
				},
			},
		},
	})
}

func testAccAgentResourceConfigTeamScoped(name string) string {
	return fmt.Sprintf(`
resource "archestra_team" "cycle" {
  name        = %[1]q
  description = "remove-cycle test team"
}

resource "archestra_agent" "cycle" {
  name          = %[1]q
  system_prompt = "You are a helpful agent."
  scope         = "team"
  teams         = [archestra_team.cycle.id]
}
`, name)
}

func testAccAgentResourceConfigOrgScoped(name string) string {
	return fmt.Sprintf(`
resource "archestra_team" "cycle" {
  name        = %[1]q
  description = "remove-cycle test team"
}

resource "archestra_agent" "cycle" {
  name          = %[1]q
  system_prompt = "You are a helpful agent."
}
`, name)
}

// TestAccAgentResource_InternalEmailMissingDomain pins the other arm
// of the cross-field validator: incoming_email_security_mode =
// "internal" without incoming_email_allowed_domain must error at
// plan time.
func TestAccAgentResource_InternalEmailMissingDomain(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "archestra_agent" "test" {
  name                          = "tf-acc-agent-internal-email-missing"
  system_prompt                 = "You are a helpful agent."
  incoming_email_enabled        = true
  incoming_email_security_mode  = "internal"
}
`,
				ExpectError: regexp.MustCompile(`incoming_email_allowed_domain must be set`),
			},
		},
	})
}

// TestAccAgentResource_BackgroundExecution round-trips the background
// execution block: create with the full shape, patch a field in place,
// then clear the block entirely (wire null).
func TestAccAgentResource_BackgroundExecution(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-agent-bgexec")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t); testAccRequireRunnerBackendEnabled(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccAgentResourceConfigBackgroundExecution(rName, 25),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("archestra_agent.bgexec", tfjsonpath.New("access_all_tools"), knownvalue.Bool(true)),
					statecheck.ExpectKnownValue("archestra_agent.bgexec", tfjsonpath.New("missing_credential_behavior"), knownvalue.StringExact("warn")),
					statecheck.ExpectKnownValue("archestra_agent.bgexec", tfjsonpath.New("background_execution").AtMapKey("image"), knownvalue.StringExact("registry.example.com/tf-acc/worker:latest")),
					statecheck.ExpectKnownValue("archestra_agent.bgexec", tfjsonpath.New("background_execution").AtMapKey("backend"), knownvalue.StringExact("kubernetes")),
					statecheck.ExpectKnownValue("archestra_agent.bgexec", tfjsonpath.New("background_execution").AtMapKey("privileged"), knownvalue.Bool(true)),
					statecheck.ExpectKnownValue("archestra_agent.bgexec", tfjsonpath.New("background_execution").AtMapKey("resources").AtMapKey("memory_limit"), knownvalue.StringExact("20Gi")),
					statecheck.ExpectKnownValue("archestra_agent.bgexec", tfjsonpath.New("background_execution").AtMapKey("credentials"), knownvalue.ListSizeExact(1)),
					statecheck.ExpectKnownValue("archestra_agent.bgexec", tfjsonpath.New("background_execution").AtMapKey("max_cost_usd"), knownvalue.Int64Exact(25)),
					statecheck.ExpectKnownValue("archestra_agent.bgexec", tfjsonpath.New("background_execution").AtMapKey("ttl_hours"), knownvalue.Null()),
				},
			},
			{
				ResourceName:      "archestra_agent.bgexec",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: testAccAgentResourceConfigBackgroundExecution(rName, 50),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("archestra_agent.bgexec", tfjsonpath.New("background_execution").AtMapKey("max_cost_usd"), knownvalue.Int64Exact(50)),
				},
			},
			{
				Config: testAccAgentResourceConfigNoBackgroundExecution(rName),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("archestra_agent.bgexec", tfjsonpath.New("background_execution"), knownvalue.Null()),
				},
			},
		},
	})
}

func testAccAgentResourceConfigBackgroundExecution(name string, maxCostUsd int) string {
	return fmt.Sprintf(`
resource "archestra_execution_credential" "github" {
  key  = "%[1]s-github"
  name = "GitHub token"
}

resource "archestra_agent" "bgexec" {
  name          = %[1]q
  system_prompt = "You run delegated repository work."

  access_all_tools            = true
  missing_credential_behavior = "warn"

  background_execution = {
    image              = "registry.example.com/tf-acc/worker:latest"
    command            = ["run-worker"]
    inference_protocol = "anthropic"
    steer_mode         = "tmux_keys"
    privileged         = true

    resources = {
      cpu_request    = "2"
      memory_request = "4Gi"
      memory_limit   = "20Gi"
    }

    credentials = [
      {
        key           = "GITHUB_TOKEN"
        scope         = "per_user"
        credential_id = archestra_execution_credential.github.key
        label         = "GitHub token"
        description   = "A personal token that can clone the repository."
        required      = true
      },
    ]

    max_cost_usd = %[2]d
  }
}
`, name, maxCostUsd)
}

func testAccAgentResourceConfigNoBackgroundExecution(name string) string {
	return fmt.Sprintf(`
resource "archestra_agent" "bgexec" {
  name          = %[1]q
  system_prompt = "You run delegated repository work."

  access_all_tools            = true
  missing_credential_behavior = "warn"
}
`, name)
}
