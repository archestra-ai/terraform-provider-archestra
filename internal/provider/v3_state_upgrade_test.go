package provider

import (
	"encoding/json"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func TestV3StateUpgradesPreserveBindingAndSecrets(t *testing.T) {
	resources := []struct {
		name string
		r    interface {
			resource.Resource
			resource.ResourceWithUpgradeState
		}
		old string
	}{
		{"agent", &AgentResource{}, `{"id":"existing-id","name":"agent","scope":"org","teams":["team"],"built_in_agent_config":{"name":"dual-llm-main-agent","max_rounds":5},"runtime":{"image":"test-image","kubernetes":{"service_account":"retired"}}}`},
		{"gateway", &McpGatewayResource{}, `{"id":"existing-id","name":"gateway","scope":"org","teams":["team"]}`},
		{"catalog", &MCPServerRegistryResource{}, `{"id":"existing-id","name":"catalog","scope":"org","teams":["team"]}`},
		{"team", &TeamResource{}, `{"id":"existing-id","name":"team","convert_tool_results_to_toon":true}`},
		{"organization", &OrganizationSettingsResource{}, `{"id":"existing-id","compression_scope":"team","global_tool_policy":"restrictive"}`},
		{"model", &LlmModelResource{}, `{"id":"existing-id","model_id":"llama3","input_modalities":["text"],"output_modalities":["text"]}`},
		{"identity", &IdentityProviderResource{}, `{"id":"existing-id","oidc_config":{"client_secret":"state-secret-must-survive","client_id":"client"}}`},
	}
	for _, item := range resources {
		t.Run(item.name, func(t *testing.T) {
			var result resource.UpgradeStateResponse
			item.r.UpgradeState(t.Context())[0].StateUpgrader(t.Context(), resource.UpgradeStateRequest{RawState: &tfprotov6.RawState{JSON: []byte(item.old)}}, &result)
			if result.Diagnostics.HasError() {
				t.Fatal(result.Diagnostics)
			}
			var id string
			if d := result.State.GetAttribute(t.Context(), path.Root("id"), &id); d.HasError() || id != "existing-id" {
				t.Fatalf("lost binding: %s %v", id, d)
			}
			var values map[string]tftypes.Value
			if err := result.State.Raw.As(&values); err != nil {
				t.Fatal(err)
			}

			for _, retired := range []string{"scope", "teams", "convert_tool_results_to_toon", "compression_scope", "global_tool_policy"} {
				if _, ok := values[retired]; ok && (item.name != "model" || retired != "teams") {
					t.Errorf("retired attribute %s remains", retired)
				}
			}
			if item.name == "agent" {
				var builtin string
				if d := result.State.GetAttribute(t.Context(), path.Root("built_in_agent_config"), &builtin); d.HasError() {
					t.Fatal(d)
				}
				var metadata map[string]any
				if err := json.Unmarshal([]byte(builtin), &metadata); err != nil || metadata["name"] != "dual-llm-main-agent" {
					t.Fatalf("lost metadata: %s", builtin)
				}
			}
			if item.name == "model" {
				for _, kind := range []string{"input", "output"} {
					if !values[kind+"_modalities"].IsNull() {
						t.Fatal("legacy discovered modalities became managed overrides")
					}
					var effective []string
					if d := result.State.GetAttribute(t.Context(), path.Root("effective_"+kind+"_modalities"), &effective); d.HasError() || len(effective) != 1 || effective[0] != "text" {
						t.Fatal("lost discovered metadata", d)
					}
				}
			}
			if item.name == "identity" {
				var secret string
				if d := result.State.GetAttribute(t.Context(), path.Root("oidc_config").AtName("client_secret"), &secret); d.HasError() || secret != "state-secret-must-survive" {
					t.Fatal("lost secret", d)
				}
			}
		})
	}
}
func TestV3StateUpgradeRejectsInvalidBinding(t *testing.T) {
	for _, raw := range []string{`{}`, `{"id":3}`, `{"id":"existing","runtime":"invalid"}`} {
		var result resource.UpgradeStateResponse
		(&AgentResource{}).UpgradeState(t.Context())[0].StateUpgrader(t.Context(), resource.UpgradeStateRequest{RawState: &tfprotov6.RawState{JSON: []byte(raw)}}, &result)
		if !result.Diagnostics.HasError() {
			t.Fatal("invalid state accepted", raw)
		}
	}
}
