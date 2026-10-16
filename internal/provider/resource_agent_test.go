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

func TestAccAgentResource_RejectsServerOwnedBuiltInConfig(t *testing.T) {
	resource.Test(t, resource.TestCase{PreCheck: func() { testAccPreCheck(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{{Config: testAccAgentResourceConfigBuiltInDualLlmMain("tf-server-owned", 5), ExpectError: regexp.MustCompile("Unsupported block type")}}})
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

// TestAccAgentResource_Runtime round-trips the runtime block: create with the
// full shape, patch a field in place,
// then clear the block entirely (wire null).
func TestAccAgentResource_Runtime(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-agent-runtime")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t); testAccRequireAgentRuntimeEnabled(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccAgentResourceConfigRuntime(rName, 25),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("archestra_agent.runtime", tfjsonpath.New("access_all_tools"), knownvalue.Bool(true)),
					statecheck.ExpectKnownValue("archestra_agent.runtime", tfjsonpath.New("missing_credential_behavior"), knownvalue.StringExact("warn")),
					statecheck.ExpectKnownValue("archestra_agent.runtime", tfjsonpath.New("runtime").AtMapKey("image"), knownvalue.StringExact("registry.example.com/tf-acc/worker:latest")),
					statecheck.ExpectKnownValue("archestra_agent.runtime", tfjsonpath.New("runtime").AtMapKey("backend"), knownvalue.StringExact("kubernetes")),
					statecheck.ExpectKnownValue("archestra_agent.runtime", tfjsonpath.New("runtime").AtMapKey("privileged"), knownvalue.Bool(false)),
					statecheck.ExpectKnownValue("archestra_agent.runtime", tfjsonpath.New("runtime").AtMapKey("resources").AtMapKey("memory_limit"), knownvalue.StringExact("20Gi")),
					statecheck.ExpectKnownValue("archestra_agent.runtime", tfjsonpath.New("runtime").AtMapKey("credentials"), knownvalue.ListSizeExact(1)),
					statecheck.ExpectKnownValue("archestra_agent.runtime", tfjsonpath.New("runtime").AtMapKey("max_cost_usd"), knownvalue.Int64Exact(25)),
					statecheck.ExpectKnownValue("archestra_agent.runtime", tfjsonpath.New("runtime").AtMapKey("ttl_hours"), knownvalue.Null()),
				},
			},
			{
				ResourceName:      "archestra_agent.runtime",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: testAccAgentResourceConfigRuntime(rName, 50),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("archestra_agent.runtime", tfjsonpath.New("runtime").AtMapKey("max_cost_usd"), knownvalue.Int64Exact(50)),
				},
			},
			{
				Config: testAccAgentResourceConfigNoRuntime(rName),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("archestra_agent.runtime", tfjsonpath.New("runtime"), knownvalue.Null()),
				},
			},
		},
	})
}

func testAccAgentResourceConfigRuntime(name string, maxCostUsd int) string {
	return fmt.Sprintf(`
resource "archestra_runtime_credential" "github" {
  key  = "%[1]s-github"
  name = "GitHub token"
}

resource "archestra_agent" "runtime" {
  name          = %[1]q
  system_prompt = "You run delegated repository work."

  access_all_tools            = true
  missing_credential_behavior = "warn"

  runtime = {
    image              = "registry.example.com/tf-acc/worker:latest"
    command            = ["run-worker"]
    inference_protocol = "anthropic"
    steer_mode         = "tmux_keys"
    privileged         = false

    resources = {
      cpu_request    = "2"
      memory_request = "4Gi"
      memory_limit   = "20Gi"
    }

    credentials = [
      {
        key           = "GITHUB_TOKEN"
        scope         = "per_user"
        credential_id = archestra_runtime_credential.github.key
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

func testAccAgentResourceConfigNoRuntime(name string) string {
	return fmt.Sprintf(`
resource "archestra_runtime_credential" "github" {
 key = "%[1]s-github"
 name = "GitHub token"
}

resource "archestra_agent" "runtime" {
  name          = %[1]q
  system_prompt = "You run delegated repository work."

  access_all_tools            = true
  missing_credential_behavior = "warn"
}
`, name)
}
