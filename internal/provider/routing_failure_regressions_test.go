package provider

import (
	"encoding/json"
	"fmt"
	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const oauthFixture = `{"id":"fixture","clientId":"public-fixture","name":"fixture","grantType":"authorization_code","scopes":["llm:proxy"],"allowedGatewayIds":[],"providerApiKeys":[],"includeFutureProviders":false,"redirectUris":["https://example.test/callback"],"labels":[],"billingTeam":null,"spendCap":null,"createdBy":null,"createdAt":"2026-01-01T00:00:00Z","updatedAt":"2026-01-01T00:00:00Z"}`

func oauthTestState(t *testing.T, r *OauthClientResource) (tfsdk.State, OauthClientResourceModel) {
	t.Helper()
	var response resource.SchemaResponse
	r.Schema(t.Context(), resource.SchemaRequest{}, &response)
	model := OauthClientResourceModel{InitialGrants: types.SetNull(permissionGrantObjectType), ClientSecret: types.StringValue("synthetic-original"), RotationNonce: types.StringValue("1")}
	var d diag.Diagnostics
	r.flatten(t.Context(), &model, []byte(oauthFixture), &d)
	if d.HasError() {
		t.Fatal(d)
	}
	state := tfsdk.State{Schema: response.Schema}
	if d := state.Set(t.Context(), &model); d.HasError() {
		t.Fatal(d)
	}
	return state, model
}
func TestOauthRotationFailureRetainsPendingNonce(t *testing.T) {
	for _, metadata := range []bool{false, true} {
		t.Run(fmt.Sprint(metadata), func(t *testing.T) {
			rotations, updates := 0, 0
			failRotation := true
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				body := oauthFixture
				if metadata {
					body = strings.Replace(body, `"name":"fixture"`, `"name":"updated"`, 1)
				}
				switch {
				case strings.HasSuffix(req.URL.Path, "/rotate-secret"):
					rotations++
					if failRotation {
						w.WriteHeader(http.StatusServiceUnavailable)
						fmt.Fprint(w, `{"error":"temporarily unavailable"}`)
						return
					}
					body = strings.TrimSuffix(body, "}") + `,"clientSecret":"synthetic-replacement"}`
				case req.Method == http.MethodPut:
					updates++
					var patch map[string]any
					if err := json.NewDecoder(req.Body).Decode(&patch); err != nil {
						t.Error(err)
					}
					if patch["grantType"] != "authorization_code" {
						t.Error("metadata update lost grant type")
					}
					if _, present := patch["clientSecret"]; present {
						t.Error("metadata update resent a credential")
					}
				default:
					t.Errorf("unexpected request %s %s", req.Method, req.URL.Path)
				}
				fmt.Fprint(w, body)
			}))
			defer server.Close()
			api, err := client.NewClientWithResponses(server.URL)
			if err != nil {
				t.Fatal(err)
			}
			r := OauthClientResource{client: api}
			state, model := oauthTestState(t, &r)
			model.RotationNonce = types.StringValue("2")
			if metadata {
				model.Name = types.StringValue("updated")
			}
			plan := tfsdk.Plan{Schema: state.Schema}
			if d := plan.Set(t.Context(), &model); d.HasError() {
				t.Fatal(d)
			}
			result := resource.UpdateResponse{State: state}
			r.Update(t.Context(), resource.UpdateRequest{State: state, Plan: plan}, &result)
			if !result.Diagnostics.HasError() {
				t.Fatal("rotation failure did not produce a diagnostic")
			}
			var retained OauthClientResourceModel
			if d := result.State.Get(t.Context(), &retained); d.HasError() {
				t.Fatal(d)
			}
			if retained.RotationNonce.ValueString() != "1" || retained.ClientSecret.ValueString() != "synthetic-original" {
				t.Fatal("failed rotation consumed the nonce or lost the old credential")
			}
			if metadata && retained.Name.ValueString() != "updated" {
				t.Fatal("successful metadata update was not recorded")
			}
			failRotation = false
			retry := resource.UpdateResponse{State: result.State}
			r.Update(t.Context(), resource.UpdateRequest{State: result.State, Plan: plan}, &retry)
			if retry.Diagnostics.HasError() {
				t.Fatal(retry.Diagnostics)
			}
			var rotated OauthClientResourceModel
			if d := retry.State.Get(t.Context(), &rotated); d.HasError() {
				t.Fatal(d)
			}
			if rotations != 2 || rotated.RotationNonce.ValueString() != "2" || rotated.ClientSecret.ValueString() != "synthetic-replacement" || rotated.ID.ValueString() != "fixture" {
				t.Fatal("pending rotation did not retry in place")
			}
			expectedUpdates := 0
			if metadata {
				expectedUpdates = 1
			}
			if updates != expectedUpdates {
				t.Fatal("retry repeated an unrelated metadata write")
			}
		})
	}
}
func TestOauthReadPaginatesWithoutRecoveringSecret(t *testing.T) {
	pages := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if req.Method != http.MethodGet || req.URL.Path != "/api/oauth-clients" {
			t.Errorf("unexpected credential read %s %s", req.Method, req.URL.Path)
		}
		if req.URL.Query().Get("limit") != "100" {
			t.Error("unexpected page size")
		}
		pages++
		rows := []json.RawMessage{}
		switch req.URL.Query().Get("offset") {
		case "0":
			for i := 0; i < 100; i++ {
				rows = append(rows, json.RawMessage(fmt.Sprintf(`{"id":"other-%d"}`, i)))
			}
		case "100":
			rows = append(rows, json.RawMessage(oauthFixture))
		default:
			t.Error("unexpected pagination offset")
		}
		if err := json.NewEncoder(w).Encode(map[string]any{"data": rows, "pagination": map[string]any{"total": 101, "limit": 100, "offset": (pages - 1) * 100}}); err != nil {
			t.Error(err)
		}
	}))
	defer server.Close()
	api, err := client.NewClientWithResponses(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	r := OauthClientResource{client: api}
	state, model := oauthTestState(t, &r)
	model.ClientSecret = types.StringNull()
	if d := state.Set(t.Context(), &model); d.HasError() {
		t.Fatal(d)
	}
	result := resource.ReadResponse{State: state}
	r.Read(t.Context(), resource.ReadRequest{State: state}, &result)
	if result.Diagnostics.HasError() {
		t.Fatal(result.Diagnostics)
	}
	var observed OauthClientResourceModel
	if d := result.State.Get(t.Context(), &observed); d.HasError() {
		t.Fatal(d)
	}
	if pages != 2 || observed.ID.ValueString() != "fixture" || !observed.ClientSecret.IsNull() {
		t.Fatal("paginated import lost the client or invented a secret")
	}
}
