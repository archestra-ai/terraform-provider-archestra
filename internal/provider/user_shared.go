package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type APIUser struct {
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

type UserTerraformModel struct {
	Id               types.String `tfsdk:"id"`
	Name             types.String `tfsdk:"name"`
	Email            types.String `tfsdk:"email"`
	EmailVerified    types.Bool   `tfsdk:"email_verified"`
	Image            types.String `tfsdk:"image"`
	Role             types.String `tfsdk:"role"`
	Banned           types.Bool   `tfsdk:"banned"`
	BanReason        types.String `tfsdk:"ban_reason"`
	BanExpires       types.String `tfsdk:"ban_expires"`
	TwoFactorEnabled types.Bool   `tfsdk:"two_factor_enabled"`
	CreatedAt        types.String `tfsdk:"created_at"`
	UpdatedAt        types.String `tfsdk:"updated_at"`
}

func timeToString(t time.Time) types.String {
	return types.StringValue(t.Format(time.RFC3339))
}

func timePointerToString(t *time.Time) types.String {
	if t == nil {
		return types.StringNull()
	}
	return types.StringValue(t.Format(time.RFC3339))
}

func mapUserToModel(u *APIUser) *UserTerraformModel {
	return &UserTerraformModel{
		Id:               types.StringValue(u.Id),
		Name:             types.StringValue(u.Name),
		Email:            types.StringValue(u.Email),
		EmailVerified:    types.BoolValue(u.EmailVerified),
		Image:            types.StringPointerValue(u.Image),
		Role:             types.StringPointerValue(u.Role),
		Banned:           types.BoolPointerValue(u.Banned),
		BanReason:        types.StringPointerValue(u.BanReason),
		BanExpires:       timePointerToString(u.BanExpires),
		TwoFactorEnabled: types.BoolPointerValue(u.TwoFactorEnabled),
		CreatedAt:        timeToString(u.CreatedAt),
		UpdatedAt:        timeToString(u.UpdatedAt),
	}
}

func getUser(ctx context.Context, c *client.ClientWithResponses, id string, email string) (*UserTerraformModel, error) {
	if id != "" {
		resp, err := c.GetUserByIdWithResponse(ctx, id)
		if err != nil {
			return nil, fmt.Errorf("unable to read user by ID, got error: %s", err)
		}

		if resp.JSON404 != nil {
			return nil, nil
		}
		if resp.JSON400 != nil {
			return nil, fmt.Errorf("bad request: usually due to missing parameters, or invalid parameters")
		}
		if resp.JSON200 != nil {
			u := APIUser(*resp.JSON200)
			return mapUserToModel(&u), nil
		}
		return nil, fmt.Errorf("expected 200 OK, got status %d", resp.StatusCode())
	}

	if email != "" {
		resp, err := c.GetUserByEmailWithResponse(ctx, email)
		if err != nil {
			return nil, fmt.Errorf("unable to read user by Email, got error: %s", err)
		}

		if resp.JSON404 != nil {
			return nil, nil
		}
		if resp.JSON400 != nil {
			return nil, fmt.Errorf("bad request: usually due to missing parameters, or invalid parameters")
		}
		if resp.JSON200 != nil {
			u := APIUser(*resp.JSON200)
			return mapUserToModel(&u), nil
		}
		return nil, fmt.Errorf("expected 200 OK, got status %d", resp.StatusCode())
	}

	return nil, fmt.Errorf("one of 'id' or 'email' must be provided")
}
