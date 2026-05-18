package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

var _ resource.Resource = &VirtualApiKeyResource{}
var _ resource.ResourceWithImportState = &VirtualApiKeyResource{}
var _ resource.ResourceWithValidateConfig = &VirtualApiKeyResource{}

func NewVirtualApiKeyResource() resource.Resource {
	return &VirtualApiKeyResource{}
}

type VirtualApiKeyResource struct {
	client *client.ClientWithResponses
}

type VirtualApiKeyResourceModel struct {
	ID                  types.String `tfsdk:"id"`
	LlmProviderApiKeyID types.String `tfsdk:"llm_provider_api_key_id"`
	Name                types.String `tfsdk:"name"`
	ExpiresAt           types.String `tfsdk:"expires_at"`
	Scope               types.String `tfsdk:"scope"`
	Teams               types.List   `tfsdk:"teams"`
	Value               types.String `tfsdk:"value"`
	SecretID            types.String `tfsdk:"secret_id"`
	AuthorID            types.String `tfsdk:"author_id"`
	AuthorName          types.String `tfsdk:"author_name"`
	CreatedAt           types.String `tfsdk:"created_at"`
	LastUsedAt          types.String `tfsdk:"last_used_at"`
}

func (r *VirtualApiKeyResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_virtual_api_key"
}

func (r *VirtualApiKeyResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Virtual API key issued against an `archestra_llm_provider_api_key`. The token `value` is returned only once at create time — treat it like an AWS access key.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Virtual API key identifier.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"llm_provider_api_key_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "ID of the parent `archestra_llm_provider_api_key`. Changing this forces a new virtual key.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Human-readable name of the virtual API key.",
				Validators: []validator.String{
					stringvalidator.LengthBetween(1, 256),
				},
			},
			"expires_at": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Expiration timestamp in RFC 3339 format (e.g. `2027-01-01T00:00:00Z`). Omit for a non-expiring key.",
				Validators: []validator.String{
					rfc3339TimeValidator{},
				},
			},
			"scope": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("org"),
				MarkdownDescription: "Visibility scope: `personal`, `team`, or `org` (default: `org`).",
				Validators: []validator.String{
					stringvalidator.OneOf(
						string(client.CreateVirtualApiKeyJSONBodyScopePersonal),
						string(client.CreateVirtualApiKeyJSONBodyScopeTeam),
						string(client.CreateVirtualApiKeyJSONBodyScopeOrg),
					),
				},
			},
			"teams": schema.ListAttribute{
				Optional:            true,
				Computed:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Team IDs the virtual key is scoped to. Required when `scope = \"team\"`; must be empty otherwise.",
				PlanModifiers:       []planmodifier.List{EmptyListOnConfigNull()},
			},
			"value": schema.StringAttribute{
				Computed:            true,
				Sensitive:           true,
				MarkdownDescription: "The full virtual API key token. **Returned only once at create time** — Terraform stores it; the backend never echoes it again.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"secret_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Internal secret-manager handle for the stored token.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"author_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "ID of the user who created the key.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"author_name": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Display name of the user who created the key.",
			},
			"created_at": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Creation timestamp (RFC 3339).",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"last_used_at": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Last-used timestamp (RFC 3339), or null if never used.",
			},
		},
	}
}

func (r *VirtualApiKeyResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// ValidateConfig enforces the scope/teams cross-field constraint the
// backend rejects with 400 at apply time:
//   - scope = "team" requires a non-empty teams list
//   - scope ≠ "team" must have an empty/unset teams list
func (r *VirtualApiKeyResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var data VirtualApiKeyResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if data.Scope.IsNull() || data.Scope.IsUnknown() {
		return
	}
	scope := data.Scope.ValueString()
	teamsSet := !data.Teams.IsNull() && !data.Teams.IsUnknown() && len(data.Teams.Elements()) > 0

	if scope == "team" && !teamsSet {
		resp.Diagnostics.AddAttributeError(
			path.Root("teams"),
			"Missing Required Attribute",
			`teams must contain at least one team ID when scope = "team"`,
		)
	}
	if scope != "team" && teamsSet {
		resp.Diagnostics.AddAttributeError(
			path.Root("teams"),
			"Invalid Attribute Value",
			fmt.Sprintf(`teams must be empty when scope = %q; team assignments are only valid with scope = "team"`, scope),
		)
	}
}

func (r *VirtualApiKeyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data VirtualApiKeyResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	parentID, err := uuid.Parse(data.LlmProviderApiKeyID.ValueString())
	if err != nil {
		resp.Diagnostics.AddAttributeError(
			path.Root("llm_provider_api_key_id"),
			"Invalid LLM provider API key ID",
			err.Error(),
		)
		return
	}

	priorNull := tftypes.NewValue(req.Plan.Schema.Type().TerraformType(ctx), nil)
	patch := MergePatch(ctx, req.Plan.Raw, priorNull, virtualApiKeyAttrSpec, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	LogPatch(ctx, "archestra_virtual_api_key Create", patch, virtualApiKeyAttrSpec)

	bodyBytes, err := json.Marshal(patch)
	if err != nil {
		resp.Diagnostics.AddError("Marshal Error", fmt.Sprintf("Unable to marshal request body: %s", err))
		return
	}
	apiResp, err := r.client.CreateVirtualApiKeyWithBodyWithResponse(ctx, parentID, "application/json", bytes.NewReader(bodyBytes))
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to create virtual API key: %s", err))
		return
	}
	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Expected 200 OK, got status %d: %s", apiResp.StatusCode(), string(apiResp.Body)),
		)
		return
	}

	created := apiResp.JSON200
	data.ID = types.StringValue(created.Id.String())
	data.Name = types.StringValue(created.Name)
	data.Scope = types.StringValue(string(created.Scope))
	data.Value = types.StringValue(created.Value)
	data.SecretID = types.StringValue(created.SecretId.String())
	data.CreatedAt = types.StringValue(created.CreatedAt.Format(time.RFC3339))
	expiresAtFromTime(&data.ExpiresAt, created.ExpiresAt)
	optionalStringFromAPI(&data.AuthorID, created.AuthorId)
	optionalStringFromAPI(&data.AuthorName, created.AuthorName)
	lastUsedAtFromTime(&data.LastUsedAt, created.LastUsedAt)
	data.Teams = teamIDsFromCreateUpdate(ctx, data.Teams, created.Teams, &resp.Diagnostics)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *VirtualApiKeyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data VirtualApiKeyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	parentID, err := uuid.Parse(data.LlmProviderApiKeyID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid parent ID", err.Error())
		return
	}
	id := data.ID.ValueString()

	found, drop, diags := r.findVirtualKeyByID(ctx, parentID, id, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	if drop || !found {
		resp.State.RemoveResource(ctx)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *VirtualApiKeyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data VirtualApiKeyResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	parentID, err := uuid.Parse(data.LlmProviderApiKeyID.ValueString())
	if err != nil {
		resp.Diagnostics.AddAttributeError(path.Root("llm_provider_api_key_id"), "Invalid parent ID", err.Error())
		return
	}
	id, err := uuid.Parse(data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid ID", err.Error())
		return
	}

	patch := MergePatch(ctx, req.Plan.Raw, req.State.Raw, virtualApiKeyAttrSpec, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	if len(patch) == 0 {
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
		return
	}
	LogPatch(ctx, "archestra_virtual_api_key Update", patch, virtualApiKeyAttrSpec)

	bodyBytes, err := json.Marshal(patch)
	if err != nil {
		resp.Diagnostics.AddError("Marshal Error", err.Error())
		return
	}
	apiResp, err := r.client.UpdateVirtualApiKeyWithBodyWithResponse(ctx, parentID, id, "application/json", bytes.NewReader(bodyBytes))
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to update virtual API key: %s", err))
		return
	}
	if IsNotFound(apiResp) {
		resp.Diagnostics.AddError(
			"Resource Deleted Outside Terraform",
			"The virtual API key was deleted on the backend between refresh and apply. "+
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

	updated := apiResp.JSON200
	data.Name = types.StringValue(updated.Name)
	data.Scope = types.StringValue(string(updated.Scope))
	data.SecretID = types.StringValue(updated.SecretId.String())
	data.CreatedAt = types.StringValue(updated.CreatedAt.Format(time.RFC3339))
	expiresAtFromTime(&data.ExpiresAt, updated.ExpiresAt)
	optionalStringFromAPI(&data.AuthorID, updated.AuthorId)
	optionalStringFromAPI(&data.AuthorName, updated.AuthorName)
	lastUsedAtFromTime(&data.LastUsedAt, updated.LastUsedAt)
	data.Teams = teamIDsFromCreateUpdate(ctx, data.Teams, updated.Teams, &resp.Diagnostics)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *VirtualApiKeyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data VirtualApiKeyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	parentID, err := uuid.Parse(data.LlmProviderApiKeyID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid parent ID", err.Error())
		return
	}
	id, err := uuid.Parse(data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid ID", err.Error())
		return
	}

	apiResp, err := r.client.DeleteVirtualApiKeyWithResponse(ctx, parentID, id)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to delete virtual API key: %s", err))
		return
	}
	if apiResp.JSON200 == nil && !IsNotFound(apiResp) {
		resp.Diagnostics.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Expected 200 OK or 404 Not Found, got status %d", apiResp.StatusCode()),
		)
	}
}

// ImportState requires the composite `<llm_provider_api_key_id>:<id>`
// because the virtual key is nested under its parent on the wire and
// Read needs both to call the parent-filtered list endpoint.
func (r *VirtualApiKeyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.SplitN(req.ID, ":", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		resp.Diagnostics.AddError(
			"Invalid Import ID",
			"Expected `<llm_provider_api_key_id>:<id>` — both halves are required. Example: `terraform import archestra_virtual_api_key.example 11111111-1111-1111-1111-111111111111:22222222-2222-2222-2222-222222222222`.",
		)
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("llm_provider_api_key_id"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), parts[1])...)
}

// findVirtualKeyByID paginates `/api/llm-virtual-keys` filtered by
// parent and writes the matching row into `data`. `drop = true` means
// the API surfaced a 404 (parent gone or filter rejected) so the caller
// should clear state.
func (r *VirtualApiKeyResource) findVirtualKeyByID(
	ctx context.Context,
	parentID openapi_types.UUID,
	id string,
	data *VirtualApiKeyResourceModel,
) (found bool, drop bool, diags diag.Diagnostics) {
	limit := 100
	offset := 0
	chatApiKeyId := parentID
	params := &client.GetAllVirtualApiKeysParams{
		Limit:        &limit,
		Offset:       &offset,
		ChatApiKeyId: &chatApiKeyId,
	}

	for {
		apiResp, err := r.client.GetAllVirtualApiKeysWithResponse(ctx, params)
		if err != nil {
			diags.AddError("API Error", fmt.Sprintf("Unable to list virtual API keys: %s", err))
			return false, false, diags
		}
		if IsNotFound(apiResp) {
			return false, true, diags
		}
		if apiResp.JSON200 == nil {
			diags.AddError("Unexpected API Response", fmt.Sprintf("Expected 200 OK, got status %d: %s", apiResp.StatusCode(), string(apiResp.Body)))
			return false, false, diags
		}

		for i := range apiResp.JSON200.Data {
			row := &apiResp.JSON200.Data[i]
			if row.Id.String() != id {
				continue
			}

			data.ID = types.StringValue(row.Id.String())
			data.LlmProviderApiKeyID = types.StringValue(row.ChatApiKeyId.String())
			data.Name = types.StringValue(row.Name)
			data.Scope = types.StringValue(string(row.Scope))
			data.SecretID = types.StringValue(row.SecretId.String())
			data.CreatedAt = types.StringValue(row.CreatedAt.Format(time.RFC3339))
			expiresAtFromTime(&data.ExpiresAt, row.ExpiresAt)
			optionalStringFromAPI(&data.AuthorID, row.AuthorId)
			optionalStringFromAPI(&data.AuthorName, row.AuthorName)
			lastUsedAtFromTime(&data.LastUsedAt, row.LastUsedAt)

			ids := make([]string, len(row.Teams))
			for j, t := range row.Teams {
				ids[j] = t.Id
			}
			list, ld := teamIDsToList(ctx, data.Teams, ids)
			diags.Append(ld...)
			data.Teams = list

			// value is never echoed by the list endpoint — preserve from
			// prior state. UseStateForUnknown carries it across refreshes;
			// don't overwrite to null here.

			return true, false, diags
		}

		if !apiResp.JSON200.Pagination.HasNext {
			return false, false, diags
		}
		offset += limit
		params.Offset = &offset
	}
}
