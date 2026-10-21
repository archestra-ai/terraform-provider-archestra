package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func testParentGrantSet(t *testing.T, action string) types.Set {
	t.Helper()
	grants := []permissionGrantModel{}
	if action != "" {
		grants = append(grants, permissionGrantModel{SubjectType: types.StringValue("organization"), SubjectID: types.StringValue("*"), Actions: types.SetValueMust(types.StringType, []attr.Value{types.StringValue(action)})})
	}
	value, d := types.SetValueFrom(t.Context(), permissionGrantObjectType, grants)
	if d.HasError() {
		t.Fatal(d)
	}
	return value
}

func testParentPermissionUpdate(t *testing.T, prior, wanted types.Set) (resource.UpdateRequest, *resource.UpdateResponse) {
	t.Helper()
	s := schema.Schema{Attributes: map[string]schema.Attribute{"id": schema.StringAttribute{Computed: true}, "permissions": initialGrantsAttribute()}}
	value := func(grants types.Set) types.Object {
		return types.ObjectValueMust(map[string]attr.Type{"id": types.StringType, "permissions": types.SetType{ElemType: permissionGrantObjectType}}, map[string]attr.Value{"id": types.StringValue("parent"), "permissions": grants})
	}
	plan, d := value(wanted).ToTerraformValue(t.Context())
	if d != nil {
		t.Fatal(d)
	}
	state, d := value(prior).ToTerraformValue(t.Context())
	if d != nil {
		t.Fatal(d)
	}
	req := resource.UpdateRequest{Plan: tfsdk.Plan{Schema: s, Raw: plan}, State: tfsdk.State{Schema: s, Raw: state}}
	return req, &resource.UpdateResponse{State: req.State}
}

func TestParentPermissionRevisionSafety(t *testing.T) {
	for _, tc := range []struct {
		name, current string
		writeStatus   int
		writes        int
		fail          bool
	}{
		{"success", "read", 200, 1, false},
		{"changed after plan", "delete", 200, 0, true},
		{"changed during update", "read", 409, 1, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			writes := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if req.Method == "GET" {
					_ = json.NewEncoder(w).Encode(map[string]any{"ownerSubject": nil, "resource": "agent", "scope": "parent", "revision": 7, "grants": []any{map[string]any{"subject": map[string]string{"type": "organization", "id": "*"}, "actions": []string{tc.current}}}})
					return
				}
				writes++
				var body struct {
					Revision int64                 `json:"revision"`
					Grants   []permissionGrantWire `json:"grants"`
				}
				if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
					t.Error(err)
				}
				if body.Revision != 7 || body.Grants == nil || len(body.Grants) != 0 {
					t.Errorf("unsafe clear request: %#v", body)
				}
				w.WriteHeader(tc.writeStatus)
				if tc.writeStatus == 409 {
					_, _ = w.Write([]byte(`{"error":{"message":"conflict"}}`))
					return
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"ownerSubject": nil, "resource": "agent", "scope": "parent", "revision": 8, "grants": []any{}})
			}))
			defer server.Close()
			api, err := client.NewClientWithResponses(server.URL)
			if err != nil {
				t.Fatal(err)
			}
			prior, wanted := testParentGrantSet(t, "read"), testParentGrantSet(t, "")
			req, resp := testParentPermissionUpdate(t, prior, wanted)
			finish := beginParentPermissionUpdate(t.Context(), api, "agent", "id", req, resp)
			finish()
			if writes != tc.writes || resp.Diagnostics.HasError() != tc.fail {
				t.Fatalf("writes=%d, diagnostics=%v", writes, resp.Diagnostics)
			}
			var observed types.Set
			if d := resp.State.GetAttribute(t.Context(), path.Root("permissions"), &observed); d.HasError() {
				t.Fatal(d)
			}
			expected := wanted
			if tc.fail {
				expected = prior
			}
			if !observed.Equal(expected) {
				t.Fatalf("failed write corrupted grants: %v", observed)
			}
		})
	}
}

func TestParentPermissionDeniedReadPreservesManagedGrants(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(403) }))
	defer server.Close()
	api, err := client.NewClientWithResponses(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	for _, managed := range []bool{false, true} {
		grants := types.SetNull(permissionGrantObjectType)
		if managed {
			grants = testParentGrantSet(t, "read")
		}
		prior := grants
		var d diag.Diagnostics
		readParentPermissions(context.Background(), api, "agent", "parent", &grants, &d)
		if d.HasError() != managed || !grants.Equal(prior) {
			t.Fatalf("managed=%v state=%v diagnostics=%v", managed, grants, d)
		}
		if managed && !strings.Contains(d.Errors()[0].Detail(), "403") {
			t.Fatal(d)
		}
	}
}
