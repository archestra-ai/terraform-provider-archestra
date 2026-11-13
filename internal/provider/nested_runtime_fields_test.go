package provider

import (
	"context"
	"encoding/json"
	"math/big"
	"reflect"
	"strings"
	"testing"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// These tests drive the provider's real plan (schema defaults, validators and
// plan modifiers), then send the planned value through the request encoder,
// a port of the API's input normalization and the response reader. A field
// is drift-free when what the API stores reads back as exactly the plan.

func TestAgentNetworkPolicyPartialConfigHasNoDrift(t *testing.T) {
	for name, tc := range map[string]struct {
		policy map[string]tftypes.Value
		want   map[string]any
	}{
		"empty block": {map[string]tftypes.Value{}, map[string]any{"egressMode": "restricted", "domainPreset": "none", "allowedDomains": []any{}, "allowedCidrs": []any{}}},
		"egress off":  {map[string]tftypes.Value{"egress_mode": str("off")}, map[string]any{"egressMode": "off", "domainPreset": "none", "allowedDomains": []any{}, "allowedCidrs": []any{}}},
		"unrestricted with CIDRs": {
			map[string]tftypes.Value{"egress_mode": str("unrestricted"), "allowed_cidrs": strSet("10.0.0.0/8")},
			map[string]any{"egressMode": "unrestricted", "domainPreset": "none", "allowedDomains": []any{}, "allowedCidrs": []any{"10.0.0.0/8"}},
		},
		"restricted allowlist only": {
			map[string]tftypes.Value{"allowed_domains": strSet("*.example.com"), "domain_preset": str("package_managers")},
			map[string]any{"egressMode": "restricted", "domainPreset": "package_managers", "allowedDomains": []any{"*.example.com"}, "allowedCidrs": []any{}},
		},
	} {
		t.Run(name, func(t *testing.T) {
			r := &AgentResource{}
			planned := planCreate(t, r, "archestra_agent", map[string]tftypes.Value{
				"name":           str("policy-agent"),
				"network_policy": objectOf(t, r, "network_policy", tc.policy),
			})
			var diags diag.Diagnostics
			patch := MergePatch(t.Context(), planned, tftypes.NewValue(planned.Type(), nil), agentAttrSpec, &diags)
			if diags.HasError() {
				t.Fatal(diags)
			}
			sent := jsonRoundTrip(t, patch["networkPolicy"])
			if !reflect.DeepEqual(sent, tc.want) {
				t.Fatalf("sent %v, want %v", sent, tc.want)
			}
			stored := normalizeNetworkPolicyLikeAPI(sent.(map[string]any))
			if !reflect.DeepEqual(stored, sent) {
				t.Fatalf("API would store %v for %v", stored, sent)
			}
			body, _ := json.Marshal(map[string]any{"networkPolicy": stored})
			read := networkPolicyFromResponse(body, &diags)
			assertReadsBackAsPlanned(t, read, planned, "network_policy", diags)
		})
	}
}

func TestAgentNetworkPolicyOmittedInheritsEnvironment(t *testing.T) {
	r := &AgentResource{}
	planned := planCreate(t, r, "archestra_agent", map[string]tftypes.Value{"name": str("inherit-agent")})
	var diags diag.Diagnostics
	patch := MergePatch(t.Context(), planned, tftypes.NewValue(planned.Type(), nil), agentAttrSpec, &diags)
	if _, sent := patch["networkPolicy"]; sent || diags.HasError() {
		t.Fatalf("an omitted policy must not be sent, so older releases accept the create: %v %v", patch, diags)
	}
	if read := networkPolicyFromResponse([]byte(`{"networkPolicy":null}`), &diags); !read.IsNull() {
		t.Fatal("a stored null policy must read as null")
	}
	if read := networkPolicyFromResponse([]byte(`{}`), &diags); !read.IsNull() || diags.HasError() {
		t.Fatal("a release without the field must read as null")
	}
}

func TestAgentNetworkPolicyRemovalClearsOverride(t *testing.T) {
	r := &AgentResource{}
	prior := planCreate(t, r, "archestra_agent", map[string]tftypes.Value{
		"name": str("clear-agent"), "network_policy": objectOf(t, r, "network_policy", map[string]tftypes.Value{"egress_mode": str("off")}),
	})
	planned := planCreate(t, r, "archestra_agent", map[string]tftypes.Value{"name": str("clear-agent")})
	var diags diag.Diagnostics
	patch := MergePatch(t.Context(), planned, prior, agentAttrSpec, &diags)
	if value, sent := patch["networkPolicy"]; !sent || value != nil || diags.HasError() {
		t.Fatalf("removing the block must send networkPolicy: null to inherit again, got %v %v", patch, diags)
	}
}

func TestAgentNetworkPolicyRejectsValuesTheAPIWouldDrop(t *testing.T) {
	for name, tc := range map[string]struct {
		policy map[string]tftypes.Value
		want   string
	}{
		"domains without restricted": {map[string]tftypes.Value{"egress_mode": str("unrestricted"), "allowed_domains": strSet("api.example.com")}, "Allowed Domains Need Restricted Egress"},
		"preset without restricted":  {map[string]tftypes.Value{"egress_mode": str("off"), "domain_preset": str("common_dependencies")}, "Domain Preset Needs Restricted Egress"},
		"CIDRs with egress off":      {map[string]tftypes.Value{"egress_mode": str("off"), "allowed_cidrs": strSet("10.0.0.0/8")}, "Allowed CIDRs Need Egress"},
		"uppercase domain":           {map[string]tftypes.Value{"allowed_domains": strSet("API.example.com")}, "lowercase domain"},
		"unknown mode":               {map[string]tftypes.Value{"egress_mode": str("open")}, "value must be one of"},
	} {
		t.Run(name, func(t *testing.T) {
			r := &AgentResource{}
			diags := validateConfig(t, r, "archestra_agent", map[string]tftypes.Value{
				"name": str("bad-agent"), "network_policy": objectOf(t, r, "network_policy", tc.policy),
			})
			if !diagnosticsContain(diags, tc.want) {
				t.Fatalf("diagnostics %v do not mention %q", diags, tc.want)
			}
		})
	}
}

func TestGatewaySubagentLoadBalancingPartialConfigHasNoDrift(t *testing.T) {
	subagent := "88888888-8888-4888-8888-888888888888"
	for name, tc := range map[string]struct {
		config map[string]tftypes.Value
		want   map[string]any
	}{
		"empty block": {map[string]tftypes.Value{}, map[string]any{"enabled": false, "algorithm": "round_robin", "retries": float64(1), "fallbackEnabled": true}},
		"weighted": {
			map[string]tftypes.Value{"enabled": tftypes.NewValue(tftypes.Bool, true), "algorithm": str("weighted"),
				"weights": tftypes.NewValue(tftypes.Map{ElementType: tftypes.Number}, map[string]tftypes.Value{subagent: tftypes.NewValue(tftypes.Number, big.NewFloat(3))})},
			map[string]any{"enabled": true, "algorithm": "weighted", "weights": map[string]any{subagent: float64(3)}, "retries": float64(1), "fallbackEnabled": true},
		},
		"no retries or fallback": {
			map[string]tftypes.Value{"retries": tftypes.NewValue(tftypes.Number, big.NewFloat(0)), "fallback_enabled": tftypes.NewValue(tftypes.Bool, false)},
			map[string]any{"enabled": false, "algorithm": "round_robin", "retries": float64(0), "fallbackEnabled": false},
		},
	} {
		t.Run(name, func(t *testing.T) {
			r := &McpGatewayResource{}
			planned := planCreate(t, r, "archestra_mcp_gateway", map[string]tftypes.Value{
				"name":                    str("balanced-gateway"),
				"subagent_load_balancing": objectOf(t, r, "subagent_load_balancing", tc.config),
			})
			var diags diag.Diagnostics
			patch := MergePatch(t.Context(), planned, tftypes.NewValue(planned.Type(), nil), mcpGatewayAttrSpec, &diags)
			if diags.HasError() {
				t.Fatal(diags)
			}
			sent := jsonRoundTrip(t, patch["subagentLoadBalancing"])
			if !reflect.DeepEqual(sent, tc.want) {
				t.Fatalf("sent %v, want %v", sent, tc.want)
			}
			body, _ := json.Marshal(map[string]any{"subagentLoadBalancing": fillLoadBalancingDefaultsLikeAPI(sent.(map[string]any))})
			read := subagentLoadBalancingFromResponse(body, &diags)
			assertReadsBackAsPlanned(t, read, planned, "subagent_load_balancing", diags)
		})
	}
}

func TestGatewaySubagentLoadBalancingOmittedOrInvalid(t *testing.T) {
	r := &McpGatewayResource{}
	planned := planCreate(t, r, "archestra_mcp_gateway", map[string]tftypes.Value{"name": str("plain-gateway")})
	var diags diag.Diagnostics
	patch := MergePatch(t.Context(), planned, tftypes.NewValue(planned.Type(), nil), mcpGatewayAttrSpec, &diags)
	if _, sent := patch["subagentLoadBalancing"]; sent {
		t.Fatal("an omitted setting must not be sent, so older releases accept the create")
	}
	if read := subagentLoadBalancingFromResponse([]byte(`{"subagentLoadBalancing":null}`), &diags); !read.IsNull() {
		t.Fatal("a gateway without the setting must read as null")
	}
	for name, tc := range map[string]struct {
		config map[string]tftypes.Value
		want   string
	}{
		"too many retries": {map[string]tftypes.Value{"retries": tftypes.NewValue(tftypes.Number, big.NewFloat(4))}, "between 0 and 3"},
		"weight key not a UUID": {map[string]tftypes.Value{"weights": tftypes.NewValue(tftypes.Map{ElementType: tftypes.Number},
			map[string]tftypes.Value{"planner": tftypes.NewValue(tftypes.Number, big.NewFloat(2))})}, "canonical subagent UUID"},
		"weight zero": {map[string]tftypes.Value{"weights": tftypes.NewValue(tftypes.Map{ElementType: tftypes.Number},
			map[string]tftypes.Value{"88888888-8888-4888-8888-888888888888": tftypes.NewValue(tftypes.Number, big.NewFloat(0))})}, "between 1 and 1000"},
	} {
		t.Run(name, func(t *testing.T) {
			diags := validateConfig(t, r, "archestra_mcp_gateway", map[string]tftypes.Value{
				"name": str("bad-gateway"), "subagent_load_balancing": objectOf(t, r, "subagent_load_balancing", tc.config),
			})
			if !diagnosticsContain(diags, tc.want) {
				t.Fatalf("diagnostics %v do not mention %q", diags, tc.want)
			}
		})
	}
}

func TestRegistryEntryDesktopConfig(t *testing.T) {
	r := &MCPServerRegistryResource{}
	desktop := objectOf(t, r, "desktop_config", map[string]tftypes.Value{"command": str("npx -y @acme/mcp-server")})
	planned := planCreate(t, r, "archestra_mcp_registry_entry", map[string]tftypes.Value{"name": str("desktop-server"), "desktop_config": desktop})
	var diags diag.Diagnostics
	patch := MergePatch(t.Context(), planned, tftypes.NewValue(planned.Type(), nil), catalogItemAttrSpec, &diags)
	if got := jsonRoundTrip(t, patch["desktopConfig"]); !reflect.DeepEqual(got, map[string]any{"command": "npx -y @acme/mcp-server"}) || diags.HasError() {
		t.Fatalf("desktopConfig = %v (%v)", got, diags)
	}

	local := planCreate(t, r, "archestra_mcp_registry_entry", map[string]tftypes.Value{"name": str("local-server"),
		"local_config": objectOf(t, r, "local_config", map[string]tftypes.Value{"command": str("node")})})
	patch = MergePatch(t.Context(), local, tftypes.NewValue(local.Type(), nil), catalogItemAttrSpec, &diags)
	if _, sent := patch["desktopConfig"]; sent {
		t.Fatal("a local entry must not send desktopConfig, so older releases accept it")
	}

	command := "npx -y @acme/mcp-server"
	response := &client.GetInternalMcpCatalogItemResponse{}
	if err := json.Unmarshal([]byte(`{"serverType":"desktop_mcp","desktopConfig":{"command":"`+command+`","url":null}}`), &response.JSON200); err != nil {
		t.Fatal(err)
	}
	assertReadsBackAsPlanned(t, desktopConfigFromResponse(response), planned, "desktop_config", diags)
	response.JSON200.ServerType = "local"
	if !desktopConfigFromResponse(response).IsNull() {
		t.Fatal("only a desktop MCP entry reads desktop_config")
	}

	for name, tc := range map[string]struct {
		attrs map[string]tftypes.Value
		want  string
	}{
		"command and url": {map[string]tftypes.Value{"desktop_config": objectOf(t, r, "desktop_config", map[string]tftypes.Value{"command": str("node"), "url": str("http://localhost:3333/mcp")})}, "not both"},
		"neither":         {map[string]tftypes.Value{"desktop_config": objectOf(t, r, "desktop_config", map[string]tftypes.Value{})}, "set command or url"},
		"bad url":         {map[string]tftypes.Value{"desktop_config": objectOf(t, r, "desktop_config", map[string]tftypes.Value{"url": str("localhost:3333")})}, "http:// or https://"},
		"with remote": {map[string]tftypes.Value{"desktop_config": desktop,
			"remote_config": objectOf(t, r, "remote_config", map[string]tftypes.Value{"url": str("https://mcp.example.com/mcp")})}, "only one of local_config, remote_config or desktop_config"},
	} {
		t.Run(name, func(t *testing.T) {
			tc.attrs["name"] = str("bad-desktop")
			if diags := validateConfig(t, r, "archestra_mcp_registry_entry", tc.attrs); !diagnosticsContain(diags, tc.want) {
				t.Fatalf("diagnostics %v do not mention %q", diags, tc.want)
			}
		})
	}
}

// === helpers ===

// normalizeNetworkPolicyLikeAPI ports normalizeNetworkPolicy from the
// platform's types/environment.ts.
func normalizeNetworkPolicyLikeAPI(in map[string]any) map[string]any {
	mode, _ := in["egressMode"].(string)
	if mode == "" {
		mode = "restricted"
	}
	or := func(key string, fallback any) any {
		if v, ok := in[key]; ok && v != nil {
			return v
		}
		return fallback
	}
	out := map[string]any{"egressMode": mode, "domainPreset": "none", "allowedDomains": []any{}, "allowedCidrs": []any{}}
	if mode == "restricted" {
		out["domainPreset"] = or("domainPreset", "none")
		out["allowedDomains"] = or("allowedDomains", []any{})
	}
	if mode != "off" {
		out["allowedCidrs"] = or("allowedCidrs", []any{})
	}
	return out
}

// fillLoadBalancingDefaultsLikeAPI applies SubagentLoadBalancingConfigSchema's
// defaults from the platform's shared/subagent-load-balancing.ts.
func fillLoadBalancingDefaultsLikeAPI(in map[string]any) map[string]any {
	out := map[string]any{"enabled": false, "algorithm": "round_robin", "retries": float64(1), "fallbackEnabled": true}
	for k, v := range in {
		out[k] = v
	}
	return out
}

func str(s string) tftypes.Value { return tftypes.NewValue(tftypes.String, s) }

func strSet(values ...string) tftypes.Value {
	items := make([]tftypes.Value, 0, len(values))
	for _, v := range values {
		items = append(items, str(v))
	}
	return tftypes.NewValue(tftypes.Set{ElementType: tftypes.String}, items)
}

func schemaObjectType(t *testing.T, r resource.Resource) tftypes.Object {
	t.Helper()
	var s resource.SchemaResponse
	r.Schema(context.Background(), resource.SchemaRequest{}, &s)
	if s.Diagnostics.HasError() {
		t.Fatal(s.Diagnostics)
	}
	return s.Schema.Type().TerraformType(context.Background()).(tftypes.Object)
}

// objectOf builds a nested attribute value with the given children set and
// every other child null, as an omitted HCL argument would be.
func objectOf(t *testing.T, r resource.Resource, attribute string, set map[string]tftypes.Value) tftypes.Value {
	t.Helper()
	typ := schemaObjectType(t, r).AttributeTypes[attribute].(tftypes.Object)
	return nullFilled(typ, set)
}

func nullFilled(typ tftypes.Object, set map[string]tftypes.Value) tftypes.Value {
	values := map[string]tftypes.Value{}
	for name, attrType := range typ.AttributeTypes {
		if v, ok := set[name]; ok {
			values[name] = v
		} else {
			values[name] = tftypes.NewValue(attrType, nil)
		}
	}
	return tftypes.NewValue(typ, values)
}

func providerServer(t *testing.T) tfprotov6.ProviderServer {
	t.Helper()
	server, err := testAccProtoV6ProviderFactories["archestra"]()
	if err != nil {
		t.Fatal(err)
	}
	return server
}

func validateConfig(t *testing.T, r resource.Resource, typeName string, attrs map[string]tftypes.Value) []*tfprotov6.Diagnostic {
	t.Helper()
	typ := schemaObjectType(t, r)
	config, err := tfprotov6.NewDynamicValue(typ, nullFilled(typ, attrs))
	if err != nil {
		t.Fatal(err)
	}
	resp, err := providerServer(t).ValidateResourceConfig(t.Context(), &tfprotov6.ValidateResourceConfigRequest{TypeName: typeName, Config: &config})
	if err != nil {
		t.Fatal(err)
	}
	return resp.Diagnostics
}

// planCreate validates a configuration and returns the provider's planned
// state for creating it.
func planCreate(t *testing.T, r resource.Resource, typeName string, attrs map[string]tftypes.Value) tftypes.Value {
	t.Helper()
	if diags := validateConfig(t, r, typeName, attrs); len(diags) > 0 {
		for _, d := range diags {
			if d.Severity == tfprotov6.DiagnosticSeverityError {
				t.Fatalf("configuration rejected: %s: %s", d.Summary, d.Detail)
			}
		}
	}
	typ := schemaObjectType(t, r)
	config, err := tfprotov6.NewDynamicValue(typ, nullFilled(typ, attrs))
	if err != nil {
		t.Fatal(err)
	}
	prior, err := tfprotov6.NewDynamicValue(typ, tftypes.NewValue(typ, nil))
	if err != nil {
		t.Fatal(err)
	}
	resp, err := providerServer(t).PlanResourceChange(t.Context(), &tfprotov6.PlanResourceChangeRequest{
		TypeName: typeName, PriorState: &prior, Config: &config, ProposedNewState: &config,
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range resp.Diagnostics {
		if d.Severity == tfprotov6.DiagnosticSeverityError {
			t.Fatalf("plan failed: %s: %s", d.Summary, d.Detail)
		}
	}
	planned, err := resp.PlannedState.Unmarshal(typ)
	if err != nil {
		t.Fatal(err)
	}
	return planned
}

func assertReadsBackAsPlanned(t *testing.T, read types.Object, planned tftypes.Value, attribute string, diags diag.Diagnostics) {
	t.Helper()
	if diags.HasError() {
		t.Fatal(diags)
	}
	var fields map[string]tftypes.Value
	if err := planned.As(&fields); err != nil {
		t.Fatal(err)
	}
	got, err := read.ToTerraformValue(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !got.Equal(fields[attribute]) {
		t.Fatalf("%s reads back as %s, but the plan is %s: Terraform would show a diff", attribute, got, fields[attribute])
	}
}

func jsonRoundTrip(t *testing.T, v any) any {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var out any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func diagnosticsContain(diags []*tfprotov6.Diagnostic, text string) bool {
	for _, d := range diags {
		if d.Severity == tfprotov6.DiagnosticSeverityError && strings.Contains(d.Summary+" "+d.Detail, text) {
			return true
		}
	}
	return false
}
