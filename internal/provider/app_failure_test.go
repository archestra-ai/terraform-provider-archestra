package provider

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os"
	"regexp"
	"sync/atomic"
	"testing"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestAccAppRecoversFollowupFailure(t *testing.T) {
	var failed atomic.Bool
	var creates atomic.Int32
	var firstID atomic.Pointer[string]
	if os.Getenv("TF_ACC") != "" {
		target, err := url.Parse(os.Getenv("ARCHESTRA_BASE_URL"))
		if err != nil {
			t.Fatal(err)
		}
		// #nosec G704 -- The acceptance runner supplies this isolated API; one owned App follow-up read is rejected.
		proxy := httputil.NewSingleHostReverseProxy(target)
		proxy.ModifyResponse = func(response *http.Response) error {
			if response.Request.Method == "POST" && response.Request.URL.Path == "/api/apps" && response.StatusCode == 200 {
				body, err := io.ReadAll(response.Body)
				if err != nil {
					return err
				}
				_ = response.Body.Close()
				response.Body = io.NopCloser(bytes.NewReader(body))
				var wire struct {
					ID string `json:"id"`
				}
				if err := json.Unmarshal(body, &wire); err != nil {
					return err
				}
				if firstID.Load() == nil {
					id := wire.ID
					firstID.Store(&id)
				}
			}
			return nil
		}
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			if req.Method == "POST" && req.URL.Path == "/api/apps" {
				creates.Add(1)
			}
			id := firstID.Load()
			if req.Method == "GET" && id != nil && req.URL.Path == "/api/apps/"+*id && failed.CompareAndSwap(false, true) {
				http.Error(w, "synthetic follow-up failure", http.StatusServiceUnavailable)
				return
			}
			proxy.ServeHTTP(w, req)
		}))
		t.Cleanup(server.Close)
		t.Setenv("ARCHESTRA_BASE_URL", server.URL)
	}
	config := appNativeConfig(acctest.RandomWithPrefix("tf-app-recovery"), "html=\"<html><head></head><body>Recovery</body></html>\"\nlocked=false\nenabled=true\n")
	resource.Test(t, resource.TestCase{PreCheck: func() { testAccPreCheck(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{
		{Config: config, ExpectError: regexp.MustCompile("App read failed")},
		{Config: config, Check: func(s *terraform.State) error {
			id := firstID.Load()
			next := s.RootModule().Resources["archestra_app.test"].Primary.ID
			if id == nil || next == *id || creates.Load() != 2 {
				return fmt.Errorf("failed-create App identity was not retained and normally replaced")
			}
			old, err := projectFileTestClient(t).GetAppWithResponse(t.Context(), *id)
			if err != nil {
				return err
			}
			if old.StatusCode() != 404 {
				return fmt.Errorf("partial App was not normally deleted: status%d", old.StatusCode())
			}
			return nil
		}},
	}})
}

func TestAccAppRecreatesMissingObject(t *testing.T) {
	var id string
	config := appNativeConfig(acctest.RandomWithPrefix("tf-app-missing"), "locked=false\n")
	resource.Test(t, resource.TestCase{PreCheck: func() { testAccPreCheck(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{
		{Config: config, Check: func(s *terraform.State) error {
			id = s.RootModule().Resources["archestra_app.test"].Primary.ID
			return nil
		}},
		{PreConfig: func() {
			result, err := projectFileTestClient(t).DeleteAppWithResponse(t.Context(), uuid.MustParse(id))
			if err != nil || result.StatusCode() != 200 {
				t.Fatalf("owned App deletion failed: %v", err)
			}
		}, Config: config, Check: func(s *terraform.State) error {
			if s.RootModule().Resources["archestra_app.test"].Primary.ID == id {
				return fmt.Errorf("missing App was not recreated")
			}
			return nil
		}},
	}})
}
