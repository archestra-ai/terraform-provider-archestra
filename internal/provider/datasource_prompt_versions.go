package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ datasource.DataSource = &PromptVersionsDataSource{}

func NewPromptVersionsDataSource() datasource.DataSource {
	return &PromptVersionsDataSource{}
}

// PromptVersionsDataSource defines the data source implementation.
type PromptVersionsDataSource struct {
	client *client.ClientWithResponses
}

// PromptVersionModel describes a single version in the list.
type PromptVersionModel struct {
	ID           types.String `tfsdk:"id"`
	Name         types.String `tfsdk:"name"`
	SystemPrompt types.String `tfsdk:"system_prompt"`
	UserPrompt   types.String `tfsdk:"user_prompt"`
	IsActive     types.Bool   `tfsdk:"is_active"`
	Version      types.Int64  `tfsdk:"version"`
	UpdatedAt    types.String `tfsdk:"updated_at"`
}

// PromptVersionsDataSourceModel describes the data source data model.
type PromptVersionsDataSourceModel struct {
	PromptID types.String         `tfsdk:"prompt_id"`
	Versions []PromptVersionModel `tfsdk:"versions"`
}

func (d *PromptVersionsDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_prompt_versions"
}

func (d *PromptVersionsDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Provides a list of versions for a specific Archestra prompt.",

		Attributes: map[string]schema.Attribute{
			"prompt_id": schema.StringAttribute{
				MarkdownDescription: "The identifier of the prompt to list versions for",
				Required:            true,
			},
			"versions": schema.ListNestedAttribute{
				MarkdownDescription: "List of prompt versions",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							MarkdownDescription: "Version identifier",
							Computed:            true,
						},
						"name": schema.StringAttribute{
							MarkdownDescription: "The name of the prompt at this version",
							Computed:            true,
						},
						"system_prompt": schema.StringAttribute{
							MarkdownDescription: "The system prompt at this version",
							Computed:            true,
						},
						"user_prompt": schema.StringAttribute{
							MarkdownDescription: "The user prompt at this version",
							Computed:            true,
						},
						"is_active": schema.BoolAttribute{
							MarkdownDescription: "Whether this version is active",
							Computed:            true,
						},
						"version": schema.Int64Attribute{
							MarkdownDescription: "The version number",
							Computed:            true,
						},
						"updated_at": schema.StringAttribute{
							MarkdownDescription: "Timestamp when this version was created/updated",
							Computed:            true,
						},
					},
				},
			},
		},
	}
}

func (d *PromptVersionsDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *PromptVersionsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data PromptVersionsDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	promptID, err := uuid.Parse(data.PromptID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid Prompt ID", fmt.Sprintf("Unable to parse prompt UUID: %s", err))
		return
	}

	apiResp, err := d.client.GetPromptVersionsWithResponse(ctx, promptID)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to list prompt versions, got error: %s", err))
		return
	}

	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Expected 200 OK, got status %d", apiResp.StatusCode()),
		)
		return
	}

	// Build versions list from Current + History
	versions := make([]PromptVersionModel, 0, 1+len(apiResp.JSON200.History))

	// Add current version
	current := apiResp.JSON200.Current
	currentVersion := PromptVersionModel{
		ID:        types.StringValue(current.Id.String()),
		Name:      types.StringValue(current.Name),
		IsActive:  types.BoolValue(true), // Current version is always active
		Version:   types.Int64Value(int64(current.Version)),
		UpdatedAt: types.StringValue(current.UpdatedAt.Format(time.RFC3339)),
	}
	if current.SystemPrompt != nil {
		currentVersion.SystemPrompt = types.StringValue(*current.SystemPrompt)
	} else {
		currentVersion.SystemPrompt = types.StringNull()
	}
	if current.UserPrompt != nil {
		currentVersion.UserPrompt = types.StringValue(*current.UserPrompt)
	} else {
		currentVersion.UserPrompt = types.StringNull()
	}
	versions = append(versions, currentVersion)

	// Add historical versions
	for _, item := range apiResp.JSON200.History {
		v := PromptVersionModel{
			ID:        types.StringNull(), // History items don't have ID
			Name:      types.StringNull(), // History items don't have Name
			IsActive:  types.BoolValue(false),
			Version:   types.Int64Value(int64(item.Version)),
			UpdatedAt: types.StringValue(item.CreatedAt),
		}

		if item.SystemPrompt != nil {
			v.SystemPrompt = types.StringValue(*item.SystemPrompt)
		} else {
			v.SystemPrompt = types.StringNull()
		}

		if item.UserPrompt != nil {
			v.UserPrompt = types.StringValue(*item.UserPrompt)
		} else {
			v.UserPrompt = types.StringNull()
		}

		versions = append(versions, v)
	}

	data.Versions = versions

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
