package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestAccOrganizationInlineDefaults(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("acceptance tests are disabled")
	}
	testAccPreCheck(t)
	api, err := client.NewClientWithResponses(os.Getenv("ARCHESTRA_BASE_URL"), client.WithRequestEditorFn(func(_ context.Context, r *http.Request) error {
		r.Header.Set("Authorization", os.Getenv("ARCHESTRA_API_KEY"))
		return nil
	}))
	if err != nil {
		t.Fatal(err)
	}
	owner := OrganizationSettingsResource{client: api}
	var d diag.Diagnostics
	starting, status := owner.listUserDefaultLimits(t.Context(), &d)
	if status != 200 || d.HasError() || len(starting) != 0 {
		t.Fatal("Inspect existing spending defaults before this isolated complete-set test", d)
	}
	environments, err := api.ListEnvironmentsWithResponse(t.Context(), nil)
	if err != nil || environments.JSON200 == nil {
		t.Fatal("read isolated environment defaults", err)
	}
	var baseline struct {
		ResourceDefaults map[string]*string `json:"resourceDefaults"`
	}
	if err := json.Unmarshal(environments.Body, &baseline); err != nil {
		t.Fatal(err)
	}
	if baseline.ResourceDefaults["agent"] != nil || baseline.ResourceDefaults["mcpGateway"] != nil {
		t.Fatal("Inspect existing agent/gateway environment defaults before this isolated test")
	}
	helper := ResourcePermissionPolicyResource{client: api}
	policyState := parentPermissionState("agent", "*")
	original, status := helper.readPolicy(t.Context(), &policyState, &d)
	if status != 200 || d.HasError() {
		t.Fatal("read original default access policy", d)
	}
	originalSet := parentPermissionGrants(t.Context(), &policyState, original, &d)
	if d.HasError() {
		t.Fatal(d)
	}
	name := acctest.RandomWithPrefix("tf-inline-defaults")
	team, err := api.CreateTeamWithBodyWithResponse(t.Context(), "application/json", strings.NewReader(fmt.Sprintf(`{"name":%q}`, name+"-recipient")))
	if err != nil || team.JSON200 == nil {
		t.Fatal("create isolated sharing recipient", err)
	}
	teamID := team.JSON200.Id
	t.Cleanup(func() {
		deleted, err := api.DeleteTeamWithResponse(context.Background(), teamID)
		if err != nil || (deleted.StatusCode() != 200 && deleted.StatusCode() != 404) {
			t.Error("delete sharing recipient", err)
		}
	})
	extra := append([]permissionGrantWire{}, original.Grants...)
	added := permissionGrantWire{}
	added.Subject.Type = "team"
	added.Subject.ID = teamID
	added.Actions = []string{"read"}
	extra = append(extra, added)
	extraPolicy := *original
	extraPolicy.Grants = extra
	extraSet := parentPermissionGrants(t.Context(), &policyState, &extraPolicy, &d)
	if d.HasError() {
		t.Fatal(d)
	}
	var globalID uuid.UUID
	var orgID string
	t.Cleanup(func() {
		var cleanupDiags diag.Diagnostics
		current, status := helper.readPolicy(context.Background(), &policyState, &cleanupDiags)
		if status != 200 || cleanupDiags.HasError() {
			t.Error("read cleanup access policy", cleanupDiags)
			return
		}
		actual := parentPermissionGrants(context.Background(), &policyState, current, &cleanupDiags)
		if !actual.Equal(originalSet) {
			if !actual.Equal(extraSet) {
				t.Error("Unexpected concurrent global-policy edit; cleanup refused")
				return
			}
			helper.writePolicy(context.Background(), &policyState, current.Revision, original.Grants, &cleanupDiags)
			if cleanupDiags.HasError() {
				t.Error("restore original access policy", cleanupDiags)
			}
		}
		rows, status := owner.listUserDefaultLimits(context.Background(), &cleanupDiags)
		if status != 200 || cleanupDiags.HasError() {
			t.Error("read owned spending-default cleanup", cleanupDiags)
			return
		}
		if row, ok := rows["default"]; ok {
			if (globalID != uuid.Nil && row.ID != globalID) || (row.LimitValue != 10 && row.LimitValue != 20 && row.LimitValue != 37) || len(row.Models) != 0 || row.CleanupInterval != "calendar_month" {
				t.Error("Unexpected spending-default edit; cleanup refused")
				return
			}
			removed, err := api.DeleteDefaultUserLimitWithResponse(context.Background(), row.ID)
			if err != nil || (removed.StatusCode() != 200 && removed.StatusCode() != 204 && removed.StatusCode() != 404) {
				t.Error("remove owned default limit", err)
			}
		}
	})
	encode := func(grants []permissionGrantWire) string {
		values := make([]map[string]any, 0, len(grants))
		for _, g := range grants {
			values = append(values, map[string]any{"subject_type": g.Subject.Type, "subject_id": g.Subject.ID, "actions": g.Actions})
		}
		raw, err := json.Marshal(values)
		if err != nil {
			t.Fatal(err)
		}
		return fmt.Sprintf("jsondecode(%q)", string(raw))
	}
	base := fmt.Sprintf(`resource "archestra_environment" "first" {name=%q}
resource "archestra_environment" "second" {name=%q}
`, name+"-first", name+"-second")
	permissions := encode(extra)
	config := func(fields string) string {
		return base + "resource \"archestra_organization_settings\" \"test\" {\n" + fields + "\n}"
	}
	initial := config(fmt.Sprintf(`environment_defaults={agents=archestra_environment.first.id,mcp_gateways=archestra_environment.second.id}
default_user_limits={default={limit_value=10,models=[],cleanup_interval="calendar_month"},(archestra_environment.first.id)={limit_value=5,models=["synthetic-model"],cleanup_interval="1h"}}
default_permissions={agents=%s}`, permissions))
	updated := config(fmt.Sprintf(`environment_defaults={agents=archestra_environment.second.id,mcp_gateways=""}
default_user_limits={default={limit_value=20,models=[],cleanup_interval="calendar_month"},(archestra_environment.second.id)={limit_value=7,models=[],cleanup_interval="24h"}}
default_permissions={agents=%s}`, permissions))
	// default_personal_agent_access_all_tools starts with 1.5.0-rc.1; older
	// releases leave it out of the configs and checks.
	withAccessAllTools := testAccAPIVersionAtLeast(t, "1.5.0-rc.1")
	accessAllTools := func(value string) (string, resource.TestCheckFunc) {
		if !withAccessAllTools {
			return "", func(*terraform.State) error { return nil }
		}
		return "\ndefault_personal_agent_access_all_tools=" + value,
			resource.TestCheckResourceAttr("archestra_organization_settings.test", "default_personal_agent_access_all_tools", value)
	}
	disableAccess, disableAccessCheck := accessAllTools("false")
	// Restore the platform default so later tests create personal agents as usual.
	restoreAccess, restoreAccessCheck := accessAllTools("true")
	cleared := config(fmt.Sprintf(`environment_defaults={agents="",mcp_gateways=""}
default_user_limits={}
default_permissions={agents=%s}%s`, encode(original.Grants), restoreAccess))
	check := func(s *terraform.State) error {
		value := s.RootModule().Resources["archestra_organization_settings.test"].Primary
		if orgID != "" && orgID != value.ID {
			return fmt.Errorf("singleton identity changed")
		}
		orgID = value.ID
		rows, status := owner.listUserDefaultLimits(t.Context(), &d)
		if status != 200 || d.HasError() {
			return fmt.Errorf("default limits unreadable: %v", d)
		}
		if row, ok := rows["default"]; ok {
			if globalID != uuid.Nil && globalID != row.ID {
				return fmt.Errorf("global default was replaced")
			}
			globalID = row.ID
		}
		return nil
	}
	drift := func() {
		r, err := api.UpdateEnvironmentResourceDefaultsWithBodyWithResponse(t.Context(), "application/json", strings.NewReader(`{"agent":null}`))
		if err != nil || r.JSON200 == nil {
			t.Fatal("inject default environment drift", err)
		}
		changed, err := api.UpdateDefaultUserLimitWithBodyWithResponse(t.Context(), globalID, "application/json", strings.NewReader(`{"limitValue":37}`))
		if err != nil || changed.JSON200 == nil {
			t.Fatal("inject spending default drift", err)
		}
		current, status := helper.readPolicy(t.Context(), &policyState, &d)
		if status != 200 || d.HasError() {
			t.Fatal(d)
		}
		if helper.writePolicy(t.Context(), &policyState, current.Revision, original.Grants, &d) == nil {
			t.Fatal("inject default sharing drift", d)
		}
	}
	resource.Test(t, resource.TestCase{ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{
		{Config: initial, Check: resource.ComposeTestCheckFunc(check, resource.TestCheckResourceAttr("archestra_organization_settings.test", "default_user_limits.%", "2"))},
		{ResourceName: "archestra_organization_settings.test", ImportState: true, ImportStateVerify: true},
		{Config: updated, Check: check, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction("archestra_organization_settings.test", plancheck.ResourceActionUpdate)}}},
		{PreConfig: drift, Config: updated, Check: check},
		{Config: config("mcp_idle_hibernation_enabled=true" + disableAccess), Check: resource.ComposeTestCheckFunc(check, resource.TestCheckResourceAttr("archestra_organization_settings.test", "default_user_limits.%", "2"), disableAccessCheck)},
		{Config: cleared, Check: resource.ComposeTestCheckFunc(check, resource.TestCheckResourceAttr("archestra_organization_settings.test", "default_user_limits.%", "0"), resource.TestCheckResourceAttr("archestra_organization_settings.test", "environment_defaults.agents", ""), restoreAccessCheck)},
		{Config: cleared, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
	}})
}

// A default policy must keep a recipient that can manage permissions; a lone
// team read grant is refused by the API and must surface as an error.
func TestAccOrganizationDefaultPermissionsRequireManageRecipient(t *testing.T) {
	api, err := client.NewClientWithResponses(os.Getenv("ARCHESTRA_BASE_URL"), client.WithRequestEditorFn(func(_ context.Context, r *http.Request) error {
		r.Header.Set("Authorization", os.Getenv("ARCHESTRA_API_KEY"))
		return nil
	}))
	if err != nil {
		t.Fatal(err)
	}
	unchanged := func(*terraform.State) error {
		var d diag.Diagnostics
		state := parentPermissionState("agent", "*")
		policy, status := (&ResourcePermissionPolicyResource{client: api}).readPolicy(context.Background(), &state, &d)
		if policy == nil || status != 200 {
			return fmt.Errorf("read agents default policy: status %d %v", status, d)
		}
		for _, grant := range policy.Grants {
			for _, action := range grant.Actions {
				if action == "manage-permissions" {
					return nil
				}
			}
		}
		return fmt.Errorf("agents default policy lost its manage-permissions recipient")
	}
	config := fmt.Sprintf(`resource "archestra_team" "readers" {name=%q}
resource "archestra_organization_settings" "test" {
 default_permissions={agents=[{subject_type="team",subject_id=archestra_team.readers.id,actions=["read"]}]}
}`, acctest.RandomWithPrefix("tf-default-readers"))
	resource.Test(t, resource.TestCase{
		PreCheck: func() { testAccPreCheck(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: unchanged,
		Steps: []resource.TestStep{
			{Config: config, ExpectError: regexp.MustCompile(`At least one recipient must keep Manage access`)},
		},
	})
}
