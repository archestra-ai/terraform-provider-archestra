package provider

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"testing"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestAccServiceAccountResource(t *testing.T) {
	config := func(name string, disabled bool) string {
		return fmt.Sprintf(`resource "archestra_service_account" "test" {
 name=%q
 role="member"
 disabled=%t
 labels=[{key="managed-by",value="terraform"}]
 }`, name, disabled)
	}
	resource.Test(t, resource.TestCase{PreCheck: func() { testAccPreCheck(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{
		{Config: config("tf-service-account", true), Check: resource.ComposeTestCheckFunc(resource.TestCheckResourceAttr("archestra_service_account.test", "disabled", "true"), resource.TestCheckResourceAttr("archestra_service_account.test", "active_token_count", "0"))},
		{ResourceName: "archestra_service_account.test", ImportState: true, ImportStateVerify: true},
		{Config: config("tf-service-account-updated", false), ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction("archestra_service_account.test", plancheck.ResourceActionUpdate)}}, Check: resource.TestCheckResourceAttr("archestra_service_account.test", "disabled", "false")},
		{Config: config("tf-service-account-updated", false), ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
	}})
}

func TestAccServiceAccountTokenResource(t *testing.T) {
	var first string
	var issued []string
	authenticate := func(token string, want int) error {
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
	check := func(mode string, want int) resource.TestCheckFunc {
		return func(state *terraform.State) error {
			token := state.RootModule().Resources["archestra_service_account_api_key.test"].Primary.Attributes["token"]
			if token == "" {
				return fmt.Errorf("creation did not persist the secret")
			}
			switch mode {
			case "first":
				first = token
				issued = append(issued, token)
			case "same":
				if token != first {
					return fmt.Errorf("metadata update changed the credential")
				}
			case "rotated":
				if token == first {
					return fmt.Errorf("rotation reused the old credential")
				}
				if err := authenticate(first, 401); err != nil {
					return err
				}
				issued = append(issued, token)
			}
			return authenticate(token, want)
		}
	}
	config := func(name string, disabled bool, expiry, nonce string) string {
		expiration := ""
		if expiry != "" {
			expiration = fmt.Sprintf("expires_at=%q", expiry)
		}
		return fmt.Sprintf(`resource "archestra_service_account" "token_parent" {
name="tf-token-parent"
 role="member"
}
 resource "archestra_service_account_api_key" "test" {
 service_account_id=archestra_service_account.token_parent.id
 name=%q
 expires_in=3600
 disabled=%t
 %s
 rotation_nonce=%q
 }`, name, disabled, expiration, nonce)
	}
	resource.Test(t, resource.TestCase{PreCheck: func() { testAccPreCheck(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(_ *terraform.State) error {
			for _, token := range issued {
				if err := authenticate(token, 401); err != nil {
					return err
				}
			}
			return nil
		},
		Steps: []resource.TestStep{
			{Config: config("tf-token", false, "", "first"), Check: check("first", 200)},
			{ResourceName: "archestra_service_account_api_key.test", ImportState: true, ImportStateVerify: true, ImportStateVerifyIgnore: []string{"token", "expires_in", "rotation_nonce", "last_used_at"}},
			{Config: config("tf-token-renamed", true, "2099-01-01T00:00:00Z", "first"), ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction("archestra_service_account_api_key.test", plancheck.ResourceActionUpdate)}}, Check: resource.ComposeTestCheckFunc(check("same", 401), resource.TestCheckResourceAttr("archestra_service_account_api_key.test", "effective_expires_at", "2099-01-01T00:00:00Z"))},
			{Config: config("tf-token-renamed", false, "", "first"), Check: resource.ComposeTestCheckFunc(check("same", 200), resource.TestCheckNoResourceAttr("archestra_service_account_api_key.test", "effective_expires_at"))},
			{Config: config("tf-token-renamed", false, "", "second"), Check: check("rotated", 200)},
			{Config: config("tf-token-renamed", false, "", "second"), ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
		}})
}

func TestAccServiceAccountTeamOwnership(t *testing.T) {
	var id string
	config := func(team string) string {
		return fmt.Sprintf(`
resource "archestra_team" "first" {name="tf-account-owner-first"}
resource "archestra_team" "second" {name="tf-account-owner-second"}
resource "archestra_service_account" "test" {
 name="tf-team-account"
 role="member"
 team_id=%s
}
`, team)
	}
	unchanged := func(s *terraform.State) error {
		if s.RootModule().Resources["archestra_service_account.test"].Primary.ID != id {
			return fmt.Errorf("team ownership replaced service account")
		}
		return nil
	}
	resource.Test(t, resource.TestCase{PreCheck: func() { testAccPreCheck(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{
		{Config: config("archestra_team.first.id"), Check: resource.ComposeTestCheckFunc(resource.TestCheckResourceAttrPair("archestra_service_account.test", "team_id", "archestra_team.first", "id"), func(s *terraform.State) error {
			id = s.RootModule().Resources["archestra_service_account.test"].Primary.ID
			return nil
		})},
		{ResourceName: "archestra_service_account.test", ImportState: true, ImportStateVerify: true},
		{Config: config("archestra_team.first.id"), PlanOnly: true},
		{Config: config("archestra_team.second.id"), Check: resource.ComposeTestCheckFunc(unchanged, resource.TestCheckResourceAttrPair("archestra_service_account.test", "team_id", "archestra_team.second", "id"))},
		{Config: config("null"), Check: resource.ComposeTestCheckFunc(unchanged, resource.TestCheckNoResourceAttr("archestra_service_account.test", "team_id"))},
		{Config: config("null"), PlanOnly: true},
	}})
}
