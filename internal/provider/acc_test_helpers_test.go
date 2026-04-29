package provider

import (
	"fmt"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// captureAttr returns a TestCheckFunc that records the value of an
// attribute into the supplied string pointer. Use it together with
// assertAttrEquals across two TestSteps to assert that an attribute's
// value didn't change between Create and Update — handy for
// `id`-stability tests on bulk resources where the schema-level
// `UseStateForUnknown` plan modifier alone doesn't tell us whether the
// Update method actually preserved the prior id.
func captureAttr(resourceName, attr string, dst *string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("resource %s not found in state", resourceName)
		}
		v, ok := rs.Primary.Attributes[attr]
		if !ok {
			return fmt.Errorf("attribute %s not found on %s", attr, resourceName)
		}
		*dst = v
		return nil
	}
}

// assertAttrEquals returns a TestCheckFunc that fails if the named
// attribute differs from the previously captured value. Pair with
// captureAttr.
func assertAttrEquals(resourceName, attr string, want *string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("resource %s not found in state", resourceName)
		}
		got, ok := rs.Primary.Attributes[attr]
		if !ok {
			return fmt.Errorf("attribute %s not found on %s", attr, resourceName)
		}
		if got != *want {
			return fmt.Errorf("%s.%s = %q, want %q", resourceName, attr, got, *want)
		}
		return nil
	}
}
