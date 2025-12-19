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
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = &ChatLLMProviderApiKeyResource{}
var _ resource.ResourceWithImportState = &ChatLLMProviderApiKeyResource{}

func NewChatLLMProviderApiKeyResource() resource.Resource {
	return &ChatLLMProviderApiKeyResource{}
}

type ChatLLMProviderApiKeyResource struct {
	client *client.ClientWithResponses
}

type ChatLLMProviderApiKeyResourceModel struct {
	ID                    types.String `tfsdk:"id"`
	Name                  types.String `tfsdk:"name"`
	ApiKey                types.String `tfsdk:"api_key"`
	LLMProvider           types.String `tfsdk:"llm_provider"`
	IsOrganizationDefault types.Bool   `tfsdk:"is_organization_default"`
}

func (r *ChatLLMProviderApiKeyResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_chat_llm_provider_api_key"
}

func (r *ChatLLMProviderApiKeyResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages Chat LLM Provider API keys in Archestra.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Chat LLM Provider API key identifier",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Name of the API key",
				Required:            true,
			},
			"api_key": schema.StringAttribute{
				MarkdownDescription: "The API key value",
				Required:            true,
				Sensitive:           true,
			},
			"llm_provider": schema.StringAttribute{
				MarkdownDescription: "LLM provider for this API key",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					stringvalidator.OneOf(
						string(client.CreateChatApiKeyJSONBodyProviderAnthropic),
						string(client.CreateChatApiKeyJSONBodyProviderGemini),
						string(client.CreateChatApiKeyJSONBodyProviderOpenai),
					),
				},
			},
			"is_organization_default": schema.BoolAttribute{
				MarkdownDescription: "Whether this API key is the organization default for the provider",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
			},
		},
	}
}

func (r *ChatLLMProviderApiKeyResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *ChatLLMProviderApiKeyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data ChatLLMProviderApiKeyResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	isDefault := data.IsOrganizationDefault.ValueBool()
	requestBody := client.CreateChatApiKeyJSONRequestBody{
		Name:                  data.Name.ValueString(),
		ApiKey:                data.ApiKey.ValueString(),
		Provider:              client.CreateChatApiKeyJSONBodyProvider(data.LLMProvider.ValueString()),
		IsOrganizationDefault: &isDefault,
	}

	apiResp, err := r.client.CreateChatApiKeyWithResponse(ctx, requestBody)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to create chat LLM provider API key, got error: %s", err))
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
	data.Name = types.StringValue(apiResp.JSON200.Name)
	data.LLMProvider = types.StringValue(string(apiResp.JSON200.Provider))
	data.IsOrganizationDefault = types.BoolValue(apiResp.JSON200.IsOrganizationDefault)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ChatLLMProviderApiKeyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data ChatLLMProviderApiKeyResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	id, err := uuid.Parse(data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid ID", fmt.Sprintf("Unable to parse chat LLM provider API key ID: %s", err))
		return
	}

	apiResp, err := r.client.GetChatApiKeyWithResponse(ctx, id)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to read chat LLM provider API key, got error: %s", err))
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

	data.Name = types.StringValue(apiResp.JSON200.Name)
	data.LLMProvider = types.StringValue(string(apiResp.JSON200.Provider))
	data.IsOrganizationDefault = types.BoolValue(apiResp.JSON200.IsOrganizationDefault)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ChatLLMProviderApiKeyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data ChatLLMProviderApiKeyResourceModel
	var state ChatLLMProviderApiKeyResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)

	if resp.Diagnostics.HasError() {
		return
	}

	id, err := uuid.Parse(data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid ID", fmt.Sprintf("Unable to parse chat LLM provider API key ID: %s", err))
		return
	}

	name := data.Name.ValueString()
	apiKey := data.ApiKey.ValueString()
	requestBody := client.UpdateChatApiKeyJSONRequestBody{
		Name:   &name,
		ApiKey: &apiKey,
	}

	apiResp, err := r.client.UpdateChatApiKeyWithResponse(ctx, id, requestBody)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to update chat LLM provider API key, got error: %s", err))
		return
	}

	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Expected 200 OK, got status %d: %s", apiResp.StatusCode(), string(apiResp.Body)),
		)
		return
	}

	if data.IsOrganizationDefault.ValueBool() != state.IsOrganizationDefault.ValueBool() {
		if data.IsOrganizationDefault.ValueBool() {
			defaultResp, err := r.client.SetChatApiKeyDefaultWithResponse(ctx, id)
			if err != nil {
				resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to set chat LLM provider API key as default, got error: %s", err))
				return
			}
			if defaultResp.JSON200 == nil {
				resp.Diagnostics.AddError(
					"Unexpected API Response",
					fmt.Sprintf("Expected 200 OK when setting default, got status %d: %s", defaultResp.StatusCode(), string(defaultResp.Body)),
				)
				return
			}
		} else {
			defaultResp, err := r.client.UnsetChatApiKeyDefaultWithResponse(ctx, id)
			if err != nil {
				resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to unset chat LLM provider API key as default, got error: %s", err))
				return
			}
			if defaultResp.JSON200 == nil {
				resp.Diagnostics.AddError(
					"Unexpected API Response",
					fmt.Sprintf("Expected 200 OK when unsetting default, got status %d: %s", defaultResp.StatusCode(), string(defaultResp.Body)),
				)
				return
			}
		}
	}

	readResp, err := r.client.GetChatApiKeyWithResponse(ctx, id)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to read chat LLM provider API key after update, got error: %s", err))
		return
	}

	if readResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Expected 200 OK on read after update, got status %d", readResp.StatusCode()),
		)
		return
	}

	data.Name = types.StringValue(readResp.JSON200.Name)
	data.LLMProvider = types.StringValue(string(readResp.JSON200.Provider))
	data.IsOrganizationDefault = types.BoolValue(readResp.JSON200.IsOrganizationDefault)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ChatLLMProviderApiKeyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data ChatLLMProviderApiKeyResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	id, err := uuid.Parse(data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid ID", fmt.Sprintf("Unable to parse chat LLM provider API key ID: %s", err))
		return
	}

	apiResp, err := r.client.DeleteChatApiKeyWithResponse(ctx, id)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to delete chat LLM provider API key, got error: %s", err))
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

func (r *ChatLLMProviderApiKeyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
