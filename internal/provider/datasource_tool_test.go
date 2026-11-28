package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccToolDataSource(t *testing.T) {
	// Look up archestra__whoami — a built-in tool that always exists
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccToolDataSourceConfig(),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"data.archestra_tool.whoami",
						tfjsonpath.New("id"),
						knownvalue.NotNull(),
					),
					statecheck.ExpectKnownValue(
						"data.archestra_tool.whoami",
						tfjsonpath.New("name"),
						knownvalue.StringExact("archestra__whoami"),
					),
					statecheck.ExpectKnownValue(
						"data.archestra_tool.whoami",
						tfjsonpath.New("description"),
						knownvalue.NotNull(),
					),
				},
				// One listing maps the built-in tools by name to the same IDs.
				Check: resource.TestCheckResourceAttrPair("data.archestra_tools.builtin", "tool_id_by_name.archestra__whoami", "data.archestra_tool.whoami", "id"),
			},
		},
	})
}

func TestToolIDsWithPrefix(t *testing.T) {
	got, diags := toolIDsWithPrefix([]toolLookup{
		{ID: "1", Name: "archestra__whoami"},
		{ID: "2", Name: "archestra__list_agents"},
		{ID: "3", Name: "github__search_archestra__notes"}, // matches the search, not the prefix
		{ID: "4", Name: "archestra__shared"},
		{ID: "5", Name: "archestra__shared"},
		{ID: "2", Name: "archestra__list_agents"}, // the same tool on a second page
	}, "archestra__")
	ids := map[string]string{}
	diags.Append(got.ElementsAs(t.Context(), &ids, false)...)
	if len(ids) != 2 || ids["archestra__whoami"] != "1" || ids["archestra__list_agents"] != "2" {
		t.Fatalf("got %v", ids)
	}
	if diags.WarningsCount() != 1 {
		t.Fatalf("a shared name must warn once, got %v", diags)
	}
}

func testAccToolDataSourceConfig() string {
	return `
data "archestra_tool" "whoami" {
  name = "archestra__whoami"
}

data "archestra_tools" "builtin" {
  name_prefix = "archestra__"
}
`
}
