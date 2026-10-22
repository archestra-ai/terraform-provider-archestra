package provider

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func TestVirtualAPIKeyImportRevealsValueOnlyForItsCreator(t *testing.T) {
	const id = "11111111-1111-4111-8111-111111111111"
	for _, tc := range []struct {
		name        string
		prior       string
		valueStatus int
		want        types.String
		wantReveals int
	}{
		{"author", "", http.StatusOK, types.StringValue("arch_vk_revealed"), 1},
		{"not-creator-or-older-platform", "", http.StatusForbidden, types.StringNull(), 1},
		{"not-revealable", "", http.StatusConflict, types.StringNull(), 1},
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
					fmt.Fprintf(w, `{"id":%q,"name":"fixture","keyType":"standard","includeFutureProviders":false,"secretId":"22222222-2222-4222-8222-222222222222","tokenStart":"arch_vk_rev","scope":"personal","organizationId":"org","providerApiKeys":[{"provider":"openai","providerApiKeyId":"33333333-3333-4333-8333-333333333333","providerApiKeyName":"primary"}],"labels":[],"teams":[],"createdBy":null,"billingTeam":{"id":"44444444-4444-4444-8444-444444444444","name":"platform"},"spendCap":{"limitValue":100,"cleanupInterval":"1d","limitId":"55555555-5555-4555-8555-555555555555","currentUsage":1.5}}`, id)
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
			// Import passes only the ID; Read then recovers what it can.
			imported := resource.ImportStateResponse{State: tfsdk.State{Schema: schema.Schema, Raw: tftypes.NewValue(schema.Schema.Type().TerraformType(t.Context()), nil)}}
			r.ImportState(t.Context(), resource.ImportStateRequest{ID: id}, &imported)
			if imported.Diagnostics.HasError() {
				t.Fatal(imported.Diagnostics)
			}
			state := imported.State
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
			// The key's identity and routing are read whether or not its value can be revealed.
			var keys []credentialProviderKeyModel
			if d := got.ProviderAPIKeys.ElementsAs(t.Context(), &keys, false); d.HasError() {
				t.Fatal(d)
			}
			limit := got.SpendCap.Attributes()["limit_value"]
			if got.ID.ValueString() != id || got.BillingTeamID.ValueString() != "44444444-4444-4444-8444-444444444444" || len(keys) != 1 || keys[0].ProviderAPIKeyID.ValueString() != "33333333-3333-4333-8333-333333333333" || limit == nil || limit.String() != "100" {
				t.Fatalf("key settings were not read: id=%v billing_team_id=%v provider_api_keys=%v spend_cap=%v", got.ID, got.BillingTeamID, keys, got.SpendCap)
			}
		})
	}
}

// The schema holds what create accepts, the ID and the provider's own
// attributes (value, rotation_nonce), and nothing else.
func TestVirtualAPIKeySchemaHoldsOnlyCreateInputsAndTheCredential(t *testing.T) {
	createFields := map[string]bool{}
	body := reflect.TypeOf(client.CreateVirtualApiKeyJSONBody{})
	for i := range body.NumField() {
		createFields[strings.Split(body.Field(i).Tag.Get("json"), ",")[0]] = true
	}
	specs := map[string]AttrSpec{}
	for _, spec := range virtualAPIKeyAttrSpec {
		specs[spec.TFName] = spec
		if spec.Kind != Synthetic && !createFields[spec.JSONName] {
			t.Errorf("%s maps to %q, which create does not accept", spec.TFName, spec.JSONName)
		}
	}
	var resp resource.SchemaResponse
	(&VirtualAPIKeyResource{}).Schema(t.Context(), resource.SchemaRequest{}, &resp)
	for name, attr := range resp.Schema.Attributes {
		spec, ok := specs[name]
		if !ok && name != "id" {
			t.Errorf("%s is not a create input, the ID or a provider attribute", name)
			continue
		}
		nested, ok := attr.(rschema.NestedAttribute)
		if !ok {
			continue
		}
		var got, want []string
		for child := range nested.GetNestedObject().GetAttributes() {
			got = append(got, child)
		}
		for _, child := range spec.Children {
			want = append(want, child.TFName)
		}
		sort.Strings(got)
		sort.Strings(want)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s holds %v; the request sends only %v", name, got, want)
		}
	}
}
