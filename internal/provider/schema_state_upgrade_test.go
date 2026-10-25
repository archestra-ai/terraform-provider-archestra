package provider

import (
	"encoding/json"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func TestSchemaUpgradePreservesManagedObjectsAndCredentials(t *testing.T) {
	cases := []struct {
		name string
		r    interface {
			resource.Resource
			resource.ResourceWithUpgradeState
		}
		version  int64
		old      string
		retained map[string]string
	}{
		{"team", &TeamResource{}, 1, `{"id":"existing-id","name":"team","external_group_ids":["engineering"],"vault_folder":"secret/data/team"}`, map[string]string{"name": "team"}},
		{"model-provider", &LLMProviderApiKeyResource{}, 1, `{"id":"existing-id","name":"provider","api_key":"synthetic-preserved-key","vault_secret_path":null,"vault_secret_key":null,"aws_secret_access_key":"synthetic-preserved-aws-key"}`, map[string]string{"api_key": "synthetic-preserved-key", "aws_secret_access_key": "synthetic-preserved-aws-key"}},
		//nolint:gosec // Synthetic saved-state identifiers, not credentials.
		{"catalog", &MCPServerRegistryResource{}, 1, `{"id":"existing-id","name":"catalog","client_secret_id":"existing-client-secret","local_config_secret_id":"existing-env-secret","local_config_vault_path":null,"local_config_vault_key":null,"oauth_client_secret_vault_path":null,"oauth_client_secret_vault_key":null}`, map[string]string{"client_secret_id": "existing-client-secret", "local_config_secret_id": "existing-env-secret"}},
		{"server", &MCPServerResource{}, 0, `{"id":"existing-id","name":"server","is_byos_vault":false,"secret_id":"existing-secret","access_token":"synthetic-preserved-token","environment_values":{"API_KEY":"synthetic-env-key"}}`, map[string]string{"secret_id": "existing-secret", "access_token": "synthetic-preserved-token"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var result resource.UpgradeStateResponse
			tc.r.UpgradeState(t.Context())[tc.version].StateUpgrader(t.Context(), resource.UpgradeStateRequest{RawState: &tfprotov6.RawState{JSON: []byte(tc.old)}}, &result)
			if result.Diagnostics.HasError() {
				t.Fatal(result.Diagnostics)
			}
			var current resource.SchemaResponse
			tc.r.Schema(t.Context(), resource.SchemaRequest{}, &current)
			if current.Schema.Version != tc.version+1 {
				t.Fatal("incorrect schema version")
			}
			for field, want := range tc.retained {
				var got string
				if d := result.State.GetAttribute(t.Context(), path.Root(field), &got); d.HasError() || got != want {
					t.Fatalf("lost %s: %v", field, d)
				}
			}
			var id string
			if d := result.State.GetAttribute(t.Context(), path.Root("id"), &id); d.HasError() || id != "existing-id" {
				t.Fatal("lost remote identity", d)
			}
			var values map[string]tftypes.Value
			if err := result.State.Raw.As(&values); err != nil {
				t.Fatal(err)
			}
			for _, name := range []string{"vault_folder", "vault_secret_path", "vault_secret_key", "local_config_vault_path", "local_config_vault_key", "oauth_client_secret_vault_path", "oauth_client_secret_vault_key", "is_byos_vault"} {
				if _, ok := values[name]; ok {
					t.Errorf("removed attribute remains: %s", name)
				}
			}
			if tc.name == "team" {
				var groups []string
				if d := result.State.GetAttribute(t.Context(), path.Root("external_group_ids"), &groups); d.HasError() || len(groups) != 1 || groups[0] != "engineering" {
					t.Fatal("lost SSO group bindings", d)
				}
			}
			if tc.name == "server" {
				var env map[string]string
				if d := result.State.GetAttribute(t.Context(), path.Root("environment_values"), &env); d.HasError() || env["API_KEY"] != "synthetic-env-key" {
					t.Fatal("lost environment credential", d)
				}
			}
		})
	}
}

func TestSchemaUpgradeRefusesInvalidState(t *testing.T) {
	for _, raw := range []string{"", `null`, `{}`, `{"id":5}`, `{"id":"existing","name":{}}`} {
		var result resource.UpgradeStateResponse
		(&TeamResource{}).UpgradeState(t.Context())[1].StateUpgrader(t.Context(), resource.UpgradeStateRequest{RawState: &tfprotov6.RawState{JSON: json.RawMessage(raw)}}, &result)
		if !result.Diagnostics.HasError() {
			t.Fatal("invalid state accepted")
		}
	}
	var result resource.UpgradeStateResponse
	(&TeamResource{}).UpgradeState(t.Context())[1].StateUpgrader(t.Context(), resource.UpgradeStateRequest{}, &result)
	if !result.Diagnostics.HasError() {
		t.Fatal("missing state accepted")
	}
}
