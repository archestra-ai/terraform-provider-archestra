package provider

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"testing"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// Infrastructure as code often authenticates with a service-account key.
// Before 1.5.0-rc.40 the team routes rejected those keys for member changes
// (403) and team reads (404), so Read dropped the team from state.
func TestAccTeamResourceWithServiceAccountKey(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("acceptance tests are disabled")
	}
	testAccPreCheck(t)
	if !testAccAPIVersionAtLeast(t, "1.5.0-rc.40") {
		t.Skip("Managing teams with a service-account key requires API 1.5.0-rc.40")
	}
	admin, err := client.NewClientWithResponses(os.Getenv("ARCHESTRA_BASE_URL"), client.WithRequestEditorFn(func(_ context.Context, req *http.Request) error {
		req.Header.Set("Authorization", os.Getenv("ARCHESTRA_API_KEY"))
		return nil
	}))
	if err != nil {
		t.Fatal(err)
	}
	// A team created with the suite's personal key names that key's user, the
	// member the service account adds below.
	probe, err := admin.CreateTeamWithResponse(t.Context(), client.CreateTeamJSONRequestBody{Name: "tf-acc-sa-team-probe"})
	if err != nil || probe.JSON200 == nil {
		t.Fatalf("create probe team: %v %s", err, probe.Body)
	}
	if _, err := admin.DeleteTeamWithResponse(t.Context(), probe.JSON200.Id); err != nil {
		t.Fatalf("delete probe team: %v", err)
	}
	if probe.JSON200.CreatedBy == nil {
		t.Fatal("ARCHESTRA_API_KEY must be a personal API key: the test adds its user to a team")
	}
	userID := *probe.JSON200.CreatedBy

	account, err := admin.CreateServiceAccountWithResponse(t.Context(), client.CreateServiceAccountJSONRequestBody{Name: "tf-acc-team-manager", Role: "admin"})
	if err != nil || account.JSON200 == nil {
		t.Fatalf("create service account: %v %s", err, account.Body)
	}
	t.Cleanup(func() {
		if _, err := admin.DeleteServiceAccountWithResponse(context.Background(), account.JSON200.Id); err != nil {
			t.Errorf("delete service account: %v", err)
		}
	})
	token, err := admin.CreateServiceAccountTokenWithResponse(t.Context(), account.JSON200.Id, client.CreateServiceAccountTokenJSONRequestBody{Name: "tf-acc-team-manager"})
	if err != nil || token.JSON200 == nil {
		t.Fatalf("create service-account key: %v %s", err, token.Body)
	}

	config := func(members string) string {
		return fmt.Sprintf(`provider "archestra" {
  api_key = %q
}

resource "archestra_team" "test" {
  name    = "tf-acc-sa-managed-team"
  members = %s
}
`, token.JSON200.Token, members)
	}
	withMember := fmt.Sprintf(`[{ user_id = %q, role = "member" }]`, userID)
	const team = "archestra_team.test"
	resource.Test(t, resource.TestCase{ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{
		{Config: config(withMember), Check: resource.ComposeTestCheckFunc(
			resource.TestCheckResourceAttr(team, "members.#", "1"),
			resource.TestCheckResourceAttr(team, "members.0.user_id", userID),
			resource.TestCheckResourceAttr(team, "created_by_service_account_id", account.JSON200.Id.String()),
		)},
		{Config: config(withMember), PlanOnly: true},
		{ResourceName: team, ImportState: true, ImportStateVerify: true, ImportStateVerifyIgnore: []string{"members"}},
		{Config: config("[]"), Check: resource.TestCheckResourceAttr(team, "members.#", "0")},
	}})
}
