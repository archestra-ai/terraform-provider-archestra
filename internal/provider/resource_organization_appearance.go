package provider

import (
	"context"
	"fmt"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = &OrganizationAppearanceResource{}
var _ resource.ResourceWithImportState = &OrganizationAppearanceResource{}

func NewOrganizationAppearanceResource() resource.Resource {
	return &OrganizationAppearanceResource{}
}

type OrganizationAppearanceResource struct {
	client *client.ClientWithResponses
}

type OrganizationAppearanceResourceModel struct {
	Font       types.String `tfsdk:"font"`
	ColorTheme types.String `tfsdk:"color_theme"`
	LogoURL    types.String `tfsdk:"logo_url"`
	LogoType   types.String `tfsdk:"logo_type"`
}

func (r *OrganizationAppearanceResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_organization_appearance"
}

func (r *OrganizationAppearanceResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages the organization appearance settings (font, color theme, logo). This is a singleton resource.",

		Attributes: map[string]schema.Attribute{
			"font": schema.StringAttribute{
				MarkdownDescription: "The custom font for the organization. Valid values: inter, lato, open-sans, roboto, source-sans-pro",
				Optional:            true,
			},
			"color_theme": schema.StringAttribute{
				MarkdownDescription: "The color theme for the organization.",
				Optional:            true,
			},
			"logo_url": schema.StringAttribute{
				MarkdownDescription: "The URL of the organization's logo.",
				Optional:            true,
			},
			"logo_type": schema.StringAttribute{
				MarkdownDescription: "The type of logo to use. Valid values: custom, default",
				Optional:            true,
			},
		},
	}
}

func (r *OrganizationAppearanceResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*client.ClientWithResponses)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *client.ClientWithResponses, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	r.client = client
}

func (r *OrganizationAppearanceResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data OrganizationAppearanceResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Create request body
	requestBody := client.UpdateOrganizationAppearanceJSONRequestBody{}

	if !data.Font.IsNull() {
		font := client.UpdateOrganizationAppearanceJSONBodyCustomFont(data.Font.ValueString())
		requestBody.CustomFont = &font
	}

	if !data.ColorTheme.IsNull() {
		theme := client.UpdateOrganizationAppearanceJSONBodyTheme(data.ColorTheme.ValueString())
		requestBody.Theme = &theme
	}

	if !data.LogoURL.IsNull() {
		logo := data.LogoURL.ValueString()
		requestBody.Logo = &logo
	}

	if !data.LogoType.IsNull() {
		logoType := client.UpdateOrganizationAppearanceJSONBodyLogoType(data.LogoType.ValueString())
		requestBody.LogoType = &logoType
	}

	// Call API - treating Create as Update for singleton
	apiResp, err := r.client.UpdateOrganizationAppearanceWithResponse(ctx, requestBody)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to create/update organization appearance, got error: %s", err))
		return
	}

	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Expected 200 OK, got status %d", apiResp.StatusCode()),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *OrganizationAppearanceResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data OrganizationAppearanceResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Call API
	apiResp, err := r.client.GetOrganizationAppearanceWithResponse(ctx)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to read organization appearance, got error: %s", err))
		return
	}

	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Expected 200 OK, got status %d", apiResp.StatusCode()),
		)
		return
	}

	// Map response to Terraform state
	if apiResp.JSON200.CustomFont != nil {
		data.Font = types.StringValue(string(*apiResp.JSON200.CustomFont))
	} else {
		data.Font = types.StringNull()
	}

	if apiResp.JSON200.Theme != nil {
		data.ColorTheme = types.StringValue(string(*apiResp.JSON200.Theme))
	} else {
		data.ColorTheme = types.StringNull()
	}

	if apiResp.JSON200.Logo != nil {
		data.LogoURL = types.StringValue(*apiResp.JSON200.Logo)
	} else {
		data.LogoURL = types.StringNull()
	}

	if apiResp.JSON200.LogoType != nil {
		data.LogoType = types.StringValue(string(*apiResp.JSON200.LogoType))
	} else {
		data.LogoType = types.StringNull()
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *OrganizationAppearanceResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data OrganizationAppearanceResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Create request body
	requestBody := client.UpdateOrganizationAppearanceJSONRequestBody{}

	if !data.Font.IsNull() {
		font := client.UpdateOrganizationAppearanceJSONBodyCustomFont(data.Font.ValueString())
		requestBody.CustomFont = &font
	}

	if !data.ColorTheme.IsNull() {
		theme := client.UpdateOrganizationAppearanceJSONBodyTheme(data.ColorTheme.ValueString())
		requestBody.Theme = &theme
	}

	if !data.LogoURL.IsNull() {
		logo := data.LogoURL.ValueString()
		requestBody.Logo = &logo
	}

	if !data.LogoType.IsNull() {
		logoType := client.UpdateOrganizationAppearanceJSONBodyLogoType(data.LogoType.ValueString())
		requestBody.LogoType = &logoType
	}

	// Call API
	apiResp, err := r.client.UpdateOrganizationAppearanceWithResponse(ctx, requestBody)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to update organization appearance, got error: %s", err))
		return
	}

	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Expected 200 OK, got status %d", apiResp.StatusCode()),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *OrganizationAppearanceResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Singleton resource, nothing to strictly "delete" on server via API.
	// We just remove it from Terraform state.
	resp.State.RemoveResource(ctx)
}

func (r *OrganizationAppearanceResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Since it's a singleton, we ignore the ID passed in import (or enforce it's "current" or "settings")
	// and read the current state.
	// Usually ImportState just sets the ID and lets Read handle it.
	// But our resource doesn't strictly use the ID in Read/Update (it calls the singleton endpoints).
	// So setting any ID is fine, but "current" is good convention.
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
