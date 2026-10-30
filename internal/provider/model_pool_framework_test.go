package provider

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"sync/atomic"
	"testing"

	tfresource "github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestModelPoolTerraformFailureDiagnostics(t *testing.T) {
	for _, failPreflight := range []bool{true, false} {
		t.Run(fmt.Sprintf("preflight=%t", failPreflight), func(t *testing.T) {
			var writes atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				if req.URL.Path == "/health" {
					fmt.Fprint(w, `{"version":"1.5.0-rc.10"}`)
					return
				}
				if req.Method == http.MethodGet && req.URL.Path == "/openapi.json" {
					if !failPreflight {
						writePoolJSON(w, modelPoolContractFixture())
						return
					}
				} else if req.Method == http.MethodPost && req.URL.Path == "/api/model-pools" {
					writes.Add(1)
				} else {
					t.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
				}
				w.WriteHeader(http.StatusNotFound)
				fmt.Fprint(w, "sensitive-upstream-value")
			}))
			defer server.Close()
			want := "model pool post returned HTTP 404"
			if failPreflight {
				want = "OpenAPI preflight GET " + server.URL + "/openapi.json returned HTTP 404"
			}
			config := fmt.Sprintf(`provider "archestra" {
 base_url = %q
 api_key = "fixture-key"
 }
 resource "archestra_model_pool" "test" {
 name = "balanced-model"
 members = [{ provider_api_key_id = %q, model_id = "gpt-4.1" }]
 }`, server.URL, poolKeyID)
			tfresource.UnitTest(t, tfresource.TestCase{
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
				Steps:                    []tfresource.TestStep{{Config: config, ExpectError: regexp.MustCompile(regexp.QuoteMeta(want))}},
			})
			if failPreflight && writes.Load() != 0 {
				t.Fatal("failed preflight allowed a model pool write")
			}
			if !failPreflight && writes.Load() != 1 {
				t.Fatalf("got %d writes, want one after valid preflight", writes.Load())
			}
		})
	}
}

// This exercises Terraform's real planning/default/import path against a
// controlled HTTP boundary, not acceptance against a deployed Archestra.
func TestModelPoolTerraformFramework(t *testing.T) {
	var remote map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if req.URL.Path == "/openapi.json" {
			writePoolJSON(w, modelPoolContractFixture())
			return
		}
		if req.URL.Path == "/health" {
			fmt.Fprint(w, `{"version":"1.5.0-rc.10"}`)
			return
		}
		if req.Header.Get("Authorization") != "fixture-key" {
			t.Error("missing provider authentication")
		}
		switch req.Method {
		case http.MethodGet:
			if remote == nil {
				writePoolJSON(w, []any{})
				return
			}
			writePoolJSON(w, []any{remote})
		case http.MethodPost, http.MethodPut:
			if err := json.NewDecoder(req.Body).Decode(&remote); err != nil {
				t.Error(err)
			}
			remote["id"] = poolTestID
			remote["virtualApiKeyIds"] = []any{}
			writePoolJSON(w, remote)
		case http.MethodDelete:
			remote = nil
			writePoolJSON(w, map[string]bool{"success": true})
		default:
			t.Errorf("unexpected %s %s", req.Method, req.URL.Path)
		}
	}))
	defer server.Close()
	config := fmt.Sprintf(`provider "archestra" {
 base_url = %q
 api_key = "fixture-key"
 }
 resource "archestra_model_pool" "test" {
 name = "balanced-model"
 members = [{ provider_api_key_id = %q, model_id = "gpt-4.1" }]
 }`, server.URL, poolKeyID)
	tfresource.UnitTest(t, tfresource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []tfresource.TestStep{
			{Config: config, Check: tfresource.ComposeTestCheckFunc(
				tfresource.TestCheckResourceAttr("archestra_model_pool.test", "id", poolTestID),
				tfresource.TestCheckResourceAttr("archestra_model_pool.test", "members.0.priority", "1"),
				tfresource.TestCheckResourceAttr("archestra_model_pool.test", "members.0.weight", "1"),
				tfresource.TestCheckResourceAttr("archestra_model_pool.test", "members.0.enabled", "true"),
				tfresource.TestCheckResourceAttr("archestra_model_pool.test", "virtual_api_key_ids.#", "0"),
			)},
			{ResourceName: "archestra_model_pool.test", ImportState: true, ImportStateVerify: true},
			{Config: config, PlanOnly: true},
		},
	})
}
