package client

import (
	"context"
	"encoding/json"
	"fmt"
)

// GetRoleById is a helper to call GetRoleWithResponse using a string ID,
// bypassing the unexported union struct in the generated code.
func (c *ClientWithResponses) GetRoleById(ctx context.Context, roleId string) (*GetRoleResponse, error) {
	idBytes, err := json.Marshal(roleId)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal roleId: %w", err)
	}

	// Construct the anonymous struct expected by the generated code
	arg := struct {
		union json.RawMessage
	}{
		union: json.RawMessage(idBytes),
	}

	return c.GetRoleWithResponse(ctx, arg)
}

// UpdateRoleById is a helper to call UpdateRoleWithResponse using a string ID.
func (c *ClientWithResponses) UpdateRoleById(ctx context.Context, roleId string, body UpdateRoleJSONRequestBody) (*UpdateRoleResponse, error) {
	idBytes, err := json.Marshal(roleId)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal roleId: %w", err)
	}

	arg := struct {
		union json.RawMessage
	}{
		union: json.RawMessage(idBytes),
	}

	return c.UpdateRoleWithResponse(ctx, arg, body)
}
