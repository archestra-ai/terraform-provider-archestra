package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

// TestAccKnowledgeConnectorResource uses a web crawler, the one connector
// type that needs no external credentials. Scheduled syncs are off; the
// creation-time sync may fail without network access, which the resource
// does not track.
func TestAccKnowledgeConnectorResource(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-connector")
	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
			// Oldest release with the current connector create contract
			// (initialGrants, syncPermissionsFromSource, assignment routes).
			if !testAccAPIVersionAtLeast(t, "1.4.0-rc.51") {
				t.Skip("knowledge connectors need Archestra 1.4.0-rc.51 or later")
			}
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccKnowledgeConnectorConfig(rName, 1, "first"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("archestra_knowledge_connector.test", "name", rName),
					resource.TestCheckResourceAttr("archestra_knowledge_connector.test", "connector_type", "web_crawler"),
					resource.TestCheckResourceAttr("archestra_knowledge_connector.test", "enabled", "false"),
					resource.TestCheckResourceAttr("archestra_knowledge_connector.test", "knowledge_base_ids.#", "1"),
					resource.TestCheckResourceAttrPair("archestra_knowledge_connector.test", "knowledge_base_ids.0", "archestra_knowledge_base.first", "id"),
					resource.TestCheckResourceAttrSet("archestra_knowledge_connector.test", "schedule"),
					resource.TestCheckResourceAttrSet("archestra_knowledge_connector.test", "fts_language"),
				),
			},
			// The API normalizes startUrl and adds config defaults; neither is drift.
			{
				Config:           testAccKnowledgeConnectorConfig(rName, 1, "first"),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}},
			},
			{
				Config: testAccKnowledgeConnectorConfig(rName, 2, "second"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("archestra_knowledge_connector.test", "knowledge_base_ids.#", "1"),
					resource.TestCheckResourceAttrPair("archestra_knowledge_connector.test", "knowledge_base_ids.0", "archestra_knowledge_base.second", "id"),
				),
			},
			{
				ResourceName:            "archestra_knowledge_connector.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"config", "knowledge_base_ids", "permissions"},
			},
		},
	})
}

func testAccKnowledgeConnectorConfig(name string, maxPages int, kb string) string {
	return fmt.Sprintf(`
resource "archestra_knowledge_base" "first" {
  name = "%[1]s-first"
}

resource "archestra_knowledge_base" "second" {
  name = "%[1]s-second"
}

resource "archestra_knowledge_connector" "test" {
  name               = %[1]q
  connector_type     = "web_crawler"
  config             = jsonencode({ startUrl = "example.com/", maxPages = %[2]d })
  schedule           = "0 3 * * *"
  enabled            = false
  knowledge_base_ids = [archestra_knowledge_base.%[3]s.id]
}
`, name, maxPages, kb)
}
