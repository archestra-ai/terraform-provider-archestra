package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/mapplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

var environmentDefaultSlots = map[string]string{"agents": "agent", "mcp_gateways": "mcpGateway", "mcp_registry": "mcpRegistry", "apps": "app", "knowledge_connectors": "knowledgeSource"}
var environmentDefaultsType = types.ObjectType{AttrTypes: map[string]attr.Type{"agents": types.StringType, "mcp_gateways": types.StringType, "mcp_registry": types.StringType, "apps": types.StringType, "knowledge_connectors": types.StringType}}
var userDefaultLimitType = types.ObjectType{AttrTypes: map[string]attr.Type{"limit_value": types.Int64Type, "models": types.ListType{ElemType: types.StringType}, "cleanup_interval": types.StringType}}

type userDefaultLimitModel struct {
	LimitValue      types.Int64  `tfsdk:"limit_value"`
	Models          types.List   `tfsdk:"models"`
	CleanupInterval types.String `tfsdk:"cleanup_interval"`
}
type userDefaultLimitWire struct {
	ID              uuid.UUID `json:"id"`
	EnvironmentID   *string   `json:"environmentId"`
	LimitValue      int64     `json:"limitValue"`
	Models          []string  `json:"model"`
	CleanupInterval string    `json:"cleanupInterval"`
}

func organizationDefaultsAttributes() map[string]schema.Attribute {
	environments := map[string]schema.Attribute{}
	for name := range environmentDefaultSlots {
		environments[name] = schema.StringAttribute{Optional: true, Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, MarkdownDescription: "Default deployment environment UUID for this resource kind. Set an empty string to use the implicit Default environment; omit to preserve."}
	}
	return map[string]schema.Attribute{
		"environment_defaults": schema.SingleNestedAttribute{Optional: true, Computed: true, Attributes: environments, PlanModifiers: []planmodifier.Object{objectplanmodifier.UseStateForUnknown()}, MarkdownDescription: "Default deployment environments by product object. Each entry is an environment UUID, not the ID of an agent or app. Omit entries to preserve them; use an empty string to reset an entry."},
		"default_user_limits": schema.MapNestedAttribute{Optional: true, Computed: true, PlanModifiers: []planmodifier.Map{mapplanmodifier.UseStateForUnknown()}, Validators: []validator.Map{defaultLimitSlotValidator{}}, MarkdownDescription: "Default member spending limits from LLM Settings, keyed by `default` for the organization or an environment UUID for an override. Omit this map to preserve all defaults; an explicit map owns the complete set, and {} removes them all. Organization-settings deletion leaves these settings intact.", NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
			"limit_value":      schema.Int64Attribute{Required: true, Validators: []validator.Int64{int64validator.Between(1, 9007199254740991)}, MarkdownDescription: "Whole USD spending allowance for the reset interval."},
			"models":           schema.ListAttribute{Required: true, ElementType: types.StringType, MarkdownDescription: "Model names covered by this default; [] covers all models."},
			"cleanup_interval": schema.StringAttribute{Required: true, Validators: []validator.String{stringvalidator.OneOf("1h", "12h", "24h", "1w", "1m", "calendar_day", "calendar_week_sunday", "calendar_week_monday", "calendar_month")}, MarkdownDescription: "Reset interval; for example calendar_month."},
		}}},
	}
}

type defaultLimitSlotValidator struct{}

func (defaultLimitSlotValidator) Description(context.Context) string {
	return "Use default or an environment UUID as each spending-limit key."
}
func (v defaultLimitSlotValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}
func (defaultLimitSlotValidator) ValidateMap(_ context.Context, req validator.MapRequest, resp *validator.MapResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	for key := range req.ConfigValue.Elements() {
		if key != "default" && !canonicalUUIDRegexp.MatchString(key) {
			resp.Diagnostics.AddAttributeError(req.Path, "Invalid default-limit environment", "Use default or a lowercase canonical environment UUID as the map key.")
		}
	}
}

func (r *OrganizationSettingsResource) readDefaults(ctx context.Context, data *OrganizationSettingsResourceModel, diags *diag.Diagnostics) {
	r.readEnvironmentDefaults(ctx, data, diags)
	if diags.HasError() {
		return
	}
	r.readUserDefaultLimits(ctx, data, diags)
}
func (r *OrganizationSettingsResource) readEnvironmentDefaults(ctx context.Context, data *OrganizationSettingsResourceModel, diags *diag.Diagnostics) {
	result, err := r.client.ListEnvironmentsWithResponse(ctx, nil)
	if err != nil {
		diags.AddError("Environment defaults read failed", err.Error())
		return
	}
	if result.StatusCode() == 403 && (data.EnvironmentDefaults.IsNull() || data.EnvironmentDefaults.IsUnknown()) {
		data.EnvironmentDefaults = types.ObjectNull(environmentDefaultsType.AttrTypes)
		return
	}
	if result.JSON200 == nil {
		diags.AddError("Environment defaults read failed", fmt.Sprintf("API returned status %d.", result.StatusCode()))
		return
	}
	var body struct {
		ResourceDefaults map[string]*string `json:"resourceDefaults"`
	}
	if err := json.Unmarshal(result.Body, &body); err != nil {
		diags.AddError("Invalid environment defaults", err.Error())
		return
	}
	values := map[string]attr.Value{}
	for name, wire := range environmentDefaultSlots {
		value, present := body.ResourceDefaults[wire]
		if !present {
			diags.AddError("Incomplete environment defaults", "The API omitted a requested default slot.")
			return
		}
		values[name] = types.StringValue("")
		if value != nil {
			values[name] = types.StringValue(*value)
		}
	}
	value, d := types.ObjectValue(environmentDefaultsType.AttrTypes, values)
	diags.Append(d...)
	if !diags.HasError() {
		data.EnvironmentDefaults = value
	}
}
func (r *OrganizationSettingsResource) listUserDefaultLimits(ctx context.Context, diags *diag.Diagnostics) (map[string]userDefaultLimitWire, int) {
	result, err := r.client.ListDefaultUserLimitsWithResponse(ctx)
	if err != nil {
		diags.AddError("Default user limits read failed", err.Error())
		return nil, 0
	}
	status := result.StatusCode()
	if status == 403 {
		return nil, status
	}
	if result.JSON200 == nil {
		diags.AddError("Default user limits read failed", fmt.Sprintf("API returned status %d.", status))
		return nil, status
	}
	var rows []userDefaultLimitWire
	if err := json.Unmarshal(result.Body, &rows); err != nil {
		diags.AddError("Invalid default user limits", err.Error())
		return nil, status
	}
	values := map[string]userDefaultLimitWire{}
	for _, row := range rows {
		key := "default"
		if row.EnvironmentID != nil {
			key = *row.EnvironmentID
		}
		if _, duplicate := values[key]; duplicate || row.ID == uuid.Nil {
			diags.AddError("Ambiguous default user limits", "The API returned duplicate environment slots or an invalid row identity.")
			return nil, status
		}
		values[key] = row
	}
	return values, status
}
func (r *OrganizationSettingsResource) readUserDefaultLimits(ctx context.Context, data *OrganizationSettingsResourceModel, diags *diag.Diagnostics) {
	rows, status := r.listUserDefaultLimits(ctx, diags)
	if status == 403 {
		if data.DefaultUserLimits.IsNull() || data.DefaultUserLimits.IsUnknown() {
			data.DefaultUserLimits = types.MapNull(userDefaultLimitType)
			return
		}
		diags.AddError("Default user limits read denied", "The previously managed defaults are no longer readable.")
		return
	}
	if diags.HasError() {
		return
	}
	values := map[string]userDefaultLimitModel{}
	for key, row := range rows {
		models := row.Models
		if models == nil {
			models = []string{}
		}
		list, d := types.ListValueFrom(ctx, types.StringType, models)
		diags.Append(d...)
		values[key] = userDefaultLimitModel{LimitValue: types.Int64Value(row.LimitValue), Models: list, CleanupInterval: types.StringValue(row.CleanupInterval)}
	}
	value, d := types.MapValueFrom(ctx, userDefaultLimitType, values)
	diags.Append(d...)
	if !diags.HasError() {
		data.DefaultUserLimits = value
	}
}

func (r *OrganizationSettingsResource) applyDefaults(ctx context.Context, plan, prior tftypes.Value, diags *diag.Diagnostics) {
	r.applyEnvironmentDefaults(ctx, lookupOrNull(plan, "environment_defaults"), lookupOrNull(prior, "environment_defaults"), diags)
	if diags.HasError() {
		return
	}
	desired, old := lookupOrNull(plan, "default_user_limits"), lookupOrNull(prior, "default_user_limits")
	if !desired.IsKnown() || desired.IsNull() || desired.Equal(old) {
		return
	}
	value, d := types.MapType{ElemType: userDefaultLimitType}.ValueFromTerraform(ctx, desired)
	if d != nil {
		diags.AddError("Invalid organization defaults", d.Error())
		return
	}
	if diags.HasError() {
		return
	}
	typed, ok := value.(types.Map)
	if !ok {
		diags.AddError("Invalid default user limits", "Expected a map of environment defaults.")
		return
	}
	var wanted map[string]userDefaultLimitModel
	diags.Append(typed.ElementsAs(ctx, &wanted, false)...)
	if !diags.HasError() {
		r.reconcileUserDefaultLimits(ctx, wanted, diags)
	}
}
func (r *OrganizationSettingsResource) applyEnvironmentDefaults(ctx context.Context, desired, prior tftypes.Value, diags *diag.Diagnostics) {
	if !desired.IsKnown() || desired.IsNull() || desired.Equal(prior) {
		return
	}
	var values map[string]tftypes.Value
	if err := desired.As(&values); err != nil {
		diags.AddError("Invalid environment defaults", err.Error())
		return
	}
	patch := map[string]any{}
	for name, wire := range environmentDefaultSlots {
		value := values[name]
		if !value.IsKnown() || value.IsNull() || value.Equal(lookupOrNull(prior, name)) {
			continue
		}
		var id string
		if err := value.As(&id); err != nil {
			diags.AddError("Invalid environment default", err.Error())
			return
		}
		patch[wire] = nil
		if id != "" {
			if !canonicalUUIDRegexp.MatchString(id) {
				diags.AddError("Invalid environment default", "Use a lowercase canonical environment UUID or an empty string.")
				return
			}
			patch[wire] = id
		}
	}
	if len(patch) == 0 {
		return
	}
	body, err := json.Marshal(patch)
	if err != nil {
		diags.AddError("Environment default encoding failed", err.Error())
		return
	}
	result, err := r.client.UpdateEnvironmentResourceDefaultsWithBodyWithResponse(ctx, "application/json", bytes.NewReader(body))
	if err != nil {
		diags.AddError("Environment defaults update failed", err.Error())
		return
	}
	if result.JSON200 == nil {
		diags.AddError("Environment defaults update failed", fmt.Sprintf("API returned status %d.", result.StatusCode()))
	}
}
func userDefaultLimitRequest(ctx context.Context, key string, want userDefaultLimitModel, diags *diag.Diagnostics) map[string]any {
	var models []string
	diags.Append(want.Models.ElementsAs(ctx, &models, false)...)
	if models == nil {
		models = []string{}
	}
	body := map[string]any{"environmentId": nil, "limitValue": want.LimitValue.ValueInt64(), "model": models, "cleanupInterval": want.CleanupInterval.ValueString()}
	if key != "default" {
		body["environmentId"] = key
	}
	return body
}
func (r *OrganizationSettingsResource) reconcileUserDefaultLimits(ctx context.Context, wanted map[string]userDefaultLimitModel, diags *diag.Diagnostics) {
	current, status := r.listUserDefaultLimits(ctx, diags)
	if status == 403 {
		diags.AddError("Default user limits update denied", "Existing defaults must be readable before replacing the complete set.")
		return
	}
	if diags.HasError() {
		return
	}
	keys := make([]string, 0, len(wanted))
	for key := range wanted {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		var existing *userDefaultLimitWire
		if currentRow, present := current[key]; present {
			existing = &currentRow
		}
		r.upsertUserDefaultLimit(ctx, key, wanted[key], existing, diags)
		if diags.HasError() {
			return
		}
	}

	// Apply desired defaults before removing extras; a failed upsert retains them.
	for key, extra := range current {
		if _, keep := wanted[key]; keep {
			continue
		}
		result, err := r.client.DeleteDefaultUserLimitWithResponse(ctx, extra.ID)
		if err != nil {
			diags.AddError("Default user limit removal failed", err.Error())
			return
		}
		if result.StatusCode() != 200 && result.StatusCode() != 204 && !IsNotFound(result) {
			diags.AddError("Default user limit removal failed", fmt.Sprintf("API returned status %d.", result.StatusCode()))
			return
		}
	}
	observed, status := r.listUserDefaultLimits(ctx, diags)
	if status != 200 || diags.HasError() {
		if !diags.HasError() {
			diags.AddError("Default user limits verification failed", "The updated defaults could not be read.")
		}
		return
	}
	if len(observed) != len(wanted) {
		diags.AddError("Default user limits verification failed", "The API did not persist the requested set of environments.")
		return
	}
	for key, want := range wanted {
		row, ok := observed[key]
		if !ok {
			diags.AddError("Default user limits verification failed", "An environment default is missing after the update.")
			return
		}
		actualModels := row.Models
		if actualModels == nil {
			actualModels = []string{}
		}
		actual, d := types.ListValueFrom(ctx, types.StringType, actualModels)
		diags.Append(d...)
		if row.LimitValue != want.LimitValue.ValueInt64() || row.CleanupInterval != want.CleanupInterval.ValueString() || !actual.Equal(want.Models) {
			diags.AddError("Default user limits verification failed", "An environment default differs from the requested spending settings.")
			return
		}
	}
}

func (r *OrganizationSettingsResource) upsertUserDefaultLimit(ctx context.Context, key string, want userDefaultLimitModel, existing *userDefaultLimitWire, diags *diag.Diagnostics) {
	payload := userDefaultLimitRequest(ctx, key, want, diags)
	if diags.HasError() {
		return
	}
	if existing != nil {
		delete(payload, "environmentId")
		if existing.LimitValue == want.LimitValue.ValueInt64() {
			delete(payload, "limitValue")
		}
		if existing.CleanupInterval == want.CleanupInterval.ValueString() {
			delete(payload, "cleanupInterval")
		}
		oldModels := existing.Models
		if oldModels == nil {
			oldModels = []string{}
		}
		previous, d := types.ListValueFrom(ctx, types.StringType, oldModels)
		diags.Append(d...)
		if previous.Equal(want.Models) {
			delete(payload, "model")
		}
		if diags.HasError() || len(payload) == 0 {
			return
		}
	}
	body, err := json.Marshal(payload)
	if err != nil {
		diags.AddError("Default user limit encoding failed", err.Error())
		return
	}
	if existing == nil {
		result, err := r.client.CreateDefaultUserLimitWithBodyWithResponse(ctx, "application/json", bytes.NewReader(body))
		if err != nil {
			diags.AddError("Default user limit creation failed", err.Error())
			return
		}
		if result.JSON200 == nil {
			diags.AddError("Default user limit creation failed", fmt.Sprintf("API returned status %d.", result.StatusCode()))
		}
		return
	}
	result, err := r.client.UpdateDefaultUserLimitWithBodyWithResponse(ctx, existing.ID, "application/json", bytes.NewReader(body))
	if err != nil {
		diags.AddError("Default user limit update failed", err.Error())
		return
	}
	if result.JSON200 == nil {
		diags.AddError("Default user limit update failed", fmt.Sprintf("API returned status %d.", result.StatusCode()))
	}
}
