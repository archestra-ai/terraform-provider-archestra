package provider

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"testing"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// nonceAdoptionSteps creates with nonce "1", forgets the object, imports it
// into fresh state with the same configuration, and checks that recording the
// nonce neither replaces nor rotates; changing it to "2" still does.
func nonceAdoptionSteps(address, fixtures string, config func(nonce string) string, kept, rotated resource.TestCheckFunc, rotation plancheck.ResourceActionType) []resource.TestStep {
	var id string
	capture := func(s *terraform.State) error {
		id = s.RootModule().Resources[address].Primary.ID
		return nil
	}
	same := func(s *terraform.State) error {
		if s.RootModule().Resources[address].Primary.ID != id {
			return fmt.Errorf("adoption replaced %s", address)
		}
		return nil
	}
	forget := fixtures + fmt.Sprintf("\nremoved {\n from = %s\n lifecycle {\n destroy = false\n }\n}\n", address)
	return []resource.TestStep{
		{Config: config("1"), Check: resource.ComposeTestCheckFunc(capture, kept)},
		{Config: forget},
		{Config: config("1"), ResourceName: address, ImportState: true, ImportStatePersist: true, ImportStateIdFunc: func(*terraform.State) (string, error) { return id, nil }},
		// Import cannot see the configured nonce, so Terraform records it in place.
		{Config: config("1"), ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction(address, plancheck.ResourceActionUpdate)}}, Check: resource.ComposeTestCheckFunc(same, kept, resource.TestCheckResourceAttr(address, "rotation_nonce", "1"))},
		{Config: config("1"), ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
		{Config: config("2"), ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction(address, rotation)}}, Check: rotated},
	}
}

func TestAccVirtualAPIKeyNonceAdoption(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-vk-adopt")
	config := func(nonce string) string {
		return fmt.Sprintf("resource \"archestra_virtual_key\" \"test\" {\nname=%q\nkey_type=\"passthrough\"\nrotation_nonce=%q\n}\n", name, nonce)
	}
	var original string
	kept := func(s *terraform.State) error {
		id := s.RootModule().Resources["archestra_virtual_key.test"].Primary.ID
		if original == "" {
			original = id
		}
		status, err := testCredentialMetadataStatus(t.Context(), "/api/llm-virtual-keys/"+original)
		if err != nil || status != http.StatusOK {
			return fmt.Errorf("original virtual key not readable: status %d %v", status, err)
		}
		return nil
	}
	rotated := func(s *terraform.State) error {
		if s.RootModule().Resources["archestra_virtual_key.test"].Primary.ID == original {
			return fmt.Errorf("nonce change did not replace the virtual key")
		}
		status, err := testCredentialMetadataStatus(t.Context(), "/api/llm-virtual-keys/"+original)
		if err != nil || status != http.StatusNotFound {
			return fmt.Errorf("replaced virtual key was not revoked: status %d %v", status, err)
		}
		return nil
	}
	resource.Test(t, resource.TestCase{PreCheck: func() { testAccPreCheck(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: nonceAdoptionSteps("archestra_virtual_key.test", "", config, kept, rotated, plancheck.ResourceActionDestroyBeforeCreate)})
}

func TestAccServiceAccountAPIKeyNonceAdoption(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-sa-adopt")
	fixtures := fmt.Sprintf("resource \"archestra_service_account\" \"parent\" {\nname=%q\nrole=\"member\"\n}\n", name)
	config := func(nonce string) string {
		return fixtures + fmt.Sprintf("resource \"archestra_service_account_api_key\" \"test\" {\nservice_account_id=archestra_service_account.parent.id\nname=%q\nexpires_in=3600\nrotation_nonce=%q\n}\n", name, nonce)
	}
	var token string
	kept := func(s *terraform.State) error {
		if current := s.RootModule().Resources["archestra_service_account_api_key.test"].Primary.Attributes["token"]; token == "" {
			token = current
		}
		return testTokenAuthenticates(token, http.StatusOK)
	}
	rotated := func(s *terraform.State) error {
		next := s.RootModule().Resources["archestra_service_account_api_key.test"].Primary.Attributes["token"]
		if next == "" || next == token {
			return fmt.Errorf("nonce change did not issue a new token")
		}
		if err := testTokenAuthenticates(token, http.StatusUnauthorized); err != nil {
			return err
		}
		return testTokenAuthenticates(next, http.StatusOK)
	}
	resource.Test(t, resource.TestCase{PreCheck: func() { testAccPreCheck(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: nonceAdoptionSteps("archestra_service_account_api_key.test", fixtures, config, kept, rotated, plancheck.ResourceActionDestroyBeforeCreate)})
}

func TestAccShareLinkNonceAdoption(t *testing.T) {
	config := func(nonce string) string {
		return shareSkillFixtures + fmt.Sprintf("\nresource \"archestra_share_link\" \"test\" {\nname=\"tf-share-adopt\"\nskill_ids=[archestra_skill.first.id]\nplugin_ids=[]\nrotation_nonce=%q\n}\n", nonce)
	}
	var clone string
	kept := func(s *terraform.State) error {
		if clone == "" {
			clone = s.RootModule().Resources["archestra_share_link.test"].Primary.Attributes["clone_url"]
		}
		return shareLinkReadable(clone)
	}
	rotated := func(s *terraform.State) error {
		code, err := shareLinkAnonymousStatus(clone)
		if err != nil {
			return err
		}
		if code != http.StatusNotFound {
			return fmt.Errorf("replaced share link still serves its old token: status %d", code)
		}
		return shareLinkReadable(s.RootModule().Resources["archestra_share_link.test"].Primary.Attributes["clone_url"])
	}
	resource.Test(t, resource.TestCase{PreCheck: func() { testAccPreCheck(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: nonceAdoptionSteps("archestra_share_link.test", shareSkillFixtures, config, kept, rotated, plancheck.ResourceActionDestroyBeforeCreate)})
}

func TestAccOauthClientNonceAdoption(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-oauth-adopt")
	config := func(nonce string) string {
		return fmt.Sprintf("resource \"archestra_oauth_client\" \"test\" {\nname=%q\nscopes=[\"llm:proxy\"]\ninclude_future_providers=true\nrotation_nonce=%q\n}\n", name, nonce)
	}
	var clientID, secret string
	kept := func(s *terraform.State) error {
		v := s.RootModule().Resources["archestra_oauth_client.test"].Primary
		if secret == "" {
			clientID, secret = v.Attributes["client_id"], v.Attributes["client_secret"]
		}
		return testOauthCredential(t.Context(), clientID, secret, true)
	}
	rotated := func(s *terraform.State) error {
		v := s.RootModule().Resources["archestra_oauth_client.test"].Primary
		if v.Attributes["client_id"] != clientID || v.Attributes["client_secret"] == "" || v.Attributes["client_secret"] == secret {
			return fmt.Errorf("nonce change did not rotate the secret in place")
		}
		if err := testOauthCredential(t.Context(), clientID, secret, false); err != nil {
			return err
		}
		return testOauthCredential(t.Context(), clientID, v.Attributes["client_secret"], true)
	}
	resource.Test(t, resource.TestCase{PreCheck: func() { testAccPreCheck(t); testAccRequireEnterprise(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: nonceAdoptionSteps("archestra_oauth_client.test", "", config, kept, rotated, plancheck.ResourceActionUpdate)})
}

func testTokenAuthenticates(token string, want int) error {
	httpClient, err := buildHTTPClient()
	if err != nil {
		return err
	}
	c, err := client.NewClientWithResponses(os.Getenv("ARCHESTRA_BASE_URL"), client.WithHTTPClient(httpClient), client.WithRequestEditorFn(func(_ context.Context, req *http.Request) error { req.Header.Set("Authorization", token); return nil }))
	if err != nil {
		return err
	}
	result, err := c.GetOrganizationWithResponse(context.Background())
	if err != nil {
		return err
	}
	if result.StatusCode() != want {
		return fmt.Errorf("credential authentication returned %d; expected %d", result.StatusCode(), want)
	}
	return nil
}
