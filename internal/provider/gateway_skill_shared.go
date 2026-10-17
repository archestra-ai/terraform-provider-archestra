package provider

import (
	"context"
	"fmt"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/diag"
)

// The publication GETs also accept internal agent IDs but return empty defaults;
// verify the publishing subtype rather than adopting that misleading empty view.
func readSkillPublicationGateway(ctx context.Context, c *client.ClientWithResponses, raw string, diags *diag.Diagnostics) bool {
	id, err := uuid.Parse(raw)
	if err != nil {
		diags.AddError("Invalid Gateway ID", err.Error())
		return false
	}
	result, err := c.GetAgentWithResponse(ctx, id)
	if err != nil {
		diags.AddError("API Error", fmt.Sprintf("Unable to read publication gateway: %s", err))
		return false
	}
	if result.StatusCode() == 404 {
		return false
	}
	if result.JSON200 == nil {
		diags.AddError("API Error", fmt.Sprintf("Unable to read publication gateway: status %d", result.StatusCode()))
		return false
	}
	if string(result.JSON200.AgentType) != "mcp_gateway" {
		diags.AddError("Invalid Publishing Target", "Skill publication belongs to an MCP gateway; internal-agent activation uses archestra_agent_skill_policy.")
		return false
	}
	return true
}
