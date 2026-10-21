package provider

import (
	"context"
	"fmt"
	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"net/http"
	"os"
	"strings"
	"testing"
)

func TestAccRuntimeCredentialResource_GitlabHost(t *testing.T) {
	key := acctest.RandomWithPrefix("tf-gitlab-definition")
	config := func(kind, host string) string {
		return fmt.Sprintf(`
resource "archestra_credential" "test" {
 key = %q
 name = "Synthetic GitLab definition"
 kind = %q
 %s
}
`, key, kind, host)
	}
	secret := config("secret", "")
	initial := config("gitlab_token", `gitlab_url="https://gitlab.example.invalid"`)
	updated := config("gitlab_token", `gitlab_url="https://updated.gitlab.example.invalid"`)
	cleared := config("gitlab_token", "")
	resource.Test(t, resource.TestCase{PreCheck: func() { testAccPreCheck(t); testAccRequireAgentRuntimeEnabled(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{
		{Config: secret},
		{Config: initial, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction("archestra_credential.test", plancheck.ResourceActionDestroyBeforeCreate)}}, Check: resource.TestCheckResourceAttr("archestra_credential.test", "gitlab_url", "https://gitlab.example.invalid")},
		{ResourceName: "archestra_credential.test", ImportState: true, ImportStateVerify: true},
		{Config: initial, PlanOnly: true},
		{Config: updated, Check: resource.TestCheckResourceAttr("archestra_credential.test", "gitlab_url", "https://updated.gitlab.example.invalid")},
		{Config: updated, PlanOnly: true, ExpectNonEmptyPlan: true, PreConfig: func() {
			c, err := client.NewClientWithResponses(os.Getenv("ARCHESTRA_BASE_URL"), client.WithRequestEditorFn(func(_ context.Context, r *http.Request) error {
				r.Header.Set("Authorization", os.Getenv("ARCHESTRA_API_KEY"))
				return nil
			}))
			if err != nil {
				t.Fatal(err)
			}
			response, err := c.UpdateRuntimeCredentialWithBodyWithResponse(t.Context(), key, "application/json", strings.NewReader(`{"gitlabUrl":"https://drift.gitlab.example.invalid"}`))
			if err != nil || response.JSON200 == nil {
				t.Fatal("Unable to create isolated credential-host drift", err)
			}
		}},
		{Config: updated, Check: resource.TestCheckResourceAttr("archestra_credential.test", "gitlab_url", "https://updated.gitlab.example.invalid")},
		{Config: cleared, Check: resource.TestCheckNoResourceAttr("archestra_credential.test", "gitlab_url")},
		{Config: cleared, PlanOnly: true},
	}})
}
