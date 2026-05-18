package provider

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccKnowledgeBaseResource exercises Create / Read / Update / Import.
func TestAccKnowledgeBaseResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccKnowledgeBaseResourceConfig("Initial KB", `description = "first version"`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("archestra_knowledge_base.test", "name", "Initial KB"),
					resource.TestCheckResourceAttr("archestra_knowledge_base.test", "description", "first version"),
					resource.TestCheckResourceAttrSet("archestra_knowledge_base.test", "id"),
					resource.TestCheckResourceAttrSet("archestra_knowledge_base.test", "status"),
					resource.TestCheckResourceAttrSet("archestra_knowledge_base.test", "created_at"),
				),
			},
			{
				ResourceName:      "archestra_knowledge_base.test",
				ImportState:       true,
				ImportStateVerify: true,
				// updated_at intentionally tracks backend writes; the
				// import refresh fetches a fresh value that may differ
				// from the create-time value by sub-second precision.
				ImportStateVerifyIgnore: []string{"updated_at"},
			},
			{
				Config: testAccKnowledgeBaseResourceConfig("Renamed KB", `description = "second version"`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("archestra_knowledge_base.test", "name", "Renamed KB"),
					resource.TestCheckResourceAttr("archestra_knowledge_base.test", "description", "second version"),
				),
			},
		},
	})
}

// TestAccKnowledgeBaseResource_ClearDescription pins the nullable
// round-trip: description set → unset → state null. The backend zod
// for the update body is `z.string().nullable().optional()` and the
// generated client serialises `description` as a plain (non-omitempty)
// pointer, so plan-null → wire `null` clears the field.
func TestAccKnowledgeBaseResource_ClearDescription(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccKnowledgeBaseResourceConfig("Described KB", `description = "to be cleared"`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("archestra_knowledge_base.test", "description", "to be cleared"),
				),
			},
			{
				Config: testAccKnowledgeBaseResourceConfig("Described KB", ""),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckNoResourceAttr("archestra_knowledge_base.test", "description"),
				),
			},
		},
	})
}

// TestAccKnowledgeBaseResource_EmptyNameRejected verifies the
// length-validator catches `name = ""` at plan time, before any
// backend round-trip.
func TestAccKnowledgeBaseResource_EmptyNameRejected(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "archestra_knowledge_base" "test" {
  name = ""
}
`,
				ExpectError: regexp.MustCompile(`Attribute name string length must be between 1 and 256`),
			},
		},
	})
}

func testAccKnowledgeBaseResourceConfig(name, descAttr string) string {
	return fmt.Sprintf(`
resource "archestra_knowledge_base" "test" {
  name = %[1]q
  %[2]s
}
`, name, descAttr)
}
