package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"regexp"
	"testing"
)

func pluginFixtureConfig(name, description string, enabled bool, changed bool) string {
	files := map[string]map[string]string{
		".claude-plugin/plugin.json": {"content": `{"name":"synthetic-plugin","version":"1.0.0"}`, "encoding": "utf8", "mode": "100644"},
		"README.md":                  {"content": "Synthetic plugin fixture.", "encoding": "utf8", "mode": "100644"},
		"scripts/example0.sh":        {"content": "#!/bin/sh\nprintf 'synthetic\\n'\n", "encoding": "utf8", "mode": "100755"},
		"assets/icon.bin":            {"content": "AAECAwQ=", "encoding": "base64", "mode": "100644"},
	}
	if changed {
		delete(files, "assets/icon.bin")
		files["README.md"]["content"] = "Changed synthetic bytes."
		files["scripts/example0.sh"]["mode"] = "100644"
	}
	encoded, err := json.Marshal(files)
	if err != nil {
		panic(err)
	}
	labels := `[{key="purpose",value="plugin"}]`
	if changed {
		labels = `[]`
	}
	return fmt.Sprintf(`resource "archestra_plugin" "test" {
 display_name=%q
 description=%q
 client_type="claude-code"
 supported_platforms=["posix"]
 enabled=%t
 labels=%s
 files=jsondecode(%q)
 }`, name, description, enabled, labels, string(encoded))
}
func TestAccPluginResource(t *testing.T) {
	metadata, resent := metadataWriteGuard(t, "/api/plugins/", "files")
	var id, hash string
	capture := func(s *terraform.State) error {
		v := s.RootModule().Resources["archestra_plugin.test"]
		id = v.Primary.ID
		hash = v.Primary.Attributes["content_hash"]
		if hash == "" {
			return fmt.Errorf("plugin content hash missing")
		}
		return nil
	}
	preserve := func(s *terraform.State) error {
		v := s.RootModule().Resources["archestra_plugin.test"]
		if v.Primary.ID != id || v.Primary.Attributes["content_hash"] != hash || resent.Load() {
			return fmt.Errorf("metadata change resent or changed plugin files")
		}
		return nil
	}
	changed := func(s *terraform.State) error {
		v := s.RootModule().Resources["archestra_plugin.test"]
		if v.Primary.ID != id || v.Primary.Attributes["content_hash"] == hash {
			return fmt.Errorf("file update failed to preserve plugin identity and change its content hash")
		}
		return nil
	}
	resource.Test(t, resource.TestCase{PreCheck: func() { testAccPreCheck(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{
		{Config: pluginFixtureConfig("tf-manual-plugin", "Initial", false, false), Check: resource.ComposeTestCheckFunc(capture, resource.TestCheckResourceAttr("archestra_plugin.test", "enabled", "false"), resource.TestCheckResourceAttr("archestra_plugin.test", "files.assets/icon.bin.encoding", "base64"), resource.TestCheckResourceAttr("archestra_plugin.test", "files.scripts/example0.sh.mode", "100755"))},
		{ResourceName: "archestra_plugin.test", ImportState: true, ImportStateVerify: true},
		{PreConfig: func() { metadata.Store(true) }, Config: pluginFixtureConfig("tf-renamed-plugin", "Metadata only", true, false), ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction("archestra_plugin.test", plancheck.ResourceActionUpdate)}}, Check: preserve},
		{PreConfig: func() { metadata.Store(false) }, Config: pluginFixtureConfig("tf-renamed-plugin", "Metadata only", true, true), Check: resource.ComposeTestCheckFunc(changed, resource.TestCheckResourceAttr("archestra_plugin.test", "files.%", "3"), resource.TestCheckNoResourceAttr("archestra_plugin.test", "files.assets/icon.bin.content"), resource.TestCheckResourceAttr("archestra_plugin.test", "files.scripts/example0.sh.mode", "100644"), resource.TestCheckResourceAttr("archestra_plugin.test", "labels.#", "0"))},
		{Config: pluginFixtureConfig("tf-renamed-plugin", "Metadata only", true, true), ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
	}})
}
func TestAccPluginRejectsInvalidInputs(t *testing.T) {
	for _, tc := range []struct{ name, config, match string }{
		{"path", `resource "archestra_plugin" "test" {
 display_name="invalid-plugin"
 client_type="claude-code"
 supported_platforms=["posix"]
 files={"../outside"={content="synthetic"}}
 }`, "Invalid plugin file path"},
		{"name", pluginFixtureConfig(" padded plugin ", "", false, false), "Noncanonical display name"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resource.Test(t, resource.TestCase{PreCheck: func() { testAccPreCheck(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{{Config: tc.config, ExpectError: regexp.MustCompile(tc.match)}}})
		})
	}
}
func TestPluginPathValidation(t *testing.T) {
	for _, tc := range []struct {
		name    string
		keys    []string
		invalid bool
	}{
		{"ordinary", []string{".claude-plugin/plugin.json", "scripts/example0.sh", "assets/icon.bin"}, false},
		{"traversal", []string{"../outside"}, true},
		{"absolute", []string{"/outside"}, true},
		{"backslash", []string{"x\\y"}, true},
		{"null", []string{"x\x00y"}, true},
		{"case collision", []string{"README.md", "readme.md"}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			files := map[string]string{}
			for _, key := range tc.keys {
				files[key] = "fixture"
			}
			value, d := types.MapValueFrom(context.Background(), types.StringType, files)
			if d.HasError() {
				t.Fatal(d)
			}
			response := validator.MapResponse{}
			pluginPathsValidator{}.ValidateMap(t.Context(), validator.MapRequest{ConfigValue: value}, &response)
			if response.Diagnostics.HasError() != tc.invalid {
				t.Fatalf("unexpected path validation result: %v", response.Diagnostics)
			}
		})
	}
}
