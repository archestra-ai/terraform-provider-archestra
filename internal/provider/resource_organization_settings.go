package provider

import (
	"context"
	"fmt"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Enum value lists derived from the generated client constants
var fontValues = []string{
	string(client.Inter),
	string(client.Lato),
	string(client.OpenSans),
	string(client.Roboto),
	string(client.SourceSansPro),
}

var colorThemeValues = []string{
	string(client.AmberMinimal),
	string(client.BoldTech),
	string(client.Bubblegum),
	string(client.Caffeine),
	string(client.Candyland),
	string(client.Catppuccin),
	string(client.Claude),
	string(client.Claymorphism),
	string(client.CleanSlate),
	string(client.CosmicNight),
	string(client.Cyberpunk),
	string(client.Doom64),
	string(client.ElegantLuxury),
	string(client.Graphite),
	string(client.KodamaGrove),
	string(client.MidnightBloom),
	string(client.MochaMousse),
	string(client.ModernMinimal),
	string(client.Mono),
	string(client.Nature),
	string(client.NeoBrutalism),
	string(client.NorthernLights),
	string(client.OceanBreeze),
	string(client.PastelDreams),
	string(client.Perpetuity),
	string(client.QuantumRose),
	string(client.RetroArcade),
	string(client.SolarDusk),
	string(client.StarryNight),
	string(client.SunsetHorizon),
	string(client.Supabase),
	string(client.T3Chat),
	string(client.Tangerine),
	string(client.Twitter),
	string(client.Vercel),
	string(client.VintagePaper),
}

var limitCleanupIntervalValues = []string{
	string(client.UpdateOrganizationJSONBodyLimitCleanupIntervalN1h),
	string(client.UpdateOrganizationJSONBodyLimitCleanupIntervalN12h),
	string(client.UpdateOrganizationJSONBodyLimitCleanupIntervalN24h),
	string(client.UpdateOrganizationJSONBodyLimitCleanupIntervalN1w),
	string(client.UpdateOrganizationJSONBodyLimitCleanupIntervalN1m),
}

var compressionScopeValues = []string{
	string(client.Organization),
	string(client.Team),
}

var _ resource.Resource = &OrganizationSettingsResource{}
var _ resource.ResourceWithImportState = &OrganizationSettingsResource{}

func NewOrganizationSettingsResource() resource.Resource {
	return &OrganizationSettingsResource{}
}

type OrganizationSettingsResource struct {
	client *client.ClientWithResponses
}

type OrganizationSettingsResourceModel struct {
	Font                     types.String `tfsdk:"font"`
	ColorTheme               types.String `tfsdk:"color_theme"`
	Logo                     types.String `tfsdk:"logo"`
	LimitCleanupInterval     types.String `tfsdk:"limit_cleanup_interval"`
	CompressionScope         types.String `tfsdk:"compression_scope"`
	OnboardingComplete       types.Bool   `tfsdk:"onboarding_complete"`
	ConvertToolResultsToToon types.Bool   `tfsdk:"convert_tool_results_to_toon"`
}

func (r *OrganizationSettingsResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_organization_settings"
}

func (r *OrganizationSettingsResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages the organization settings (font, color theme, logo, cleanup interval, compression scope, and onboarding). This is a singleton resource.",

		Attributes: map[string]schema.Attribute{
			"font": schema.StringAttribute{
				MarkdownDescription: "The custom font for the organization.",
				Optional:            true,
				Validators: []validator.String{
					stringvalidator.OneOf(fontValues...),
				},
			},
			"color_theme": schema.StringAttribute{
				MarkdownDescription: "The color theme for the organization.",
				Optional:            true,
				Validators: []validator.String{
					stringvalidator.OneOf(colorThemeValues...),
				},
			},
			"logo": schema.StringAttribute{
				MarkdownDescription: "The organization's logo. This should be a base64 encoded string.",
				Optional:            true,
			},
			"limit_cleanup_interval": schema.StringAttribute{
				MarkdownDescription: "The interval for cleaning up limits. Valid values: 1h, 12h, 24h, 1w, 1m",
				Optional:            true,
				Validators: []validator.String{
					stringvalidator.OneOf(limitCleanupIntervalValues...),
				},
			},
			"compression_scope": schema.StringAttribute{
				MarkdownDescription: "The scope for compression. Valid values: organization, team",
				Optional:            true,
				Validators: []validator.String{
					stringvalidator.OneOf(compressionScopeValues...),
				},
			},
			"onboarding_complete": schema.BoolAttribute{
				MarkdownDescription: "Whether onboarding is complete for the organization.",
				Optional:            true,
			},
			"convert_tool_results_to_toon": schema.BoolAttribute{
				MarkdownDescription: "Whether to convert tool results to Toon format.",
				Optional:            true,
			},
		},
	}
}

func (r *OrganizationSettingsResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *OrganizationSettingsResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data OrganizationSettingsResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Create request body
	requestBody := client.UpdateOrganizationJSONBody{}

	if !data.Font.IsNull() {
		font := client.UpdateOrganizationJSONBodyCustomFont(data.Font.ValueString())
		requestBody.CustomFont = &font
	}

	if !data.ColorTheme.IsNull() {
		theme := client.UpdateOrganizationJSONBodyTheme(data.ColorTheme.ValueString())
		requestBody.Theme = &theme
	}

	if !data.Logo.IsNull() {
		logo := data.Logo.ValueString()
		requestBody.Logo = &logo
	}

	if !data.LimitCleanupInterval.IsNull() {
		interval := client.UpdateOrganizationJSONBodyLimitCleanupInterval(data.LimitCleanupInterval.ValueString())
		requestBody.LimitCleanupInterval = &interval
	}

	if !data.CompressionScope.IsNull() {
		scope := client.UpdateOrganizationJSONBodyCompressionScope(data.CompressionScope.ValueString())
		requestBody.CompressionScope = &scope
	}

	if !data.OnboardingComplete.IsNull() {
		onboarding := data.OnboardingComplete.ValueBool()
		requestBody.OnboardingComplete = &onboarding
	}

	if !data.ConvertToolResultsToToon.IsNull() {
		convert := data.ConvertToolResultsToToon.ValueBool()
		requestBody.ConvertToolResultsToToon = &convert
	}

	// Call API - treating Create as Update for singleton
	apiResp, err := r.client.UpdateOrganizationWithResponse(ctx, client.UpdateOrganizationJSONRequestBody(requestBody))
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to create/update organization settings, got error: %s", err))
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

func (r *OrganizationSettingsResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data OrganizationSettingsResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Call API
	apiResp, err := r.client.GetOrganizationWithResponse(ctx)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to read organization settings, got error: %s", err))
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
	if string(apiResp.JSON200.CustomFont) != "" {
		data.Font = types.StringValue(string(apiResp.JSON200.CustomFont))
	} else {
		data.Font = types.StringNull()
	}

	if string(apiResp.JSON200.Theme) != "" {
		data.ColorTheme = types.StringValue(string(apiResp.JSON200.Theme))
	} else {
		data.ColorTheme = types.StringNull()
	}

	if apiResp.JSON200.Logo != nil {
		data.Logo = types.StringValue(*apiResp.JSON200.Logo)
	} else {
		data.Logo = types.StringNull()
	}

	if apiResp.JSON200.LimitCleanupInterval != nil {
		data.LimitCleanupInterval = types.StringValue(string(*apiResp.JSON200.LimitCleanupInterval))
	} else {
		data.LimitCleanupInterval = types.StringNull()
	}

	if string(apiResp.JSON200.CompressionScope) != "" {
		data.CompressionScope = types.StringValue(string(apiResp.JSON200.CompressionScope))
	} else {
		data.CompressionScope = types.StringNull()
	}

	data.OnboardingComplete = types.BoolValue(apiResp.JSON200.OnboardingComplete)

	data.ConvertToolResultsToToon = types.BoolValue(apiResp.JSON200.ConvertToolResultsToToon)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *OrganizationSettingsResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data OrganizationSettingsResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Create request body
	requestBody := client.UpdateOrganizationJSONBody{}

	if !data.Font.IsNull() {
		font := client.UpdateOrganizationJSONBodyCustomFont(data.Font.ValueString())
		requestBody.CustomFont = &font
	}

	if !data.ColorTheme.IsNull() {
		theme := client.UpdateOrganizationJSONBodyTheme(data.ColorTheme.ValueString())
		requestBody.Theme = &theme
	}

	if !data.Logo.IsNull() {
		logo := data.Logo.ValueString()
		requestBody.Logo = &logo
	}

	if !data.LimitCleanupInterval.IsNull() {
		interval := client.UpdateOrganizationJSONBodyLimitCleanupInterval(data.LimitCleanupInterval.ValueString())
		requestBody.LimitCleanupInterval = &interval
	}

	if !data.CompressionScope.IsNull() {
		scope := client.UpdateOrganizationJSONBodyCompressionScope(data.CompressionScope.ValueString())
		requestBody.CompressionScope = &scope
	}

	if !data.OnboardingComplete.IsNull() {
		onboarding := data.OnboardingComplete.ValueBool()
		requestBody.OnboardingComplete = &onboarding
	}

	if !data.ConvertToolResultsToToon.IsNull() {
		convert := data.ConvertToolResultsToToon.ValueBool()
		requestBody.ConvertToolResultsToToon = &convert
	}

	// Call API
	apiResp, err := r.client.UpdateOrganizationWithResponse(ctx, client.UpdateOrganizationJSONRequestBody(requestBody))
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to update organization settings, got error: %s", err))
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

func (r *OrganizationSettingsResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Singleton resource, nothing to strictly "delete" on server via API.
	// We just remove it from Terraform state.
	resp.State.RemoveResource(ctx)
}

func (r *OrganizationSettingsResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Since it's a singleton, we ignore the ID passed in import (or enforce it's "current" or "settings")
	// and read the current state.
	// Usually ImportState just sets the ID and lets Read handle it.
	// But our resource doesn't strictly use the ID in Read/Update (it calls the singleton endpoints).
	// So setting any ID is fine, but "current" is good convention.
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
