package provider

import (
	"strings"
	"testing"
)

func TestStripProviderBlocksKeepsResources(t *testing.T) {
	config := "terraform {\n  required_providers {\n    archestra = { source = \"archestra-ai/archestra\" }\n  }\n}\n\nprovider \"archestra\" {\n  # keys from the environment\n}\n\nresource \"archestra_team\" \"main\" {\n  name = \"demo\"\n}\n"
	got := stripProviderBlocks(config)
	if strings.Contains(got, "required_providers") || strings.Contains(got, "provider \"archestra\"") {
		t.Fatalf("provider blocks were kept:\n%s", got)
	}
	if !strings.Contains(got, "resource \"archestra_team\" \"main\"") || !strings.Contains(got, "name = \"demo\"") {
		t.Fatalf("resource block was lost:\n%s", got)
	}
	// The basic example itself must reduce to resources only.
	example := exampleConfig(t, "../../examples/basic")
	if strings.Contains(example, "required_providers") || !strings.Contains(example, "resource \"archestra_agent\" \"main\"") {
		t.Fatalf("basic example was not reduced to resources:\n%s", example[:200])
	}
}
