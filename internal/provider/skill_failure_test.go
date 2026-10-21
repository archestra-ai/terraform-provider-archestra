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
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func skillTestModel() SkillResourceModel {
	return SkillResourceModel{GitSource: types.ObjectNull(skillGitSourceType.AttrTypes), InitialGrants: types.SetNull(permissionGrantObjectType), ID: types.StringValue("11111111-1111-4111-8111-111111111111"), Name: types.StringValue("fixture"), Description: types.StringValue("Synthetic fixture"), Content: types.StringValue("Synthetic body."), Templated: types.BoolValue(false), Metadata: types.MapNull(types.StringType), AllowedTools: types.SetNull(types.StringType), Files: types.MapValueMust(skillFileType, map[string]attr.Value{}), FileDigests: types.MapNull(types.StringType), EnvironmentIDs: types.SetValueMust(types.StringType, []attr.Value{}), Labels: []AgentLabelModel{}, LatestVersion: types.Int64Value(1)}
}
func TestSkillManifestAndFileEncoding(t *testing.T) {
	plan := skillTestModel()
	plan.Name = types.StringValue("A quoted \"fixture\"")
	var diags diag.Diagnostics
	encoded := plan.request(t.Context(), map[string]any{}, &diags)
	if diags.HasError() {
		t.Fatal(diags)
	}
	var body struct {
		Content string `json:"content"`
	}
	if err := json.Unmarshal(encoded, &body); err != nil {
		t.Fatal(err)
	}
	manifest := body.Content
	if !strings.HasPrefix(manifest, "---\n{") {
		t.Fatalf("manifest delimiters were not literal newlines: %q", manifest)
	}
	parts := strings.SplitN(manifest, "\n---\n", 2)
	var header map[string]any
	if err := json.Unmarshal([]byte(strings.TrimPrefix(parts[0], "---\n")), &header); err != nil {
		t.Fatal(err)
	}
	if header["name"] != plan.Name.ValueString() || parts[1] != plan.Content.ValueString() {
		t.Fatal("manifest corrupted quoted metadata or body")
	}

	encodedFiles, err := json.Marshal(encodeSkillFiles(map[string]any{"b": map[string]any{"content": "AAEC", "encoding": "base64"}, "a": map[string]any{"content": "text", "encoding": "utf8"}}))
	if err != nil {
		t.Fatal(err)
	}
	var files []struct {
		Path     string `json:"path"`
		Encoding string `json:"encoding"`
	}
	if err := json.Unmarshal(encodedFiles, &files); err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 || files[0].Path != "a" || files[1].Encoding != "base64" {
		t.Fatalf("file encoding %#v", files)
	}
	empty, err := json.Marshal(encodeSkillFiles(map[string]any{}))
	if err != nil {
		t.Fatal(err)
	}
	if string(empty) != "[]" {
		t.Fatalf("empty files encoded as %s", empty)
	}

}
func TestSkillReadFailuresAndUpdateConflict(t *testing.T) {
	for _, status := range []int{403, 500, 404, 409} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				calls++
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(status)
				fmt.Fprint(w, `{}`)
			}))
			defer server.Close()
			c, err := client.NewClientWithResponses(server.URL)
			if err != nil {
				t.Fatal(err)
			}
			r := SkillResource{client: c}
			var schema resource.SchemaResponse
			r.Schema(t.Context(), resource.SchemaRequest{}, &schema)
			state := tfsdk.State{Schema: schema.Schema}
			value := skillTestModel()
			if d := state.Set(t.Context(), &value); d.HasError() {
				t.Fatal(d)
			}
			if status == 409 {
				value.Content = types.StringValue("Changed pending body.")
				plan := tfsdk.Plan{Schema: schema.Schema}
				if d := plan.Set(t.Context(), &value); d.HasError() {
					t.Fatal(d)
				}
				var response resource.UpdateResponse
				r.Update(t.Context(), resource.UpdateRequest{Plan: plan, State: state}, &response)
				if !response.Diagnostics.HasError() || calls != 1 {
					t.Fatal("conflicting update was retried or accepted", response.Diagnostics, calls)
				}
				return
			}
			response := resource.ReadResponse{State: state}
			r.Read(t.Context(), resource.ReadRequest{State: state}, &response)
			if response.Diagnostics.HasError() != (status != 404) {
				t.Fatal(response.Diagnostics)
			}
			if response.State.Raw.IsNull() != (status == 404) {
				t.Fatal("read removed state without API absence")
			}
		})
	}
}

func TestSkillManifestOnlyEditsReachVersionedUpdate(t *testing.T) {
	cases := []struct {
		name string
		edit func(*SkillResourceModel)
	}{
		{"name", func(m *SkillResourceModel) { m.Name = types.StringValue("renamed") }},
		{"description", func(m *SkillResourceModel) { m.Description = types.StringValue("Changed description") }},
		{"license", func(m *SkillResourceModel) { m.License = types.StringValue("MIT") }},
		{"compatibility", func(m *SkillResourceModel) { m.Compatibility = types.StringValue("linux") }},
		{"agent_name", func(m *SkillResourceModel) { m.AgentName = types.StringValue("helper") }},
		{"templated", func(m *SkillResourceModel) { m.Templated = types.BoolValue(true) }},
		{"metadata", func(m *SkillResourceModel) {
			m.Metadata = types.MapValueMust(types.StringType, map[string]attr.Value{"purpose": types.StringValue("changed")})
		}},
		{"allowed_tools", func(m *SkillResourceModel) {
			m.AllowedTools = types.SetValueMust(types.StringType, []attr.Value{types.StringValue("Read")})
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
				calls++
				var body struct {
					Content     string `json:"content"`
					BaseVersion int    `json:"baseVersion"`
				}
				if err := json.NewDecoder(request.Body).Decode(&body); err != nil || !strings.HasPrefix(body.Content, "---\n{") || body.BaseVersion != 1 {
					t.Error("manifest edit did not carry rendered content and the observed version", err)
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusConflict)
				fmt.Fprint(w, `{}`)
			}))
			defer server.Close()
			api, err := client.NewClientWithResponses(server.URL)
			if err != nil {
				t.Fatal(err)
			}
			r := SkillResource{client: api}
			var responseSchema resource.SchemaResponse
			r.Schema(t.Context(), resource.SchemaRequest{}, &responseSchema)
			before := skillTestModel()
			state := tfsdk.State{Schema: responseSchema.Schema}
			if d := state.Set(t.Context(), &before); d.HasError() {
				t.Fatal(d)
			}
			after := before
			tc.edit(&after)
			plan := tfsdk.Plan{Schema: responseSchema.Schema}
			if d := plan.Set(t.Context(), &after); d.HasError() {
				t.Fatal(d)
			}
			var response resource.UpdateResponse
			r.Update(t.Context(), resource.UpdateRequest{Plan: plan, State: state}, &response)
			if calls != 1 || !response.Diagnostics.HasError() {
				t.Fatal("manifest-only update was skipped or conflict was retried", calls, response.Diagnostics)
			}
		})
	}
}
