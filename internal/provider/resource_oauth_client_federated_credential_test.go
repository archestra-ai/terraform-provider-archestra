package provider

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestAccOauthClientFederatedCredentialResource(t *testing.T) {
	if os.Getenv("TF_ACC") != "" && !testAccAPIVersionAtLeast(t, "1.5.0-rc.41") {
		t.Skip("OAuth client federation and icons need API 1.5.0-rc.41 or later")
	}
	const name = "archestra_oauth_client_federated_credential.test"
	config := func(icon, credentialName, subject, conditions string) string {
		return fmt.Sprintf(`resource "archestra_oauth_client" "federated_parent" {
 name="tf-federated-oauth-parent"
 scopes=["llm:proxy"]
 include_future_providers=true
 provider_api_keys=[]
 %s
}
resource "archestra_oauth_client_federated_credential" "test" {
 oauth_client_id=archestra_oauth_client.federated_parent.id
 name=%q
 issuer="https://token.actions.githubusercontent.com"
 audience="https://archestra.example.com"
 subject=%q
 %s
}`, icon, credentialName, subject, conditions)
	}
	var id string
	unchanged := func(s *terraform.State) error {
		if s.RootModule().Resources[name].Primary.ID != id {
			return fmt.Errorf("an in-place change replaced the federated credential")
		}
		return nil
	}
	resource.Test(t, resource.TestCase{PreCheck: func() { testAccPreCheck(t); testAccRequireEnterprise(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{
		{Config: config(`icon="🚀"`, "tf-main", "repo:acme/infra:ref:refs/heads/main", `claim_conditions={repository_id="1"}`), Check: resource.ComposeTestCheckFunc(
			resource.TestCheckResourceAttr("archestra_oauth_client.federated_parent", "icon", "🚀"),
			resource.TestCheckResourceAttr(name, "claim_conditions.repository_id", "1"),
			resource.TestCheckResourceAttrPair("archestra_oauth_client.federated_parent", "id", name, "oauth_client_id"),
			func(s *terraform.State) error { id = s.RootModule().Resources[name].Primary.ID; return nil },
		)},
		{ResourceName: name, ImportState: true, ImportStateVerify: true},
		{Config: config(`icon="🚀"`, "tf-production", "repo:acme/infra:environment:production", ""), ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction(name, plancheck.ResourceActionUpdate)}}, Check: resource.ComposeTestCheckFunc(unchanged,
			resource.TestCheckResourceAttr(name, "name", "tf-production"),
			resource.TestCheckResourceAttr(name, "claim_conditions.%", "0"),
		)},
		// Removing the icon clears it.
		{Config: config("", "tf-production", "repo:acme/infra:environment:production", ""), Check: resource.ComposeTestCheckFunc(unchanged, resource.TestCheckNoResourceAttr("archestra_oauth_client.federated_parent", "icon"))},
		{Config: config("", "tf-production", "repo:acme/infra:environment:production", ""), ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
	}})
}
