package provider

import (
	"context"
	"fmt"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &ArchestraPromptDataSource{}

func NewArchestraPromptDataSource() datasource.DataSource {
	return &ArchestraPromptDataSource{}
}

type ArchestraPromptDataSource struct {
	client *client.ClientWithResponses
}

type ArchestraPromptDataSourceModel struct {
	ID           types.String `tfsdk:"prompt_id"`
	ProfileId    types.String `tfsdk:"profile_id"`
	Name         types.String `tfsdk:"name"`
	SystemPrompt types.String `tfsdk:"system_prompt"`
	Prompt       types.String `tfsdk:"prompt"`
	IsActive     types.Bool   `tfsdk:"is_active"`
	Version      types.Int64  `tfsdk:"version"`
}

func (d *ArchestraPromptDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_archestra_prompt"
}

func (d *ArchestraPromptDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Fetches an existing Archestra prompt by ID or name.",

		Attributes: map[string]schema.Attribute{
			"prompt_id": schema.StringAttribute{
				MarkdownDescription: "Prompt identifier",
				Optional:            true,
				Computed:            true,
			},
			"profile_id": schema.StringAttribute{
				MarkdownDescription: "The Profile ID for the prompt",
				Computed:            true,
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "The name of the prompt",
				Optional:            true,
			},
			"system_prompt": schema.StringAttribute{
				MarkdownDescription: "System prompt",
				Computed:            true,
			},
			"prompt": schema.StringAttribute{
				MarkdownDescription: "the Main Prompt",
				Computed:            true,
			},
			"is_active": schema.BoolAttribute{
				MarkdownDescription: "",
				Computed:            true,
			},
			"version": schema.Int64Attribute{
				MarkdownDescription: "Version of the prompt",
				Computed:            true,
			},
		},
	}
}

func (d *ArchestraPromptDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*client.ClientWithResponses)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *client.ClientWithResponses, got: %T", req.ProviderData),
		)
		return
	}

	d.client = client
}

func (d *ArchestraPromptDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data ArchestraPromptDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if !data.ID.IsNull() {
		Id, err := uuid.Parse(data.ID.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Invalid Prompt  ID", fmt.Sprintf("Unable to get Prompt: %s", err))
			return
		}
		getResp, apiErr := d.client.GetPromptWithResponse(ctx, Id)
		if apiErr != nil {
			resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to read prompt, got error: %s", apiErr))
			return
		}
		if getResp.JSON200 == nil {
			resp.Diagnostics.AddError("Unexpected API Response", fmt.Sprintf("Expected 200 OK, got status %d", getResp.StatusCode()))
			return
		}

		data.ID = types.StringValue(getResp.JSON200.Id.String())
		data.Name = types.StringValue(getResp.JSON200.Name)
		data.Prompt = types.StringPointerValue(getResp.JSON200.UserPrompt)
		if getResp.JSON200.SystemPrompt != nil {
			data.SystemPrompt = types.StringValue(*getResp.JSON200.SystemPrompt)
		} else {
			data.SystemPrompt = types.StringNull()
		}
		data.Version = types.Int64Value(int64(getResp.JSON200.Version))
		data.IsActive = types.BoolValue(getResp.JSON200.IsActive)
		data.ProfileId = types.StringValue(getResp.JSON200.AgentId.String())

		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
	} else if !data.Name.IsNull() {
		promptsResp, apiErr := d.client.GetPromptsWithResponse(ctx)
		if apiErr != nil {
			resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to read prompts, got error: %s", apiErr))
			return
		}
		if promptsResp.JSON200 == nil {
			resp.Diagnostics.AddError("Unexpected API Response", fmt.Sprintf("Expected 200 OK, got status %d", promptsResp.StatusCode()))
			return
		}
		found := false

		for _, p := range *promptsResp.JSON200 {
			if p.Name == data.Name.ValueString() {
				found = true

				data.ID = types.StringValue(p.Id.String())
				data.Name = types.StringValue(p.Name)
				data.Prompt = types.StringPointerValue(p.UserPrompt)

				if p.SystemPrompt != nil {
					data.SystemPrompt = types.StringValue(*p.SystemPrompt)
				} else {
					data.SystemPrompt = types.StringNull()
				}

				data.IsActive = types.BoolValue(p.IsActive)
				data.Version = types.Int64Value(int64(p.Version))
				data.ProfileId = types.StringValue(p.AgentId.String())

				resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
				break
			}
		}

		if !found {
			resp.Diagnostics.AddError(
				"Not Found",
				fmt.Sprintf("Prompt with name '%s' not found", data.Name.ValueString()),
			)
			return
		}
	} else {
		resp.Diagnostics.AddError("Invalid Configuration", "Either 'id' or 'name' must be provided")
		return
	}
}
