package provider

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestPermissionPolicyDeleteConflictAndMissingTarget(t *testing.T) {
	for _, tc := range []struct {
		name      string
		status    int
		revision  int
		wantError bool
	}{
		{"concurrent grant change", 200, 8, true},
		{"target already deleted", 404, 0, false},
		{"read forbidden", 403, 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			writes := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				if req.Method != "GET" {
					writes++
					t.Error("delete must not overwrite an unreviewed policy")
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tc.status)
				if tc.status == 200 {
					fmt.Fprintf(w, `{"revision":%d,"grants":[],"effectiveActions":[]}`, tc.revision)
				} else {
					fmt.Fprint(w, `{"error":{"message":"not available"}}`)
				}
			}))
			defer server.Close()
			c, err := client.NewClientWithResponses(server.URL)
			if err != nil {
				t.Fatal(err)
			}
			r := ResourcePermissionPolicyResource{client: c}
			ctx := t.Context()
			var s resource.SchemaResponse
			r.Schema(ctx, resource.SchemaRequest{}, &s)
			state := tfsdk.State{Schema: s.Schema}
			data := ResourcePermissionPolicyResourceModel{ID: types.StringValue("environment/*"), ResourceType: types.StringValue("environment"), ResourceID: types.StringValue("*"), Revision: types.Int64Value(7), Grants: types.SetNull(permissionGrantObjectType), OwnerID: types.StringNull(), EffectiveActions: types.SetNull(types.StringType)}
			if ds := state.Set(ctx, &data); ds.HasError() {
				t.Fatal(ds)
			}
			var response resource.DeleteResponse
			r.Delete(ctx, resource.DeleteRequest{State: state}, &response)
			if response.Diagnostics.HasError() != tc.wantError {
				t.Fatalf("diagnostics=%v", response.Diagnostics)
			}
			if writes != 0 {
				t.Fatalf("unexpected writes=%d", writes)
			}
		})
	}
}

func TestPermissionPolicyReadMissingRemovesState(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) { w.WriteHeader(404) }))
	defer server.Close()
	c, err := client.NewClientWithResponses(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	r := ResourcePermissionPolicyResource{client: c}
	ctx := t.Context()
	var s resource.SchemaResponse
	r.Schema(ctx, resource.SchemaRequest{}, &s)
	state := tfsdk.State{Schema: s.Schema}
	data := ResourcePermissionPolicyResourceModel{ID: types.StringValue("environment/*"), ResourceType: types.StringValue("environment"), ResourceID: types.StringValue("*"), Revision: types.Int64Value(7), Grants: types.SetNull(permissionGrantObjectType), OwnerID: types.StringNull(), EffectiveActions: types.SetNull(types.StringType)}
	if ds := state.Set(ctx, &data); ds.HasError() {
		t.Fatal(ds)
	}
	response := resource.ReadResponse{State: state}
	r.Read(ctx, resource.ReadRequest{State: state}, &response)
	if response.Diagnostics.HasError() || !response.State.Raw.IsNull() {
		t.Fatalf("state not removed: %v", response.Diagnostics)
	}
}
