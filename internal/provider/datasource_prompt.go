package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ datasource.DataSource = &PromptDataSource{}

func NewPromptDataSource() datasource.DataSource {
	return &PromptDataSource{}
}

// PromptDataSource defines the data source implementation.
type PromptDataSource struct {
	client *client.ClientWithResponses
}

// PromptDataSourceModel describes the data source data model.
type PromptDataSourceModel struct {
	ID             types.String `tfsdk:"id"`
	Name           types.String `tfsdk:"name"`
	ProfileID      types.String `tfsdk:"profile_id"`
	SystemPrompt   types.String `tfsdk:"system_prompt"`
	UserPrompt     types.String `tfsdk:"user_prompt"`
	IsActive       types.Bool   `tfsdk:"is_active"`
	Version        types.Int64  `tfsdk:"version"`
	ParentPromptID types.String `tfsdk:"parent_prompt_id"`
	CreatedAt      types.String `tfsdk:"created_at"`
	UpdatedAt      types.String `tfsdk:"updated_at"`
}

func (d *PromptDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_prompt"
}

func (d *PromptDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Provides information about a specific Archestra prompt.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Prompt identifier",
				Optional:            true,
				Computed:            true,
				Validators: []validator.String{
					stringvalidator.ExactlyOneOf(path.Expressions{
						path.MatchRoot("id"),
						path.MatchRoot("name"),
					}...),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "The name of the prompt",
				Optional:            true,
				Computed:            true,
			},
			"profile_id": schema.StringAttribute{
				MarkdownDescription: "The profile identifier this prompt belongs to",
				Computed:            true,
			},
			"system_prompt": schema.StringAttribute{
				MarkdownDescription: "The system prompt template",
				Computed:            true,
			},
			"user_prompt": schema.StringAttribute{
				MarkdownDescription: "The user prompt template",
				Computed:            true,
			},
			"is_active": schema.BoolAttribute{
				MarkdownDescription: "Whether the prompt is active",
				Computed:            true,
			},
			"version": schema.Int64Attribute{
				MarkdownDescription: "The version of the prompt",
				Computed:            true,
			},
			"parent_prompt_id": schema.StringAttribute{
				MarkdownDescription: "The identifier of the parent prompt if this is a version",
				Computed:            true,
			},
			"created_at": schema.StringAttribute{
				MarkdownDescription: "Timestamp when the prompt was created",
				Computed:            true,
			},
			"updated_at": schema.StringAttribute{
				MarkdownDescription: "Timestamp when the prompt was last updated",
				Computed:            true,
			},
		},
	}
}

func (d *PromptDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*client.ClientWithResponses)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *client.ClientWithResponses, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	d.client = client
}

func (d *PromptDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data PromptDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var foundItem *struct {
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
	}

	if !data.ID.IsNull() {
		promptID, err := uuid.Parse(data.ID.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Invalid ID", fmt.Sprintf("Unable to parse prompt ID: %s", err))
			return
		}

		apiResp, err := d.client.GetPromptWithResponse(ctx, promptID)
		if err != nil {
			resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to read prompt, got error: %s", err))
			return
		}

		if apiResp.JSON404 != nil {
			resp.Diagnostics.AddError("Not Found", "Prompt not found with the specified ID")
			return
		}

		if apiResp.JSON200 == nil {
			resp.Diagnostics.AddError(
				"Unexpected API Response",
				fmt.Sprintf("Expected 200 OK, got status %d", apiResp.StatusCode()),
			)
			return
		}
		foundItem = apiResp.JSON200
	} else {
		// Lookup by name
		apiResp, err := d.client.GetPromptsWithResponse(ctx)
		if err != nil {
			resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to list prompts, got error: %s", err))
			return
		}

		if apiResp.JSON200 == nil {
			resp.Diagnostics.AddError(
				"Unexpected API Response",
				fmt.Sprintf("Expected 200 OK, got status %d", apiResp.StatusCode()),
			)
			return
		}

		targetName := data.Name.ValueString()
		for _, item := range *apiResp.JSON200 {
			if item.Name == targetName {
				// We need to copy because &item will point to the loop variable
				itemCopy := item
				foundItem = &itemCopy
				break
			}
		}

		if foundItem == nil {
			resp.Diagnostics.AddError("Not Found", fmt.Sprintf("Prompt not found with name: %s", targetName))
			return
		}
	}

	d.mapResponseToModel(foundItem, &data)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (d *PromptDataSource) mapResponseToModel(item *struct {
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
}, data *PromptDataSourceModel) {
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
