package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &OptimizationRuleResource{}
var _ resource.ResourceWithImportState = &OptimizationRuleResource{}

func NewOptimizationRuleResource() resource.Resource {
	return &OptimizationRuleResource{}
}

// OptimizationRuleResource defines the resource implementation.
type OptimizationRuleResource struct {
	client *client.ClientWithResponses
}

// OptimizationRuleConditionModel represents a single condition.
type OptimizationRuleConditionModel struct {
	MaxLength types.Int64 `tfsdk:"max_length"`
	HasTools  types.Bool  `tfsdk:"has_tools"`
}

// OptimizationRuleResourceModel describes the resource data model.
type OptimizationRuleResourceModel struct {
	ID          types.String `tfsdk:"id"`
	EntityType  types.String `tfsdk:"entity_type"`
	EntityID    types.String `tfsdk:"entity_id"`
	LLMProvider types.String `tfsdk:"llm_provider"`
	TargetModel types.String `tfsdk:"target_model"`
	Enabled     types.Bool   `tfsdk:"enabled"`
	Conditions  types.List   `tfsdk:"conditions"`
}

func (r *OptimizationRuleResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_optimization_rule"
}

func (r *OptimizationRuleResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages cost optimization rules in Archestra.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Optimization rule identifier",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"entity_type": schema.StringAttribute{
				MarkdownDescription: "Entity type: organization, team, or agent",
				Required:            true,
				Validators: []validator.String{
					stringvalidator.OneOf("organization", "team", "agent"),
				},
			},
			"entity_id": schema.StringAttribute{
				MarkdownDescription: "Entity ID this rule applies to",
				Required:            true,
			},
			"llm_provider": schema.StringAttribute{
				MarkdownDescription: "LLM provider: openai, anthropic, or gemini",
				Required:            true,
				Validators: []validator.String{
					stringvalidator.OneOf("openai", "anthropic", "gemini"),
				},
			},
			"target_model": schema.StringAttribute{
				MarkdownDescription: "Target model to switch to",
				Required:            true,
			},
			"enabled": schema.BoolAttribute{
				MarkdownDescription: "Whether the rule is enabled",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(true),
			},
			"conditions": schema.ListNestedAttribute{
				MarkdownDescription: "Conditions that trigger the optimization",
				Required:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"max_length": schema.Int64Attribute{
							MarkdownDescription: "Maximum token length threshold",
							Optional:            true,
						},
						"has_tools": schema.BoolAttribute{
							MarkdownDescription: "Whether tools are present",
							Optional:            true,
						},
					},
				},
			},
		},
	}
}

func (r *OptimizationRuleResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// buildConditionsJSON converts Terraform conditions to a slice of JSON-serializable maps.
func buildConditionsJSON(ctx context.Context, conditionsList types.List) ([]map[string]interface{}, diag.Diagnostics) {
	var diags diag.Diagnostics
	var conditions []OptimizationRuleConditionModel

	diags.Append(conditionsList.ElementsAs(ctx, &conditions, false)...)
	if diags.HasError() {
		return nil, diags
	}

	apiConditions := make([]map[string]interface{}, 0, len(conditions))
	for _, cond := range conditions {
		if !cond.MaxLength.IsNull() && !cond.MaxLength.IsUnknown() {
			apiConditions = append(apiConditions, map[string]interface{}{
				"maxLength": cond.MaxLength.ValueInt64(),
			})
		}

		if !cond.HasTools.IsNull() && !cond.HasTools.IsUnknown() {
			apiConditions = append(apiConditions, map[string]interface{}{
				"hasTools": cond.HasTools.ValueBool(),
			})
		}
	}

	return apiConditions, diags
}

func (r *OptimizationRuleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data OptimizationRuleResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	apiConditions, diags := buildConditionsJSON(ctx, data.Conditions)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	enabled := data.Enabled.ValueBool()
	requestBody := map[string]interface{}{
		"entityId":    data.EntityID.ValueString(),
		"entityType":  data.EntityType.ValueString(),
		"provider":    data.LLMProvider.ValueString(),
		"targetModel": data.TargetModel.ValueString(),
		"enabled":     enabled,
		"conditions":  apiConditions,
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		resp.Diagnostics.AddError("Marshal Error", fmt.Sprintf("Unable to marshal request body: %s", err))
		return
	}

	apiResp, err := r.client.CreateOptimizationRuleWithBodyWithResponse(ctx, "application/json", bytes.NewReader(jsonBody))
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to create optimization rule, got error: %s", err))
		return
	}

	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Expected 200 OK, got status %d: %s", apiResp.StatusCode(), string(apiResp.Body)),
		)
		return
	}

	data.ID = types.StringValue(apiResp.JSON200.Id.String())
	data.EntityID = types.StringValue(apiResp.JSON200.EntityId)
	data.EntityType = types.StringValue(string(apiResp.JSON200.EntityType))
	data.LLMProvider = types.StringValue(string(apiResp.JSON200.Provider))
	data.TargetModel = types.StringValue(apiResp.JSON200.TargetModel)
	data.Enabled = types.BoolValue(apiResp.JSON200.Enabled)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *OptimizationRuleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data OptimizationRuleResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	ruleID := data.ID.ValueString()

	// The API only has GetOptimizationRules (list), not GetOptimizationRule (single).
	// We need to list all rules and find the one matching our ID.
	// Use retry logic for eventual consistency - the rule may not appear immediately after creation.
	retryConfig := DefaultRetryConfig(fmt.Sprintf("Optimization rule %s", ruleID))

	// optimizationRuleResult holds the extracted data we need from the API response
	type optimizationRuleResult struct {
		EntityID    string
		EntityType  string
		Provider    string
		TargetModel string
		Enabled     bool
	}

	result, found, err := RetryUntilFound(ctx, retryConfig, func() (optimizationRuleResult, bool, error) {
		apiResp, err := r.client.GetOptimizationRulesWithResponse(ctx)
		if err != nil {
			return optimizationRuleResult{}, false, fmt.Errorf("unable to read optimization rules: %w", err)
		}

		if apiResp.JSON200 == nil {
			return optimizationRuleResult{}, false, fmt.Errorf("expected 200 OK, got status %d: %s", apiResp.StatusCode(), string(apiResp.Body))
		}

		rules := *apiResp.JSON200
		tflog.Debug(ctx, fmt.Sprintf("Looking for rule %s in %d rules returned by API", ruleID, len(rules)))
		for _, rule := range rules {
			tflog.Debug(ctx, fmt.Sprintf("Found rule: %s (entity: %s, type: %s)", rule.Id.String(), rule.EntityId, rule.EntityType))
		}

		// Find the rule with matching ID
		for _, rule := range rules {
			if rule.Id.String() == ruleID {
				return optimizationRuleResult{
					EntityID:    rule.EntityId,
					EntityType:  string(rule.EntityType),
					Provider:    string(rule.Provider),
					TargetModel: rule.TargetModel,
					Enabled:     rule.Enabled,
				}, true, nil
			}
		}

		return optimizationRuleResult{}, false, nil
	})

	if err != nil {
		resp.Diagnostics.AddError("API Error", err.Error())
		return
	}

	if !found {
		tflog.Warn(ctx, fmt.Sprintf("Rule %s not found in API response after retries, removing from state", ruleID))
		resp.State.RemoveResource(ctx)
		return
	}

	data.EntityID = types.StringValue(result.EntityID)
	data.EntityType = types.StringValue(result.EntityType)
	data.LLMProvider = types.StringValue(result.Provider)
	data.TargetModel = types.StringValue(result.TargetModel)
	data.Enabled = types.BoolValue(result.Enabled)
	// Keep existing conditions since we can't easily parse the union type back

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *OptimizationRuleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data OptimizationRuleResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	id, err := uuid.Parse(data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid ID", fmt.Sprintf("Unable to parse optimization rule ID: %s", err))
		return
	}

	apiConditions, diags := buildConditionsJSON(ctx, data.Conditions)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	requestBody := map[string]interface{}{
		"entityId":    data.EntityID.ValueString(),
		"entityType":  data.EntityType.ValueString(),
		"provider":    data.LLMProvider.ValueString(),
		"targetModel": data.TargetModel.ValueString(),
		"enabled":     data.Enabled.ValueBool(),
		"conditions":  apiConditions,
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		resp.Diagnostics.AddError("Marshal Error", fmt.Sprintf("Unable to marshal request body: %s", err))
		return
	}

	apiResp, err := r.client.UpdateOptimizationRuleWithBodyWithResponse(ctx, id, "application/json", bytes.NewReader(jsonBody))
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to update optimization rule, got error: %s", err))
		return
	}

	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Expected 200 OK, got status %d", apiResp.StatusCode()),
		)
		return
	}

	data.EntityID = types.StringValue(apiResp.JSON200.EntityId)
	data.EntityType = types.StringValue(string(apiResp.JSON200.EntityType))
	data.LLMProvider = types.StringValue(string(apiResp.JSON200.Provider))
	data.TargetModel = types.StringValue(apiResp.JSON200.TargetModel)
	data.Enabled = types.BoolValue(apiResp.JSON200.Enabled)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *OptimizationRuleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data OptimizationRuleResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	id, err := uuid.Parse(data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid ID", fmt.Sprintf("Unable to parse optimization rule ID: %s", err))
		return
	}

	apiResp, err := r.client.DeleteOptimizationRuleWithResponse(ctx, id)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to delete optimization rule, got error: %s", err))
		return
	}

	if apiResp.JSON200 == nil && apiResp.JSON404 == nil {
		resp.Diagnostics.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Expected 200 OK or 404 Not Found, got status %d", apiResp.StatusCode()),
		)
		return
	}
}

func (r *OptimizationRuleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
