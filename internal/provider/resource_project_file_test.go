package provider

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/google/uuid"
	fwresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func projectFileTestClient(t *testing.T) *client.ClientWithResponses {
	t.Helper()
	c, err := client.NewClientWithResponses(os.Getenv("ARCHESTRA_BASE_URL"), client.WithRequestEditorFn(func(_ context.Context, r *http.Request) error {
		r.Header.Set("Authorization", os.Getenv("ARCHESTRA_API_KEY"))
		return nil
	}))
	if err != nil {
		t.Fatal(err)
	}
	return c
}
func projectFileConfig(name, filename string, content []byte) string {
	return fmt.Sprintf(`resource "archestra_project" "test" { name=%q }
 resource "archestra_project_file" "test" {
 project_id=archestra_project.test.id
 filename=%q
 content_base64=%q
 }`, name, filename, base64.StdEncoding.EncodeToString(content))
}
func TestAccProjectFileText(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-project-file")
	var id, fileID string
	config := func(s string) string { return projectFileConfig(name, "notes.txt", []byte(s)) }
	capture := func(s *terraform.State) error {
		v := s.RootModule().Resources["archestra_project_file.test"].Primary
		id = v.ID
		fileID = v.Attributes["file_id"]
		return nil
	}
	same := func(content string) resource.TestCheckFunc {
		return resource.ComposeTestCheckFunc(resource.TestCheckResourceAttr("archestra_project_file.test", "content_base64", base64.StdEncoding.EncodeToString([]byte(content))), resource.TestCheckResourceAttr("archestra_project_file.test", "sha256", fmt.Sprintf("%x", sha256.Sum256([]byte(content)))), func(s *terraform.State) error {
			if s.RootModule().Resources["archestra_project_file.test"].Primary.ID != id {
				return fmt.Errorf("text edit replaced the file")
			}
			return nil
		})
	}
	resource.Test(t, resource.TestCase{PreCheck: func() { testAccPreCheck(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, CheckDestroy: func(*terraform.State) error {
		r, err := projectFileTestClient(t).GetSkillSandboxArtifactWithResponse(t.Context(), fileID)
		if err != nil {
			return err
		}
		if r.StatusCode() != 404 {
			return fmt.Errorf("destroyed file remains readable: %d", r.StatusCode())
		}
		return nil
	}, Steps: []resource.TestStep{
		{Config: config("original"), Check: resource.ComposeTestCheckFunc(capture, resource.TestCheckResourceAttr("archestra_project_file.test", "size_bytes", "8"))},
		{ResourceName: "archestra_project_file.test", ImportState: true, ImportStateVerify: true},
		{Config: config("original"), PlanOnly: true},
		{Config: config("updated ✓"), Check: same("updated ✓")},
		{Config: config(""), Check: resource.ComposeTestCheckFunc(same(""), resource.TestCheckResourceAttr("archestra_project_file.test", "size_bytes", "0"))},
		{Config: config(""), PlanOnly: true},
		{PreConfig: func() {
			r, err := projectFileTestClient(t).UpdateSkillSandboxArtifactContentWithResponse(t.Context(), fileID, client.UpdateSkillSandboxArtifactContentJSONRequestBody{Content: "out-of-band"})
			if err != nil || r.JSON200 == nil {
				t.Fatal("unable to modify owned file", err)
			}
		}, Config: config(""), PlanOnly: true, ExpectNonEmptyPlan: true},
		{Config: config(""), Check: same("")},
		{Config: config("restored"), Check: same("restored")},
	}})
}
func TestAccProjectFileBinaryReplacement(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-project-binary")
	var oldID string
	resource.Test(t, resource.TestCase{PreCheck: func() { testAccPreCheck(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{
		{Config: projectFileConfig(name, "blob.bin", []byte{0, 255, 1}), Check: func(s *terraform.State) error {
			oldID = s.RootModule().Resources["archestra_project_file.test"].Primary.Attributes["file_id"]
			return nil
		}},
		{ResourceName: "archestra_project_file.test", ImportState: true, ImportStateVerify: true},
		{Config: projectFileConfig(name, "blob.bin", []byte{0, 254, 2}), ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction("archestra_project_file.test", plancheck.ResourceActionDestroyBeforeCreate)}}, Check: func(s *terraform.State) error {
			v := s.RootModule().Resources["archestra_project_file.test"].Primary
			if v.Attributes["file_id"] == oldID {
				return fmt.Errorf("binary edit retained the old identity")
			}
			r, err := projectFileTestClient(t).GetSkillSandboxArtifactWithResponse(t.Context(), oldID)
			if err != nil {
				return err
			}
			if r.StatusCode() != 404 {
				return fmt.Errorf("replaced binary remains readable")
			}
			return nil
		}},
	}})
}
func TestAccProjectFileCollisionCompensates(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-file-collision")
	var projectID uuid.UUID
	var existing string
	create := fmt.Sprintf(`resource "archestra_project" "test" { name=%q }`, name)
	config := projectFileConfig(name, "notes.txt", []byte("managed"))
	resource.Test(t, resource.TestCase{PreCheck: func() { testAccPreCheck(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{
		{Config: create, Check: func(s *terraform.State) error {
			projectID = uuid.MustParse(s.RootModule().Resources["archestra_project.test"].Primary.ID)
			r, err := projectFileTestClient(t).UploadProjectFilesWithResponse(t.Context(), projectID, client.UploadProjectFilesJSONRequestBody{Name: "notes.txt", DataBase64: base64.StdEncoding.EncodeToString([]byte("existing"))})
			if err != nil {
				return err
			}
			if r.JSON200 == nil {
				return fmt.Errorf("unable to upload collision fixture: %d", r.StatusCode())
			}
			existing = r.JSON200.Id.String()
			return nil
		}},
		{Config: config, ExpectError: regexp.MustCompile("Project file name collision")},
		{PreConfig: func() {
			r, err := projectFileTestClient(t).GetProjectFilesWithResponse(t.Context(), projectID)
			if err != nil || r.JSON200 == nil {
				t.Fatal("unable to list collision fixture", err)
			}
			if len(*r.JSON200) != 1 {
				t.Fatalf("collision left extra file rows: %d", len(*r.JSON200))
			}
			raw, err := projectFileTestClient(t).GetSkillSandboxArtifactWithResponse(t.Context(), existing)
			if err != nil || string(raw.Body) != "existing" {
				t.Fatal("collision changed existing file", err)
			}
			deleted, err := projectFileTestClient(t).DeleteSkillSandboxArtifactWithResponse(t.Context(), existing)
			if err != nil || deleted.StatusCode() != 200 {
				t.Fatal("unable to delete owned collision fixture", err)
			}
		}, Config: config, Check: resource.TestCheckResourceAttr("archestra_project_file.test", "content_base64", base64.StdEncoding.EncodeToString([]byte("managed")))},
	}})
}
func TestAccProjectFileRejectsInvalidInput(t *testing.T) {
	for _, tc := range []struct{ name, filename, content, diagnostic string }{
		{"empty", "notes.txt", "", "Empty initial upload"},
		{"reserved", "instructions.md", "YQ==", "Invalid project filename"},
		{"path", "../escape.txt", "YQ==", "Invalid project filename"},
		{"base64", "notes.txt", "not-base64", "Invalid project file bytes"},
		{"noncanonical", "notes.txt", "YR==", "Invalid project file bytes"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			config := fmt.Sprintf(`resource "archestra_project_file" "test" {
 project_id="11111111-1111-4111-8111-111111111111"
 filename=%q
 content_base64=%q
 }`, tc.filename, tc.content)
			resource.Test(t, resource.TestCase{PreCheck: func() { testAccPreCheck(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{{Config: config, PlanOnly: true, ExpectError: regexp.MustCompile(tc.diagnostic)}}})
		})
	}
}
func projectFileUnitState(t *testing.T, r *ProjectFileResource) tfsdk.State {
	t.Helper()
	var sr fwresource.SchemaResponse
	r.Schema(t.Context(), fwresource.SchemaRequest{}, &sr)
	s := tfsdk.State{Schema: sr.Schema}
	data := ProjectFileResourceModel{ID: types.StringValue("11111111-1111-4111-8111-111111111111:22222222-2222-4222-8222-222222222222"), ProjectID: types.StringValue("11111111-1111-4111-8111-111111111111"), FileID: types.StringValue("22222222-2222-4222-8222-222222222222"), Filename: types.StringValue("notes.txt"), ContentBase64: types.StringValue("YQ=="), UploadMIMEType: types.StringValue("")}
	if d := s.Set(t.Context(), &data); d.HasError() {
		t.Fatal(d)
	}
	return s
}
func TestProjectFileFailureAndScopePreserveState(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		scope  string
	}{
		{"forbidden", 403, "11111111-1111-4111-8111-111111111111"}, {"missing-bytes", 404, "11111111-1111-4111-8111-111111111111"}, {"server-error", 500, "11111111-1111-4111-8111-111111111111"}, {"wrong-scope", 200, "33333333-3333-4333-8333-333333333333"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			deletes, downloads := 0, 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				if strings.Contains(req.URL.Path, "/projects/") {
					w.Header().Set("Content-Type", "application/json")
					_ = json.NewEncoder(w).Encode([]map[string]any{{"id": "22222222-2222-4222-8222-222222222222", "filename": "notes.txt", "mimeType": "text/plain", "projectId": tc.scope, "downloadRef": "22222222-2222-4222-8222-222222222222", "downloadable": true, "sizeBytes": 1}})
					return
				}
				if req.Method == http.MethodDelete {
					deletes++
				} else {
					downloads++
				}
				w.WriteHeader(tc.status)
			}))
			defer srv.Close()
			c, err := client.NewClientWithResponses(srv.URL)
			if err != nil {
				t.Fatal(err)
			}
			r := ProjectFileResource{client: c}
			state := projectFileUnitState(t, &r)
			response := fwresource.ReadResponse{State: state}
			r.Read(t.Context(), fwresource.ReadRequest{State: state}, &response)
			if !response.Diagnostics.HasError() || response.State.Raw.IsNull() {
				t.Fatal("failed read forgot existing state", response.Diagnostics)
			}
			if tc.name == "wrong-scope" {
				deleted := fwresource.DeleteResponse{State: state}
				r.Delete(t.Context(), fwresource.DeleteRequest{State: state}, &deleted)
				if !deleted.Diagnostics.HasError() || deletes != 0 || downloads != 0 {
					t.Fatal("scope mismatch reached raw file operations")
				}
			}
		})
	}
}
