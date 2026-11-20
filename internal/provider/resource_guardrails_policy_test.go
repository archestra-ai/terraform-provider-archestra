package provider

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

// The policy text is adopted, changed, refreshed without drift and imported.
// The organization has one policy, so the test restores the text it found.
func TestAccGuardrailsPolicyResource(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Acceptance test: set TF_ACC=1")
	}
	testAccPreCheck(t)
	testAccRequireEnterprise(t)
	api, err := client.NewClientWithResponses(os.Getenv("ARCHESTRA_BASE_URL"), client.WithRequestEditorFn(func(_ context.Context, r *http.Request) error {
		r.Header.Set("Authorization", os.Getenv("ARCHESTRA_API_KEY"))
		return nil
	}))
	if err != nil {
		t.Fatal(err)
	}
	original, err := api.GetGuardrailsPolicyWithResponse(context.Background())
	if err != nil {
		t.Fatalf("read the current guardrails policy: %v", err)
	}
	if original.JSON200 == nil {
		t.Fatalf("read the current guardrails policy: status %d; OpenAPPA guardrails must be enabled", original.StatusCode())
	}
	t.Cleanup(func() {
		latest, err := api.GetGuardrailsPolicyWithResponse(context.Background())
		if err != nil || latest.JSON200 == nil {
			t.Errorf("restore the guardrails policy: %v", err)
			return
		}
		if _, err := api.UpdateGuardrailsPolicyWithResponse(context.Background(), client.UpdateGuardrailsPolicyJSONRequestBody{Content: original.JSON200.Content, ExpectedRevision: latest.JSON200.Revision}); err != nil {
			t.Errorf("restore the guardrails policy: %v", err)
		}
	})

	config := func(marker string) string {
		// HCL interpolates template sequences inside quoted strings.
		content := strings.NewReplacer("${", "$${", "%{", "%%{").Replace(original.JSON200.Content + "\n# " + marker + "\n")
		return fmt.Sprintf("resource \"archestra_guardrails_policy\" \"test\" {\n  content = %q\n}\n", content)
	}
	const name = "archestra_guardrails_policy.test"
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config("tf-acc first"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(name, "id", original.JSON200.OrganizationId),
					resource.TestCheckResourceAttrSet(name, "content_hash"),
				),
			},
			{Config: config("tf-acc first"), ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
			{
				Config: config("tf-acc second"),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction(name, plancheck.ResourceActionUpdate),
				}},
			},
			{ResourceName: name, ImportState: true, ImportStateVerify: true},
		},
	})
}
