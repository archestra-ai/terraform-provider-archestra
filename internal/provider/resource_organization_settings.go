package provider

import (
	"context"
	"fmt"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	allowedFonts = []string{
		string(client.Inter),
		string(client.Lato),
		string(client.OpenSans),
		string(client.Roboto),
		string(client.SourceSansPro),
	}

	allowedThemes = []string{
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

	allowedCleanupIntervals = []string{
		string(client.UpdateOrganizationJSONBodyLimitCleanupIntervalN1h),
		string(client.UpdateOrganizationJSONBodyLimitCleanupIntervalN12h),
		string(client.UpdateOrganizationJSONBodyLimitCleanupIntervalN24h),
		string(client.UpdateOrganizationJSONBodyLimitCleanupIntervalN1w),
		string(client.UpdateOrganizationJSONBodyLimitCleanupIntervalN1m),
	}

	allowedCompressionScopes = []string{
		string(client.Organization),
		string(client.Team),
	}
)

var _ resource.Resource = &OrganizationSettingsResource{}
var _ resource.ResourceWithImportState = &OrganizationSettingsResource{}

func NewOrganizationSettingsResource() resource.Resource {
	return &OrganizationSettingsResource{}
}

type OrganizationSettingsResource struct {
	client *client.ClientWithResponses
}

type OrganizationSettingsResourceModel struct {
	ID                       types.String `tfsdk:"id"`
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
		MarkdownDescription: "Manages organization settings in Archestra. This is a singleton resource - only one instance exists per organization.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Organization identifier",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"font": schema.StringAttribute{
				MarkdownDescription: "Custom font for the organization",
				Optional:            true,
				Computed:            true,
				Validators: []validator.String{
					stringvalidator.OneOf(allowedFonts...),
				},
			},
			"color_theme": schema.StringAttribute{
				MarkdownDescription: "Color theme for the organization",
				Optional:            true,
				Computed:            true,
				Validators: []validator.String{
					stringvalidator.OneOf(allowedThemes...),
				},
			},
			"logo": schema.StringAttribute{
				MarkdownDescription: "Base64 encoded logo image for the organization",
				Optional:            true,
				Computed:            true,
				Sensitive:           true,
			},
			"limit_cleanup_interval": schema.StringAttribute{
				MarkdownDescription: "Interval for cleaning up limits. Set to null to disable.",
				Optional:            true,
				Computed:            true,
				Validators: []validator.String{
					stringvalidator.OneOf(allowedCleanupIntervals...),
				},
			},
			"compression_scope": schema.StringAttribute{
				MarkdownDescription: "Compression scope for the organization",
				Optional:            true,
				Computed:            true,
				Validators: []validator.String{
					stringvalidator.OneOf(allowedCompressionScopes...),
				},
			},
			"onboarding_complete": schema.BoolAttribute{
				MarkdownDescription: "Whether the organization onboarding is complete",
				Optional:            true,
				Computed:            true,
			},
			"convert_tool_results_to_toon": schema.BoolAttribute{
				MarkdownDescription: "Whether to convert tool results to TOON format",
				Optional:            true,
				Computed:            true,
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
			fmt.Sprintf("Expected *client.ClientWithResponses, got: %T", req.ProviderData),
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

	requestBody := buildUpdateRequestBody(data)

	apiResp, err := r.client.UpdateOrganizationWithResponse(ctx, client.UpdateOrganizationJSONRequestBody(requestBody))
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to update organization settings: %s", err))
		return
	}

	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError("Unexpected API Response", fmt.Sprintf("Expected 200 OK, got status %d", apiResp.StatusCode()))
		return
	}

	mapResponseToState(&data, apiResp.JSON200)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *OrganizationSettingsResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data OrganizationSettingsResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := r.client.GetOrganizationWithResponse(ctx)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to read organization settings: %s", err))
		return
	}

	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError("Unexpected API Response", fmt.Sprintf("Expected 200 OK, got status %d", apiResp.StatusCode()))
		return
	}

	mapResponseToState(&data, apiResp.JSON200)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *OrganizationSettingsResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data OrganizationSettingsResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	requestBody := buildUpdateRequestBody(data)

	apiResp, err := r.client.UpdateOrganizationWithResponse(ctx, client.UpdateOrganizationJSONRequestBody(requestBody))
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to update organization settings: %s", err))
		return
	}

	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError("Unexpected API Response", fmt.Sprintf("Expected 200 OK, got status %d", apiResp.StatusCode()))
		return
	}

	mapResponseToState(&data, apiResp.JSON200)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *OrganizationSettingsResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	resp.State.RemoveResource(ctx)
}

func (r *OrganizationSettingsResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func buildUpdateRequestBody(plan OrganizationSettingsResourceModel) client.UpdateOrganizationJSONBody {
	requestBody := client.UpdateOrganizationJSONBody{}

	if !plan.Font.IsNull() {
		font := client.UpdateOrganizationJSONBodyCustomFont(plan.Font.ValueString())
		requestBody.CustomFont = &font
	}

	if !plan.ColorTheme.IsNull() {
		theme := client.UpdateOrganizationJSONBodyTheme(plan.ColorTheme.ValueString())
		requestBody.Theme = &theme
	}

	if !plan.Logo.IsNull() {
		logo := plan.Logo.ValueString()
		requestBody.Logo = &logo
	}

	if !plan.LimitCleanupInterval.IsNull() {
		interval := client.UpdateOrganizationJSONBodyLimitCleanupInterval(plan.LimitCleanupInterval.ValueString())
		requestBody.LimitCleanupInterval = &interval
	}

	if !plan.CompressionScope.IsNull() {
		scope := client.UpdateOrganizationJSONBodyCompressionScope(plan.CompressionScope.ValueString())
		requestBody.CompressionScope = &scope
	}

	if !plan.OnboardingComplete.IsNull() {
		val := plan.OnboardingComplete.ValueBool()
		requestBody.OnboardingComplete = &val
	}

	if !plan.ConvertToolResultsToToon.IsNull() {
		val := plan.ConvertToolResultsToToon.ValueBool()
		requestBody.ConvertToolResultsToToon = &val
	}

	return requestBody
}

func mapResponseToState(data *OrganizationSettingsResourceModel, org any) {
	switch v := org.(type) {
	// Case 1: Update Response
	case *struct {
		CompressionScope         client.UpdateOrganization200CompressionScope
		ConvertToolResultsToToon bool
		CreatedAt                interface{}
		CustomFont               client.UpdateOrganization200CustomFont
		Id                       string
		LimitCleanupInterval     *client.UpdateOrganization200LimitCleanupInterval
		Logo                     *string
		Metadata                 *string
		Name                     string
		OnboardingComplete       bool
		Slug                     string
		Theme                    client.UpdateOrganization200Theme
	}:
		data.ID = types.StringValue(v.Id)

		if string(v.CustomFont) != "" {
			data.Font = types.StringValue(string(v.CustomFont))
		} else {
			data.Font = types.StringNull()
		}

		if string(v.Theme) != "" {
			data.ColorTheme = types.StringValue(string(v.Theme))
		} else {
			data.ColorTheme = types.StringNull()
		}

		if v.Logo != nil {
			data.Logo = types.StringValue(*v.Logo)
		} else {
			data.Logo = types.StringNull()
		}

		if v.LimitCleanupInterval != nil {
			data.LimitCleanupInterval = types.StringValue(string(*v.LimitCleanupInterval))
		} else {
			data.LimitCleanupInterval = types.StringNull()
		}

		if string(v.CompressionScope) != "" {
			data.CompressionScope = types.StringValue(string(v.CompressionScope))
		} else {
			data.CompressionScope = types.StringNull()
		}

		data.OnboardingComplete = types.BoolValue(v.OnboardingComplete)

		data.ConvertToolResultsToToon = types.BoolValue(v.ConvertToolResultsToToon)

	// Case 2: Get Response
	case *struct {
		CompressionScope         client.GetOrganization200CompressionScope
		ConvertToolResultsToToon bool
		CreatedAt                interface{}
		CustomFont               client.GetOrganization200CustomFont
		Id                       string
		LimitCleanupInterval     *client.GetOrganization200LimitCleanupInterval
		Logo                     *string
		Metadata                 *string
		Name                     string
		OnboardingComplete       bool
		Slug                     string
		Theme                    client.GetOrganization200Theme
	}:
		data.ID = types.StringValue(v.Id)

		if string(v.CustomFont) != "" {
			data.Font = types.StringValue(string(v.CustomFont))
		} else {
			data.Font = types.StringNull()
		}

		if string(v.Theme) != "" {
			data.ColorTheme = types.StringValue(string(v.Theme))
		} else {
			data.ColorTheme = types.StringNull()
		}

		if v.Logo != nil {
			data.Logo = types.StringValue(*v.Logo)
		} else {
			data.Logo = types.StringNull()
		}

		if v.LimitCleanupInterval != nil {
			data.LimitCleanupInterval = types.StringValue(string(*v.LimitCleanupInterval))
		} else {
			data.LimitCleanupInterval = types.StringNull()
		}

		if string(v.CompressionScope) != "" {
			data.CompressionScope = types.StringValue(string(v.CompressionScope))
		} else {
			data.CompressionScope = types.StringNull()
		}

		data.OnboardingComplete = types.BoolValue(v.OnboardingComplete)

		data.ConvertToolResultsToToon = types.BoolValue(v.ConvertToolResultsToToon)
	}
}
