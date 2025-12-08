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
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
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

// apiClientLimitationError is the standard error message for this resource.
// The optimization rules API endpoints are not yet available in the generated API client.
// This resource will be fully functional once the API client is regenerated with the
// Archestra backend running via 'make codegen-api-client'.
const apiClientLimitationError = "The optimization rules API endpoints are not yet available in the generated API client. " +
	"This resource will be fully functional after running 'make codegen-api-client' with the Archestra backend running. " +
	"Please see the provider documentation for instructions on regenerating the API client."

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

func (r *OptimizationRuleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	resp.Diagnostics.AddError("API Client Limitation", apiClientLimitationError)
}

func (r *OptimizationRuleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	resp.Diagnostics.AddError("API Client Limitation", apiClientLimitationError)
}

func (r *OptimizationRuleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError("API Client Limitation", apiClientLimitationError)
}

func (r *OptimizationRuleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	resp.Diagnostics.AddError("API Client Limitation", apiClientLimitationError)
}

func (r *OptimizationRuleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
