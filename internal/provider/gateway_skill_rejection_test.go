package provider

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccMCPGatewaySkillPublicationRejectsTemplate(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-unpublishable")
	resource.Test(t, resource.TestCase{PreCheck: func() { testAccPreCheck(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{{Config: fmt.Sprintf(`
resource "archestra_mcp_gateway" "test" {name=%q}
resource "archestra_skill" "test" {
 name=%q
 description="Synthetic per-user template"
 content="Hello {{user.name}}"
 templated=true
 files={}
 environment_ids=[]
 labels=[]
}
resource "archestra_mcp_gateway_skill_publication" "test" {
 gateway_id=archestra_mcp_gateway.test.id
 access_all_skills=false
 skill_ids=[archestra_skill.test.id]
}
`, name, name), ExpectError: regexp.MustCompile("status 422")}}})
}
func TestAccMCPGatewaySkillPublicationRejectsInternalAgent(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-not-a-gateway")
	resource.Test(t, resource.TestCase{PreCheck: func() { testAccPreCheck(t); testAccRequireAgentRuntimeEnabled(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{{Config: fmt.Sprintf(`
resource "archestra_agent" "test" {
 name=%q
 system_prompt="Synthetic subtype rejection fixture."
}
resource "archestra_mcp_gateway_skill_publication" "test" {
 gateway_id=archestra_agent.test.id
 access_all_skills=false
 skill_ids=[]
}
`, name), ExpectError: regexp.MustCompile("status 400")}}})
}
