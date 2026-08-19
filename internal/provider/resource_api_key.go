package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

var _ resource.Resource = &ApiKeyResource{}
var _ resource.ResourceWithImportState = &ApiKeyResource{}

func NewApiKeyResource() resource.Resource {
	return &ApiKeyResource{}
}

type ApiKeyResource struct {
	client *client.ClientWithResponses
}

type ApiKeyResourceModel struct {
	ID               types.String `tfsdk:"id"`
	Name             types.String `tfsdk:"name"`
	ExpiresInSeconds types.Int64  `tfsdk:"expires_in_seconds"`
	Key              types.String `tfsdk:"key"`
	Prefix           types.String `tfsdk:"prefix"`
	Start            types.String `tfsdk:"start"`
	Enabled          types.Bool   `tfsdk:"enabled"`
	ExpiresAt        types.String `tfsdk:"expires_at"`
	LastRequest      types.String `tfsdk:"last_request"`
	CreatedAt        types.String `tfsdk:"created_at"`
	UpdatedAt        types.String `tfsdk:"updated_at"`
}

// apiKeyAPIResponse mirrors the JSON shape returned by both
// GetApiKey and CreateApiKey. JSON-roundtripping the raw body lets a
// single mapper handle every endpoint's response without depending on
// the generated client's per-endpoint anonymous structs.
type apiKeyAPIResponse struct {
	CreatedAt   time.Time  `json:"createdAt"`
	Enabled     *bool      `json:"enabled"`
	ExpiresAt   *time.Time `json:"expiresAt"`
	ID          string     `json:"id"`
	Key         string     `json:"key"`
	LastRequest *time.Time `json:"lastRequest"`
	Name        *string    `json:"name"`
	Prefix      *string    `json:"prefix"`
	Start       *string    `json:"start"`
	UpdatedAt   time.Time  `json:"updatedAt"`
}

func (r *ApiKeyResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_api_key"
}

func (r *ApiKeyResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Platform `arch_...` API key for authenticating to the Archestra API itself. Treat the returned `key` like an AWS access key — it's returned once at create time and never echoed back. The backend exposes no Update endpoint, so every input field is RequiresReplace.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "API key identifier.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Human-readable name. Used by the UI to distinguish keys; omit for a null backend name.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"expires_in_seconds": schema.Int64Attribute{
				Optional:            true,
				MarkdownDescription: "Lifetime in seconds from creation; minimum 86400 (1 day) enforced by Better Auth. Omit for a non-expiring key. The backend converts this into an absolute `expires_at` timestamp on Create.",
				Validators:          []validator.Int64{int64validator.AtLeast(86400)},
				PlanModifiers:       []planmodifier.Int64{int64planmodifier.RequiresReplace()},
			},
			"key": schema.StringAttribute{
				Computed:            true,
				Sensitive:           true,
				MarkdownDescription: "The full API key value (format `arch_...`). **Returned only once at create time** — Terraform stores it; the backend never echoes it again.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"prefix": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Public prefix of the key, e.g. `arch_`.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"start": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "First few characters of the key, shown in the UI's key list for identification (the full key is shown only at create time).",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"enabled": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "Whether the key authenticates successfully. Backend-controlled — there is no provider-side way to flip this; revoke a key by destroying the resource.",
			},
			"expires_at": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Absolute expiry timestamp (RFC 3339), or null for non-expiring keys.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"last_request": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Timestamp (RFC 3339) of the last request authenticated with this key, or null if never used.",
			},
			"created_at": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Creation timestamp (RFC 3339).",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"updated_at": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Last-update timestamp (RFC 3339).",
			},
		},
	}
}

func (r *ApiKeyResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.ClientWithResponses)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *client.ClientWithResponses, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}
	r.client = c
}

func (r *ApiKeyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data ApiKeyResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	priorNull := tftypes.NewValue(req.Plan.Schema.Type().TerraformType(ctx), nil)
	patch := MergePatch(ctx, req.Plan.Raw, priorNull, apiKeyAttrSpec, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	LogPatch(ctx, "archestra_api_key Create", patch, apiKeyAttrSpec)

	bodyBytes, err := json.Marshal(patch)
	if err != nil {
		resp.Diagnostics.AddError("Marshal Error", err.Error())
		return
	}
	apiResp, err := r.client.CreateApiKeyWithBodyWithResponse(ctx, "application/json", bytes.NewReader(bodyBytes))
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to create API key: %s", err))
		return
	}
	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Expected 200 OK, got status %d: %s", apiResp.StatusCode(), string(apiResp.Body)),
		)
		return
	}

	if !flattenApiKeyBody(&data, apiResp.Body, &resp.Diagnostics, true) {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ApiKeyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data ApiKeyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := r.client.GetApiKeyWithResponse(ctx, data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to read API key: %s", err))
		return
	}
	if IsNotFound(apiResp) {
		resp.State.RemoveResource(ctx)
		return
	}
	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Expected 200 OK, got status %d: %s", apiResp.StatusCode(), string(apiResp.Body)),
		)
		return
	}

	// `key` is preserved from prior state — backend never returns it
	// after Create. flattenApiKeyBody's `applyKey=false` honors this.
	if !flattenApiKeyBody(&data, apiResp.Body, &resp.Diagnostics, false) {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Update is a no-op: every input field on the schema is
// RequiresReplace, so the framework destroys+recreates instead of
// calling Update. The method exists only to satisfy the Resource
// interface.
func (r *ApiKeyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data ApiKeyResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ApiKeyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data ApiKeyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := r.client.DeleteApiKeyWithResponse(ctx, data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to delete API key: %s", err))
		return
	}
	if apiResp.JSON200 == nil && !IsNotFound(apiResp) {
		resp.Diagnostics.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Expected 200 OK or 404 Not Found, got status %d: %s", apiResp.StatusCode(), string(apiResp.Body)),
		)
	}
}

// ImportState passes the API key id through. The next refresh
// populates everything except `key` (backend never re-returns it) and
// `expires_in_seconds` (input-only, never echoed). Document via
// `ImportStateVerifyIgnore` in tests.
func (r *ApiKeyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// flattenApiKeyBody decodes the API response body and projects it
// into state. `applyKey=true` lets Create populate the one-shot
// token; Read calls with `applyKey=false` so the prior-state token
// survives refresh.
func flattenApiKeyBody(data *ApiKeyResourceModel, body []byte, diags *diag.Diagnostics, applyKey bool) bool {
	var src apiKeyAPIResponse
	if err := json.Unmarshal(body, &src); err != nil {
		diags.AddError("API Response Decode Error", "Unable to decode API key response: "+err.Error())
		return false
	}

	data.ID = types.StringValue(src.ID)
	optionalStringFromAPI(&data.Name, src.Name)
	optionalStringFromAPI(&data.Prefix, src.Prefix)
	optionalStringFromAPI(&data.Start, src.Start)
	if src.Enabled != nil {
		data.Enabled = types.BoolValue(*src.Enabled)
	} else {
		data.Enabled = types.BoolValue(false)
	}
	data.CreatedAt = types.StringValue(src.CreatedAt.Format(time.RFC3339))
	data.UpdatedAt = types.StringValue(src.UpdatedAt.Format(time.RFC3339))

	if src.ExpiresAt != nil {
		data.ExpiresAt = types.StringValue(src.ExpiresAt.Format(time.RFC3339))
	} else if !data.ExpiresAt.IsNull() {
		data.ExpiresAt = types.StringNull()
	}
	if src.LastRequest != nil {
		data.LastRequest = types.StringValue(src.LastRequest.Format(time.RFC3339))
	} else if !data.LastRequest.IsNull() {
		data.LastRequest = types.StringNull()
	}

	if applyKey && src.Key != "" {
		data.Key = types.StringValue(src.Key)
	}
	return true
}
