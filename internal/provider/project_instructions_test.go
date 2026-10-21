package provider

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"testing"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestAccProjectInstructions(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-project-instructions")
	var id string
	config := func(instructions string) string {
		return fmt.Sprintf("resource \"archestra_project\" \"test\" {\n name=%q\n %s\n}\n", name, instructions)
	}
	initial := config(`instructions="# Synthetic instructions\nInspect only owned local fixtures."`)
	updated := config(`instructions="Updated synthetic instructions."`)
	omitted := config("")
	c := func() *client.ClientWithResponses {
		c, err := client.NewClientWithResponses(os.Getenv("ARCHESTRA_BASE_URL"), client.WithRequestEditorFn(func(_ context.Context, r *http.Request) error {
			r.Header.Set("Authorization", os.Getenv("ARCHESTRA_API_KEY"))
			return nil
		}))
		if err != nil {
			t.Fatal(err)
		}
		return c
	}
	drift := func(content string) {
		response, err := c().SetProjectInstructionsWithResponse(t.Context(), uuid.MustParse(id), client.SetProjectInstructionsJSONRequestBody{Content: content})
		if err != nil || response.JSON200 == nil {
			t.Fatal("Unable to write isolated project instruction drift", err)
		}
	}
	resource.Test(t, resource.TestCase{PreCheck: func() { testAccPreCheck(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{
		{Config: initial, Check: func(s *terraform.State) error {
			id = s.RootModule().Resources["archestra_project.test"].Primary.ID
			return nil
		}},
		{ResourceName: "archestra_project.test", ImportState: true, ImportStateVerify: true},
		{Config: initial, PlanOnly: true},
		{Config: updated, Check: resource.TestCheckResourceAttr("archestra_project.test", "instructions", "Updated synthetic instructions.")},
		{Config: updated, PlanOnly: true, ExpectNonEmptyPlan: true, PreConfig: func() { drift("Out-of-band synthetic instructions.") }},
		{Config: updated, Check: resource.TestCheckResourceAttr("archestra_project.test", "instructions", "Updated synthetic instructions.")},
		{Config: omitted, Check: resource.TestCheckResourceAttr("archestra_project.test", "instructions", "Updated synthetic instructions.")},
		{Config: omitted, PlanOnly: true, PreConfig: func() { drift("Unmanaged synthetic instructions.") }},
		{Config: omitted, Check: resource.TestCheckResourceAttr("archestra_project.test", "instructions", "Unmanaged synthetic instructions.")},
		{Config: config(`instructions=""`), Check: resource.TestCheckResourceAttr("archestra_project.test", "instructions", "")},
		{Config: config(`instructions=""`), PlanOnly: true},
	}})
}
