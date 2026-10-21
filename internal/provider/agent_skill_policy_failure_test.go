package provider

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func emptySkillPolicyModel() AgentSkillPolicyResourceModel {
	return AgentSkillPolicyResourceModel{ID: types.StringValue("11111111-1111-4111-8111-111111111111"), AgentID: types.StringValue("11111111-1111-4111-8111-111111111111"), Mode: types.StringValue("manual"), Revision: types.Int64Value(7), AllowedReferences: types.SetValueMust(skillReferenceType, []attr.Value{}), ExcludedReferences: types.SetValueMust(skillReferenceType, []attr.Value{})}
}
func TestSkillPolicyReadAndDeleteFailures(t *testing.T) {
	for _, tc := range []struct {
		name                     string
		status, hidden, revision int
		wantError                bool
	}{
		{"forbidden", 403, 0, 7, true}, {"server error", 500, 0, 7, true}, {"unavailable reference", 200, 1, 7, true}, {"concurrent revision", 200, 0, 8, true}, {"agent absent", 404, 0, 7, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			writes := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				if req.Method != "GET" {
					writes++
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tc.status)
				fmt.Fprintf(w, `{"mode":"all","revision":%d,"allowedReferences":[],"excludedReferences":[],"hiddenAllowedCount":%d,"hiddenExcludedCount":0}`, tc.revision, tc.hidden)
			}))
			defer server.Close()
			c, err := client.NewClientWithResponses(server.URL)
			if err != nil {
				t.Fatal(err)
			}
			r := AgentSkillPolicyResource{client: c}
			var s resource.SchemaResponse
			r.Schema(t.Context(), resource.SchemaRequest{}, &s)
			state := tfsdk.State{Schema: s.Schema}
			data := emptySkillPolicyModel()
			if d := state.Set(t.Context(), &data); d.HasError() {
				t.Fatal(d)
			}
			read := resource.ReadResponse{State: state}
			r.Read(t.Context(), resource.ReadRequest{State: state}, &read)
			readError := tc.wantError && tc.name != "concurrent revision"
			if read.Diagnostics.HasError() != readError {
				t.Fatal(read.Diagnostics)
			}
			if read.State.Raw.IsNull() != (tc.status == 404) {
				t.Fatal("Read dropped state without a 404")
			}
			var deleted resource.DeleteResponse
			r.Delete(t.Context(), resource.DeleteRequest{State: state}, &deleted)
			if deleted.Diagnostics.HasError() != tc.wantError {
				t.Fatal(deleted.Diagnostics)
			}
			if writes != 0 {
				t.Fatal("Unsafe policy write", writes)
			}
		})
	}
}
func TestSkillPolicyPatchConflictDoesNotRetry(t *testing.T) {
	writes := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writes++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(409)
		fmt.Fprint(w, `{}`)
	}))
	defer server.Close()
	c, err := client.NewClientWithResponses(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	r := AgentSkillPolicyResource{client: c}
	plan := emptySkillPolicyModel()
	var diags diag.Diagnostics
	r.replace(t.Context(), &plan, &skillPolicyWire{Mode: "all", Revision: 7}, &diags)
	if !diags.HasError() || writes != 1 {
		t.Fatalf("diagnostics %v writes %d", diags, writes)
	}
}
func TestSkillPolicyDiffPreservesReferenceIdentity(t *testing.T) {
	native := skillReferenceWire{Source: "native", SkillID: "11111111-1111-4111-8111-111111111111"}
	external := skillReferenceWire{Source: "external_mcp", McpServerID: "22222222-2222-4222-8222-222222222222", URI: "skill://fixture"}
	plugin := skillReferenceWire{Source: "plugin", PluginID: "33333333-3333-4333-8333-333333333333", SkillPath: "skills/fixture"}
	before := skillPolicyWire{AllowedReferences: []skillReferenceWire{native, external}, ExcludedReferences: []skillReferenceWire{plugin}}
	target := skillPolicyWire{AllowedReferences: []skillReferenceWire{plugin, external}, ExcludedReferences: []skillReferenceWire{native}}
	ops := skillPolicyDiff(before, target)
	if len(ops) != 4 {
		t.Fatalf("operations %+v", ops)
	}
	for _, op := range ops {
		if op.Reference == external {
			t.Fatal("unchanged reference was rewritten")
		}
	}
	if len(skillPolicyDiff(target, target)) != 0 {
		t.Fatal("no-op policy diff was nonempty")
	}
}
