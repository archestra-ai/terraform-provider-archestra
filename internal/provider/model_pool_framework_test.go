package provider

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	tfresource "github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// This exercises Terraform's real planning/default/import path against a
// controlled HTTP boundary, not acceptance against a deployed Archestra.
func TestModelPoolTerraformFramework(t *testing.T) {
	var remote map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
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
