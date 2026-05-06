package provider

import (
	"fmt"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// captureID returns a TestCheckFunc that records a resource's `id`
// attribute into the supplied string pointer. Use it together with
// assertIDEquals across two TestSteps to assert that the id didn't
// change between Create and Update — handy for id-stability tests on
// bulk resources where the schema-level `UseStateForUnknown` plan
// modifier alone doesn't tell us whether the Update method actually
// preserved the prior id.
func captureID(resourceName string, dst *string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("resource %s not found in state", resourceName)
		}
		v, ok := rs.Primary.Attributes["id"]
		if !ok {
			return fmt.Errorf("attribute id not found on %s", resourceName)
		}
		*dst = v
		return nil
	}
}

// assertIDEquals returns a TestCheckFunc that fails if a resource's
// `id` attribute differs from the previously captured value. Pair with
// captureID.
func assertIDEquals(resourceName string, want *string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("resource %s not found in state", resourceName)
		}
		got, ok := rs.Primary.Attributes["id"]
		if !ok {
			return fmt.Errorf("attribute id not found on %s", resourceName)
		}
		if got != *want {
			return fmt.Errorf("%s.id = %q, want %q", resourceName, got, *want)
		}
		return nil
	}
}
