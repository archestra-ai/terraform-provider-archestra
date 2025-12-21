package provider

import (
	"context"
	"fmt"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &ArchestraPromptVersionsDataSource{}

func NewArchestraPromptVersionsDataSource() datasource.DataSource {
	return &ArchestraPromptVersionsDataSource{}
}

type ArchestraPromptVersionsDataSource struct {
	client *client.ClientWithResponses
}

type ArchestraPromptVersionsDataSourceModel struct {
	ID       types.String `tfsdk:"prompt_id"`
	Versions types.List   `tfsdk:"versions"`
}

type PromptVersionModel struct {
	ID            types.String `tfsdk:"prompt_id"`
	Version       types.Int64  `tfsdk:"version"`
	CreatedAt     types.String `tfsdk:"created_at"`
	UserPrompt    types.String `tfsdk:"user_prompt"`
	SystemPrompt  types.String `tfsdk:"system_prompt"`
	Name          types.String `tfsdk:"name"`
}

func (d *ArchestraPromptVersionsDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_archestra_prompt_versions"
}

func (d *ArchestraPromptVersionsDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists versions of a specific Archestra prompt.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The ID of the prompt",
				Required:            true,
			},
			"versions": schema.ListNestedAttribute{
				MarkdownDescription: "List of prompt versions",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"prompt_id": schema.StringAttribute{
							MarkdownDescription: "Version identifier",
							Computed:            true,
						},
						"version": schema.Int64Attribute{
							MarkdownDescription: "Version number",
							Computed:            true,
						},
						"created_at": schema.StringAttribute{
							MarkdownDescription: "Creation timestamp",
							Computed:            true,
						},
						"user_prompt": schema.StringAttribute{
							MarkdownDescription: "User prompt content",
							Computed:            true,
						},
						"system_prompt": schema.StringAttribute{
							MarkdownDescription: "System prompt content",
							Computed:            true,
						},
						"name": schema.StringAttribute{
							MarkdownDescription: "Name of the prompt version",
							Computed:            true,
						},
					},
				},
			},
		},
	}
}

func (d *ArchestraPromptVersionsDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *ArchestraPromptVersionsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data ArchestraPromptVersionsDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	ID, err := uuid.Parse(data.ID.ValueString())
    if err != nil {
		resp.Diagnostics.AddError("Invalid Parent ID", fmt.Sprintf("Unable to get Prompt: %s", err))
		return
	}

	versionsResp, err := d.client.GetPromptVersionsWithResponse(ctx, ID)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to read prompt versions, got error: %s", err))
		return
	}

	if versionsResp.JSON200 == nil {
		resp.Diagnostics.AddError("Unexpected API Response", fmt.Sprintf("Expected 200 OK, got status %d", versionsResp.StatusCode()))
		return
	}

	// Map versions to list
	var versionList []PromptVersionModel
	for _, v := range *versionsResp.JSON200 {
		versionList = append(versionList, PromptVersionModel{
			ID:            types.StringValue(v.Id.String()),
			Version:       types.Int64Value(int64(v.Version)),
			CreatedAt:     types.StringValue(v.CreatedAt.String()),
			UserPrompt:    types.StringPointerValue(v.UserPrompt),
			SystemPrompt:  types.StringPointerValue(v.SystemPrompt),
			Name:          types.StringValue(v.Name),
		})
	}

	data.Versions, _ = types.ListValueFrom(ctx, types.ObjectType{AttrTypes: map[string]attr.Type{
		"id":             types.StringType,
		"version":        types.Int64Type,
		"created_at":     types.StringType,
		"user_prompt":    types.StringType,
		"system_prompt":  types.StringType,
		"name":           types.StringType,
	}}, versionList)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
