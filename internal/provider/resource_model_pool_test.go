package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
)

const poolTestID = "11111111-1111-4111-8111-111111111111"
const poolKeyID = "22222222-2222-4222-8222-222222222222"
const poolVirtualKeyID = "33333333-3333-4333-8333-333333333333"
const poolTestState = `{"id":"` + poolTestID + `","name":"balanced-model","members":[{"provider_api_key_id":"` + poolKeyID + `","model_id":"gpt-4.1","priority":1,"weight":3,"enabled":true}],"virtual_api_key_ids":["` + poolVirtualKeyID + `"]}`

func poolFixtureState(t *testing.T, r *ModelPoolResource, values string) tfsdk.State {
	t.Helper()
	var schema resource.SchemaResponse
	r.Schema(t.Context(), resource.SchemaRequest{}, &schema)
	raw, err := projectStateJSON([]byte(values), schema.Schema.Type().TerraformType(t.Context()))
	if err != nil {
		t.Fatal(err)
	}
	value, err := (tfprotov6.RawState{JSON: raw}).Unmarshal(schema.Schema.Type().TerraformType(t.Context()))
	if err != nil {
		t.Fatal(err)
	}
	return tfsdk.State{Schema: schema.Schema, Raw: value}
}
func poolFixtureResource(t *testing.T, handler http.HandlerFunc) *ModelPoolResource {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path == "/prefix/openapi.json" {
			writePoolJSON(w, modelPoolContractFixture())
			return
		}
		handler(w, req)
	}))
	t.Cleanup(server.Close)
	api, err := client.NewClientWithResponses(server.URL+"/prefix/", client.WithHTTPClient(server.Client()), client.WithRequestEditorFn(func(_ context.Context, r *http.Request) error {
		r.Header.Set("Authorization", "fixture-key")
		return nil
	}))
	if err != nil {
		t.Fatal(err)
	}
	pool := &ModelPoolResource{}
	var configured resource.ConfigureResponse
	pool.Configure(t.Context(), resource.ConfigureRequest{ProviderData: api}, &configured)
	if configured.Diagnostics.HasError() {
		t.Fatal(configured.Diagnostics)
	}
	return pool
}
func poolWire() map[string]any {
	return map[string]any{"id": poolTestID, "name": "balanced-model", "members": []any{map[string]any{"id": "ephemeral-member-id", "providerApiKeyId": poolKeyID, "providerApiKeyName": "shared-key", "provider": "openai", "modelId": "gpt-4.1", "priority": 1, "weight": 3, "enabled": true, "benchedUntil": nil, "benchReason": nil}}, "virtualApiKeyIds": []string{poolVirtualKeyID}}
}
func writePoolJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(value)
}

func TestModelPoolLifecycle(t *testing.T) {
	var remote = poolWire()
	writes, deletes := 0, 0
	r := poolFixtureResource(t, func(w http.ResponseWriter, req *http.Request) {
		if req.Header.Get("Authorization") != "fixture-key" {
			t.Error("configured auth was lost")
		}
		if !strings.HasPrefix(req.URL.Path, "/prefix/api/model-pools") {
			t.Errorf("lost base path: %s", req.URL.Path)
		}
		switch req.Method {
		case http.MethodPost, http.MethodPut:
			writes++
			var body map[string]any
			if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
				t.Error(err)
			}
			if len(body) != 2 || body["members"] == nil || body["virtualApiKeyIds"] != nil {
				t.Errorf("replacement omitted settings: %#v", body)
			}
			if req.Header.Get("Content-Type") != "application/json" {
				t.Error("missing JSON content type")
			}
			if req.Method == http.MethodPut && req.URL.Path != "/prefix/api/model-pools/"+poolTestID {
				t.Error("update lost pool identity")
			}
			grants := remote["virtualApiKeyIds"]
			remote = body
			remote["virtualApiKeyIds"] = grants
			remote["id"] = poolTestID
			writePoolJSON(w, remote)
		case http.MethodGet:
			if remote == nil {
				writePoolJSON(w, []any{})
				return
			}
			writePoolJSON(w, []any{remote})
		case http.MethodDelete:
			deletes++
			remote = nil
			writePoolJSON(w, map[string]bool{"success": true})
		default:
			t.Errorf("unexpected method %s", req.Method)
		}
	})
	state := poolFixtureState(t, r, poolTestState)
	var created resource.CreateResponse
	created.State = state
	r.Create(t.Context(), resource.CreateRequest{Plan: tfsdk.Plan(state)}, &created)
	if created.Diagnostics.HasError() {
		t.Fatal(created.Diagnostics)
	}
	var imported resource.ImportStateResponse
	imported.State = poolFixtureState(t, r, `{}`)
	r.ImportState(t.Context(), resource.ImportStateRequest{ID: poolTestID}, &imported)
	if imported.Diagnostics.HasError() {
		t.Fatal(imported.Diagnostics)
	}
	var read resource.ReadResponse
	read.State = imported.State
	r.Read(t.Context(), resource.ReadRequest{State: imported.State}, &read)
	if read.Diagnostics.HasError() || !read.State.Raw.Equal(created.State.Raw) {
		t.Fatalf("import failed to adopt the existing pool: %v", read.Diagnostics)
	}
	if writes != 1 {
		t.Fatal("import wrote to remote pool")
	}
	remote["name"] = "edited-in-ui"
	var drift resource.ReadResponse
	drift.State = read.State
	r.Read(t.Context(), resource.ReadRequest{State: read.State}, &drift)
	var name string
	drift.Diagnostics.Append(drift.State.GetAttribute(t.Context(), path.Root("name"), &name)...)
	if drift.Diagnostics.HasError() || name != "edited-in-ui" {
		t.Fatalf("out-of-band name was hidden: %s %v", name, drift.Diagnostics)
	}
	updatedPlan := poolFixtureState(t, r, strings.Replace(poolTestState, `"weight":3`, `"weight":7`, 1))
	var updated resource.UpdateResponse
	updated.State = drift.State
	r.Update(t.Context(), resource.UpdateRequest{Plan: tfsdk.Plan(updatedPlan), State: drift.State}, &updated)
	if updated.Diagnostics.HasError() || !updated.State.Raw.Equal(updatedPlan.Raw) {
		t.Fatalf("full replacement failed: %v", updated.Diagnostics)
	}
	if grants, ok := remote["virtualApiKeyIds"].([]string); !ok || len(grants) != 1 || grants[0] != poolVirtualKeyID {
		t.Fatal("updating pool settings changed credential-owned grants")
	}
	var deleted resource.DeleteResponse
	r.Delete(t.Context(), resource.DeleteRequest{State: updated.State}, &deleted)
	if deleted.Diagnostics.HasError() || deletes != 1 {
		t.Fatalf("delete failed: %v", deleted.Diagnostics)
	}
	r.Delete(t.Context(), resource.DeleteRequest{State: updated.State}, &deleted)
	if deleted.Diagnostics.HasError() || deletes != 1 {
		t.Fatal("missing pool deletion was not idempotent")
	}
	var missing resource.ReadResponse
	missing.State = updated.State
	r.Read(t.Context(), resource.ReadRequest{State: updated.State}, &missing)
	if missing.Diagnostics.HasError() || !missing.State.Raw.IsNull() {
		t.Fatal("missing pool retained in state")
	}
}

func TestModelPoolReadFailurePreservesState(t *testing.T) {
	for _, body := range []string{"null", "{}", `[{"id":"` + poolTestID + `","name":"incomplete"}]`, "not-json", "404", "403", "500"} {
		t.Run(body, func(t *testing.T) {
			r := poolFixtureResource(t, func(w http.ResponseWriter, req *http.Request) {
				if req.Method != "GET" {
					t.Fatal("read failure must prevent deletion")
				}
				switch body {
				case "404":
					w.WriteHeader(404)
				case "403":
					w.WriteHeader(403)
				case "500":
					w.WriteHeader(500)
				}
				fmt.Fprint(w, body)
			})
			state := poolFixtureState(t, r, poolTestState)
			read := resource.ReadResponse{State: state}
			r.Read(t.Context(), resource.ReadRequest{State: state}, &read)
			if !read.Diagnostics.HasError() || !read.State.Raw.Equal(state.Raw) {
				t.Fatalf("failed read dropped or rewrote state: %v", read.Diagnostics)
			}
			if strings.HasPrefix(body, "[{") {
				return
			}
			var deleted resource.DeleteResponse
			r.Delete(t.Context(), resource.DeleteRequest{State: state}, &deleted)
			if !deleted.Diagnostics.HasError() {
				t.Fatal("failed list treated as successful deletion")
			}
		})
	}
}

func TestModelPoolWriteFailuresKeepPriorState(t *testing.T) {
	for _, status := range []int{400, 401, 403, 404, 409, 500} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			r := poolFixtureResource(t, func(w http.ResponseWriter, req *http.Request) {
				w.WriteHeader(status)
				fmt.Fprint(w, `{"error":"sensitive-upstream-value"}`)
			})
			state := poolFixtureState(t, r, poolTestState)
			updated := resource.UpdateResponse{State: state}
			r.Update(t.Context(), resource.UpdateRequest{Plan: tfsdk.Plan(state), State: state}, &updated)
			if !updated.Diagnostics.HasError() || !updated.State.Raw.Equal(state.Raw) {
				t.Fatal("failed update changed state")
			}
			if strings.Contains(fmt.Sprint(updated.Diagnostics), "sensitive-upstream-value") {
				t.Fatal("diagnostics leaked upstream body")
			}
		})
	}
}

func TestModelPoolDuplicateValidationAndImport(t *testing.T) {
	r := &ModelPoolResource{}
	for _, id := range []string{"balanced-model", "../model-pools", strings.ToUpper(poolTestID[:8]) + "INVALID"} {
		resp := resource.ImportStateResponse{State: poolFixtureState(t, r, `{}`)}
		r.ImportState(t.Context(), resource.ImportStateRequest{ID: id}, &resp)
		if !resp.Diagnostics.HasError() {
			t.Fatalf("accepted invalid import %q", id)
		}
	}
	for _, key := range []string{poolKeyID, "44444444-4444-4444-8444-444444444444"} {
		members := fmt.Sprintf(`{"id":"%s","name":"balanced","members":[{"provider_api_key_id":"%s","model_id":"upstream","priority":1,"weight":1,"enabled":true},{"provider_api_key_id":"%s","model_id":"upstream","priority":2,"weight":2,"enabled":false}],"virtual_api_key_ids":[]}`, poolTestID, poolKeyID, key)
		state := poolFixtureState(t, r, members)
		var validated resource.ValidateConfigResponse
		r.ValidateConfig(t.Context(), resource.ValidateConfigRequest{Config: tfsdk.Config(state)}, &validated)
		if validated.Diagnostics.HasError() != (key == poolKeyID) {
			t.Fatalf("duplicate validation: %v", validated.Diagnostics)
		}
	}
}

func TestModelPoolReadRecordsCascadeDeletedMembers(t *testing.T) {
	remote := poolWire()
	remote["members"] = []any{}
	r := poolFixtureResource(t, func(w http.ResponseWriter, req *http.Request) {
		writePoolJSON(w, []any{remote})
	})
	state := poolFixtureState(t, r, poolTestState)
	read := resource.ReadResponse{State: state}
	r.Read(t.Context(), resource.ReadRequest{State: state}, &read)
	if read.Diagnostics.HasError() {
		t.Fatal(read.Diagnostics)
	}
	var data ModelPoolResourceModel
	if d := read.State.Get(t.Context(), &data); d.HasError() {
		t.Fatal(d)
	}
	if data.ID.ValueString() != poolTestID || len(data.Members.Elements()) != 0 {
		t.Fatal("deleting the last provider key must expose an empty pool for drift repair")
	}
}

func TestModelPoolCreateRetainsIdentityAfterMappingFailure(t *testing.T) {
	r := poolFixtureResource(t, func(w http.ResponseWriter, req *http.Request) {
		writePoolJSON(w, map[string]any{"id": poolTestID})
	})
	state := poolFixtureState(t, r, strings.Replace(poolTestState, `"id":"`+poolTestID+`",`, "", 1))
	created := resource.CreateResponse{State: state}
	r.Create(t.Context(), resource.CreateRequest{Plan: tfsdk.Plan(state)}, &created)
	if !created.Diagnostics.HasError() {
		t.Fatal("incomplete response must fail mapping")
	}
	var id string
	if d := created.State.GetAttribute(t.Context(), path.Root("id"), &id); d.HasError() {
		t.Fatal(d)
	}
	if id != poolTestID {
		t.Fatal("successful creation lost its identity after mapping failed")
	}
}

func TestModelPoolPlanValidation(t *testing.T) {
	server := providerserver.NewProtocol6(New("test")())()
	for _, tc := range []struct{ name, config string }{
		{"valid", poolTestState},
		{"priority-zero", strings.Replace(poolTestState, `"priority":1`, `"priority":0`, 1)},
		{"priority-over-limit", strings.Replace(poolTestState, `"priority":1`, `"priority":101`, 1)},
		{"weight-zero", strings.Replace(poolTestState, `"weight":3`, `"weight":0`, 1)},
		{"weight-over-limit", strings.Replace(poolTestState, `"weight":3`, `"weight":1001`, 1)},
		{"name-trim", strings.Replace(poolTestState, `"balanced-model"`, `" balanced-model "`, 1)},
		{"empty-model", strings.Replace(poolTestState, `"gpt-4.1"`, `""`, 1)},
		{"invalid-provider-reference", strings.Replace(poolTestState, poolKeyID, "not-a-uuid", 1)},
		{"read-only-grants", poolTestState},
		{"no-members", `{"name":"balanced-model","members":[],"virtual_api_key_ids":[]}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var fields map[string]any
			if err := json.Unmarshal([]byte(tc.config), &fields); err != nil {
				t.Fatal(err)
			}
			delete(fields, "id")
			if tc.name != "read-only-grants" {
				delete(fields, "virtual_api_key_ids")
			}
			encoded, err := json.Marshal(fields)
			if err != nil {
				t.Fatal(err)
			}
			config := string(encoded)
			state := poolFixtureState(t, &ModelPoolResource{}, config)
			value, err := tfprotov6.NewDynamicValue(state.Raw.Type(), state.Raw)
			if err != nil {
				t.Fatal(err)
			}
			response, err := server.ValidateResourceConfig(t.Context(), &tfprotov6.ValidateResourceConfigRequest{TypeName: "archestra_model_pool", Config: &value})
			if err != nil {
				t.Fatal(err)
			}
			hasError := false
			for _, diagnostic := range response.Diagnostics {
				if diagnostic.Severity == tfprotov6.DiagnosticSeverityError {
					hasError = true
				}
			}
			if hasError != (tc.name != "valid") {
				t.Fatalf("unexpected validation result: %v", response.Diagnostics)
			}
		})
	}
}

func TestModelPoolReadIgnoresMemberOrderAndReportsRoutingDrift(t *testing.T) {
	var source map[string]any
	if err := json.Unmarshal([]byte(poolTestState), &source); err != nil {
		t.Fatal(err)
	}
	members, ok := source["members"].([]any)
	if !ok {
		t.Fatal("fixture members are not an array")
	}
	second := map[string]any{"provider_api_key_id": "44444444-4444-4444-8444-444444444444", "model_id": "backup", "priority": 2, "weight": 1, "enabled": false}
	source["members"] = append(members, second)
	stateJSON, err := json.Marshal(source)
	if err != nil {
		t.Fatal(err)
	}
	remote := poolWire()
	backup := map[string]any{"providerApiKeyId": second["provider_api_key_id"], "modelId": "backup", "priority": 2, "weight": 1, "enabled": false}
	remoteMembers, ok := remote["members"].([]any)
	if !ok {
		t.Fatal("fixture members are not an array")
	}
	remote["members"] = []any{backup, remoteMembers[0]}
	r := poolFixtureResource(t, func(w http.ResponseWriter, req *http.Request) { writePoolJSON(w, []any{remote}) })
	state := poolFixtureState(t, r, string(stateJSON))
	read := resource.ReadResponse{State: state}
	r.Read(t.Context(), resource.ReadRequest{State: state}, &read)
	if read.Diagnostics.HasError() || !read.State.Raw.Equal(state.Raw) {
		t.Fatalf("API member ordering caused drift: %v", read.Diagnostics)
	}
	backup["priority"], backup["weight"], backup["enabled"] = 1, 10, true
	r.Read(t.Context(), resource.ReadRequest{State: state}, &read)
	if read.Diagnostics.HasError() || read.State.Raw.Equal(state.Raw) {
		t.Fatalf("API routing changes were hidden: %v", read.Diagnostics)
	}
}

func modelPoolContractFixture() map[string]any {
	op := map[string]any{"requestBody": map[string]any{"content": map[string]any{"application/json": map[string]any{"schema": map[string]any{"properties": map[string]any{"name": map[string]any{}, "members": map[string]any{}}}}}}}
	return map[string]any{"paths": map[string]any{"/api/model-pools": map[string]any{"post": op}, "/api/model-pools/{id}": map[string]any{"put": op}}}
}
