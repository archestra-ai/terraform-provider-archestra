package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccProjectResource(t *testing.T) {
	initial := `resource "archestra_project" "test" {
name="tf-project-test"
description="Initial"
labels=[{key="stage",value="test"}]
}`
	updated := `resource "archestra_project" "test" {
name="tf-project-test-updated"
labels=[]
}`
	resource.Test(t, resource.TestCase{PreCheck: func() { testAccPreCheck(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{
		{Config: initial, Check: resource.TestCheckResourceAttr("archestra_project.test", "labels.#", "1")},
		{ResourceName: "archestra_project.test", ImportState: true, ImportStateVerify: true},
		{Config: updated, Check: resource.ComposeTestCheckFunc(resource.TestCheckNoResourceAttr("archestra_project.test", "description"), resource.TestCheckResourceAttr("archestra_project.test", "labels.#", "0"))},
		{Config: updated, PlanOnly: true},
	}})
}
func TestAccProjectResourceOmittedLabels(t *testing.T) {
	config := `resource "archestra_project" "test" {name="tf-project-minimal"}`
	resource.Test(t, resource.TestCase{PreCheck: func() { testAccPreCheck(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{{Config: config}, {ResourceName: "archestra_project.test", ImportState: true, ImportStateVerify: true}, {Config: config, PlanOnly: true}}})
}
