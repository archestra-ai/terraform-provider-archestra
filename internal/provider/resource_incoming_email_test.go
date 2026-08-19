package provider

import (
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccIncomingEmailResource_NoProviderConfigured pins the 400
// "Incoming email provider not configured" path. The local test
// stack doesn't ship a Microsoft Graph credential, so this is the
// only deterministic path we can exercise here. Happy-path coverage
// requires a real Outlook tenant + webhookUrl reachable from the
// platform's egress.
func TestAccIncomingEmailResource_NoProviderConfigured(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "archestra_incoming_email" "test" {
  webhook_url = "https://example.invalid/archestra-incoming-email"
}
`,
				ExpectError: regexp.MustCompile(`(?i)Incoming email provider not configured|400`),
			},
		},
	})
}

// TestAccIncomingEmailResource_HttpsRequired pins the client-side
// scheme validator before the backend round-trip.
func TestAccIncomingEmailResource_HttpsRequired(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "archestra_incoming_email" "bad" {
  webhook_url = "http://insecure.example/path"
}
`,
				ExpectError: regexp.MustCompile(`webhook_url must use https://`),
			},
		},
	})
}
