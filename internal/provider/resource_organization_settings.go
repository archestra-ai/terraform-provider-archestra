package provider

import (
	"context"
	"fmt"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
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
		MarkdownDescription: "Manages organization settings in Archestra. This is a singleton resource - only one instance can exist per organization. Note: Running `terraform destroy` will only remove this resource from Terraform state; the organization settings will remain unchanged on the server.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Organization identifier",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"font": schema.StringAttribute{
				MarkdownDescription: "Custom font for the organization UI",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString(string(client.Inter)),
				Validators: []validator.String{
					stringvalidator.OneOf(
						string(client.Inter),
						string(client.Lato),
						string(client.OpenSans),
						string(client.Roboto),
						string(client.SourceSansPro),
					),
				},
			},
			"color_theme": schema.StringAttribute{
				MarkdownDescription: "Color theme for the organization UI",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString(string(client.ModernMinimal)),
				Validators: []validator.String{
					stringvalidator.OneOf(
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
					),
				},
			},
			"logo": schema.StringAttribute{
				MarkdownDescription: "Base64 encoded logo image for the organization",
				Optional:            true,
			},
			"limit_cleanup_interval": schema.StringAttribute{
				MarkdownDescription: "Interval for cleaning up usage limits. Valid values: 1h, 12h, 24h, 1w, 1m. Set to null to disable.",
				Optional:            true,
				Validators: []validator.String{
					stringvalidator.OneOf(
						string(client.UpdateOrganizationJSONBodyLimitCleanupIntervalN1h),
						string(client.UpdateOrganizationJSONBodyLimitCleanupIntervalN12h),
						string(client.UpdateOrganizationJSONBodyLimitCleanupIntervalN24h),
						string(client.UpdateOrganizationJSONBodyLimitCleanupIntervalN1w),
						string(client.UpdateOrganizationJSONBodyLimitCleanupIntervalN1m),
					),
				},
			},
			"compression_scope": schema.StringAttribute{
				MarkdownDescription: "Scope for tool results compression",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString(string(client.Organization)),
				Validators: []validator.String{
					stringvalidator.OneOf(
						string(client.Organization),
						string(client.Team),
					),
				},
			},
			"onboarding_complete": schema.BoolAttribute{
				MarkdownDescription: "Whether organization onboarding is complete",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
			},
			"convert_tool_results_to_toon": schema.BoolAttribute{
				MarkdownDescription: "Whether to convert tool results to TOON format for compression",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
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

	requestBody := r.buildUpdateRequest(&data)

	apiResp, err := r.client.UpdateOrganizationWithResponse(ctx, requestBody)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to update organization settings, got error: %s", err))
		return
	}

	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Expected 200 OK, got status %d: %s", apiResp.StatusCode(), string(apiResp.Body)),
		)
		return
	}

	r.mapResponseToModel(&data, apiResp)

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

	data.ID = types.StringValue(apiResp.JSON200.Id)
	data.Font = types.StringValue(string(apiResp.JSON200.CustomFont))
	data.ColorTheme = types.StringValue(string(apiResp.JSON200.Theme))
	data.CompressionScope = types.StringValue(string(apiResp.JSON200.CompressionScope))
	data.OnboardingComplete = types.BoolValue(apiResp.JSON200.OnboardingComplete)
	data.ConvertToolResultsToToon = types.BoolValue(apiResp.JSON200.ConvertToolResultsToToon)

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

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *OrganizationSettingsResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data OrganizationSettingsResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	requestBody := r.buildUpdateRequest(&data)

	apiResp, err := r.client.UpdateOrganizationWithResponse(ctx, requestBody)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to update organization settings, got error: %s", err))
		return
	}

	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Expected 200 OK, got status %d: %s", apiResp.StatusCode(), string(apiResp.Body)),
		)
		return
	}

	r.mapResponseToModel(&data, apiResp)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *OrganizationSettingsResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Organization settings cannot be deleted via API.
	// Removing from Terraform state only - the organization settings will remain on the server.
}

func (r *OrganizationSettingsResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *OrganizationSettingsResource) buildUpdateRequest(data *OrganizationSettingsResourceModel) client.UpdateOrganizationJSONRequestBody {
	requestBody := client.UpdateOrganizationJSONRequestBody{}

	if !data.Font.IsNull() && !data.Font.IsUnknown() {
		font := client.UpdateOrganizationJSONBodyCustomFont(data.Font.ValueString())
		requestBody.CustomFont = &font
	}

	if !data.ColorTheme.IsNull() && !data.ColorTheme.IsUnknown() {
		theme := client.UpdateOrganizationJSONBodyTheme(data.ColorTheme.ValueString())
		requestBody.Theme = &theme
	}

	if !data.Logo.IsNull() && !data.Logo.IsUnknown() {
		logo := data.Logo.ValueString()
		requestBody.Logo = &logo
	}

	if !data.LimitCleanupInterval.IsNull() && !data.LimitCleanupInterval.IsUnknown() {
		interval := client.UpdateOrganizationJSONBodyLimitCleanupInterval(data.LimitCleanupInterval.ValueString())
		requestBody.LimitCleanupInterval = &interval
	}

	if !data.CompressionScope.IsNull() && !data.CompressionScope.IsUnknown() {
		scope := client.UpdateOrganizationJSONBodyCompressionScope(data.CompressionScope.ValueString())
		requestBody.CompressionScope = &scope
	}

	if !data.OnboardingComplete.IsNull() && !data.OnboardingComplete.IsUnknown() {
		onboarding := data.OnboardingComplete.ValueBool()
		requestBody.OnboardingComplete = &onboarding
	}

	if !data.ConvertToolResultsToToon.IsNull() && !data.ConvertToolResultsToToon.IsUnknown() {
		convert := data.ConvertToolResultsToToon.ValueBool()
		requestBody.ConvertToolResultsToToon = &convert
	}

	return requestBody
}

func (r *OrganizationSettingsResource) mapResponseToModel(data *OrganizationSettingsResourceModel, org *client.UpdateOrganizationResponse) {
	if org.JSON200 == nil {
		return
	}

	resp := org.JSON200
	data.ID = types.StringValue(resp.Id)
	data.Font = types.StringValue(string(resp.CustomFont))
	data.ColorTheme = types.StringValue(string(resp.Theme))
	data.CompressionScope = types.StringValue(string(resp.CompressionScope))
	data.OnboardingComplete = types.BoolValue(resp.OnboardingComplete)
	data.ConvertToolResultsToToon = types.BoolValue(resp.ConvertToolResultsToToon)

	if resp.Logo != nil {
		data.Logo = types.StringValue(*resp.Logo)
	} else {
		data.Logo = types.StringNull()
	}

	if resp.LimitCleanupInterval != nil {
		data.LimitCleanupInterval = types.StringValue(string(*resp.LimitCleanupInterval))
	} else {
		data.LimitCleanupInterval = types.StringNull()
	}
}
