package client

import (
	"context"
	"fmt"
	"net/http"
)

// GrantTeamMcpServerAccess grants a team access to an MCP server
func (c *Client) GrantTeamMcpServerAccess(ctx context.Context, serverID, teamID string) error {
	// API Endpoint: POST /api/v1/mcp-servers/{server_id}/teams/{team_id}
	path := fmt.Sprintf("/api/v1/mcp-servers/%s/teams/%s", serverID, teamID)

	// Note: We use c.BaseURL. If 'BaseURL' is red, try 'HostURL'.
	req, err := http.NewRequestWithContext(ctx, "POST", c.BaseURL+path, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Note: We use c.doRequest. If 'doRequest' is red, try 'do'.
	if err := c.doRequest(req, nil); err != nil {
		return fmt.Errorf("failed to grant team access: %w", err)
	}

	return nil
}

// RevokeTeamMcpServerAccess revokes a team's access to an MCP server
func (c *Client) RevokeTeamMcpServerAccess(ctx context.Context, serverID, teamID string) error {
	path := fmt.Sprintf("/api/v1/mcp-servers/%s/teams/%s", serverID, teamID)

	req, err := http.NewRequestWithContext(ctx, "DELETE", c.BaseURL+path, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	if err := c.doRequest(req, nil); err != nil {
		return fmt.Errorf("failed to revoke team access: %w", err)
	}

	return nil
}