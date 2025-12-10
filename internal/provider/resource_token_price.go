package provider

import (
	"context"
	"fmt"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &TokenPriceResource{}
var _ resource.ResourceWithImportState = &TokenPriceResource{}

func NewTokenPriceResource() resource.Resource {
	return &TokenPriceResource{}
}

// TokenPriceResource defines the resource implementation.
type TokenPriceResource struct {
	client *client.ClientWithResponses
}

// TokenPriceResourceModel describes the resource data model.
type TokenPriceResourceModel struct {
	ID                    types.String `tfsdk:"id"`
	LLMProvider           types.String `tfsdk:"llm_provider"`
	Model                 types.String `tfsdk:"model"`
	PricePerMillionInput  types.String `tfsdk:"price_per_million_input"`
	PricePerMillionOutput types.String `tfsdk:"price_per_million_output"`
}

func (r *TokenPriceResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_token_price"
}

func (r *TokenPriceResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages token pricing for LLM models in Archestra.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Token price identifier",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"llm_provider": schema.StringAttribute{
				MarkdownDescription: "LLM provider: openai, anthropic, or gemini",
				Required:            true,
				Validators: []validator.String{
					stringvalidator.OneOf("openai", "anthropic", "gemini"),
				},
			},
			"model": schema.StringAttribute{
				MarkdownDescription: "The model name",
				Required:            true,
			},
			"price_per_million_input": schema.StringAttribute{
				MarkdownDescription: "Price per million input tokens",
				Required:            true,
			},
			"price_per_million_output": schema.StringAttribute{
				MarkdownDescription: "Price per million output tokens",
				Required:            true,
			},
		},
	}
}

func (r *TokenPriceResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *TokenPriceResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data TokenPriceResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	requestBody := client.CreateTokenPriceJSONRequestBody{
		Provider:              client.SupportedProvidersInput(data.LLMProvider.ValueString()),
		Model:                 data.Model.ValueString(),
		PricePerMillionInput:  data.PricePerMillionInput.ValueString(),
		PricePerMillionOutput: data.PricePerMillionOutput.ValueString(),
	}

	apiResp, err := r.client.CreateTokenPriceWithResponse(ctx, requestBody)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to create token price, got error: %s", err))
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
	data.LLMProvider = types.StringValue(apiResp.JSON200.Provider)
	data.Model = types.StringValue(apiResp.JSON200.Model)
	data.PricePerMillionInput = types.StringValue(apiResp.JSON200.PricePerMillionInput)
	data.PricePerMillionOutput = types.StringValue(apiResp.JSON200.PricePerMillionOutput)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *TokenPriceResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data TokenPriceResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	id, err := uuid.Parse(data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid ID", fmt.Sprintf("Unable to parse token price ID: %s", err))
		return
	}

	apiResp, err := r.client.GetTokenPriceWithResponse(ctx, id)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to read token price, got error: %s", err))
		return
	}

	if apiResp.JSON404 != nil {
		resp.State.RemoveResource(ctx)
		return
	}

	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Expected 200 OK, got status %d", apiResp.StatusCode()),
		)
		return
	}

	data.LLMProvider = types.StringValue(apiResp.JSON200.Provider)
	data.Model = types.StringValue(apiResp.JSON200.Model)
	data.PricePerMillionInput = types.StringValue(apiResp.JSON200.PricePerMillionInput)
	data.PricePerMillionOutput = types.StringValue(apiResp.JSON200.PricePerMillionOutput)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *TokenPriceResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data TokenPriceResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	id, err := uuid.Parse(data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid ID", fmt.Sprintf("Unable to parse token price ID: %s", err))
		return
	}

	provider := client.SupportedProvidersInput(data.LLMProvider.ValueString())
	model := data.Model.ValueString()
	priceInput := data.PricePerMillionInput.ValueString()
	priceOutput := data.PricePerMillionOutput.ValueString()

	requestBody := client.UpdateTokenPriceJSONRequestBody{
		Provider:              &provider,
		Model:                 &model,
		PricePerMillionInput:  &priceInput,
		PricePerMillionOutput: &priceOutput,
	}

	apiResp, err := r.client.UpdateTokenPriceWithResponse(ctx, id, requestBody)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to update token price, got error: %s", err))
		return
	}

	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Expected 200 OK, got status %d", apiResp.StatusCode()),
		)
		return
	}

	data.LLMProvider = types.StringValue(apiResp.JSON200.Provider)
	data.Model = types.StringValue(apiResp.JSON200.Model)
	data.PricePerMillionInput = types.StringValue(apiResp.JSON200.PricePerMillionInput)
	data.PricePerMillionOutput = types.StringValue(apiResp.JSON200.PricePerMillionOutput)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *TokenPriceResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data TokenPriceResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	id, err := uuid.Parse(data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid ID", fmt.Sprintf("Unable to parse token price ID: %s", err))
		return
	}

	apiResp, err := r.client.DeleteTokenPriceWithResponse(ctx, id)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to delete token price, got error: %s", err))
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

func (r *TokenPriceResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
