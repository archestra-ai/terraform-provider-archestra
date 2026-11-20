package provider

import (
	"fmt"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

// resourceClient reads the API client the provider configured.
func resourceClient(req resource.ConfigureRequest, resp *resource.ConfigureResponse) *client.ClientWithResponses {
	if req.ProviderData == nil {
		return nil
	}
	apiClient, ok := req.ProviderData.(*client.ClientWithResponses)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Resource Configure Type", fmt.Sprintf("Expected *client.ClientWithResponses, got: %T", req.ProviderData))
		return nil
	}
	return apiClient
}
