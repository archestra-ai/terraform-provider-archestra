package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccEnvironmentResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck: func() { testAccPreCheck(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: testEnvironmentConfig("Initial", true), Check: resource.ComposeTestCheckFunc(resource.TestCheckResourceAttr("archestra_environment.test", "name", "tf-environment-Initial"), resource.TestCheckResourceAttr("archestra_environment.test", "labels.#", "1"))},
			{ResourceName: "archestra_environment.test", ImportState: true, ImportStateVerify: true},
			{Config: testEnvironmentConfig("Updated", true), Check: resource.TestCheckResourceAttr("archestra_environment.test", "description", "Updated")},
			{Config: testEnvironmentConfig("Updated", true), PlanOnly: true},
			{Config: testEnvironmentConfig("Cleared", false), Check: resource.ComposeTestCheckFunc(resource.TestCheckNoResourceAttr("archestra_environment.test", "description"), resource.TestCheckNoResourceAttr("archestra_environment.test", "network_policy"), resource.TestCheckResourceAttr("archestra_environment.test", "labels.#", "0"))},
			{Config: testEnvironmentConfig("Cleared", false), PlanOnly: true},
		},
	})
}
func testEnvironmentConfig(suffix string, policies bool) string {
	fields := `labels=[]`
	if policies {
		fields = fmt.Sprintf(`description=%q
validation_regex="^[a-z]+$"
trusted_image_registries=["ghcr.io/example"]
labels=[{key="stage",value=%q}]
network_policy={egress_mode="restricted",domain_preset="none",allowed_domains=["api.example.com"],allowed_cidrs=[]}`, suffix, suffix)
	}
	return fmt.Sprintf(`resource "archestra_environment" "test" {
name=%q
%s
}`, "tf-environment-"+suffix, fields)
}

func TestAccEnvironmentResourceOmittedLabels(t *testing.T) {
	config := `resource "archestra_environment" "test" {name="tf-environment-minimal"}`
	resource.Test(t, resource.TestCase{PreCheck: func() { testAccPreCheck(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{
		{Config: config},
		{ResourceName: "archestra_environment.test", ImportState: true, ImportStateVerify: true},
		{Config: config, PlanOnly: true},
	}})
}
