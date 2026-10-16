package provider

import (
	"encoding/json"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
)

func TestLLMKeyV2StateUpgradePreservesIdentityAndSecret(t *testing.T) {
	for _, primary := range []bool{true, false} {
		r := LLMProviderApiKeyResource{}
		prior := map[string]any{"id": "fixture-key-id", "name": "existing-key", "api_key": "synthetic-preserved-secret", "llm_provider": "ollama", "is_organization_default": primary, "base_url": nil, "scope": "org", "team_id": nil, "vault_secret_path": nil, "vault_secret_key": nil}
		encoded, _ := json.Marshal(prior)
		var response resource.UpgradeStateResponse
		r.UpgradeState(t.Context())[0].StateUpgrader(t.Context(), resource.UpgradeStateRequest{RawState: &tfprotov6.RawState{JSON: encoded}}, &response)
		if response.Diagnostics.HasError() {
			t.Fatal(response.Diagnostics)
		}
		var got LLMProviderApiKeyResourceModel
		if ds := response.State.Get(t.Context(), &got); ds.HasError() {
			t.Fatal(ds)
		}
		if got.ID.ValueString() != "fixture-key-id" || got.ApiKey.ValueString() != "synthetic-preserved-secret" || got.IsPrimary.ValueBool() != primary {
			t.Fatal("state upgrade changed key identity, secret, or primary flag")
		}
		if !got.IsAgentKey.IsNull() {
			t.Fatal("ownership must be refreshed from the API")
		}
	}
}
func TestLLMKeyV2StateUpgradeRejectsInvalidState(t *testing.T) {
	r := LLMProviderApiKeyResource{}
	var response resource.UpgradeStateResponse
	r.UpgradeState(t.Context())[0].StateUpgrader(t.Context(), resource.UpgradeStateRequest{RawState: &tfprotov6.RawState{JSON: []byte(`{"id":3}`)}}, &response)
	if !response.Diagnostics.HasError() {
		t.Fatal("invalid state must not be silently discarded")
	}
}
