package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccOrganizationSettingsCurrentDomains(t *testing.T) {
	initial := `resource "archestra_organization_settings" "test" {
 online_mcp_catalog_enabled = false
 online_skill_catalog_enabled = false
 allow_tool_auto_assignment = false
 connection_skills_enabled = false
 connection_plugins_enabled = false
 connection_client_order = jsonencode(["codex"])
 connection_base_urls = [{url="http://127.0.0.1:19003"}]
 popular_agent_overrides = jsonencode({codex={hidden=false,position=1}})
 model_provider_overrides = jsonencode({ollama={hidden=false,displayName="Local Ollama"}})
 default_environment_description = "Initial local environment"
 oauth_access_token_lifetime_seconds = 1200
 session_max_age_seconds = 7200
 require_two_factor = false
 kb_bm25_k1 = 1.4
 kb_bm25_b = 0.6
}`
	updated := `resource "archestra_organization_settings" "test" {
 online_mcp_catalog_enabled = true
 online_skill_catalog_enabled = true
 allow_tool_auto_assignment = true
 connection_skills_enabled = true
 connection_plugins_enabled = true
 connection_client_order = jsonencode(["codex","claude-code"])
 connection_base_urls = [{url="http://127.0.0.1:19003",description="Updated",is_default=true,visible=true}]
 popular_agent_overrides = jsonencode({codex={hidden=false,position=2}})
 model_provider_overrides = jsonencode({ollama={hidden=false,displayName="Updated Ollama"}})
 default_environment_description = "Updated local environment"
 oauth_access_token_lifetime_seconds = 1800
 session_max_age_seconds = 10800
 require_two_factor = false
 kb_bm25_k1 = 1.5
 kb_bm25_b = 0.7
}`
	resource.Test(t, resource.TestCase{PreCheck: func() { testAccPreCheck(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{
		{Config: initial, Check: resource.ComposeTestCheckFunc(resource.TestCheckResourceAttr("archestra_organization_settings.test", "oauth_access_token_lifetime_seconds", "1200"), resource.TestCheckResourceAttr("archestra_organization_settings.test", "kb_bm25_b", "0.6"))},
		{ResourceName: "archestra_organization_settings.test", ImportState: true, ImportStateVerify: true},
		{Config: initial, PlanOnly: true},
		{Config: updated, Check: resource.ComposeTestCheckFunc(resource.TestCheckResourceAttr("archestra_organization_settings.test", "connection_plugins_enabled", "true"), resource.TestCheckResourceAttr("archestra_organization_settings.test", "default_environment_description", "Updated local environment"))},
		{Config: updated, PlanOnly: true},
	}})
}

func TestFlattenCurrentOrganizationSettingsModelTiers(t *testing.T) {
	t.Parallel()
	const fast, smart = "11111111-1111-4111-8111-111111111111", "22222222-2222-4222-8222-222222222222"
	tests := []struct {
		name                string
		body                string
		wantFast, wantSmart types.String
	}{
		{"set", `{"defaultFastModelId":"` + fast + `","defaultSmartModelId":"` + smart + `"}`, types.StringValue(fast), types.StringValue(smart)},
		{"cleared", `{"defaultFastModelId":null}`, types.StringNull(), types.StringNull()},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			data := OrganizationSettingsResourceModel{DefaultFastModelId: types.StringValue(fast), DefaultSmartModelId: types.StringValue(smart)}
			var diags diag.Diagnostics
			flattenCurrentOrganizationSettings(&data, []byte(tt.body), &diags)
			if diags.HasError() {
				t.Fatalf("flatten: %v", diags)
			}
			if !data.DefaultFastModelId.Equal(tt.wantFast) || !data.DefaultSmartModelId.Equal(tt.wantSmart) {
				t.Errorf("got fast=%s smart=%s, want fast=%s smart=%s", data.DefaultFastModelId, data.DefaultSmartModelId, tt.wantFast, tt.wantSmart)
			}
		})
	}
}
