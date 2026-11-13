package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"regexp"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/hashicorp/terraform-plugin-framework-validators/float64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/float64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

var _ resource.ResourceWithImportState = &AutoRouterResource{}

func NewAutoRouterResource() resource.Resource { return &AutoRouterResource{} }

type AutoRouterResource struct{ client *client.AutoRouterClient }

type AutoRouterResourceModel struct {
	ID               types.String  `tfsdk:"id"`
	Name             types.String  `tfsdk:"name"`
	BudgetUsdPerDay  types.Float64 `tfsdk:"budget_usd_per_day"`
	BudgetPreset     types.String  `tfsdk:"budget_preset"`
	Tiers            types.Object  `tfsdk:"tiers"`
	DecisionModel    types.Object  `tfsdk:"decision_model"`
	Enabled          types.Bool    `tfsdk:"enabled"`
	VirtualAPIKeyIDs types.Set     `tfsdk:"virtual_api_key_ids"`
}

var autoRouterTierType = types.ObjectType{AttrTypes: map[string]attr.Type{
	"model_id": types.StringType, "provider_api_key_id": types.StringType,
}}
var autoRouterTiersType = types.ObjectType{AttrTypes: map[string]attr.Type{
	"fast": autoRouterTierType, "normal": autoRouterTierType, "smart": autoRouterTierType,
}}
var autoRouterDecisionModelType = types.ObjectType{AttrTypes: map[string]attr.Type{
	"api_key_id": types.StringType, "model": types.StringType,
}}

var autoRouterTierSpec = []AttrSpec{
	{TFName: "model_id", JSONName: "modelId", Kind: Scalar},
	{TFName: "provider_api_key_id", JSONName: "providerApiKeyId", Kind: Scalar},
}

var autoRouterAttrSpec = []AttrSpec{
	{TFName: "name", JSONName: "name", Kind: Scalar},
	// The update API cannot clear a custom budget; omitting it keeps the stored one.
	{TFName: "budget_usd_per_day", JSONName: "budgetUsdPerDay", Kind: Scalar, OmitOnNull: true},
	{TFName: "budget_preset", JSONName: "budgetPreset", Kind: Scalar},
	{TFName: "tiers", JSONName: "tiers", Kind: AtomicObject, Children: []AttrSpec{
		{TFName: "fast", JSONName: "fast", Kind: AtomicObject, Children: autoRouterTierSpec},
		{TFName: "normal", JSONName: "normal", Kind: AtomicObject, Children: autoRouterTierSpec},
		{TFName: "smart", JSONName: "smart", Kind: AtomicObject, Children: autoRouterTierSpec},
	}},
	{TFName: "decision_model", JSONName: "decisionModel", Kind: AtomicObject, OmitOnNull: true, Encoder: autoRouterDecisionModelEncoder, Children: []AttrSpec{
		{TFName: "api_key_id", JSONName: "apiKeyId", Kind: Scalar},
		{TFName: "model", JSONName: "model", Kind: Scalar},
	}},
}

func (r *AutoRouterResource) AttrSpecs() []AttrSpec { return autoRouterAttrSpec }
func (r *AutoRouterResource) APIShape() any         { return client.GetAutoRouterResponse{} }

// Planning bookkeeping and usage are runtime state, not configuration:
// generations and status track fits, usage counts the agents that use the
// router, and usesPastSessions is set only on an organization's migrated router.
func (r *AutoRouterResource) KnownIntentionallySkipped() []string {
	return []string{"usesPastSessions", "usage", "generation", "plannedGeneration", "status"}
}

func (r *AutoRouterResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_auto_router"
}

func (r *AutoRouterResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	tier := func(name string) schema.SingleNestedAttribute {
		return schema.SingleNestedAttribute{Required: true, MarkdownDescription: fmt.Sprintf("The %s tier: a tool-capable chat model and the provider key that serves it.", name), Attributes: map[string]schema.Attribute{
			"model_id":            schema.StringAttribute{Required: true, MarkdownDescription: "Database UUID of the model row (`archestra_model` ID), not the provider-facing model name.", Validators: []validator.String{stringvalidator.RegexMatches(canonicalUUIDRegexp, "Use a lowercase canonical model UUID.")}},
			"provider_api_key_id": schema.StringAttribute{Required: true, MarkdownDescription: "UUID of the `archestra_model_provider` key that serves this tier: shared with the whole organization and serving `model_id`.", Validators: []validator.String{stringvalidator.RegexMatches(canonicalUUIDRegexp, "Use a lowercase canonical provider key UUID.")}},
		}}
	}
	resp.Schema = schema.Schema{
		MarkdownDescription: "ALPHA. An auto router serves the `auto` (or `auto:<name>`) model: it scores each session and routes it to a fast, normal or smart tier within a daily budget. Requires API 1.5.0-rc.29 or later with `ARCHESTRA_AUTO_ROUTING_ENABLED=true` (or `ARCHESTRA_BETA=true`). Agents, MCP gateways, OAuth clients, virtual keys and the organization default reference a router by ID.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Computed: true, MarkdownDescription: "Auto router UUID.", PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"name": schema.StringAttribute{Required: true, MarkdownDescription: "Model name clients request: `auto`, or `auto:<name>` with a lowercase name.", Validators: []validator.String{
				stringvalidator.RegexMatches(regexp.MustCompile(`^auto(:[a-z0-9][a-z0-9._-]{0,62})?$`), "Use `auto` or `auto:<name>`: lowercase letters, digits, dots, underscores and hyphens, starting with a letter or digit, at most 63 characters."),
			}},
			"budget_usd_per_day": schema.Float64Attribute{Optional: true, Computed: true, PlanModifiers: []planmodifier.Float64{float64planmodifier.UseStateForUnknown()}, MarkdownDescription: "Custom daily budget in USD. The API cannot clear it: removing it keeps the stored value, which applies again when `budget_preset` is unset.", Validators: []validator.Float64{float64validator.Between(0.01, 9999999999.99)}},
			"budget_preset":      schema.StringAttribute{Optional: true, MarkdownDescription: "Budget preset that derives the daily budget from current usage: `cost_efficient`, `balanced` or `intelligence`. Overrides `budget_usd_per_day` while set.", Validators: []validator.String{stringvalidator.OneOf("cost_efficient", "balanced", "intelligence")}},
			"tiers": schema.SingleNestedAttribute{Required: true, MarkdownDescription: "The three tiers sessions are routed between. Every tier model must be a tool-capable chat model, and tiers with long-context prices must share one threshold.", Attributes: map[string]schema.Attribute{
				"fast": tier("fast"), "normal": tier("normal"), "smart": tier("smart"),
			}},
			"decision_model": schema.SingleNestedAttribute{Optional: true, Computed: true, PlanModifiers: []planmodifier.Object{objectplanmodifier.UseStateForUnknown()}, MarkdownDescription: "Model that scores sessions. Omit it for the platform default model on the organization's primary key of the default decisions provider.", Attributes: map[string]schema.Attribute{
				"api_key_id": schema.StringAttribute{Optional: true, MarkdownDescription: "Organization-wide key of the decisions provider (`jev`) for the decision model; null uses the organization's primary key of that provider."},
				"model":      schema.StringAttribute{Required: true, MarkdownDescription: "Decision model ID the decisions provider offers, such as `jev-1.13.0`."},
			}},
			"enabled":             schema.BoolAttribute{Computed: true, PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()}, MarkdownDescription: "Whether the router currently routes sessions (auto routing is on for the organization)."},
			"virtual_api_key_ids": schema.SetAttribute{Computed: true, PlanModifiers: []planmodifier.Set{setplanmodifier.UseStateForUnknown()}, ElementType: types.StringType, MarkdownDescription: "Virtual keys granted this router through their own `auto_router_ids`."},
		},
	}
}

func (r *AutoRouterResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	api, ok := req.ProviderData.(*client.ClientWithResponses)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Resource Configure Type", fmt.Sprintf("Expected API client, got %T", req.ProviderData))
		return
	}
	var err error
	r.client, err = client.NewAutoRouterClient(api)
	if err != nil {
		resp.Diagnostics.AddError("Auto Router Client Configuration Failed", err.Error())
	}
}

func (r *AutoRouterResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data AutoRouterResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	patch := MergePatch(ctx, req.Plan.Raw, tftypes.NewValue(req.Plan.Raw.Type(), nil), autoRouterAttrSpec, &resp.Diagnostics)
	body := autoRouterBody(ctx, "Create", patch, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	router, err := r.client.Write(ctx, "", bytes.NewReader(body))
	if err != nil {
		resp.Diagnostics.AddError("Auto Router Creation Failed", err.Error())
		return
	}
	if !canonicalUUIDRegexp.MatchString(router.ID) {
		resp.Diagnostics.AddError("Invalid Auto Router Response", "Creation did not return a canonical router UUID.")
		return
	}
	// Keep the created identity even if mapping fails, so refresh or destroy
	// can recover the remote router instead of orphaning it.
	data.ID = types.StringValue(router.ID)
	data.Enabled = types.BoolValue(router.Enabled)
	data.VirtualAPIKeyIDs = types.SetNull(types.StringType)
	if data.BudgetUsdPerDay.IsUnknown() {
		data.BudgetUsdPerDay = types.Float64Null()
	}
	if data.DecisionModel.IsUnknown() {
		data.DecisionModel = types.ObjectNull(autoRouterDecisionModelType.AttrTypes)
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	flattenAutoRouter(ctx, &data, router, &resp.Diagnostics)
	if !resp.Diagnostics.HasError() {
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
	}
}

func (r *AutoRouterResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data AutoRouterResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if data.ID.IsNull() || data.ID.IsUnknown() || data.ID.ValueString() == "" {
		resp.State.RemoveResource(ctx)
		return
	}
	router, err := r.client.Get(ctx, data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Auto Router Read Failed", err.Error())
		return
	}
	if router == nil {
		resp.State.RemoveResource(ctx)
		return
	}
	flattenAutoRouter(ctx, &data, router, &resp.Diagnostics)
	if !resp.Diagnostics.HasError() {
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
	}
}

func (r *AutoRouterResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data AutoRouterResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	patch := MergePatch(ctx, req.Plan.Raw, req.State.Raw, autoRouterAttrSpec, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	// A custom budget sent on its own clears the preset, so restate a preset
	// the configuration keeps.
	if _, budgetChanged := patch["budgetUsdPerDay"]; budgetChanged {
		if _, presetSent := patch["budgetPreset"]; !presetSent && !data.BudgetPreset.IsNull() && !data.BudgetPreset.IsUnknown() {
			patch["budgetPreset"] = data.BudgetPreset.ValueString()
		}
	}
	body := autoRouterBody(ctx, "Update", patch, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	router, err := r.client.Write(ctx, data.ID.ValueString(), bytes.NewReader(body))
	if err != nil {
		resp.Diagnostics.AddError("Auto Router Update Failed", err.Error())
		return
	}
	if router.ID != data.ID.ValueString() {
		resp.Diagnostics.AddError("Invalid Auto Router Response", "Update returned a different router UUID.")
		return
	}
	flattenAutoRouter(ctx, &data, router, &resp.Diagnostics)
	if !resp.Diagnostics.HasError() {
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
	}
}

func (r *AutoRouterResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data AutoRouterResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	status, err := r.client.Delete(ctx, data.ID.ValueString())
	if status == 404 {
		// The feature gate also answers 404; only a live API proves deletion.
		router, getErr := r.client.Get(ctx, data.ID.ValueString())
		if getErr == nil && router == nil {
			return
		}
	}
	if err != nil {
		resp.Diagnostics.AddError("Auto Router Deletion Failed", err.Error())
	}
}

func (r *AutoRouterResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	if !canonicalUUIDRegexp.MatchString(req.ID) {
		resp.Diagnostics.AddError("Invalid Auto Router Import ID", "Import using the router's lowercase canonical UUID from GET /api/auto-routing/routers.")
		return
	}
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// flattenAutoRouter maps the API router onto state. Response tiers carry the
// model row as `id` and the key as `apiKey.id`; a tier whose model or key was
// deleted reads back as null, so the plan restores it.
func flattenAutoRouter(ctx context.Context, data *AutoRouterResourceModel, router *client.AutoRouter, diags *diag.Diagnostics) {
	if !canonicalUUIDRegexp.MatchString(router.ID) || router.Name == "" || router.VirtualAPIKeyIDs == nil {
		diags.AddError("Invalid Auto Router Response", "Expected a router UUID, name and virtual-key grants.")
		return
	}
	data.ID = types.StringValue(router.ID)
	data.Name = types.StringValue(router.Name)
	data.Enabled = types.BoolValue(router.Enabled)
	data.BudgetUsdPerDay = types.Float64PointerValue(router.BudgetUsdPerDay)
	data.BudgetPreset = types.StringPointerValue(router.BudgetPreset)
	tier := func(t *client.AutoRouterTier) attr.Value {
		if t == nil {
			return types.ObjectNull(autoRouterTierType.AttrTypes)
		}
		value, d := types.ObjectValue(autoRouterTierType.AttrTypes, map[string]attr.Value{
			"model_id": types.StringValue(t.ID), "provider_api_key_id": types.StringValue(t.APIKey.ID),
		})
		diags.Append(d...)
		return value
	}
	var d diag.Diagnostics
	data.Tiers, d = types.ObjectValue(autoRouterTiersType.AttrTypes, map[string]attr.Value{
		"fast": tier(router.Tiers.Fast), "normal": tier(router.Tiers.Normal), "smart": tier(router.Tiers.Smart),
	})
	diags.Append(d...)
	data.DecisionModel, d = types.ObjectValue(autoRouterDecisionModelType.AttrTypes, map[string]attr.Value{
		"api_key_id": types.StringPointerValue(router.DecisionModel.APIKeyID), "model": types.StringValue(router.DecisionModel.Model),
	})
	diags.Append(d...)
	data.VirtualAPIKeyIDs, d = types.SetValueFrom(ctx, types.StringType, router.VirtualAPIKeyIDs)
	diags.Append(d...)
}

// The API requires `apiKeyId` whenever a decision model is sent; null selects
// the organization's primary key, but the generic encoder drops null children.
func autoRouterDecisionModelEncoder(value any) any {
	if fields, ok := value.(map[string]any); ok {
		if _, present := fields["apiKeyId"]; !present {
			fields["apiKeyId"] = nil
		}
	}
	return value
}

func autoRouterBody(ctx context.Context, operation string, patch map[string]any, diags *diag.Diagnostics) []byte {
	if diags.HasError() {
		return nil
	}
	LogPatch(ctx, "archestra_auto_router "+operation, patch, autoRouterAttrSpec)
	body, err := json.Marshal(patch)
	if err != nil {
		diags.AddError("Auto Router Encoding Failed", err.Error())
	}
	return body
}
