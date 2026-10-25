package provider

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

// The getting-started example must apply on the tested release and leave an
// empty plan. Validation alone cannot catch API-side rejections such as a
// personal install lending tools or an unpaired model and key.
func TestAccExampleBasic(t *testing.T) {
	config := exampleConfig(t, filepath.Join("..", "..", "examples", "basic"))
	resource.Test(t, resource.TestCase{PreCheck: func() { testAccPreCheck(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{
		{Config: config, Check: resource.ComposeTestCheckFunc(
			resource.TestCheckResourceAttrSet("archestra_agent.main", "id"),
			resource.TestCheckResourceAttr("archestra_agent.main", "tools.#", "1"),
			resource.TestCheckResourceAttrPair("archestra_agent.main", "model_id", "archestra_model.llama3", "id"),
		)},
		{Config: config, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
	}})
}

// exampleConfig joins an example directory's .tf files without their
// top-level terraform and provider blocks, which the test framework supplies.
func exampleConfig(t *testing.T, dir string) string {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(dir, "*.tf"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no .tf files in %s: %v", dir, err)
	}
	var config strings.Builder
	for _, file := range files {
		content, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		config.WriteString(stripProviderBlocks(string(content)))
		config.WriteString("\n")
	}
	return config.String()
}

// stripProviderBlocks drops `terraform {` and `provider "archestra" {` blocks.
// Formatted HCL closes a top-level block with a `}` at the start of a line.
func stripProviderBlocks(config string) string {
	var kept []string
	skipping := false
	for _, line := range strings.Split(config, "\n") {
		if !skipping && (strings.HasPrefix(line, "terraform {") || strings.HasPrefix(line, "provider \"archestra\" {")) {
			skipping = true
			continue
		}
		if skipping {
			if line == "}" {
				skipping = false
			}
			continue
		}
		kept = append(kept, line)
	}
	return strings.Join(kept, "\n")
}
