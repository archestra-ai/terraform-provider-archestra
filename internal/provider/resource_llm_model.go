package provider

import (
	"context"
	"fmt"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = &LlmModelResource{}
var _ resource.ResourceWithImportState = &LlmModelResource{}

func NewLlmModelResource() resource.Resource {
	return &LlmModelResource{}
}

type LlmModelResource struct {
	client *client.ClientWithResponses
}

type LlmModelResourceModel struct {
	ID                          types.String `tfsdk:"id"`
	ModelID                     types.String `tfsdk:"model_id"`
	Provider                    types.String `tfsdk:"llm_provider"`
	Description                 types.String `tfsdk:"description"`
	ContextLength               types.Int64  `tfsdk:"context_length"`
	CustomPricePerMillionInput  types.String `tfsdk:"custom_price_per_million_input"`
	CustomPricePerMillionOutput types.String `tfsdk:"custom_price_per_million_output"`
	Ignored                     types.Bool   `tfsdk:"ignored"`
	InputModalities             types.List   `tfsdk:"input_modalities"`
	OutputModalities            types.List   `tfsdk:"output_modalities"`
	PricePerMillionInput        types.String `tfsdk:"price_per_million_input"`
	PricePerMillionOutput       types.String `tfsdk:"price_per_million_output"`
	IsCustomPrice               types.Bool   `tfsdk:"is_custom_price"`
	PriceSource                 types.String `tfsdk:"price_source"`
}

func (r *LlmModelResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_llm_model"
}

func (r *LlmModelResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages an LLM model's custom pricing and settings in Archestra. " +
			"Models are discovered automatically from configured LLM provider API keys. " +
			"This resource adopts an existing model by `model_id` and allows customizing its pricing. " +
			"Destroying this resource only removes it from Terraform state — the model remains in Archestra.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Model UUID (internal identifier)",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"model_id": schema.StringAttribute{
				MarkdownDescription: "The model identifier (e.g., `gpt-4o`, `claude-sonnet-4-20250514`). Used to look up the model on create.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"llm_provider": schema.StringAttribute{
				MarkdownDescription: "The LLM provider (e.g., `openai`, `anthropic`)",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "Model description",
				Computed:            true,
			},
			"context_length": schema.Int64Attribute{
				MarkdownDescription: "Maximum context length in tokens",
				Computed:            true,
			},
			"custom_price_per_million_input": schema.StringAttribute{
				MarkdownDescription: "Custom price per million input tokens (overrides provider pricing)",
				Optional:            true,
			},
			"custom_price_per_million_output": schema.StringAttribute{
				MarkdownDescription: "Custom price per million output tokens (overrides provider pricing)",
				Optional:            true,
			},
			"ignored": schema.BoolAttribute{
				MarkdownDescription: "Whether the model is ignored (hidden from model selection)",
				Optional:            true,
				Computed:            true,
			},
			"input_modalities": schema.ListAttribute{
				MarkdownDescription: "Input modality overrides. Valid values: `text`, `image`, `audio`, `video`, `pdf`",
				Optional:            true,
				Computed:            true,
				ElementType:         types.StringType,
				PlanModifiers: []planmodifier.List{
					listplanmodifier.UseStateForUnknown(),
				},
			},
			"output_modalities": schema.ListAttribute{
				MarkdownDescription: "Output modality overrides. Valid values: `text`, `image`, `audio`",
				Optional:            true,
				Computed:            true,
				ElementType:         types.StringType,
				PlanModifiers: []planmodifier.List{
					listplanmodifier.UseStateForUnknown(),
				},
			},
			"price_per_million_input": schema.StringAttribute{
				MarkdownDescription: "Effective price per million input tokens (computed from custom or provider pricing)",
				Computed:            true,
			},
			"price_per_million_output": schema.StringAttribute{
				MarkdownDescription: "Effective price per million output tokens (computed from custom or provider pricing)",
				Computed:            true,
			},
			"is_custom_price": schema.BoolAttribute{
				MarkdownDescription: "Whether custom pricing is active",
				Computed:            true,
			},
			"price_source": schema.StringAttribute{
				MarkdownDescription: "Source of the current pricing: `custom`, `models_dev`, or `default`",
				Computed:            true,
			},
		},
	}
}

func (r *LlmModelResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *LlmModelResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data LlmModelResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Look up the model by model_id from the list of all models
	modelsResp, err := r.client.GetModelsWithApiKeysWithResponse(ctx)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to list models: %s", err))
		return
	}

	if modelsResp.JSON200 == nil {
		resp.Diagnostics.AddError("Unexpected API Response", fmt.Sprintf("Expected 200 OK, got status %d", modelsResp.StatusCode()))
		return
	}

	targetModelID := data.ModelID.ValueString()
	var foundID *uuid.UUID

	for _, model := range *modelsResp.JSON200 {
		if model.ModelId == targetModelID {
			id := model.Id
			foundID = &id
			break
		}
	}

	if foundID == nil {
		resp.Diagnostics.AddError(
			"Model Not Found",
			fmt.Sprintf("Model '%s' not found. Models are discovered from configured LLM provider API keys. Ensure the provider has been synced.", targetModelID),
		)
		return
	}

	data.ID = types.StringValue(foundID.String())

	// Apply custom pricing if set
	if !data.CustomPricePerMillionInput.IsNull() || !data.CustomPricePerMillionOutput.IsNull() || !data.Ignored.IsNull() || !data.InputModalities.IsNull() || !data.OutputModalities.IsNull() {
		updateBody := client.UpdateModelJSONRequestBody{}

		if !data.CustomPricePerMillionInput.IsNull() && !data.CustomPricePerMillionInput.IsUnknown() {
			v := data.CustomPricePerMillionInput.ValueString()
			updateBody.CustomPricePerMillionInput = &v
		}
		if !data.CustomPricePerMillionOutput.IsNull() && !data.CustomPricePerMillionOutput.IsUnknown() {
			v := data.CustomPricePerMillionOutput.ValueString()
			updateBody.CustomPricePerMillionOutput = &v
		}
		if !data.Ignored.IsNull() && !data.Ignored.IsUnknown() {
			v := data.Ignored.ValueBool()
			updateBody.Ignored = &v
		}
		if !data.InputModalities.IsNull() && !data.InputModalities.IsUnknown() {
			var vals []string
			data.InputModalities.ElementsAs(ctx, &vals, false)
			modalities := make([]client.UpdateModelJSONBodyInputModalities, len(vals))
			for i, v := range vals {
				modalities[i] = client.UpdateModelJSONBodyInputModalities(v)
			}
			updateBody.InputModalities = &modalities
		}
		if !data.OutputModalities.IsNull() && !data.OutputModalities.IsUnknown() {
			var vals []string
			data.OutputModalities.ElementsAs(ctx, &vals, false)
			modalities := make([]client.UpdateModelJSONBodyOutputModalities, len(vals))
			for i, v := range vals {
				modalities[i] = client.UpdateModelJSONBodyOutputModalities(v)
			}
			updateBody.OutputModalities = &modalities
		}

		updateResp, err := r.client.UpdateModelWithResponse(ctx, *foundID, updateBody)
		if err != nil {
			resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to update model pricing: %s", err))
			return
		}
		if updateResp.JSON200 == nil {
			resp.Diagnostics.AddError("Unexpected API Response", fmt.Sprintf("Expected 200 OK, got status %d: %s", updateResp.StatusCode(), string(updateResp.Body)))
			return
		}
	}

	// Read back full state
	r.readModelState(ctx, &data, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *LlmModelResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data LlmModelResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	r.readModelState(ctx, &data, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *LlmModelResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data LlmModelResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id, err := uuid.Parse(data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid ID", fmt.Sprintf("Unable to parse model ID: %s", err))
		return
	}

	updateBody := client.UpdateModelJSONRequestBody{}

	if !data.CustomPricePerMillionInput.IsNull() {
		v := data.CustomPricePerMillionInput.ValueString()
		updateBody.CustomPricePerMillionInput = &v
	}
	if !data.CustomPricePerMillionOutput.IsNull() {
		v := data.CustomPricePerMillionOutput.ValueString()
		updateBody.CustomPricePerMillionOutput = &v
	}
	if !data.Ignored.IsNull() {
		v := data.Ignored.ValueBool()
		updateBody.Ignored = &v
	}
	if !data.InputModalities.IsNull() && !data.InputModalities.IsUnknown() {
		var vals []string
		data.InputModalities.ElementsAs(ctx, &vals, false)
		modalities := make([]client.UpdateModelJSONBodyInputModalities, len(vals))
		for i, v := range vals {
			modalities[i] = client.UpdateModelJSONBodyInputModalities(v)
		}
		updateBody.InputModalities = &modalities
	}
	if !data.OutputModalities.IsNull() && !data.OutputModalities.IsUnknown() {
		var vals []string
		data.OutputModalities.ElementsAs(ctx, &vals, false)
		modalities := make([]client.UpdateModelJSONBodyOutputModalities, len(vals))
		for i, v := range vals {
			modalities[i] = client.UpdateModelJSONBodyOutputModalities(v)
		}
		updateBody.OutputModalities = &modalities
	}

	updateResp, err := r.client.UpdateModelWithResponse(ctx, id, updateBody)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to update model: %s", err))
		return
	}

	if updateResp.JSON200 == nil {
		resp.Diagnostics.AddError("Unexpected API Response", fmt.Sprintf("Expected 200 OK, got status %d: %s", updateResp.StatusCode(), string(updateResp.Body)))
		return
	}

	r.readModelState(ctx, &data, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *LlmModelResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Models cannot be deleted — they are discovered from LLM providers.
	// Removing from Terraform state only.

	var data LlmModelResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Clear custom pricing on destroy so the model reverts to provider defaults
	id, err := uuid.Parse(data.ID.ValueString())
	if err != nil {
		return
	}

	updateBody := client.UpdateModelJSONRequestBody{}
	// Send null to clear custom pricing
	updateBody.CustomPricePerMillionInput = nil
	updateBody.CustomPricePerMillionOutput = nil
	falseVal := false
	updateBody.Ignored = &falseVal

	_, _ = r.client.UpdateModelWithResponse(ctx, id, updateBody)
}

func (r *LlmModelResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Import by model_id string (e.g., "gpt-4o"), not UUID
	modelID := req.ID

	modelsResp, err := r.client.GetModelsWithApiKeysWithResponse(ctx)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to list models: %s", err))
		return
	}

	if modelsResp.JSON200 == nil {
		resp.Diagnostics.AddError("Unexpected API Response", fmt.Sprintf("Expected 200 OK, got status %d", modelsResp.StatusCode()))
		return
	}

	for _, model := range *modelsResp.JSON200 {
		if model.ModelId == modelID || model.Id.String() == modelID {
			resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), model.Id.String())...)
			resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("model_id"), model.ModelId)...)
			return
		}
	}

	resp.Diagnostics.AddError("Model Not Found", fmt.Sprintf("Model '%s' not found", modelID))
}

func (r *LlmModelResource) readModelState(ctx context.Context, data *LlmModelResourceModel, diags *diag.Diagnostics) {
	modelsResp, err := r.client.GetModelsWithApiKeysWithResponse(ctx)
	if err != nil {
		diags.AddError("API Error", fmt.Sprintf("Unable to read models: %s", err))
		return
	}

	if modelsResp.JSON200 == nil {
		diags.AddError("Unexpected API Response", fmt.Sprintf("Expected 200 OK, got status %d", modelsResp.StatusCode()))
		return
	}

	targetID := data.ID.ValueString()

	for _, model := range *modelsResp.JSON200 {
		if model.Id.String() == targetID {
			data.ModelID = types.StringValue(model.ModelId)
			data.Provider = types.StringValue(string(model.Provider))
			data.Ignored = types.BoolValue(model.Ignored)
			data.IsCustomPrice = types.BoolValue(model.IsCustomPrice)
			data.PriceSource = types.StringValue(string(model.PriceSource))

			if model.Description != nil {
				data.Description = types.StringValue(*model.Description)
			} else {
				data.Description = types.StringNull()
			}

			if model.ContextLength != nil {
				data.ContextLength = types.Int64Value(int64(*model.ContextLength))
			} else {
				data.ContextLength = types.Int64Null()
			}

			if model.InputModalities != nil {
				vals := make([]string, len(*model.InputModalities))
				for i, v := range *model.InputModalities {
					vals[i] = string(v)
				}
				listVal, diag := types.ListValueFrom(ctx, types.StringType, vals)
				diags.Append(diag...)
				data.InputModalities = listVal
			} else {
				data.InputModalities = types.ListNull(types.StringType)
			}

			if model.OutputModalities != nil {
				vals := make([]string, len(*model.OutputModalities))
				for i, v := range *model.OutputModalities {
					vals[i] = string(v)
				}
				listVal, diag := types.ListValueFrom(ctx, types.StringType, vals)
				diags.Append(diag...)
				data.OutputModalities = listVal
			} else {
				data.OutputModalities = types.ListNull(types.StringType)
			}

			if model.CustomPricePerMillionInput != nil {
				data.CustomPricePerMillionInput = types.StringValue(*model.CustomPricePerMillionInput)
			} else {
				data.CustomPricePerMillionInput = types.StringNull()
			}

			if model.CustomPricePerMillionOutput != nil {
				data.CustomPricePerMillionOutput = types.StringValue(*model.CustomPricePerMillionOutput)
			} else {
				data.CustomPricePerMillionOutput = types.StringNull()
			}

			if model.PricePerMillionInput != nil {
				data.PricePerMillionInput = types.StringValue(*model.PricePerMillionInput)
			} else {
				data.PricePerMillionInput = types.StringNull()
			}

			if model.PricePerMillionOutput != nil {
				data.PricePerMillionOutput = types.StringValue(*model.PricePerMillionOutput)
			} else {
				data.PricePerMillionOutput = types.StringNull()
			}

			return
		}
	}

	diags.AddError("Model Not Found", fmt.Sprintf("Model with ID %s no longer exists", targetID))
}
