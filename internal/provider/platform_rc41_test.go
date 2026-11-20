package provider

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
)

func rc41State(t *testing.T, r resource.Resource, values string) tfsdk.State {
	t.Helper()
	var schema resource.SchemaResponse
	r.Schema(t.Context(), resource.SchemaRequest{}, &schema)
	typ := schema.Schema.Type().TerraformType(t.Context())
	raw, err := projectStateJSON([]byte(values), typ)
	if err != nil {
		t.Fatal(err)
	}
	value, err := (tfprotov6.RawState{JSON: raw}).Unmarshal(typ)
	if err != nil {
		t.Fatal(err)
	}
	return tfsdk.State{Schema: schema.Schema, Raw: value}
}

func rc41Client(t *testing.T, handler http.HandlerFunc) *client.ClientWithResponses {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	api, err := client.NewClientWithResponses(server.URL, client.WithHTTPClient(server.Client()))
	if err != nil {
		t.Fatal(err)
	}
	return api
}

func TestMcpRuntimeTiersLifecycle(t *testing.T) {
	const orgID = "44444444-4444-4444-8444-444444444444"
	// The API returns tiers smallest first, with derived memory and explicit overrides.
	resolved := `{"tiers":[
		{"name":"small","cpuRequest":0.1,"cpuLimit":null,"memoryRequestMiB":205,"memoryLimitMiB":410},
		{"name":"large","cpuRequest":2,"cpuLimit":4,"memoryRequestMiB":2048,"memoryLimitMiB":8192,"memoryRequestMiBOverride":2048}]}`
	var bodies []string
	api := rc41Client(t, func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case req.URL.Path == "/api/organization" && req.Method == http.MethodGet:
			_, _ = io.WriteString(w, `{"id":"`+orgID+`"}`)
		case req.URL.Path == "/api/organization/mcp-runtime-tiers" && req.Method == http.MethodPatch:
			body, _ := io.ReadAll(req.Body)
			bodies = append(bodies, string(body))
			_, _ = io.WriteString(w, resolved)
		case req.URL.Path == "/api/organization/mcp-runtime-tiers" && req.Method == http.MethodGet:
			_, _ = io.WriteString(w, resolved)
		default:
			t.Errorf("unexpected %s %s", req.Method, req.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	})
	r := &McpRuntimeTiersResource{client: api}
	plan := rc41State(t, r, `{"tiers":[{"name":"large","cpu_request":2,"cpu_limit":4,"memory_request_mib":2048},{"name":"small","cpu_request":0.1}]}`)
	var created resource.CreateResponse
	created.State = plan
	r.Create(t.Context(), resource.CreateRequest{Plan: tfsdk.Plan(plan)}, &created)
	if created.Diagnostics.HasError() {
		t.Fatal(created.Diagnostics)
	}
	// Unset limits are sent as null: an omitted memory value keeps the old override.
	want := `{"tiers":[{"cpuLimit":4,"cpuRequest":2,"memoryLimitMiB":null,"memoryRequestMiB":2048,"name":"large"},{"cpuLimit":null,"cpuRequest":0.1,"memoryLimitMiB":null,"memoryRequestMiB":null,"name":"small"}]}`
	if len(bodies) != 1 || bodies[0] != want {
		t.Fatalf("request body = %v, want %s", bodies, want)
	}
	var model McpRuntimeTiersResourceModel
	created.Diagnostics.Append(created.State.Get(t.Context(), &model)...)
	var tiers []mcpRuntimeTierModel
	created.Diagnostics.Append(model.Tiers.ElementsAs(t.Context(), &tiers, false)...)
	if created.Diagnostics.HasError() {
		t.Fatal(created.Diagnostics)
	}
	if model.ID.ValueString() != orgID || len(tiers) != 2 || tiers[0].Name.ValueString() != "large" {
		t.Fatalf("configured order or identity lost: %v %v", model.ID, tiers)
	}
	if tiers[0].MemoryRequestMiB.ValueInt64() != 2048 || !tiers[0].MemoryLimitMiB.IsNull() || tiers[0].EffectiveMemoryLimitMiB.ValueInt64() != 8192 {
		t.Errorf("large tier = %+v", tiers[0])
	}
	if !tiers[1].CPULimit.IsNull() || !tiers[1].MemoryRequestMiB.IsNull() || tiers[1].EffectiveMemoryRequestMiB.ValueInt64() != 205 || tiers[1].CPURequest.ValueFloat64() != 0.1 {
		t.Errorf("small tier = %+v", tiers[1])
	}
	var read resource.ReadResponse
	read.State = created.State
	r.Read(t.Context(), resource.ReadRequest{State: created.State}, &read)
	if read.Diagnostics.HasError() || !read.State.Raw.Equal(created.State.Raw) {
		t.Fatalf("refresh changed state: %v", read.Diagnostics)
	}
	var deleted resource.DeleteResponse
	r.Delete(t.Context(), resource.DeleteRequest{State: read.State}, &deleted)
	if deleted.Diagnostics.HasError() || len(bodies) != 2 || bodies[1] != "null" {
		t.Fatalf("destroy did not restore the defaults: %v %v", bodies, deleted.Diagnostics)
	}
}

func TestMcpRuntimeTiersMissingAPIExplainsRelease(t *testing.T) {
	api := rc41Client(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	r := &McpRuntimeTiersResource{client: api}
	state := rc41State(t, r, `{"id":"org","tiers":[{"name":"S","cpu_request":0.1}]}`)
	var read resource.ReadResponse
	read.State = state
	r.Read(t.Context(), resource.ReadRequest{State: state}, &read)
	if !read.Diagnostics.HasError() || read.Diagnostics.Errors()[0].Summary() != "MCP Runtime Tiers Unavailable" {
		t.Fatalf("diagnostics = %v", read.Diagnostics)
	}
}

func TestOauthClientFederatedCredentialIDs(t *testing.T) {
	client, credential, err := oauthClientFederatedCredentialIDs("client-1:55555555-5555-4555-8555-555555555555")
	if err != nil || client != "client-1" || credential.String() != "55555555-5555-4555-8555-555555555555" {
		t.Fatalf("got %q %v %v", client, credential, err)
	}
	for _, id := range []string{"", "client-1", ":55555555-5555-4555-8555-555555555555", "client-1:not-a-uuid"} {
		if _, _, err := oauthClientFederatedCredentialIDs(id); err == nil {
			t.Errorf("%q was accepted", id)
		}
	}
}

func TestOauthClientFederatedCredentialReadsFromList(t *testing.T) {
	const credential = "55555555-5555-4555-8555-555555555555"
	api := rc41Client(t, func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path != "/api/oauth-clients/client-1/federated-credentials" {
			t.Errorf("unexpected path %s", req.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]map[string]any{{
			"id": credential, "oauthClientId": "client-1", "name": "ci", "issuer": "https://token.actions.githubusercontent.com",
			"audience": "https://archestra.example.com", "subject": "repo:acme/infra:ref:refs/heads/main", "claimConditions": map[string]string{},
			"lastUsedAt": nil, "createdAt": "2026-11-20T10:00:00.000Z", "updatedAt": "2026-11-20T10:00:00.000Z",
		}})
	})
	r := &OauthClientFederatedCredentialResource{client: api}
	state := rc41State(t, r, `{"id":"client-1:`+credential+`"}`)
	var read resource.ReadResponse
	read.State = state
	r.Read(t.Context(), resource.ReadRequest{State: state}, &read)
	var model OauthClientFederatedCredentialResourceModel
	read.Diagnostics.Append(read.State.Get(t.Context(), &model)...)
	if read.Diagnostics.HasError() {
		t.Fatal(read.Diagnostics)
	}
	if model.OauthClientID.ValueString() != "client-1" || model.Name.ValueString() != "ci" || len(model.ClaimConditions.Elements()) != 0 || !model.LastUsedAt.IsNull() {
		t.Fatalf("model = %+v", model)
	}
	missing := rc41State(t, r, `{"id":"client-1:66666666-6666-4666-8666-666666666666"}`)
	read = resource.ReadResponse{State: missing}
	r.Read(t.Context(), resource.ReadRequest{State: missing}, &read)
	if read.Diagnostics.HasError() || !read.State.Raw.IsNull() {
		t.Fatal("a deleted credential stayed in state")
	}
}

func TestCatalogRuntimeTierKeepsConfiguredSpelling(t *testing.T) {
	body := `{"localConfig":{"command":"npx","runtimeTier":"M"}}`
	got := mapCatalogLocalConfig(t.Context(), parseCatalogResp(t, body), types.ObjectNull(localConfigAttrTypes))
	if asString(t, got.Attributes()["runtime_tier"]).ValueString() != "M" {
		t.Fatalf("runtime_tier = %v", got.Attributes()["runtime_tier"])
	}
	configured := got.Attributes()
	configured["runtime_tier"] = types.StringValue("m")
	priorObj := types.ObjectValueMust(localConfigAttrTypes, configured)
	got = mapCatalogLocalConfig(t.Context(), parseCatalogResp(t, body), priorObj)
	if asString(t, got.Attributes()["runtime_tier"]).ValueString() != "m" {
		t.Fatalf("a case-only difference became drift: %v", got.Attributes()["runtime_tier"])
	}
	configured["runtime_tier"] = types.StringValue("L")
	got = mapCatalogLocalConfig(t.Context(), parseCatalogResp(t, body), types.ObjectValueMust(localConfigAttrTypes, configured))
	if asString(t, got.Attributes()["runtime_tier"]).ValueString() != "M" {
		t.Fatalf("a different tier was hidden: %v", got.Attributes()["runtime_tier"])
	}
}

func TestRoleAcceptsGuardrailsYellPermission(t *testing.T) {
	actions := currentRoleActions["guardrailsYell"]
	if len(actions) != 2 || actions[0] != "read" || actions[1] != "admin" {
		t.Fatalf("guardrailsYell actions = %v", actions)
	}
}
