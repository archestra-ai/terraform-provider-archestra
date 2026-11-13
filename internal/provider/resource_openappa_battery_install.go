package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"time"

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
)

var (
	_ resource.ResourceWithImportState    = &OpenappaBatteryInstallResource{}
	_ resource.ResourceWithValidateConfig = &OpenappaBatteryInstallResource{}
)

func NewOpenappaBatteryInstallResource() resource.Resource {
	return &OpenappaBatteryInstallResource{}
}

type OpenappaBatteryInstallResource struct{ client *client.ClientWithResponses }

type OpenappaBatteryInstallResourceModel struct {
	ID                 types.String `tfsdk:"id"`
	BatteryName        types.String `tfsdk:"battery_name"`
	CatalogID          types.String `tfsdk:"catalog_id"`
	PackageHash        types.String `tfsdk:"package_hash"`
	Enabled            types.Bool   `tfsdk:"enabled"`
	Status             types.String `tfsdk:"status"`
	Target             types.String `tfsdk:"target"`
	LastError          types.String `tfsdk:"last_error"`
	CredentialBindings types.Map    `tfsdk:"credential_bindings"`
	CreatedAt          types.String `tfsdk:"created_at"`
	UpdatedAt          types.String `tfsdk:"updated_at"`
}

// Only enabled is writable after creation (PATCH); the other inputs are
// create-only and replace the install.
var openappaBatteryInstallAttrSpec = []AttrSpec{
	{TFName: "battery_name", JSONName: "batteryName", Kind: Scalar, CreateOnly: true},
	{TFName: "catalog_id", JSONName: "catalogId", Kind: Scalar, OmitOnNull: true, CreateOnly: true},
	{TFName: "package_hash", JSONName: "packageHash", Kind: Scalar, OmitOnNull: true, CreateOnly: true},
	{TFName: "enabled", JSONName: "enabled", Kind: Scalar},
}

func (r *OpenappaBatteryInstallResource) AttrSpecs() []AttrSpec {
	return openappaBatteryInstallAttrSpec
}
func (r *OpenappaBatteryInstallResource) APIShape() any {
	return client.GetOpenappaBatteryInstallResponse{}
}
func (r *OpenappaBatteryInstallResource) KnownIntentionallySkipped() []string {
	// Organization ownership is server bookkeeping; the rest are read-only
	// computed attributes that AttrSpec (the write surface) does not list.
	return []string{"organizationId", "id", "status", "target", "lastError", "credentialBindings", "createdAt", "updatedAt"}
}

func (r *OpenappaBatteryInstallResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_openappa_battery_install"
}

func (r *OpenappaBatteryInstallResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	replace := []planmodifier.String{stringplanmodifier.RequiresReplace()}
	resp.Schema = schema.Schema{
		MarkdownDescription: "Includes an OpenAPPA guardrails battery in the organization's policy and, for a battery that governs MCP servers, attaches it to one registry entry. " +
			"A battery made of annotators alone governs the whole organization and is installed without `catalog_id`. " +
			"Writes edit the organization's policy text, so they fail while a GitHub repository owns that text. " +
			"Credential bindings are managed separately and are read-only here. Import an existing install by UUID. Requires API 1.5.0-rc.1 or later.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true, MarkdownDescription: "Battery install UUID.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"battery_name": schema.StringAttribute{
				Required: true, PlanModifiers: replace,
				MarkdownDescription: "Name of a bundled or uploaded battery, as `GET /api/openappa/batteries` lists it.",
				Validators:          []validator.String{stringvalidator.LengthBetween(1, 100)},
			},
			"catalog_id": schema.StringAttribute{
				Optional: true, PlanModifiers: replace,
				MarkdownDescription: "Registry entry (`archestra_mcp_registry_entry`) the battery governs. Omit for a battery that governs the organization.",
				Validators:          []validator.String{stringvalidator.RegexMatches(canonicalUUIDRegexp, "Use a lowercase canonical registry entry UUID.")},
			},
			"package_hash": schema.StringAttribute{
				Optional: true, PlanModifiers: replace,
				MarkdownDescription: "SHA-256 of an uploaded battery package to run; omit for the bundled battery of that name.",
				Validators:          []validator.String{stringvalidator.RegexMatches(sha256HexRegexp, "Use the package's lowercase 64-character SHA-256.")},
			},
			"enabled": schema.BoolAttribute{
				Optional: true, Computed: true, Default: booldefault.StaticBool(true),
				MarkdownDescription: "Whether the battery is attached to `catalog_id` (default `true`). `false` detaches it but keeps the battery included; only installs with a `catalog_id` can be disabled.",
			},
			"status": schema.StringAttribute{
				Computed: true, MarkdownDescription: "Install state, such as `active`, `missing_credentials`, `naming_conflict` or `unavailable`.",
			},
			"target": schema.StringAttribute{
				Computed: true, MarkdownDescription: "Target the battery deploys as (`multi-user`), or null for its base alone.",
			},
			"last_error": schema.StringAttribute{
				Computed: true, MarkdownDescription: "Most recent error applying the battery, if any.",
			},
			"credential_bindings": schema.MapAttribute{
				Computed: true, ElementType: types.StringType,
				MarkdownDescription: "Helper variables bound to organization credentials, keyed by variable name.",
			},
			"created_at": schema.StringAttribute{
				Computed: true, MarkdownDescription: "Creation time (RFC 3339).",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"updated_at": schema.StringAttribute{
				Computed: true, MarkdownDescription: "Last update time (RFC 3339).",
			},
		},
	}
}

func (r *OpenappaBatteryInstallResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.ClientWithResponses)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Resource Configure Type", fmt.Sprintf("Expected *client.ClientWithResponses, got %T", req.ProviderData))
		return
	}
	r.client = c
}

func (r *OpenappaBatteryInstallResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var catalogID types.String
	var enabled types.Bool
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("catalog_id"), &catalogID)...)
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("enabled"), &enabled)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if catalogID.IsNull() && !enabled.IsNull() && !enabled.IsUnknown() && !enabled.ValueBool() {
		resp.Diagnostics.AddAttributeError(path.Root("enabled"), "Organization Battery Cannot Be Disabled",
			"A battery without catalog_id governs the organization; remove the resource to drop it instead of setting enabled = false.")
	}
}

func (r *OpenappaBatteryInstallResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data OpenappaBatteryInstallResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	body := client.CreateOpenappaBatteryInstallJSONRequestBody{BatteryName: data.BatteryName.ValueString()}
	if !data.CatalogID.IsNull() {
		id, err := uuid.Parse(data.CatalogID.ValueString())
		if err != nil {
			resp.Diagnostics.AddAttributeError(path.Root("catalog_id"), "Invalid Catalog ID", err.Error())
			return
		}
		body.CatalogId = &id
	}
	if !data.PackageHash.IsNull() {
		hash := data.PackageHash.ValueString()
		body.PackageHash = &hash
	}
	created, err := r.client.CreateOpenappaBatteryInstallWithResponse(ctx, body)
	if err != nil {
		resp.Diagnostics.AddError("Battery Install Failed", err.Error())
		return
	}
	if created.StatusCode() != http.StatusOK {
		resp.Diagnostics.AddError("Battery Install Failed", fmt.Sprintf("Status %d: %s", created.StatusCode(), string(created.Body)))
		return
	}
	// The create response describes the battery, not the install row, so find
	// the row by (battery, catalog), which the API keeps unique.
	id := r.findInstallID(ctx, data.BatteryName.ValueString(), data.CatalogID, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	if id == "" {
		resp.Diagnostics.AddError("Battery Install Not Found",
			fmt.Sprintf("The API accepted %s but listed no install for it.", data.BatteryName.ValueString()))
		return
	}
	// Keep the identity before anything else can fail, so destroy can clean up.
	data.ID = types.StringValue(id)
	enabled := data.Enabled
	data.Enabled = types.BoolValue(true)
	nullOpenappaBatteryInstallComputed(&data)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !enabled.IsNull() && !enabled.IsUnknown() && !enabled.ValueBool() {
		r.setEnabled(ctx, id, false, &resp.Diagnostics)
		if resp.Diagnostics.HasError() {
			return
		}
	}
	if found := r.read(ctx, &data, &resp.Diagnostics); !found && !resp.Diagnostics.HasError() {
		resp.Diagnostics.AddError("Battery Install Not Found", "The install disappeared right after it was created.")
	}
	if !resp.Diagnostics.HasError() {
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
	}
}

func (r *OpenappaBatteryInstallResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data OpenappaBatteryInstallResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if data.ID.IsNull() || data.ID.IsUnknown() || data.ID.ValueString() == "" {
		resp.State.RemoveResource(ctx)
		return
	}
	if !r.read(ctx, &data, &resp.Diagnostics) {
		if !resp.Diagnostics.HasError() {
			resp.State.RemoveResource(ctx)
		}
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *OpenappaBatteryInstallResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data OpenappaBatteryInstallResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	patch := MergePatch(ctx, req.Plan.Raw, req.State.Raw, openappaBatteryInstallAttrSpec, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	if enabled, ok := patch["enabled"].(bool); ok {
		r.setEnabled(ctx, data.ID.ValueString(), enabled, &resp.Diagnostics)
		if resp.Diagnostics.HasError() {
			return
		}
	}
	if found := r.read(ctx, &data, &resp.Diagnostics); !found && !resp.Diagnostics.HasError() {
		resp.Diagnostics.AddError("Battery Install Not Found", "The install disappeared during the update.")
	}
	if !resp.Diagnostics.HasError() {
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
	}
}

func (r *OpenappaBatteryInstallResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data OpenappaBatteryInstallResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	id, err := uuid.Parse(data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid Battery Install ID", err.Error())
		return
	}
	deleted, err := r.client.DeleteOpenappaBatteryInstallWithResponse(ctx, id)
	if err != nil {
		resp.Diagnostics.AddError("Battery Install Deletion Failed", err.Error())
		return
	}
	switch deleted.StatusCode() {
	case http.StatusOK, http.StatusNotFound:
	default:
		resp.Diagnostics.AddError("Battery Install Deletion Failed", fmt.Sprintf("Status %d: %s", deleted.StatusCode(), string(deleted.Body)))
	}
}

func (r *OpenappaBatteryInstallResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	if !canonicalUUIDRegexp.MatchString(req.ID) {
		resp.Diagnostics.AddError("Invalid Battery Install Import ID",
			"Import using the install's lowercase canonical UUID, as installs[].id in GET /api/openappa/batteries lists it.")
		return
	}
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// read refreshes data from the API; false when the install no longer exists.
func (r *OpenappaBatteryInstallResource) read(ctx context.Context, data *OpenappaBatteryInstallResourceModel, diags *diag.Diagnostics) bool {
	id, err := uuid.Parse(data.ID.ValueString())
	if err != nil {
		diags.AddError("Invalid Battery Install ID", err.Error())
		return false
	}
	got, err := r.client.GetOpenappaBatteryInstallWithResponse(ctx, id)
	if err != nil {
		diags.AddError("Battery Install Read Failed", err.Error())
		return false
	}
	if got.StatusCode() == http.StatusNotFound {
		return false
	}
	if got.JSON200 == nil {
		diags.AddError("Battery Install Read Failed", fmt.Sprintf("Status %d: %s", got.StatusCode(), string(got.Body)))
		return false
	}
	j := got.JSON200
	data.ID = types.StringValue(j.Id.String())
	data.BatteryName = types.StringValue(j.BatteryName)
	if j.CatalogId != nil {
		data.CatalogID = types.StringValue(j.CatalogId.String())
	} else {
		data.CatalogID = types.StringNull()
	}
	data.PackageHash = types.StringPointerValue(j.PackageHash)
	data.Enabled = types.BoolValue(j.Enabled)
	data.Status = types.StringValue(string(j.Status))
	data.Target = types.StringNull()
	if j.Target != "" {
		data.Target = types.StringValue(string(j.Target))
	}
	data.LastError = types.StringPointerValue(j.LastError)
	bindings, d := types.MapValueFrom(ctx, types.StringType, nonNilStringMap(j.CredentialBindings))
	diags.Append(d...)
	data.CredentialBindings = bindings
	data.CreatedAt = types.StringValue(j.CreatedAt.UTC().Format(time.RFC3339Nano))
	data.UpdatedAt = types.StringValue(j.UpdatedAt.UTC().Format(time.RFC3339Nano))
	return true
}

func (r *OpenappaBatteryInstallResource) setEnabled(ctx context.Context, id string, enabled bool, diags *diag.Diagnostics) {
	installID, err := uuid.Parse(id)
	if err != nil {
		diags.AddError("Invalid Battery Install ID", err.Error())
		return
	}
	updated, err := r.client.UpdateOpenappaBatteryInstallWithResponse(ctx, installID, client.UpdateOpenappaBatteryInstallJSONRequestBody{Enabled: &enabled})
	if err != nil {
		diags.AddError("Battery Install Update Failed", err.Error())
		return
	}
	if updated.StatusCode() != http.StatusOK {
		diags.AddError("Battery Install Update Failed", fmt.Sprintf("Status %d: %s", updated.StatusCode(), string(updated.Body)))
	}
}

func (r *OpenappaBatteryInstallResource) findInstallID(ctx context.Context, batteryName string, catalogID types.String, diags *diag.Diagnostics) string {
	listed, err := r.client.GetOpenappaBatteriesWithResponse(ctx)
	if err != nil {
		diags.AddError("Battery List Failed", err.Error())
		return ""
	}
	if listed.StatusCode() != http.StatusOK {
		diags.AddError("Battery List Failed", fmt.Sprintf("Status %d: %s", listed.StatusCode(), string(listed.Body)))
		return ""
	}
	var summaries []struct {
		Installs []struct {
			ID          string  `json:"id"`
			BatteryName string  `json:"batteryName"`
			CatalogID   *string `json:"catalogId"`
		} `json:"installs"`
	}
	if err := json.Unmarshal(listed.Body, &summaries); err != nil {
		diags.AddError("Battery List Failed", "Unexpected response: "+err.Error())
		return ""
	}
	for _, summary := range summaries {
		for _, install := range summary.Installs {
			if install.BatteryName != batteryName {
				continue
			}
			if catalogID.IsNull() && install.CatalogID == nil ||
				!catalogID.IsNull() && install.CatalogID != nil && *install.CatalogID == catalogID.ValueString() {
				return install.ID
			}
		}
	}
	return ""
}

func nullOpenappaBatteryInstallComputed(data *OpenappaBatteryInstallResourceModel) {
	data.Status = types.StringNull()
	data.Target = types.StringNull()
	data.LastError = types.StringNull()
	data.CredentialBindings = types.MapNull(types.StringType)
	data.CreatedAt = types.StringNull()
	data.UpdatedAt = types.StringNull()
}

func nonNilStringMap(values map[string]string) map[string]string {
	if values == nil {
		return map[string]string{}
	}
	return values
}

var sha256HexRegexp = regexp.MustCompile(`^[0-9a-f]{64}$`)
