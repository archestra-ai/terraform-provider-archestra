package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/diag"
)

// The backend zod schema rejects a runtime object with *missing*
// required-nullable keys ("received undefined") while accepting explicit
// nulls, and the generic AtomicObject encoder drops null sub-attributes —
// this pins the encoder's null back-filling that bridges the two.
func TestEncodeRuntimeFillsRequiredNullableKeys(t *testing.T) {
	t.Parallel()

	got, ok := encodeRuntime(map[string]any{
		"image":             "img:latest",
		"inferenceProtocol": "anthropic",
		"backend":           "kubernetes",
		"steerMode":         "tmux_keys",
		"privileged":        true,
	}).(map[string]any)
	if !ok {
		t.Fatal("expected a map back")
	}

	for _, key := range []string{"command", "resources", "environment", "credentials", "ttlHours", "idleTimeoutMinutes"} {
		v, present := got[key]
		if !present {
			t.Errorf("required-nullable key %q missing from encoded object", key)
		} else if v != nil {
			t.Errorf("expected key %q to be back-filled as null, got %v", key, v)
		}
	}

	// maxCostUsd is genuinely optional on the wire — must stay omitted.
	if _, present := got["maxCostUsd"]; present {
		t.Error("maxCostUsd must stay omitted when unset")
	}
	// Present values must be left alone.
	if got["image"] != "img:latest" {
		t.Errorf("image clobbered: %v", got["image"])
	}
}

func TestEncodeRuntimePreservesExplicitValues(t *testing.T) {
	t.Parallel()

	got, ok := encodeRuntime(map[string]any{
		"image":       "img:latest",
		"command":     []any{"run"},
		"credentials": []any{map[string]any{"key": "K"}},
		"ttlHours":    float64(4),
	}).(map[string]any)
	if !ok {
		t.Fatal("expected a map back")
	}
	if cmd, ok := got["command"].([]any); !ok || len(cmd) != 1 {
		t.Errorf("command clobbered: %v", got["command"])
	}
	if got["ttlHours"] != float64(4) {
		t.Errorf("ttlHours clobbered: %v", got["ttlHours"])
	}
}

func TestRuntimeFromResponse(t *testing.T) {
	t.Parallel()

	var diags diag.Diagnostics

	// Absent and null runtime fields both map to no dedicated runtime.
	if m := runtimeFromResponse(t.Context(), []byte(`{"id":"x"}`), &diags); m != nil {
		t.Error("expected nil model when the field is absent")
	}
	if m := runtimeFromResponse(t.Context(), []byte(`{"runtime":null}`), &diags); m != nil {
		t.Error("expected nil model when the field is null")
	}

	body := []byte(`{"runtime":{
		"image":"img:latest",
		"command":["run-worker"],
		"inferenceProtocol":"anthropic",
		"backend":"kubernetes",
		"steerMode":"tmux_keys",
		"privileged":true,
		"resources":{"cpuRequest":"2","memoryRequest":"4Gi","cpuLimit":null,"memoryLimit":"20Gi"},
		"environment":null,
		"credentials":[{"key":"GITHUB_TOKEN","scope":"per_user","credentialId":"github","label":"GitHub token","description":null,"required":true}],
		"ttlHours":null,
		"maxCostUsd":25,
		"idleTimeoutMinutes":null
	}}`)
	m := runtimeFromResponse(t.Context(), body, &diags)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if m == nil {
		t.Fatal("expected a model")
	}
	if m.Image.ValueString() != "img:latest" || m.SteerMode.ValueString() != "tmux_keys" || !m.Privileged.ValueBool() {
		t.Errorf("scalar round-trip broken: %+v", m)
	}
	if m.Command.IsNull() || len(m.Command.Elements()) != 1 {
		t.Errorf("command round-trip broken: %v", m.Command)
	}
	if m.Resources == nil || m.Resources.MemoryLimit.ValueString() != "20Gi" || !m.Resources.CpuLimit.IsNull() {
		t.Errorf("resources round-trip broken: %+v", m.Resources)
	}
	if !m.Environment.IsNull() {
		t.Errorf("null environment must flatten to a null list, got %+v", m.Environment)
	}
	var creds []AgentRuntimeCredentialModel
	if d := m.Credentials.ElementsAs(t.Context(), &creds, false); d.HasError() {
		t.Fatalf("credentials list conversion: %v", d)
	}
	if len(creds) != 1 || creds[0].CredentialId.ValueString() != "github" || !creds[0].Description.IsNull() {
		t.Errorf("credentials round-trip broken: %+v", creds)
	}
	if m.MaxCostUsd.ValueInt64() != 25 || !m.TtlHours.IsNull() || !m.IdleTimeoutMinutes.IsNull() {
		t.Errorf("numeric round-trip broken: %+v", m)
	}
}
