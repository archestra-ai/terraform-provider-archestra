package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/google/uuid"
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
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

var _ resource.Resource = &LLMProviderApiKeyResource{}
var _ resource.ResourceWithImportState = &LLMProviderApiKeyResource{}

func NewLLMProviderApiKeyResource() resource.Resource {
	return &LLMProviderApiKeyResource{}
}

type LLMProviderApiKeyResource struct {
	client *client.ClientWithResponses
}

type LLMProviderApiKeyResourceModel struct {
	ID                    types.String `tfsdk:"id"`
	Name                  types.String `tfsdk:"name"`
	ApiKey                types.String `tfsdk:"api_key"`
	LLMProvider           types.String `tfsdk:"llm_provider"`
	IsOrganizationDefault types.Bool   `tfsdk:"is_organization_default"`
	BaseUrl               types.String `tfsdk:"base_url"`
	Scope                 types.String `tfsdk:"scope"`
	TeamID                types.String `tfsdk:"team_id"`
	VaultSecretPath       types.String `tfsdk:"vault_secret_path"`
	VaultSecretKey        types.String `tfsdk:"vault_secret_key"`
}

func (r *LLMProviderApiKeyResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_llm_provider_api_key"
}

func (r *LLMProviderApiKeyResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "API key for an LLM provider (OpenAI, Anthropic, etc.). Models are auto-discovered after creation.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "LLM Provider API key identifier",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Name of the API key",
				Required:            true,
			},
			"api_key": schema.StringAttribute{
				MarkdownDescription: "The API key value. Mutually exclusive with `vault_secret_path`/`vault_secret_key`. In BYOS (READONLY_VAULT) mode the backend requires the vault pair and rejects inline `api_key`.",
				Optional:            true,
				Sensitive:           true,
				Validators: []validator.String{
					stringvalidator.ConflictsWith(
						path.MatchRoot("vault_secret_path"),
						path.MatchRoot("vault_secret_key"),
					),
				},
			},
			"llm_provider": schema.StringAttribute{
				MarkdownDescription: "LLM provider for this API key",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					stringvalidator.OneOf(
						string(client.CreateLlmProviderApiKeyJSONBodyProviderAnthropic),
						string(client.CreateLlmProviderApiKeyJSONBodyProviderAzure),
						string(client.CreateLlmProviderApiKeyJSONBodyProviderBedrock),
						string(client.CreateLlmProviderApiKeyJSONBodyProviderCerebras),
						string(client.CreateLlmProviderApiKeyJSONBodyProviderCohere),
						string(client.CreateLlmProviderApiKeyJSONBodyProviderDeepseek),
						string(client.CreateLlmProviderApiKeyJSONBodyProviderGemini),
						string(client.CreateLlmProviderApiKeyJSONBodyProviderGroq),
						string(client.CreateLlmProviderApiKeyJSONBodyProviderMinimax),
						string(client.CreateLlmProviderApiKeyJSONBodyProviderMistral),
						string(client.CreateLlmProviderApiKeyJSONBodyProviderOllama),
						string(client.CreateLlmProviderApiKeyJSONBodyProviderOpenai),
						string(client.CreateLlmProviderApiKeyJSONBodyProviderOpenrouter),
						string(client.CreateLlmProviderApiKeyJSONBodyProviderPerplexity),
						string(client.CreateLlmProviderApiKeyJSONBodyProviderVllm),
						string(client.CreateLlmProviderApiKeyJSONBodyProviderXai),
						string(client.CreateLlmProviderApiKeyJSONBodyProviderZhipuai),
					),
				},
			},
			"is_organization_default": schema.BoolAttribute{
				MarkdownDescription: "Whether this API key is the primary key for the provider",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
			},
			"base_url": schema.StringAttribute{
				MarkdownDescription: "Custom base URL for the LLM provider endpoint",
				Optional:            true,
			},
			"scope": schema.StringAttribute{
				MarkdownDescription: "Visibility scope for the API key: `personal`, `team`, or `org`",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("personal"),
				Validators: []validator.String{
					stringvalidator.OneOf(
						string(client.CreateLlmProviderApiKeyJSONBodyScopePersonal),
						string(client.CreateLlmProviderApiKeyJSONBodyScopeTeam),
						string(client.CreateLlmProviderApiKeyJSONBodyScopeOrg),
					),
				},
			},
			"team_id": schema.StringAttribute{
				MarkdownDescription: "Team ID for team-scoped keys",
				Optional:            true,
			},
			"vault_secret_path": schema.StringAttribute{
				MarkdownDescription: "Path to the secret in the vault. Must be set together with `vault_secret_key` and cannot be combined with `api_key`.",
				Optional:            true,
				Validators: []validator.String{
					stringvalidator.AlsoRequires(path.MatchRoot("vault_secret_key")),
					stringvalidator.ConflictsWith(path.MatchRoot("api_key")),
				},
			},
			"vault_secret_key": schema.StringAttribute{
				MarkdownDescription: "Key within the vault secret. Must be set together with `vault_secret_path` and cannot be combined with `api_key`.",
				Optional:            true,
				Validators: []validator.String{
					stringvalidator.AlsoRequires(path.MatchRoot("vault_secret_path")),
					stringvalidator.ConflictsWith(path.MatchRoot("api_key")),
				},
			},
		},
	}
}

func (r *LLMProviderApiKeyResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *LLMProviderApiKeyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data LLMProviderApiKeyResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	prior := tftypes.NewValue(req.Plan.Raw.Type(), nil)
	patch := MergePatch(ctx, req.Plan.Raw, prior, llmProviderApiKeyAttrSpec, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	LogPatch(ctx, "archestra_llm_provider_api_key Create", patch, llmProviderApiKeyAttrSpec)

	bodyBytes, err := json.Marshal(patch)
	if err != nil {
		resp.Diagnostics.AddError("Marshal Error", fmt.Sprintf("Unable to marshal request body: %s", err))
		return
	}
	apiResp, err := r.client.CreateLlmProviderApiKeyWithBodyWithResponse(ctx, "application/json", bytes.NewReader(bodyBytes))
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to create LLM provider API key, got error: %s", err))
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
	data.IsOrganizationDefault = types.BoolValue(apiResp.JSON200.IsPrimary)
	data.Scope = types.StringValue(string(apiResp.JSON200.Scope))

	if apiResp.JSON200.BaseUrl != nil {
		data.BaseUrl = types.StringValue(*apiResp.JSON200.BaseUrl)
	}

	if apiResp.JSON200.TeamId != nil {
		data.TeamID = types.StringValue(*apiResp.JSON200.TeamId)
	}

	// VaultSecretPath and VaultSecretKey are not in the Create response;
	// they are preserved from plan and will be read back via Read.

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *LLMProviderApiKeyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data LLMProviderApiKeyResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	id, err := uuid.Parse(data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid ID", fmt.Sprintf("Unable to parse LLM provider API key ID: %s", err))
		return
	}

	apiResp, err := r.client.GetLlmProviderApiKeyWithResponse(ctx, id)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to read LLM provider API key, got error: %s", err))
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
	data.IsOrganizationDefault = types.BoolValue(apiResp.JSON200.IsPrimary)
	data.Scope = types.StringValue(string(apiResp.JSON200.Scope))

	if apiResp.JSON200.BaseUrl != nil {
		data.BaseUrl = types.StringValue(*apiResp.JSON200.BaseUrl)
	} else {
		data.BaseUrl = types.StringNull()
	}

	if apiResp.JSON200.TeamId != nil {
		data.TeamID = types.StringValue(*apiResp.JSON200.TeamId)
	} else {
		data.TeamID = types.StringNull()
	}

	// VaultSecretPath and VaultSecretKey are write-only on the backend
	// (consumed on create, never echoed). Preserve whatever is already in
	// state so imports/refreshes don't drop the values. Only override if the
	// API actually returned them (future-proofing).
	if apiResp.JSON200.VaultSecretPath != nil {
		data.VaultSecretPath = types.StringValue(*apiResp.JSON200.VaultSecretPath)
	}
	if apiResp.JSON200.VaultSecretKey != nil {
		data.VaultSecretKey = types.StringValue(*apiResp.JSON200.VaultSecretKey)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *LLMProviderApiKeyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data LLMProviderApiKeyResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	id, err := uuid.Parse(data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid ID", fmt.Sprintf("Unable to parse LLM provider API key ID: %s", err))
		return
	}

	patch := MergePatch(ctx, req.Plan.Raw, req.State.Raw, llmProviderApiKeyAttrSpec, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	if len(patch) == 0 {
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
		return
	}
	LogPatch(ctx, "archestra_llm_provider_api_key Update", patch, llmProviderApiKeyAttrSpec)

	bodyBytes, err := json.Marshal(patch)
	if err != nil {
		resp.Diagnostics.AddError("Marshal Error", fmt.Sprintf("Unable to marshal request body: %s", err))
		return
	}
	apiResp, err := r.client.UpdateLlmProviderApiKeyWithBodyWithResponse(ctx, id, "application/json", bytes.NewReader(bodyBytes))
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to update LLM provider API key, got error: %s", err))
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
			fmt.Sprintf("Expected 200 OK, got status %d: %s", apiResp.StatusCode(), string(apiResp.Body)),
		)
		return
	}

	data.Name = types.StringValue(apiResp.JSON200.Name)
	data.LLMProvider = types.StringValue(string(apiResp.JSON200.Provider))
	data.IsOrganizationDefault = types.BoolValue(apiResp.JSON200.IsPrimary)
	data.Scope = types.StringValue(string(apiResp.JSON200.Scope))

	if apiResp.JSON200.BaseUrl != nil {
		data.BaseUrl = types.StringValue(*apiResp.JSON200.BaseUrl)
	}

	if apiResp.JSON200.TeamId != nil {
		data.TeamID = types.StringValue(*apiResp.JSON200.TeamId)
	}

	// VaultSecretPath and VaultSecretKey are not in the Update response;
	// they are preserved from plan and will be read back via Read.

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *LLMProviderApiKeyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data LLMProviderApiKeyResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	id, err := uuid.Parse(data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid ID", fmt.Sprintf("Unable to parse LLM provider API key ID: %s", err))
		return
	}

	apiResp, err := r.client.DeleteLlmProviderApiKeyWithResponse(ctx, id)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to delete LLM provider API key, got error: %s", err))
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

func (r *LLMProviderApiKeyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
