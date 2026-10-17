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

func TestAccKnowledgeFileRecoversFollowupFailure(t *testing.T) {
	var failed atomic.Bool
	var uploads atomic.Int32
	var firstID atomic.Pointer[string]
	if os.Getenv("TF_ACC") != "" {
		target, err := url.Parse(os.Getenv("ARCHESTRA_BASE_URL"))
		if err != nil {
			t.Fatal(err)
		}
		// #nosec G704 -- The acceptance runner explicitly supplies this isolated API; the proxy injects one owned upload follow-up failure.
		proxy := httputil.NewSingleHostReverseProxy(target)
		proxy.ModifyResponse = func(response *http.Response) error {
			if response.Request.Method == http.MethodPost && response.Request.URL.Path == "/api/knowledge-files" && response.StatusCode == 200 {
				body, err := io.ReadAll(response.Body)
				if err != nil {
					return err
				}
				_ = response.Body.Close()
				response.Body = io.NopCloser(bytes.NewReader(body))
				var value struct {
					ID string `json:"id"`
				}
				if err := json.Unmarshal(body, &value); err != nil {
					return err
				}
				if firstID.Load() == nil {
					id := value.ID
					firstID.Store(&id)
				}
			}
			return nil
		}
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			if req.Method == http.MethodPost && req.URL.Path == "/api/knowledge-files" {
				uploads.Add(1)
			}
			if req.Method == http.MethodGet && req.URL.Path == "/api/knowledge-files" && firstID.Load() != nil && failed.CompareAndSwap(false, true) {
				http.Error(w, "synthetic follow-up failure", http.StatusServiceUnavailable)
				return
			}
			proxy.ServeHTTP(w, req)
		}))
		t.Cleanup(server.Close)
		t.Setenv("ARCHESTRA_BASE_URL", server.URL)
	}
	config := knowledgeFileTestConfig(acctest.RandomWithPrefix("tf-knowledge-recovery")+".md", "Synthetic recovery fixture.", "labels=[]")
	resource.Test(t, resource.TestCase{PreCheck: func() { testAccPreCheck(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{
		{Config: config, ExpectError: regexp.MustCompile("Knowledge file listing failed")},
		{Config: config, Check: func(s *terraform.State) error {
			first := firstID.Load()
			next := s.RootModule().Resources["archestra_knowledge_file.test"].Primary.ID
			if first == nil || *first == next || uploads.Load() != 2 {
				return fmt.Errorf("failed-create recovery did not replace the retained partial identity")
			}
			r, err := projectFileTestClient(t).GetKnowledgeFileContentWithResponse(t.Context(), uuid.MustParse(*first))
			if err != nil {
				return err
			}
			if r.StatusCode() != 404 {
				return fmt.Errorf("partial identity was not normally deleted: status%d", r.StatusCode())
			}
			return nil
		}},
	}})
}

func TestAccKnowledgeFileRecreatesMissingDocument(t *testing.T) {
	var id string
	config := knowledgeFileTestConfig(acctest.RandomWithPrefix("tf-knowledge-missing")+".md", "Synthetic disappearance fixture.", "labels=[]")
	resource.Test(t, resource.TestCase{PreCheck: func() { testAccPreCheck(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{
		{Config: config, Check: func(s *terraform.State) error {
			id = s.RootModule().Resources["archestra_knowledge_file.test"].Primary.ID
			return nil
		}},
		{PreConfig: func() {
			r, err := projectFileTestClient(t).DeleteKnowledgeFileWithResponse(t.Context(), uuid.MustParse(id))
			if err != nil || r.StatusCode() != 200 {
				t.Fatalf("unable to remove owned document: %v", err)
			}
		}, Config: config, Check: func(s *terraform.State) error {
			if s.RootModule().Resources["archestra_knowledge_file.test"].Primary.ID == id {
				return fmt.Errorf("missing document was not recreated")
			}
			return nil
		}},
	}})
}
