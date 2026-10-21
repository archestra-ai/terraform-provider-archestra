package provider

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestProjectAppsWriteOnlyTheDifference(t *testing.T) {
	const (
		project = "11111111-1111-4111-8111-111111111111"
		keep    = "22222222-2222-4222-8222-222222222222"
		add     = "33333333-3333-4333-8333-333333333333"
		drop    = "44444444-4444-4444-8444-444444444444"
		gone    = "55555555-5555-4555-8555-555555555555"
	)
	var calls []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		calls = append(calls, req.Method+" "+strings.TrimPrefix(req.URL.Path, "/api/projects/"+project+"/apps/"))
		w.Header().Set("Content-Type", "application/json")
		if strings.HasSuffix(req.URL.Path, gone) {
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprint(w, `{"error":{"message":"App is not linked to this project","type":"api_not_found_error"}}`)
			return
		}
		fmt.Fprint(w, `{"ok":true}`)
	}))
	defer server.Close()
	c, err := client.NewClientWithResponses(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	set := func(values ...string) types.Set {
		value, d := types.SetValueFrom(t.Context(), types.StringType, values)
		if d.HasError() {
			t.Fatal(d)
		}
		return value
	}
	data := ProjectResourceModel{ID: types.StringValue(project), AppIDs: set(keep, add)}
	var d diag.Diagnostics
	if !(&ProjectResource{client: c}).writeApps(t.Context(), &data, set(keep, drop, gone), &d) || d.HasError() {
		t.Fatal("an already removed link must not fail the update", d)
	}
	sort.Strings(calls)
	if fmt.Sprint(calls) != fmt.Sprint([]string{"DELETE " + drop, "DELETE " + gone, "PUT " + add}) {
		t.Fatalf("unexpected link writes: %v", calls)
	}
}

func TestAccProjectAppLinks(t *testing.T) {
	api, err := client.NewClientWithResponses(os.Getenv("ARCHESTRA_BASE_URL"), client.WithRequestEditorFn(func(_ context.Context, r *http.Request) error {
		r.Header.Set("Authorization", os.Getenv("ARCHESTRA_API_KEY"))
		return nil
	}))
	if err != nil {
		t.Fatal(err)
	}
	name := acctest.RandomWithPrefix("tf-project-apps")
	config := func(appIDs string) string {
		apps := ""
		for _, key := range []string{"a", "b"} {
			apps += fmt.Sprintf("resource \"archestra_app\" %q {\nname=%q\nhtml=\"<html><head></head><body>%s</body></html>\"\nlocked=false\nenabled=true\n}\n", key, name+"-"+key, key)
		}
		return apps + fmt.Sprintf("resource \"archestra_project\" \"test\" {\nname=%q\n%s\n}\n", name, appIDs)
	}
	ids := func(s *terraform.State, keys ...string) (project string, apps []string) {
		for _, key := range keys {
			apps = append(apps, s.RootModule().Resources["archestra_app."+key].Primary.ID)
		}
		return s.RootModule().Resources["archestra_project.test"].Primary.ID, apps
	}
	linked := func(keys ...string) resource.TestCheckFunc {
		return func(s *terraform.State) error {
			project, want := ids(s, keys...)
			result, err := api.GetProjectAppsWithResponse(t.Context(), uuid.MustParse(project))
			if err != nil || result.JSON200 == nil {
				return fmt.Errorf("read project apps: %v", err)
			}
			var got []string
			for _, app := range *result.JSON200 {
				got = append(got, app.Id.String())
			}
			sort.Strings(got)
			sort.Strings(want)
			if fmt.Sprint(got) != fmt.Sprint(want) {
				return fmt.Errorf("linked apps %v, want %v", got, want)
			}
			if count := s.RootModule().Resources["archestra_project.test"].Primary.Attributes["app_ids.#"]; count != fmt.Sprint(len(want)) {
				return fmt.Errorf("state has %s app_ids, want %d", count, len(want))
			}
			return nil
		}
	}
	var project, appA, appB string
	capture := func(s *terraform.State) error {
		project, _ = ids(s)
		_, apps := ids(s, "a", "b")
		appA, appB = apps[0], apps[1]
		return nil
	}
	outOfBand := func(link bool, app *string) func() {
		return func() {
			projectID, appID := uuid.MustParse(project), uuid.MustParse(*app)
			status := 0
			if link {
				result, err := api.LinkProjectAppWithResponse(t.Context(), projectID, appID)
				if err != nil {
					t.Fatal(err)
				}
				status = result.StatusCode()
			} else {
				result, err := api.UnlinkProjectAppWithResponse(t.Context(), projectID, appID)
				if err != nil {
					t.Fatal(err)
				}
				status = result.StatusCode()
			}
			if status != http.StatusOK {
				t.Fatalf("out-of-band link change returned %d", status)
			}
		}
	}
	onlyB := config("app_ids=[archestra_app.b.id]")
	omitted := config("")
	resource.Test(t, resource.TestCase{PreCheck: func() { testAccPreCheck(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{
		{Config: config("app_ids=[archestra_app.a.id]"), Check: resource.ComposeTestCheckFunc(capture, linked("a"), resource.TestCheckTypeSetElemAttrPair("archestra_project.test", "app_ids.*", "archestra_app.a", "id"))},
		{Config: config("app_ids=[archestra_app.a.id, archestra_app.b.id]"), Check: linked("a", "b")},
		{Config: onlyB, Check: linked("b")},
		// An out-of-band unlink is drift that the next apply repairs.
		{PreConfig: outOfBand(false, &appB), Config: onlyB, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction("archestra_project.test", plancheck.ResourceActionUpdate)}}, Check: linked("b")},
		{Config: onlyB, PlanOnly: true},
		// Import adopts the existing links.
		{ResourceName: "archestra_project.test", ImportState: true, ImportStateVerify: true},
		{Config: config("app_ids=[]"), Check: linked()},
		// Omission leaves links unmanaged: an out-of-band link is observed and kept.
		{PreConfig: outOfBand(true, &appA), Config: omitted, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}, Check: linked("a")},
	}})
}
