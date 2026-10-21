package provider

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"testing"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func appNativeConfig(name, fields string) string {
	return fmt.Sprintf("resource \"archestra_app\" \"test\" {\nname=%q\n%s\n}\n", name, fields)
}

func TestAccAppLifecycle(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-app")
	html := "<html><head></head><body><h1>First version</h1></body></html>"
	next := "<html><head></head><body><h1>Second version</h1></body></html>"
	config := func(name, html, permissions, other string) string {
		return appNativeConfig(name, fmt.Sprintf("html=%q\nbrowser_permissions=%s\nenabled=true\nlocked=false\n%s", html, permissions, other))
	}
	initial := config(name, html, "[]", "labels=[]\npermissions=[]\nopen_in_fullscreen=false")
	metadata := config(name+"-renamed", html, "[]", "description=\"Managed App\"\nicon=\"A\"\nlabels=[{key=\"purpose\",value=\"test\"}]\npermissions=[]\nopen_in_fullscreen=true")
	content := config(name+"-renamed", next, "[]", "labels=[]\npermissions=[]\nopen_in_fullscreen=false")
	withPermission := config(name+"-renamed", next, "[\"camera\"]", "labels=[]\npermissions=[]")
	var id string
	same := func(s *terraform.State) error {
		if s.RootModule().Resources["archestra_app.test"].Primary.ID != id {
			return fmt.Errorf("App update replaced identity")
		}
		return nil
	}
	resource.Test(t, resource.TestCase{PreCheck: func() { testAccPreCheck(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{
		{Config: initial, Check: resource.ComposeTestCheckFunc(resource.TestCheckResourceAttr("archestra_app.test", "html", html), resource.TestCheckResourceAttr("archestra_app.test", "content_version", "1"), func(s *terraform.State) error {
			id = s.RootModule().Resources["archestra_app.test"].Primary.ID
			return nil
		})},
		{ResourceName: "archestra_app.test", ImportState: true, ImportStateVerify: true},
		{Config: metadata, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction("archestra_app.test", plancheck.ResourceActionUpdate)}}, Check: resource.ComposeTestCheckFunc(same, resource.TestCheckResourceAttr("archestra_app.test", "content_version", "1"), resource.TestCheckResourceAttr("archestra_app.test", "open_in_fullscreen", "true"))},
		{Config: content, Check: resource.ComposeTestCheckFunc(same, resource.TestCheckResourceAttr("archestra_app.test", "content_version", "2"), resource.TestCheckNoResourceAttr("archestra_app.test", "description"), resource.TestCheckNoResourceAttr("archestra_app.test", "icon"), resource.TestCheckResourceAttr("archestra_app.test", "labels.#", "0"))},
		{Config: withPermission, Check: resource.ComposeTestCheckFunc(same, resource.TestCheckResourceAttr("archestra_app.test", "content_version", "3"), resource.TestCheckResourceAttr("archestra_app.test", "browser_permissions.#", "1"))},
		{Config: content, Check: resource.ComposeTestCheckFunc(same, resource.TestCheckResourceAttr("archestra_app.test", "content_version", "4"), resource.TestCheckResourceAttr("archestra_app.test", "browser_permissions.#", "0"))},
		{Config: content, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
		{PreConfig: func() {
			appNativePatch(t, id, map[string]any{"html": "<html><head></head><body>External change</body></html>", "baseVersion": 4})
		}, Config: content, Check: resource.ComposeTestCheckFunc(same, resource.TestCheckResourceAttr("archestra_app.test", "html", next), resource.TestCheckResourceAttr("archestra_app.test", "content_version", "6"))},
		{ResourceName: "archestra_app.test", ImportState: true, ImportStateVerify: true},
	}})
}

func TestAccAppOmittedContentAndSettings(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-app-preserve")
	var id, original string
	config := func(name string) string { return appNativeConfig(name, "") }
	apiHTML := "<html><head></head><body>Authored outside Terraform.</body></html>"
	resource.Test(t, resource.TestCase{PreCheck: func() { testAccPreCheck(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{
		{Config: appNativeConfig(name, "enabled=true\nlocked=false"), Check: func(s *terraform.State) error {
			v := s.RootModule().Resources["archestra_app.test"].Primary
			id = v.ID
			original = v.Attributes["html"]
			if original == "" {
				return fmt.Errorf("default starter was not read")
			}
			return nil
		}},
		{PreConfig: func() {
			appNativePatch(t, id, map[string]any{"html": apiHTML, "uiPermissions": map[string]any{"microphone": map[string]any{}}, "baseVersion": 1})
			api := projectFileTestClient(t)
			locked, err := api.LockAppWithResponse(t.Context(), uuid.MustParse(id))
			if err != nil || locked.StatusCode() != 200 {
				t.Fatalf("owned App lock failed: %v", err)
			}
			disabled, err := api.DisableAppWithResponse(t.Context(), uuid.MustParse(id))
			if err != nil || disabled.StatusCode() != 200 {
				t.Fatalf("owned App disable failed: %v", err)
			}
		}, Config: config(name + "-renamed"), Check: resource.ComposeTestCheckFunc(resource.TestCheckResourceAttr("archestra_app.test", "html", apiHTML), resource.TestCheckResourceAttr("archestra_app.test", "content_version", "2"), resource.TestCheckResourceAttr("archestra_app.test", "browser_permissions.#", "1"), resource.TestCheckResourceAttr("archestra_app.test", "enabled", "false"), resource.TestCheckResourceAttr("archestra_app.test", "locked", "true"))},
		{Config: config(name + "-renamed"), ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
		{ResourceName: "archestra_app.test", ImportState: true, ImportStateVerify: true},
		{Config: appNativeConfig(name+"-renamed", "enabled=true\nlocked=false"), Check: resource.ComposeTestCheckFunc(resource.TestCheckResourceAttr("archestra_app.test", "html", apiHTML), resource.TestCheckResourceAttr("archestra_app.test", "content_version", "2"))},
	}})
}

func TestAccAppInlineTools(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-app-tools")
	catalog := `local_config={command="npx",arguments=["-y","@modelcontextprotocol/server-filesystem","/tmp"]}`
	if endpoint := os.Getenv("ARCHESTRA_TEST_MCP_URL"); endpoint != "" {
		catalog = fmt.Sprintf(`remote_config={url=%q}`, endpoint)
	}
	base := fmt.Sprintf("resource \"archestra_mcp_registry_entry\" \"test\" {\nname=%q\nmultitenant=false\n%s\n}\nresource \"archestra_mcp_server\" \"test\" {\nname=%q\ncatalog_id=archestra_mcp_registry_entry.test.id\nscope=\"org\"\n}\n", name+"-catalog", catalog, name+"-install")
	config := func(tools string) string { return base + appNativeConfig(name, "tools="+tools+"\nlocked=false") }
	initial := config("[for tool in archestra_mcp_server.test.tools:{tool_id=tool.id,mcp_server_id=archestra_mcp_server.test.id,credential_resolution_mode=\"static\"}]")
	dynamic := config("[{tool_id=archestra_mcp_server.test.tools[0].id,credential_resolution_mode=\"dynamic\"}]")
	resource.Test(t, resource.TestCase{PreCheck: func() { testAccPreCheck(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{
		{Config: initial, Check: resource.TestCheckResourceAttrPair("archestra_app.test", "tools.#", "archestra_mcp_server.test", "tools.#")},
		{ResourceName: "archestra_app.test", ImportState: true, ImportStateVerify: true},
		{Config: dynamic, Check: resource.ComposeTestCheckFunc(resource.TestCheckResourceAttr("archestra_app.test", "tools.#", "1"), resource.TestCheckTypeSetElemNestedAttrs("archestra_app.test", "tools.*", map[string]string{"credential_resolution_mode": "dynamic"}))},
		{Config: dynamic, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
		{Config: config("[]"), Check: resource.TestCheckResourceAttr("archestra_app.test", "tools.#", "0")},
		{Config: config("[]"), ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
	}})
}

func TestAccAppExplicitLifecycle(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-app-lifecycle")
	config := func(enabled, locked bool) string {
		return appNativeConfig(name, fmt.Sprintf("enabled=%t\nlocked=%t\n", enabled, locked))
	}
	resource.Test(t, resource.TestCase{PreCheck: func() { testAccPreCheck(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{
		{Config: config(true, false)},
		{Config: config(false, true), Check: resource.ComposeTestCheckFunc(resource.TestCheckResourceAttr("archestra_app.test", "enabled", "false"), resource.TestCheckResourceAttr("archestra_app.test", "locked", "true"))},
		{Config: config(false, true), ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
		{Config: config(true, false), Check: resource.ComposeTestCheckFunc(resource.TestCheckResourceAttr("archestra_app.test", "enabled", "true"), resource.TestCheckResourceAttr("archestra_app.test", "locked", "false"))},
	}})
}

func TestAccAppRejectsInvalidSlug(t *testing.T) {
	resource.Test(t, resource.TestCase{PreCheck: func() { testAccPreCheck(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{
		{Config: appNativeConfig("invalid-app-slug", `slug="catalog"`), ExpectError: regexp.MustCompile("Invalid Attribute Value")},
	}})
}

func appNativePatch(t *testing.T, id string, patch map[string]any) {
	t.Helper()
	body, err := json.Marshal(patch)
	if err != nil {
		t.Fatal(err)
	}
	response, err := projectFileTestClient(t).UpdateAppWithBodyWithResponse(t.Context(), uuid.MustParse(id), "application/json", bytes.NewReader(body))
	if err != nil || response.StatusCode() != http.StatusOK {
		t.Fatalf("owned App edit failed: %v", err)
	}
}
