package provider

import (
	"time"

	"github.com/hashicorp/terraform-plugin-framework/types"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

// PromptModel describes the data model for both prompt resource and data source.
type PromptModel struct {
	ID             types.String `tfsdk:"id"`
	ProfileID      types.String `tfsdk:"profile_id"`
	Name           types.String `tfsdk:"name"`
	SystemPrompt   types.String `tfsdk:"system_prompt"`
	UserPrompt     types.String `tfsdk:"user_prompt"`
	IsActive       types.Bool   `tfsdk:"is_active"`
	Version        types.Int64  `tfsdk:"version"`
	ParentPromptID types.String `tfsdk:"parent_prompt_id"`
	CreatedAt      types.String `tfsdk:"created_at"`
	UpdatedAt      types.String `tfsdk:"updated_at"`
}

// mapPromptResponseToModel maps a prompt API response to a Terraform model.
func mapPromptResponseToModel(item *struct {
	AgentId        openapi_types.UUID  `json:"agentId"`
	CreatedAt      time.Time           `json:"createdAt"`
	Id             openapi_types.UUID  `json:"id"`
	IsActive       bool                `json:"isActive"`
	Name           string              `json:"name"`
	OrganizationId string              `json:"organizationId"`
	ParentPromptId *openapi_types.UUID `json:"parentPromptId"`
	SystemPrompt   *string             `json:"systemPrompt"`
	UpdatedAt      time.Time           `json:"updatedAt"`
	UserPrompt     *string             `json:"userPrompt"`
	Version        int                 `json:"version"`
}, data *PromptModel) {
	data.ID = types.StringValue(item.Id.String())
	data.ProfileID = types.StringValue(item.AgentId.String())
	data.Name = types.StringValue(item.Name)
	data.IsActive = types.BoolValue(item.IsActive)
	data.Version = types.Int64Value(int64(item.Version))
	data.CreatedAt = types.StringValue(item.CreatedAt.Format(time.RFC3339))
	data.UpdatedAt = types.StringValue(item.UpdatedAt.Format(time.RFC3339))

	if item.SystemPrompt != nil {
		data.SystemPrompt = types.StringValue(*item.SystemPrompt)
	} else {
		data.SystemPrompt = types.StringNull()
	}

	if item.UserPrompt != nil {
		data.UserPrompt = types.StringValue(*item.UserPrompt)
	} else {
		data.UserPrompt = types.StringNull()
	}

	if item.ParentPromptId != nil {
		data.ParentPromptID = types.StringValue(item.ParentPromptId.String())
	} else {
		data.ParentPromptID = types.StringNull()
	}
}
