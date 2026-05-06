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
			},
		},
	})
}

func testAccToolDataSourceConfig() string {
	return `
data "archestra_tool" "whoami" {
  name = "archestra__whoami"
}
`
}
