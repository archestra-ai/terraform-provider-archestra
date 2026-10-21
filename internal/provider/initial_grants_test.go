package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"testing"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestAccInitialGrants(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-initial-grants")
	config := func(grants, description string) string {
		return fmt.Sprintf(`
resource "archestra_team" "test" {name=%q}
locals { grants=%s }
resource "archestra_agent" "test" {
 name=%q
 description=%q
 permissions=local.grants
}
resource "archestra_mcp_gateway" "test" {
 name=%q
 permissions=local.grants
}
resource "archestra_project" "test" {
 name=%q
 permissions=local.grants
}
resource "archestra_knowledge_base" "test" {
 name=%q
 permissions=local.grants
}
resource "archestra_mcp_registry_entry" "test" {
 name=%q
 description=%q
 remote_config={url="http://127.0.0.1:19999/mcp"}
 permissions=local.grants
}
resource "archestra_skill" "test" {
 name=%q
 description="Synthetic grant fixture"
 content="Synthetic body."
 files={}
 environment_ids=[]
 labels=[]
 permissions=local.grants
}
resource "archestra_credential" "test" {
 key=%q
 name="Synthetic empty credential definition"
 description=%q
 permissions=local.grants
}
resource "archestra_model_provider" "test" {
 name=%q
 llm_provider="ollama"
 %s
 base_url=%q
 shared=true
 permissions=local.grants
}
`, name, grants, name+"-agent", description, name+"-gateway", name+"-project", name+"-knowledge", name+"-catalog", description, name+"-skill", name+"-credential", description, name+"-llm", testAccLLMKeyMaterial(), os.Getenv("ARCHESTRA_TEST_LLM_BASE_URL"))
	}
	granted := `[{subject_type="team",subject_id=archestra_team.test.id,actions=["read"]}]`
	identities := map[string]string{}
	var skillVersion string
	expectEmpty := false
	verify := func(s *terraform.State) error {
		c, err := client.NewClientWithResponses(os.Getenv("ARCHESTRA_BASE_URL"), client.WithRequestEditorFn(func(_ context.Context, r *http.Request) error {
			r.Header.Set("Authorization", os.Getenv("ARCHESTRA_API_KEY"))
			return nil
		}))
		if err != nil {
			return err
		}
		team := s.RootModule().Resources["archestra_team.test"].Primary.ID
		version := s.RootModule().Resources["archestra_skill.test"].Primary.Attributes["latest_version"]
		if skillVersion != "" && version != skillVersion {
			return fmt.Errorf("sharing-only update created a skill version")
		}
		skillVersion = version
		for name, resourceType := range map[string]string{"archestra_agent.test": "agent", "archestra_mcp_gateway.test": "mcpGateway", "archestra_project.test": "project", "archestra_knowledge_base.test": "knowledgeBase", "archestra_mcp_registry_entry.test": "mcpRegistry", "archestra_skill.test": "skill", "archestra_model_provider.test": "llmProviderApiKey", "archestra_credential.test": "credential"} {
			id := s.RootModule().Resources[name].Primary.ID
			if resourceType == "credential" {
				id = s.RootModule().Resources[name].Primary.Attributes["definition_id"]
			}
			if previous, ok := identities[name]; ok && previous != id {
				return fmt.Errorf("metadata update replaced %s", name)
			}
			identities[name] = id

			response, err := c.GetResourcePermissionsWithResponse(t.Context(), client.GetResourcePermissionsParamsResource(resourceType), id)
			if err != nil {
				return err
			}
			if response.JSON200 == nil {
				return fmt.Errorf("unable to read initial policy for %s: HTTP %d", name, response.StatusCode())
			}
			found := false
			var policy permissionPolicyWire
			if err := json.Unmarshal(response.Body, &policy); err != nil {
				return err
			}
			if expectEmpty {
				for _, grant := range policy.Grants {
					if policy.OwnerID == nil || grant.Subject.Type != "user" || grant.Subject.ID != *policy.OwnerID {
						return fmt.Errorf("permissions did not clear for %s", name)
					}
				}
				continue
			}

			for _, grant := range policy.Grants {
				if grant.Subject.Type == "team" && grant.Subject.ID == team && len(grant.Actions) == 1 && grant.Actions[0] == "read" {
					found = true
				}
			}
			if !found {
				return fmt.Errorf("initial team grant was not persisted for %s", name)
			}
		}
		return nil
	}
	resource.Test(t, resource.TestCase{PreCheck: func() {
		testAccPreCheck(t)
		if os.Getenv("ARCHESTRA_TEST_LLM_BASE_URL") == "" {
			t.Fatal("ARCHESTRA_TEST_LLM_BASE_URL must point to the isolated model fixture")
		}
	}, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{
		{Config: config(granted, "initial"), Check: verify},
		{Config: config(granted, "initial"), PlanOnly: true},
		{Config: config(granted, "updated metadata"), Check: verify},
		{Config: config(granted, "updated metadata"), PlanOnly: true},
		{PreConfig: func() { expectEmpty = true }, Config: config(`[]`, "updated metadata"), Check: verify},
	}})
}
