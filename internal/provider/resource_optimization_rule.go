package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
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
			// TODO(backend): the OneOf below only validates that the value
			// is in the platform's accepted enum — it doesn't check that
			// the user actually has a configured key for that provider.
			// A rule for `anthropic` on a backend with zero anthropic
			// credentials creates successfully and fails silently at
			// LLM-call time. Right fix is in the platform repo: reject
			// `POST /optimization-rules` with a 4xx when no provider key
			// exists for the requested provider. A provider-side
			// ModifyPlan pre-flight (mirroring resource_team.go's TOON
			// guard) is the alternative if the backend can't change.
			// Until either lands, the MarkdownDescription warns users
			// that mismatches surface at runtime, not at apply.
			"llm_provider": schema.StringAttribute{
				MarkdownDescription: "LLM provider this rule routes against. Must match a provider you have configured via `archestra_llm_provider_api_key.llm_provider` — the same 17-provider enum the backend accepts (anthropic, azure, bedrock, cerebras, cohere, deepseek, gemini, groq, minimax, mistral, ollama, openai, openrouter, perplexity, vllm, xai, zhipuai). The provider does **not** verify a key exists for the value you set; mismatches surface at LLM-call time, not at apply.",
				Required:            true,
				Validators: []validator.String{
					stringvalidator.OneOf(
						string(client.CreateOptimizationRuleJSONBodyProviderAnthropic),
						string(client.CreateOptimizationRuleJSONBodyProviderAzure),
						string(client.CreateOptimizationRuleJSONBodyProviderBedrock),
						string(client.CreateOptimizationRuleJSONBodyProviderCerebras),
						string(client.CreateOptimizationRuleJSONBodyProviderCohere),
						string(client.CreateOptimizationRuleJSONBodyProviderDeepseek),
						string(client.CreateOptimizationRuleJSONBodyProviderGemini),
						string(client.CreateOptimizationRuleJSONBodyProviderGroq),
						string(client.CreateOptimizationRuleJSONBodyProviderMinimax),
						string(client.CreateOptimizationRuleJSONBodyProviderMistral),
						string(client.CreateOptimizationRuleJSONBodyProviderOllama),
						string(client.CreateOptimizationRuleJSONBodyProviderOpenai),
						string(client.CreateOptimizationRuleJSONBodyProviderOpenrouter),
						string(client.CreateOptimizationRuleJSONBodyProviderPerplexity),
						string(client.CreateOptimizationRuleJSONBodyProviderVllm),
						string(client.CreateOptimizationRuleJSONBodyProviderXai),
						string(client.CreateOptimizationRuleJSONBodyProviderZhipuai),
					),
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
							MarkdownDescription: "Maximum token length threshold. Must be at least 1.",
							Optional:            true,
							Validators: []validator.Int64{
								int64validator.AtLeast(1),
							},
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

func (r *OptimizationRuleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data OptimizationRuleResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	prior := tftypes.NewValue(req.Plan.Raw.Type(), nil)
	patch := MergePatch(ctx, req.Plan.Raw, prior, optimizationRuleAttrSpec, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	LogPatch(ctx, "archestra_optimization_rule Create", patch, optimizationRuleAttrSpec)

	jsonBody, err := json.Marshal(patch)
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

	// TODO(backend): expose `GET /api/optimization-rules/{id}` so Read
	// can fetch a single rule by ID instead of listing-and-filtering.
	// Non-idempotent reads have been reported in some self-hosted
	// environments (apply succeeds, then plan refresh removes the rule
	// from state), with three plausible causes none reproduced in CI:
	// (1) the list endpoint returns a scoped subset that excludes some
	// entity_type scopes; (2) implicit pagination caps the response;
	// (3) backend create-vs-list eventual-consistency exceeds the
	// 20-retry / ~80s budget below. The not-found warning at the
	// bottom of this function captures last list size + sample IDs so
	// the next failure tells us which it is. Once the GET-by-id
	// endpoint lands, drop the list-and-filter loop entirely.
	retryConfig := DefaultRetryConfig(fmt.Sprintf("Optimization rule %s", ruleID))

	type optimizationRuleResult struct {
		EntityID      string
		EntityType    string
		Provider      string
		TargetModel   string
		Enabled       bool
		RawConditions json.RawMessage
	}
	// returnedIDs captures the IDs the last List call observed, so the
	// not-found diagnostic below can show whether the list was empty
	// (likely scoping/pagination) or non-empty without our ID (likely
	// case mismatch / backend bug).
	var returnedIDs []string
	var lastListSize int

	// The generated `Conditions []GetOptimizationRules_200_Conditions_Item`
	// type wraps the union members in an unexported `json.RawMessage`, so
	// re-marshaling the typed value produces `[{}, {}]`. We parse the raw
	// HTTP body directly to recover the discriminated entries.
	type rawRule struct {
		Id         string            `json:"id"`
		Conditions []json.RawMessage `json:"conditions"`
	}

	result, found, err := RetryUntilFound(ctx, retryConfig, func() (optimizationRuleResult, bool, error) {
		apiResp, err := r.client.GetOptimizationRulesWithResponse(ctx)
		if err != nil {
			return optimizationRuleResult{}, false, fmt.Errorf("unable to read optimization rules: %w", err)
		}

		if apiResp.JSON200 == nil {
			return optimizationRuleResult{}, false, fmt.Errorf("expected 200 OK, got status %d: %s", apiResp.StatusCode(), string(apiResp.Body))
		}

		var rawRules []rawRule
		if err := json.Unmarshal(apiResp.Body, &rawRules); err != nil {
			return optimizationRuleResult{}, false, fmt.Errorf("parse raw conditions: %w", err)
		}
		rawByID := make(map[string]json.RawMessage, len(rawRules))
		for _, rr := range rawRules {
			b, _ := json.Marshal(rr.Conditions)
			rawByID[rr.Id] = b
		}

		rules := *apiResp.JSON200
		lastListSize = len(rules)
		returnedIDs = returnedIDs[:0]
		for _, rule := range rules {
			returnedIDs = append(returnedIDs, rule.Id.String())
		}
		tflog.Debug(ctx, fmt.Sprintf("Looking for rule %s in %d rules returned by API", ruleID, len(rules)))

		for _, rule := range rules {
			if rule.Id.String() == ruleID {
				return optimizationRuleResult{
					EntityID:      rule.EntityId,
					EntityType:    string(rule.EntityType),
					Provider:      string(rule.Provider),
					TargetModel:   rule.TargetModel,
					Enabled:       rule.Enabled,
					RawConditions: rawByID[ruleID],
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
		sample := returnedIDs
		if len(sample) > 5 {
			sample = sample[:5]
		}
		// Keep the state intact rather than RemoveResource: list-absence
		// is a weak signal (the list endpoint may be scoped or paginated,
		// see TODO above), and removing-then-recreating would create an
		// orphan row on every apply when the rule actually still exists.
		// A plan-time warning is loud enough that users notice; if the
		// rule was legitimately deleted out-of-band, `terraform state rm`
		// is the explicit recovery path.
		resp.Diagnostics.AddWarning(
			"Optimization rule not found in list response",
			fmt.Sprintf(
				"Rule %s wasn't returned by GET /api/optimization-rules after %d retries "+
					"(last list size=%d, sample IDs=%v). State is preserved — recreating would "+
					"orphan the existing backend row. If the rule still exists (run `terraform "+
					"state show <addr>` and check the UI), this is the known non-idempotent-Read "+
					"bug; the platform fix is `GET /api/optimization-rules/{id}` (see TODO in "+
					"resource_optimization_rule.go Read). If the rule was deleted out-of-band, "+
					"run `terraform state rm <addr>` to drop it from state, then re-apply.",
				ruleID, retryConfig.MaxRetries, lastListSize, sample,
			),
		)
		return
	}

	data.EntityID = types.StringValue(result.EntityID)
	data.EntityType = types.StringValue(result.EntityType)
	data.LLMProvider = types.StringValue(result.Provider)
	data.TargetModel = types.StringValue(result.TargetModel)
	data.Enabled = types.BoolValue(result.Enabled)

	condList, condDiags := flattenOptimizationConditions(result.RawConditions)
	resp.Diagnostics.Append(condDiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	data.Conditions = condList

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// flattenOptimizationConditions parses the wire union back into the HCL list.
// Each wire entry has exactly one of `maxLength` / `hasTools` (per the backend
// zod union — see platform/backend/src/types/optimization-rule.ts:14-24); we
// produce one HCL row per wire entry with the matching field set and the
// other field null.
func flattenOptimizationConditions(raw json.RawMessage) (types.List, diag.Diagnostics) {
	objType := types.ObjectType{AttrTypes: map[string]attr.Type{
		"max_length": types.Int64Type,
		"has_tools":  types.BoolType,
	}}
	if len(raw) == 0 {
		return types.ListNull(objType), nil
	}
	var items []map[string]json.RawMessage
	if err := json.Unmarshal(raw, &items); err != nil {
		var diags diag.Diagnostics
		diags.AddError("Failed to parse optimization rule conditions", err.Error())
		return types.ListNull(objType), diags
	}

	values := make([]attr.Value, 0, len(items))
	for _, item := range items {
		fields := map[string]attr.Value{
			"max_length": types.Int64Null(),
			"has_tools":  types.BoolNull(),
		}
		if rawN, ok := item["maxLength"]; ok {
			var n int64
			if err := json.Unmarshal(rawN, &n); err == nil {
				fields["max_length"] = types.Int64Value(n)
			}
		}
		if rawB, ok := item["hasTools"]; ok {
			var b bool
			if err := json.Unmarshal(rawB, &b); err == nil {
				fields["has_tools"] = types.BoolValue(b)
			}
		}
		obj, _ := types.ObjectValue(objType.AttrTypes, fields)
		values = append(values, obj)
	}
	out, diags := types.ListValue(objType, values)
	return out, diags
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

	patch := MergePatch(ctx, req.Plan.Raw, req.State.Raw, optimizationRuleAttrSpec, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	LogPatch(ctx, "archestra_optimization_rule Update", patch, optimizationRuleAttrSpec)

	jsonBody, err := json.Marshal(patch)
	if err != nil {
		resp.Diagnostics.AddError("Marshal Error", fmt.Sprintf("Unable to marshal request body: %s", err))
		return
	}

	apiResp, err := r.client.UpdateOptimizationRuleWithBodyWithResponse(ctx, id, "application/json", bytes.NewReader(jsonBody))
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to update optimization rule, got error: %s", err))
		return
	}
	if IsNotFound(apiResp) {
		resp.Diagnostics.AddError(
			"Resource Deleted Outside Terraform",
			"The resource was deleted on the backend between refresh and apply. "+
				"Re-run `terraform apply` — the next refresh drops it from state and the plan recreates it.",
		)
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
