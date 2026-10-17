package provider

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestKnowledgeFileRefresh(t *testing.T) {
	for _, tc := range []struct {
		name                        string
		listStatus, contentStatus   int
		mismatch, emptyPage, absent bool
		wantError, wantFound        bool
	}{
		{"pagination and actual bytes", 200, 200, false, false, false, false, true},
		{"listing forbidden retains identity", 403, 200, false, false, false, true, false},
		{"listing not found is not document absence", 404, 200, false, false, false, true, false},
		{"content forbidden retains identity", 200, 403, false, false, false, true, false},
		{"content disappearance requires refresh", 200, 404, false, false, false, true, false},
		{"hash conflict fails closed", 200, 200, true, false, false, true, false},
		{"invalid pagination", 200, 200, false, true, false, true, false},
		{"absent after final page", 200, 404, false, false, true, false, false},
		{"readable document missing from listing retains identity", 200, 200, false, false, true, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			id := "22222222-2222-4222-8222-222222222222"
			raw := []byte("synthetic document\n")
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				if req.URL.Path == "/api/knowledge-files/"+id+"/content" {
					w.WriteHeader(tc.contentStatus)
					if tc.contentStatus == 200 {
						_, _ = w.Write(raw)
					}
					return
				}
				if req.URL.Path != "/api/knowledge-files" {
					t.Errorf("unexpected path %s", req.URL.Path)
					w.WriteHeader(500)
					return
				}
				calls++
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tc.listStatus)
				if tc.listStatus != 200 {
					return
				}
				if req.URL.Query().Get("limit") != "100" {
					t.Error("listing must request bounded pages")
				}
				rows := []any{}
				hasNext := false
				if calls == 1 {
					hasNext = true
					if !tc.emptyPage {
						rows = append(rows, map[string]any{"id": "11111111-1111-4111-8111-111111111111"})
					}
				} else if !tc.absent {
					hash := fmt.Sprintf("%x", sha256.Sum256(raw))
					if tc.mismatch {
						hash = strings.Repeat("0", 64)
					}
					rows = append(rows, map[string]any{"id": id, "filename": "document.md", "mimeType": "text/markdown", "sizeBytes": len(raw), "contentHash": hash, "directoryId": nil, "labels": []any{map[string]string{"key": "fixture", "value": "true"}}, "knowledgeBases": []any{map[string]string{"id": "kb"}}})
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"data": rows, "pagination": map[string]any{"hasNext": hasNext}})
			}))
			defer server.Close()
			c, err := client.NewClientWithResponses(server.URL)
			if err != nil {
				t.Fatal(err)
			}
			data := KnowledgeFileResourceModel{ID: types.StringValue(id)}
			var diags diag.Diagnostics
			found := (&KnowledgeFileResource{client: c}).refresh(t.Context(), &data, &diags)
			if found != tc.wantFound || diags.HasError() != tc.wantError {
				t.Fatalf("found=%v diagnostics=%v", found, diags)
			}
			if data.ID.ValueString() != id {
				t.Fatal("read lost the stable identity")
			}
			if found && (data.ContentBase64.ValueString() != base64.StdEncoding.EncodeToString(raw) || calls != 2 || !data.DirectoryID.IsNull() || len(data.Labels.Elements()) != 1 || len(data.KnowledgeBaseIDs.Elements()) != 1) {
				t.Fatalf("read did not preserve actual document state: %#v", data)
			}
		})
	}
}

func TestKnowledgeFileContentValidation(t *testing.T) {
	for _, tc := range []struct {
		value string
		valid bool
	}{{"", false}, {"YQ==", true}, {"YR==", false}, {"YQ==\n", false}, {"not base64", false}} {
		var resp validator.StringResponse
		knowledgeFileContentValidator{}.ValidateString(context.Background(), validator.StringRequest{Path: path.Root("content_base64"), ConfigValue: types.StringValue(tc.value)}, &resp)
		if resp.Diagnostics.HasError() == tc.valid {
			t.Errorf("value %q diagnostics %v", tc.value, resp.Diagnostics)
		}
	}
}

func knowledgeFileTestConfig(name, content, extra string) string {
	return fmt.Sprintf(`resource "archestra_knowledge_file" "test" {
 filename=%q
 mime_type="text/markdown"
 content_base64=%q
 permissions=[]
 %s
 }`, name, base64.StdEncoding.EncodeToString([]byte(content)), extra)
}

func TestAccKnowledgeFileLifecycle(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-knowledge-file") + ".md"
	var id string
	first := knowledgeFileTestConfig(name, "# Synthetic\nOriginal body.\n", `labels=[{key="stage",value="initial"}]`)
	dir := `resource "archestra_knowledge_directory" "test" { name="` + name + `" }` + "\n"
	moved := dir + knowledgeFileTestConfig("renamed-"+name, "# Synthetic\nOriginal body.\n", `directory_id=archestra_knowledge_directory.test.id
 labels=[{key="stage",value="moved"}]
 permissions=[{subject_type="organization",subject_id="*",actions=["read"]}]`)
	// Replace the default permissions only in the one moved configuration.
	moved = strings.Replace(moved, " permissions=[]\n", "", 1)
	root := knowledgeFileTestConfig("renamed-"+name, "# Synthetic\nOriginal body.\n", `labels=[]`)
	capture := func(s *terraform.State) error {
		id = s.RootModule().Resources["archestra_knowledge_file.test"].Primary.ID
		return nil
	}
	preserved := func(s *terraform.State) error {
		if s.RootModule().Resources["archestra_knowledge_file.test"].Primary.ID != id {
			return fmt.Errorf("metadata change replaced document")
		}
		return nil
	}
	checkDestroy := func(*terraform.State) error {
		r, err := projectFileTestClient(t).GetKnowledgeFileContentWithResponse(t.Context(), uuid.MustParse(id))
		if err != nil {
			return err
		}
		if r.StatusCode() != 404 {
			return fmt.Errorf("deleted file remains readable: %d", r.StatusCode())
		}
		return nil
	}
	resource.Test(t, resource.TestCase{PreCheck: func() { testAccPreCheck(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, CheckDestroy: checkDestroy, Steps: []resource.TestStep{
		{Config: first, Check: resource.ComposeTestCheckFunc(capture, resource.TestCheckResourceAttr("archestra_knowledge_file.test", "knowledge_base_ids.#", "0"))},
		{ResourceName: "archestra_knowledge_file.test", ImportState: true, ImportStateVerify: true},
		{Config: first, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
		{Config: moved, Check: resource.ComposeTestCheckFunc(preserved, resource.TestCheckResourceAttr("archestra_knowledge_file.test", "permissions.#", "1"))},
		{ResourceName: "archestra_knowledge_file.test", ImportState: true, ImportStateVerify: true},
		{PreConfig: func() {
			_, err := projectFileTestClient(t).UpdateKnowledgeFileWithBodyWithResponse(t.Context(), uuid.MustParse(id), "application/json", strings.NewReader(`{"filename":"drift.md","labels":[]}`))
			if err != nil {
				t.Fatal(err)
			}
		}, Config: moved, PlanOnly: true, ExpectNonEmptyPlan: true},
		{Config: moved, Check: preserved},
		{Config: root, Check: resource.ComposeTestCheckFunc(preserved, resource.TestCheckResourceAttr("archestra_knowledge_file.test", "labels.#", "0"), resource.TestCheckResourceAttr("archestra_knowledge_file.test", "permissions.#", "0"), resource.TestCheckNoResourceAttr("archestra_knowledge_file.test", "directory_id"))},
		{Config: root, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
	}})
}

func TestAccKnowledgeFileContentReplacement(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-knowledge-replace") + ".md"
	var id string
	resource.Test(t, resource.TestCase{PreCheck: func() { testAccPreCheck(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{
		{Config: knowledgeFileTestConfig(name, "Original body.", ""), Check: func(s *terraform.State) error {
			id = s.RootModule().Resources["archestra_knowledge_file.test"].Primary.ID
			return nil
		}},
		{Config: knowledgeFileTestConfig(name, "Updated body.", ""), ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction("archestra_knowledge_file.test", plancheck.ResourceActionDestroyBeforeCreate)}}, Check: func(s *terraform.State) error {
			if s.RootModule().Resources["archestra_knowledge_file.test"].Primary.ID == id {
				return fmt.Errorf("content replacement retained identity")
			}
			r, err := projectFileTestClient(t).GetKnowledgeFileContentWithResponse(t.Context(), uuid.MustParse(id))
			if err != nil {
				return err
			}
			if r.StatusCode() != 404 {
				return fmt.Errorf("replaced file remains readable")
			}
			return nil
		}},
	}})
}

func TestAccKnowledgeFileRejectsInvalidBytes(t *testing.T) {
	config := strings.Replace(knowledgeFileTestConfig("invalid.md", "synthetic", ""), `content_base64="c3ludGhldGlj"`, `content_base64="YR=="`, 1)
	resource.Test(t, resource.TestCase{PreCheck: func() { testAccPreCheck(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{{Config: config, ExpectError: regexp.MustCompile("Invalid document bytes")}}})
}
