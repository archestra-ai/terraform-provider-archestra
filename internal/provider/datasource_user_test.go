package provider

import (
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

const testAccUserDataSourceEnvVar = "ARCHES_TEST_USER_ID"

func TestAccUserDataSource_Basic(t *testing.T) {
	userID := os.Getenv(testAccUserDataSourceEnvVar)
	if userID == "" {
		t.Skipf("set %s to run user data source acceptance test", testAccUserDataSourceEnvVar)
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccUserDataSourceConfig(userID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.archestra_user.test", "id", userID),
					resource.TestCheckResourceAttrSet("data.archestra_user.test", "email"),
				),
			},
		},
	})
}

func testAccUserDataSourceConfig(userID string) string {
	return `
data "archestra_user" "test" {
  id = "` + userID + `"
}
`
}
