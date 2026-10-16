package provider

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func projectInstructionsTestState(t *testing.T, r *ProjectResource) (tfsdk.State, ProjectResourceModel) {
	t.Helper()
	var schema resource.SchemaResponse
	r.Schema(t.Context(), resource.SchemaRequest{}, &schema)
	value := ProjectResourceModel{ID: types.StringValue("11111111-1111-4111-8111-111111111111"), Name: types.StringValue("fixture"), Instructions: types.StringValue("original")}
	state := tfsdk.State{Schema: schema.Schema}
	if d := state.Set(t.Context(), &value); d.HasError() {
		t.Fatal(d)
	}
	return state, value
}
func TestProjectInstructionsUpdateOnlyTouchesChangedOwnerFields(t *testing.T) {
	for _, instructionsChanged := range []bool{false, true} {
		t.Run(fmt.Sprint(instructionsChanged), func(t *testing.T) {
			parentWrites, instructionWrites := 0, 0
			content := "original"
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if strings.HasSuffix(req.URL.Path, "/instructions") {
					if req.Method == "PUT" {
						instructionWrites++
						var body struct {
							Content string `json:"content"`
						}
						if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
							t.Error(err)
						}
						content = body.Content
						fmt.Fprint(w, `{"ok":true}`)
						return
					}
					_ = json.NewEncoder(w).Encode(map[string]string{"content": content})
					return
				}
				if req.Method == "PATCH" {
					parentWrites++
				}
				fmt.Fprint(w, `{"id":"11111111-1111-4111-8111-111111111111","name":"fixture","viewerRole":"owner","createdAt":"2026-01-01T00:00:00Z","conversationCount":0,"labels":[]}`)
			}))
			defer server.Close()
			c, err := client.NewClientWithResponses(server.URL)
			if err != nil {
				t.Fatal(err)
			}
			r := ProjectResource{client: c}
			state, value := projectInstructionsTestState(t, &r)
			if instructionsChanged {
				value.Instructions = types.StringValue("updated")
			} else {
				value.Name = types.StringValue("updated name")
			}
			plan := tfsdk.Plan{Schema: state.Schema}
			if d := plan.Set(t.Context(), &value); d.HasError() {
				t.Fatal(d)
			}
			response := resource.UpdateResponse{State: state}
			r.Update(t.Context(), resource.UpdateRequest{State: state, Plan: plan}, &response)
			if response.Diagnostics.HasError() {
				t.Fatal(response.Diagnostics)
			}
			if instructionsChanged && (instructionWrites != 1 || parentWrites != 0) {
				t.Fatal("instructions-only edit called the unrelated project metadata API", parentWrites, instructionWrites)
			}
			if !instructionsChanged && (instructionWrites != 0 || parentWrites != 1) {
				t.Fatal("metadata edit resent unchanged owner-only instructions", parentWrites, instructionWrites)
			}
		})
	}
}
func TestProjectInstructionReadFailureDoesNotForgetReadableProject(t *testing.T) {
	for _, status := range []int{403, 404, 500} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if strings.HasSuffix(req.URL.Path, "/instructions") {
					w.WriteHeader(status)
					fmt.Fprint(w, `{}`)
					return
				}
				fmt.Fprint(w, `{"id":"11111111-1111-4111-8111-111111111111","name":"fixture","viewerRole":"owner","createdAt":"2026-01-01T00:00:00Z","conversationCount":0,"labels":[]}`)
			}))
			defer server.Close()
			c, err := client.NewClientWithResponses(server.URL)
			if err != nil {
				t.Fatal(err)
			}
			r := ProjectResource{client: c}
			state, _ := projectInstructionsTestState(t, &r)
			response := resource.ReadResponse{State: state}
			r.Read(t.Context(), resource.ReadRequest{State: state}, &response)
			if !response.Diagnostics.HasError() || response.State.Raw.IsNull() {
				t.Fatal("instructions failure lost parent state", response.Diagnostics)
			}
		})
	}
}
