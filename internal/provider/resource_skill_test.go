package provider

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestAccSkillResource(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-skill")
	var id string
	config := func(updated bool) string {
		body, extras, files, env, labels := "Inspect only local synthetic fixtures.", `license="MIT"
 metadata={purpose="fixture"}
 allowed_tools=["Read","Bash"]`, `{"references/README.md"={content="Synthetic reference",encoding="utf8"},"assets/sample.bin"={content="AAECAw==",encoding="base64"}}`, "[archestra_environment.test.id]", `[{key="test",value="owned"}]`
		if updated {
			body = "Hello {{user.name}}; inspect only synthetic fixtures.\n"
			extras = `templated=true
 allowed_tools=[]`
			files = `{"references/README.md"={content="Updated synthetic reference",encoding="utf8"}}`
			env = "[]"
			labels = "[]"
		}
		return fmt.Sprintf(`
resource "archestra_environment" "test" {name=%q}
resource "archestra_skill" "test" {
 name=%q
 description="Synthetic versioned skill fixture"
 content=%q
 icon="book"
 %s
 files=%s
 environment_ids=%s
 labels=%s
}
`, name+"-environment", name, body, extras, files, env, labels)
	}
	initial, updated := config(false), config(true)
	resource.Test(t, resource.TestCase{PreCheck: func() { testAccPreCheck(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{
		{Config: initial, Check: resource.ComposeAggregateTestCheckFunc(resource.TestCheckResourceAttr("archestra_skill.test", "latest_version", "1"), resource.TestCheckResourceAttr("archestra_skill.test", "files.%", "2"), resource.TestCheckResourceAttr("archestra_skill.test", "environment_ids.#", "1"), func(s *terraform.State) error {
			id = s.RootModule().Resources["archestra_skill.test"].Primary.ID
			return nil
		})},
		{ResourceName: "archestra_skill.test", ImportState: true, ImportStateVerify: true},
		{Config: initial, PlanOnly: true},
		{Config: updated, Check: resource.ComposeAggregateTestCheckFunc(resource.TestCheckResourceAttr("archestra_skill.test", "latest_version", "2"), resource.TestCheckResourceAttr("archestra_skill.test", "files.%", "1"), resource.TestCheckResourceAttr("archestra_skill.test", "environment_ids.#", "0"), resource.TestCheckResourceAttr("archestra_skill.test", "labels.#", "0"), resource.TestCheckResourceAttr("archestra_skill.test", "templated", "true"), resource.TestCheckResourceAttr("archestra_skill.test", "allowed_tools.#", "0"), resource.TestCheckNoResourceAttr("archestra_skill.test", "license"))},
		{Config: updated, PlanOnly: true},
		{Config: updated, PlanOnly: true, ExpectNonEmptyPlan: true, PreConfig: func() {
			c, err := client.NewClientWithResponses(os.Getenv("ARCHESTRA_BASE_URL"), client.WithRequestEditorFn(func(_ context.Context, r *http.Request) error {
				r.Header.Set("Authorization", os.Getenv("ARCHESTRA_API_KEY"))
				return nil
			}))
			if err != nil {
				t.Fatal(err)
			}
			current, err := c.GetSkillWithResponse(t.Context(), uuid.MustParse(id))
			if err != nil || current.JSON200 == nil {
				t.Fatal("Unable to read isolated skill", err)
			}
			body := fmt.Sprintf(`{"baseVersion":%d,"content":%q}`, current.JSON200.LatestVersion, "---\nname: "+name+"\ndescription: Synthetic versioned skill fixture\n---\nOut-of-band synthetic drift.\n")
			response, err := c.UpdateSkillWithBodyWithResponse(t.Context(), uuid.MustParse(id), "application/json", strings.NewReader(body))
			if err != nil || response.JSON200 == nil {
				t.Fatal("Unable to write isolated skill drift", err)
			}
		}},
		{Config: updated, Check: resource.TestCheckResourceAttr("archestra_skill.test", "content", "Hello {{user.name}}; inspect only synthetic fixtures.\n")},
		{Config: updated, PlanOnly: true},
		{Config: strings.Replace(updated, `files={"references/README.md"={content="Updated synthetic reference",encoding="utf8"}}`, `files={}`, 1), Check: resource.TestCheckResourceAttr("archestra_skill.test", "files.%", "0")},
	}})
}
