package provider

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func TestVirtualAPIKeyReadRevealsValueOnlyForItsAuthor(t *testing.T) {
	const id = "11111111-1111-4111-8111-111111111111"
	for _, tc := range []struct {
		name        string
		prior       string
		valueStatus int
		want        types.String
		wantReveals int
	}{
		{"author", "", http.StatusOK, types.StringValue("arch_vk_revealed"), 1},
		{"not-author", "", http.StatusForbidden, types.StringNull(), 1},
		{"value-already-in-state", "arch_vk_created", http.StatusOK, types.StringValue("arch_vk_created"), 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reveals := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch {
				case req.URL.Path == "/api/llm-virtual-keys/"+id+"/value":
					reveals++
					w.WriteHeader(tc.valueStatus)
					if tc.valueStatus == http.StatusOK {
						fmt.Fprint(w, `{"value":"arch_vk_revealed"}`)
					} else {
						fmt.Fprint(w, `{"error":{"message":"Only the key's creator can reveal its value.","type":"api_authorization_error"}}`)
					}
				case req.URL.Path == "/api/llm-virtual-keys/"+id:
					fmt.Fprintf(w, `{"id":%q,"name":"fixture","keyType":"passthrough","includeFutureProviders":false,"secretId":"22222222-2222-4222-8222-222222222222","tokenStart":"arch_vk_rev","scope":"personal","organizationId":"org","providerApiKeys":[],"labels":[],"teams":[],"createdBy":null}`, id)
				case strings.Contains(req.URL.Path, "permission"):
					w.WriteHeader(http.StatusForbidden)
					fmt.Fprint(w, `{"error":{"message":"denied","type":"api_authorization_error"}}`)
				default:
					t.Errorf("unexpected request %s %s", req.Method, req.URL.Path)
					w.WriteHeader(http.StatusNotFound)
				}
			}))
			defer server.Close()
			c, err := client.NewClientWithResponses(server.URL)
			if err != nil {
				t.Fatal(err)
			}
			r := VirtualAPIKeyResource{client: c}
			var schema resource.SchemaResponse
			r.Schema(t.Context(), resource.SchemaRequest{}, &schema)
			state := tfsdk.State{Schema: schema.Schema, Raw: tftypes.NewValue(schema.Schema.Type().TerraformType(t.Context()), nil)}
			if d := state.SetAttribute(t.Context(), path.Root("id"), id); d.HasError() {
				t.Fatal(d)
			}
			if tc.prior != "" {
				if d := state.SetAttribute(t.Context(), path.Root("value"), tc.prior); d.HasError() {
					t.Fatal(d)
				}
			}
			response := resource.ReadResponse{State: state}
			r.Read(t.Context(), resource.ReadRequest{State: state}, &response)
			if response.Diagnostics.HasError() {
				t.Fatal(response.Diagnostics)
			}
			var got VirtualAPIKeyResourceModel
			if d := response.State.Get(t.Context(), &got); d.HasError() {
				t.Fatal(d)
			}
			if !got.Value.Equal(tc.want) || reveals != tc.wantReveals {
				t.Fatalf("value=%v reveals=%d", got.Value, reveals)
			}
			if got.SecretID.ValueString() != "22222222-2222-4222-8222-222222222222" || got.TokenStart.ValueString() != "arch_vk_rev" {
				t.Fatalf("key identity was not kept: secret_id=%v token_start=%v", got.SecretID, got.TokenStart)
			}
		})
	}
}
