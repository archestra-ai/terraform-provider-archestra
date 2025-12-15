package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccToolInvocationPolicyResource(t *testing.T) {
	rName := acctest.RandStringFromCharSet(8, acctest.CharSetAlphaNum)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccToolInvocationPolicyResourceConfig(rName),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"archestra_tool_invocation_policy.test",
						tfjsonpath.New("argument_name"),
						knownvalue.StringExact("path"),
					),
					statecheck.ExpectKnownValue(
						"archestra_tool_invocation_policy.test",
						tfjsonpath.New("operator"),
						knownvalue.StringExact("contains"),
					),
					statecheck.ExpectKnownValue(
						"archestra_tool_invocation_policy.test",
						tfjsonpath.New("value"),
						knownvalue.StringExact("/etc/"),
					),
					statecheck.ExpectKnownValue(
						"archestra_tool_invocation_policy.test",
						tfjsonpath.New("action"),
						knownvalue.StringExact("block_always"),
					),
					statecheck.ExpectKnownValue(
						"archestra_tool_invocation_policy.test",
						tfjsonpath.New("reason"),
						knownvalue.StringExact("Block access to system configuration files"),
					),
				},
			},
			{
				ResourceName:      "archestra_tool_invocation_policy.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: testAccToolInvocationPolicyResourceConfigUpdated(rName),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"archestra_tool_invocation_policy.test",
						tfjsonpath.New("argument_name"),
						knownvalue.StringExact("path"),
					),
					statecheck.ExpectKnownValue(
						"archestra_tool_invocation_policy.test",
						tfjsonpath.New("operator"),
						knownvalue.StringExact("startsWith"),
					),
					statecheck.ExpectKnownValue(
						"archestra_tool_invocation_policy.test",
						tfjsonpath.New("value"),
						knownvalue.StringExact("/var/log/"),
					),
					statecheck.ExpectKnownValue(
						"archestra_tool_invocation_policy.test",
						tfjsonpath.New("action"),
						knownvalue.StringExact("allow_when_context_is_untrusted"),
					),
					statecheck.ExpectKnownValue(
						"archestra_tool_invocation_policy.test",
						tfjsonpath.New("reason"),
						knownvalue.StringExact("Allow log file access in untrusted contexts"),
					),
				},
			},
		},
	})
}

func TestAccToolInvocationPolicyResource_WithoutReason(t *testing.T) {
	rName := acctest.RandStringFromCharSet(8, acctest.CharSetAlphaNum)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccToolInvocationPolicyResourceConfigNoReason(rName),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"archestra_tool_invocation_policy.noreason",
						tfjsonpath.New("argument_name"),
						knownvalue.StringExact("command"),
					),
					statecheck.ExpectKnownValue(
						"archestra_tool_invocation_policy.noreason",
						tfjsonpath.New("action"),
						knownvalue.StringExact("block_always"),
					),
				},
			},
		},
	})
}

func testAccToolInvocationPolicyResourceConfigNoReason(rName string) string {
	return fmt.Sprintf(`
resource "archestra_profile" "noreason" {
  name = "tip-noreason-profile-%[1]s"
}

data "archestra_profile_tool" "noreason" {
  profile_id = archestra_profile.noreason.id
  tool_name  = "archestra__whoami"
}

resource "archestra_tool_invocation_policy" "noreason" {
  profile_tool_id = data.archestra_profile_tool.noreason.id
  argument_name   = "command"
  operator        = "equal"
  value           = "rm -rf"
  action          = "block_always"
}
`, rName)
}

func TestAccToolInvocationPolicyResource_RegexOperator(t *testing.T) {
	rName := acctest.RandStringFromCharSet(8, acctest.CharSetAlphaNum)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccToolInvocationPolicyResourceConfigRegex(rName),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"archestra_tool_invocation_policy.regex",
						tfjsonpath.New("operator"),
						knownvalue.StringExact("regex"),
					),
					statecheck.ExpectKnownValue(
						"archestra_tool_invocation_policy.regex",
						tfjsonpath.New("value"),
						knownvalue.StringExact("^/home/[a-z]+/.ssh/.*"),
					),
				},
			},
		},
	})
}

func testAccToolInvocationPolicyResourceConfig(rName string) string {
	return fmt.Sprintf(`
resource "archestra_profile" "test" {
  name = "tip-test-profile-%[1]s"
}

data "archestra_profile_tool" "test" {
  profile_id = archestra_profile.test.id
  tool_name  = "archestra__whoami"
}

resource "archestra_tool_invocation_policy" "test" {
  profile_tool_id = data.archestra_profile_tool.test.id
  argument_name   = "path"
  operator        = "contains"
  value           = "/etc/"
  action          = "block_always"
  reason          = "Block access to system configuration files"
}
`, rName)
}

func testAccToolInvocationPolicyResourceConfigUpdated(rName string) string {
	return fmt.Sprintf(`
resource "archestra_profile" "test" {
  name = "tip-test-profile-%[1]s"
}

data "archestra_profile_tool" "test" {
  profile_id = archestra_profile.test.id
  tool_name  = "archestra__whoami"
}

resource "archestra_tool_invocation_policy" "test" {
  profile_tool_id = data.archestra_profile_tool.test.id
  argument_name   = "path"
  operator        = "startsWith"
  value           = "/var/log/"
  action          = "allow_when_context_is_untrusted"
  reason          = "Allow log file access in untrusted contexts"
}
`, rName)
}

func testAccToolInvocationPolicyResourceConfigRegex(rName string) string {
	return fmt.Sprintf(`
resource "archestra_profile" "regex" {
  name = "tip-regex-profile-%[1]s"
}

data "archestra_profile_tool" "regex" {
  profile_id = archestra_profile.regex.id
  tool_name  = "archestra__whoami"
}

resource "archestra_tool_invocation_policy" "regex" {
  profile_tool_id = data.archestra_profile_tool.regex.id
  argument_name   = "path"
  operator        = "regex"
  value           = "^/home/[a-z]+/.ssh/.*"
  action          = "block_always"
  reason          = "Block SSH key access using regex pattern"
}
`, rName)
}
