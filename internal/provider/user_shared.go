package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
)

// UserSharedModel maps to the API response for user details
type UserSharedModel struct {
	BanExpires       *time.Time `json:"banExpires"`
	BanReason        *string    `json:"banReason"`
	Banned           *bool      `json:"banned"`
	CreatedAt        time.Time  `json:"createdAt"`
	Email            string     `json:"email"`
	EmailVerified    bool       `json:"emailVerified"`
	Id               string     `json:"id"`
	Image            *string    `json:"image"`
	Name             string     `json:"name"`
	Role             *string    `json:"role"`
	TwoFactorEnabled *bool      `json:"twoFactorEnabled"`
	UpdatedAt        time.Time  `json:"updatedAt"`
}

func getUser(ctx context.Context, c *client.ClientWithResponses, id string, email string) (*UserSharedModel, error) {
	if id != "" {
		// Lookup by ID
		userResp, err := c.GetUserByIdWithResponse(ctx, id)
		if err != nil {
			return nil, fmt.Errorf("unable to read user by ID, got error: %s", err)
		}

		if userResp.JSON404 != nil {
			return nil, nil // Not found
		}

		if userResp.JSON400 != nil {
			return nil, fmt.Errorf("bad request: usually due to missing parameters, or invalid parameters")
		}

		if userResp.JSON200 == nil {
			return nil, fmt.Errorf("expected 200 OK, got status %d", userResp.StatusCode())
		}

		// The API client returns an anonymous struct that matches UserSharedModel
		// We need to manually map it or type assert if possible.
		// Since we can't easily cast anonymous structs from different packages if they are not identical in the eyes of Go compiler (field tags etc),
		// and the generated code likely returns *struct{...}, we might need to rely on field-by-field copy or just assume it matches exactly.
		// However, based on datasource_user.go, it was assigning directly.
		// Let's assume the generated type is structuraly identical to what we defined.
		// To be safe and clean, let's map it field by field to avoid type mismatch issues if the generated type is named or slightly different/anonymous.

		u := userResp.JSON200
		return &UserSharedModel{
			BanExpires:       u.BanExpires,
			BanReason:        u.BanReason,
			Banned:           u.Banned,
			CreatedAt:        u.CreatedAt,
			Email:            u.Email,
			EmailVerified:    u.EmailVerified,
			Id:               u.Id,
			Image:            u.Image,
			Name:             u.Name,
			Role:             u.Role,
			TwoFactorEnabled: u.TwoFactorEnabled,
			UpdatedAt:        u.UpdatedAt,
		}, nil
	}

	if email != "" {
		// Lookup by Email
		userResp, err := c.GetUserByEmailWithResponse(ctx, email)
		if err != nil {
			return nil, fmt.Errorf("unable to read user by Email, got error: %s", err)
		}

		if userResp.JSON404 != nil {
			return nil, nil // Not found
		}

		if userResp.JSON400 != nil {
			return nil, fmt.Errorf("bad request: usually due to missing parameters, or invalid parameters")
		}

		if userResp.JSON200 == nil {
			return nil, fmt.Errorf("expected 200 OK, got status %d", userResp.StatusCode())
		}

		u := userResp.JSON200
		return &UserSharedModel{
			BanExpires:       u.BanExpires,
			BanReason:        u.BanReason,
			Banned:           u.Banned,
			CreatedAt:        u.CreatedAt,
			Email:            u.Email,
			EmailVerified:    u.EmailVerified,
			Id:               u.Id,
			Image:            u.Image,
			Name:             u.Name,
			Role:             u.Role,
			TwoFactorEnabled: u.TwoFactorEnabled,
			UpdatedAt:        u.UpdatedAt,
		}, nil
	}

	return nil, fmt.Errorf("one of 'id' or 'email' must be provided")
}
