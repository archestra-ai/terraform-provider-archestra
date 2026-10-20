package provider

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

const appTestID = "11111111-1111-4111-8111-111111111111"
const appTestToolID = "22222222-2222-4222-8222-222222222222"
const appTestOtherToolID = "33333333-3333-4333-8333-333333333333"

func appTestModel(t *testing.T) AppResourceModel {
	t.Helper()
	return AppResourceModel{ID: types.StringValue(appTestID), Name: types.StringValue("App"), Slug: types.StringValue("app"), HTML: types.StringValue("<!doctype html><html><body>Original</body></html>"), BrowserPermissions: types.SetValueMust(types.StringType, []attr.Value{}), Tools: types.SetValueMust(agentInlineToolType, []attr.Value{}), Permissions: types.SetNull(permissionGrantObjectType), ContentVersion: types.Int64Value(1), Enabled: types.BoolValue(true), Locked: types.BoolValue(false), OpenInFullscreen: types.BoolValue(false), Labels: []AgentLabelModel{}}
}
func appTestTools(t *testing.T, ids ...string) types.Set {
	t.Helper()
	values := make([]agentInlineTool, 0, len(ids))
	for _, id := range ids {
		values = append(values, agentInlineTool{ToolID: types.StringValue(id), McpServerID: types.StringNull(), CredentialResolutionMode: types.StringValue("static")})
	}
	set, d := types.SetValueFrom(t.Context(), agentInlineToolType, values)
	if d.HasError() {
		t.Fatal(d)
	}
	return set
}
func appTestAPI(t *testing.T, handler http.HandlerFunc) *AppResource {
	t.Helper()
	s := httptest.NewServer(handler)
	t.Cleanup(s.Close)
	api, err := client.NewClientWithResponses(s.URL)
	if err != nil {
		t.Fatal(err)
	}
	return &AppResource{client: api}
}
func appTestSnapshot(t *testing.T, w http.ResponseWriter, req *http.Request, version int, toolIDs ...string) bool {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	switch {
	case req.Method == "GET" && req.URL.Path == "/api/apps/"+appTestID:
		_ = json.NewEncoder(w).Encode(map[string]any{"id": appTestID, "name": "App", "slug": "app", "latestVersion": version, "enabled": true, "locked": false, "openInFullscreen": false, "labels": []any{}})
		return true
	case req.Method == "GET" && strings.Contains(req.URL.Path, "/versions/"):
		_ = json.NewEncoder(w).Encode(map[string]any{"appId": appTestID, "version": version, "html": "<!doctype html><html><body>Original</body></html>", "uiPermissions": nil})
		return true
	case req.Method == "GET" && req.URL.Path == "/api/apps/"+appTestID+"/tools":
		rows := make([]any, 0, len(toolIDs))
		for _, id := range toolIDs {
			rows = append(rows, map[string]any{"id": id, "mcpServerId": nil, "credentialResolutionMode": "static"})
		}
		_ = json.NewEncoder(w).Encode(rows)
		return true
	}
	return false
}
func appTestStatePlan(t *testing.T, r *AppResource, prior, desired AppResourceModel) (tfsdk.State, tfsdk.Plan) {
	t.Helper()
	var s resource.SchemaResponse
	r.Schema(t.Context(), resource.SchemaRequest{}, &s)
	state := tfsdk.State{Schema: s.Schema}
	plan := tfsdk.Plan{Schema: s.Schema}
	if d := state.Set(t.Context(), &prior); d.HasError() {
		t.Fatal(d)
	}
	if d := plan.Set(t.Context(), &desired); d.HasError() {
		t.Fatal(d)
	}
	return state, plan
}

func TestAppRefreshRetainsIdentityOnIncompleteReads(t *testing.T) {
	for _, tc := range []struct {
		name                                            string
		appStatus, versionStatus                        int
		wrongApp, wrongVersion, missingAssignmentFields bool
		wantFound, wantError                            bool
	}{
		{"normal snapshot", 200, 200, false, false, false, true, false},
		{"direct absence", 404, 200, false, false, false, false, false},
		{"forbidden", 403, 200, false, false, false, false, true},
		{"version missing retains identity", 200, 404, false, false, false, false, true},
		{"unexpected content owner", 200, 200, true, false, false, false, true},
		{"unexpected version", 200, 200, false, true, false, false, true},
		{"old assignment contract", 200, 200, false, false, true, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := appTestAPI(t, func(w http.ResponseWriter, req *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if req.URL.Path == "/api/apps/"+appTestID {
					w.WriteHeader(tc.appStatus)
					if tc.appStatus == 200 {
						_ = json.NewEncoder(w).Encode(map[string]any{"id": appTestID, "latestVersion": 1})
					} else {
						_, _ = w.Write([]byte(`{"error":{"message":"synthetic read failure"}}`))
					}
					return
				}
				if strings.Contains(req.URL.Path, "/versions/") {
					w.WriteHeader(tc.versionStatus)
					id, version := appTestID, 1
					if tc.wrongApp {
						id = appTestOtherToolID
					}
					if tc.wrongVersion {
						version = 2
					}
					if tc.versionStatus == 200 {
						_ = json.NewEncoder(w).Encode(map[string]any{"appId": id, "version": version, "html": "<html>Fixture</html>", "uiPermissions": map[string]any{"clipboardWrite": map[string]any{}}})
					}
					return
				}
				if tc.missingAssignmentFields {
					_, _ = w.Write([]byte(`[{"id":"` + appTestToolID + `"}]`))
					return
				}
				_, _ = w.Write([]byte(`[]`))
			})
			data := appTestModel(t)
			var d diag.Diagnostics
			found := r.refresh(t.Context(), &data, &d)
			if found != tc.wantFound || d.HasError() != tc.wantError {
				t.Fatalf("found=%v, diagnostics=%v", found, d)
			}
			if data.ID.ValueString() != appTestID {
				t.Fatal("read changed identity")
			}
			if found && !data.BrowserPermissions.Equal(types.SetValueMust(types.StringType, []attr.Value{types.StringValue("clipboard_write")})) {
				t.Fatal("browser permission was not observed")
			}
		})
	}
}

func TestAppUpdatePreconditionsPreventMutations(t *testing.T) {
	for _, tc := range []struct {
		name          string
		version       int
		observedTools []string
	}{{"new content version", 2, []string{appTestToolID}}, {"changed tool assignment", 1, []string{appTestOtherToolID}}} {
		t.Run(tc.name, func(t *testing.T) {
			mutations := 0
			r := appTestAPI(t, func(w http.ResponseWriter, req *http.Request) {
				if appTestSnapshot(t, w, req, tc.version, tc.observedTools...) {
					return
				}
				mutations++
				w.WriteHeader(500)
			})
			prior := appTestModel(t)
			prior.Tools = appTestTools(t, appTestToolID)
			desired := prior
			desired.Name = types.StringValue("Changed")
			desired.HTML = types.StringValue("<html>Changed</html>")
			desired.Tools = appTestTools(t)
			state, plan := appTestStatePlan(t, r, prior, desired)
			resp := resource.UpdateResponse{State: state}
			r.Update(t.Context(), resource.UpdateRequest{State: state, Plan: plan}, &resp)
			if !resp.Diagnostics.HasError() || mutations != 0 {
				t.Fatalf("mutations=%d diagnostics=%v", mutations, resp.Diagnostics)
			}
		})
	}
}

func TestAppPermissionEditUsesVersionPreconditionAndCurrentHTML(t *testing.T) {
	patches := 0
	r := appTestAPI(t, func(w http.ResponseWriter, req *http.Request) {
		if appTestSnapshot(t, w, req, 1) {
			return
		}
		if req.Method != "PATCH" {
			t.Errorf("unexpected mutation %s %s", req.Method, req.URL.Path)
			w.WriteHeader(500)
			return
		}
		patches++
		var body map[string]any
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		perms, ok := body["uiPermissions"].(map[string]any)
		if body["baseVersion"] != float64(1) || body["html"] != appTestModel(t).HTML.ValueString() || !ok || len(perms) != 1 || perms["camera"] == nil {
			t.Errorf("unsafe version update: %#v", body)
		}
		w.WriteHeader(409)
	})
	prior := appTestModel(t)
	desired := prior
	desired.BrowserPermissions = types.SetValueMust(types.StringType, []attr.Value{types.StringValue("camera")})
	state, plan := appTestStatePlan(t, r, prior, desired)
	resp := resource.UpdateResponse{State: state}
	r.Update(t.Context(), resource.UpdateRequest{State: state, Plan: plan}, &resp)
	if !resp.Diagnostics.HasError() || patches != 1 {
		t.Fatalf("patches=%d diagnostics=%v", patches, resp.Diagnostics)
	}
}

func TestAppFailedNewToolKeepsExistingAssignments(t *testing.T) {
	mutations := []string{}
	r := appTestAPI(t, func(w http.ResponseWriter, req *http.Request) {
		mutations = append(mutations, req.Method)
		w.WriteHeader(403)
	})
	prior := appTestModel(t)
	prior.Tools = appTestTools(t, appTestToolID)
	desired := prior
	desired.Tools = appTestTools(t, appTestOtherToolID)
	var d diag.Diagnostics
	if r.writeTools(t.Context(), &desired, &prior, &d) || !d.HasError() || len(mutations) != 1 || mutations[0] != "POST" {
		t.Fatalf("mutations=%v diagnostics=%v", mutations, d)
	}
}

func TestAppPartialCreateRetainsUUID(t *testing.T) {
	creates := 0
	r := appTestAPI(t, func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if req.Method == "POST" && req.URL.Path == "/api/apps" {
			creates++
			var body map[string]any
			if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
				t.Error(err)
			}
			if body["openInChat"] != false {
				t.Error("creation opened a conversation")
			}
			if env, ok := body["environmentId"]; !ok || env != nil {
				t.Error("creation did not explicitly select the organization default environment")
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"id": appTestID, "name": "App", "slug": "app", "latestVersion": 1})
			return
		}
		w.WriteHeader(503)
	})
	model := appTestModel(t)
	model.ID = types.StringUnknown()
	_, plan := appTestStatePlan(t, r, model, model)
	resp := resource.CreateResponse{State: tfsdk.State{Schema: plan.Schema}}
	r.Create(t.Context(), resource.CreateRequest{Plan: plan}, &resp)
	var id types.String
	if d := resp.State.GetAttribute(t.Context(), path.Root("id"), &id); d.HasError() {
		t.Fatal(d)
	}
	if !resp.Diagnostics.HasError() || id.ValueString() != appTestID || creates != 1 {
		t.Fatalf("id=%v creates=%d diagnostics=%v", id, creates, resp.Diagnostics)
	}
}

func TestAppHTMLValidatorCountsUTF8Bytes(t *testing.T) {
	for _, tc := range []struct {
		value string
		fail  bool
	}{{strings.Repeat("é", 256*1024), false}, {strings.Repeat("é", 256*1024+1), true}} {
		resp := validator.StringResponse{}
		appHTMLValidator{}.ValidateString(t.Context(), validator.StringRequest{ConfigValue: types.StringValue(tc.value), Path: path.Root("html")}, &resp)
		if resp.Diagnostics.HasError() != tc.fail {
			t.Fatal(resp.Diagnostics)
		}
	}
}

func TestAppLifecycleHonorsExplicitSettings(t *testing.T) {
	for _, tc := range []struct {
		name                      string
		enabled, locked, explicit bool
		expected                  []string
	}{
		{"omission preserves", true, false, false, nil},
		{"explicit availability before edit", true, false, true, []string{"enable", "unlock"}},
		{"explicit protection after edit", false, true, true, []string{"disable", "lock"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := []string{}
			r := appTestAPI(t, func(w http.ResponseWriter, req *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if req.Method != "POST" {
					t.Errorf("unexpected %s", req.Method)
				}
				calls = append(calls, strings.TrimPrefix(req.URL.Path, "/api/apps/"+appTestID+"/"))
				_, _ = w.Write([]byte(`{}`))
			})
			observed := appTestModel(t)
			observed.Enabled = types.BoolValue(!tc.enabled)
			observed.Locked = types.BoolValue(!tc.locked)
			desired := observed
			if tc.explicit {
				desired.Enabled = types.BoolValue(tc.enabled)
				desired.Locked = types.BoolValue(tc.locked)
			} else {
				desired.Enabled = types.BoolNull()
				desired.Locked = types.BoolNull()
			}
			var d diag.Diagnostics
			if !r.applyLifecycle(t.Context(), &desired, &observed, true, &d) {
				t.Fatal(d)
			}
			if tc.explicit && !tc.enabled && len(calls) != 0 {
				t.Fatal("App was disabled/locked before its content edit")
			}
			if !r.applyLifecycle(t.Context(), &desired, &observed, false, &d) {
				t.Fatal(d)
			}
			if strings.Join(calls, ",") != strings.Join(tc.expected, ",") {
				t.Fatalf("calls=%v expected=%v", calls, tc.expected)
			}
		})
	}
}

func TestAppDestroyNeverUnlocks(t *testing.T) {
	for _, status := range []int{200, 204, 404, 409} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			calls := 0
			r := appTestAPI(t, func(w http.ResponseWriter, req *http.Request) {
				calls++
				if req.Method != "DELETE" {
					t.Errorf("destruction issued %s", req.Method)
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(status)
				if status != 204 {
					_, _ = w.Write([]byte(`{}`))
				}
			})
			model := appTestModel(t)
			model.Locked = types.BoolValue(true)
			state, _ := appTestStatePlan(t, r, model, model)
			resp := resource.DeleteResponse{}
			r.Delete(t.Context(), resource.DeleteRequest{State: state}, &resp)
			if calls != 1 || resp.Diagnostics.HasError() != (status == 409) {
				t.Fatalf("calls=%d diagnostics=%v", calls, resp.Diagnostics)
			}
		})
	}
}
