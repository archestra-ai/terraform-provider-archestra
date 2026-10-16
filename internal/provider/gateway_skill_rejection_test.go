package provider

import (
	"fmt"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"regexp"
	"testing"
)

func TestAccGatewayInlineSkillsRejectTemplate(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-unpublishable")
	resource.Test(t, resource.TestCase{PreCheck: func() { testAccPreCheck(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{{Config: fmt.Sprintf(`
resource "archestra_skill" "test" {
 name=%q
 description="Synthetic per-user template"
 content="Hello {{user.name}}"
 templated=true
 files={}
 environment_ids=[]
 labels=[]
}
resource "archestra_mcp_gateway" "test" {
 name=%q
 skills={mode="manual",ids=[archestra_skill.test.id],excluded_ids=[]}
}
`, name, name), ExpectError: regexp.MustCompile("status 422")}}})
}
