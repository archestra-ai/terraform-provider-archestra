package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"regexp"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

var _ resource.ResourceWithImportState = &AppResource{}
var _ resource.ResourceWithModifyPlan = &AppResource{}
var _ resource.ResourceWithValidateConfig = &AppResource{}

func NewAppResource() resource.Resource { return &AppResource{} }

type AppResource struct{ client *client.ClientWithResponses }
type AppResourceModel struct {
	ID                        types.String      `tfsdk:"id"`
	Name                      types.String      `tfsdk:"name"`
	Slug                      types.String      `tfsdk:"slug"`
	Description               types.String      `tfsdk:"description"`
	HTML                      types.String      `tfsdk:"html"`
	BrowserPermissions        types.Set         `tfsdk:"browser_permissions"`
	EnvironmentID             types.String      `tfsdk:"environment_id"`
	Icon                      types.String      `tfsdk:"icon"`
	Labels                    []AgentLabelModel `tfsdk:"labels"`
	Enabled                   types.Bool        `tfsdk:"enabled"`
	Locked                    types.Bool        `tfsdk:"locked"`
	OpenInFullscreen          types.Bool        `tfsdk:"open_in_fullscreen"`
	Tools                     types.Set         `tfsdk:"tools"`
	Permissions               types.Set         `tfsdk:"permissions"`
	ContentVersion            types.Int64       `tfsdk:"content_version"`
	OrganizationID            types.String      `tfsdk:"organization_id"`
	AuthorID                  types.String      `tfsdk:"author_id"`
	AuthorName                types.String      `tfsdk:"author_name"`
	ViewerRole                types.String      `tfsdk:"viewer_role"`
	CreatedByServiceAccountID types.String      `tfsdk:"created_by_service_account_id"`
	BackingServerID           types.String      `tfsdk:"backing_server_id"`
	CreatedAt                 types.String      `tfsdk:"created_at"`
	UpdatedAt                 types.String      `tfsdk:"updated_at"`
}

var appAttrSpec = []AttrSpec{
	initialGrantsAttrSpec,
	{TFName: "name", JSONName: "name", Kind: Scalar},
	{TFName: "slug", JSONName: "slug", Kind: Scalar, OmitOnNull: true},
	{TFName: "description", JSONName: "description", Kind: Scalar},
	{TFName: "html", Kind: Synthetic, Sensitive: true},
	{TFName: "browser_permissions", Kind: Synthetic},
	{TFName: "environment_id", JSONName: "environmentId", Kind: Scalar},
	{TFName: "icon", JSONName: "icon", Kind: Scalar},
	{TFName: "labels", JSONName: "labels", Kind: Set, Children: []AttrSpec{{TFName: "key", JSONName: "key", Kind: Scalar}, {TFName: "value", JSONName: "value", Kind: Scalar}}},
	{TFName: "enabled", Kind: Synthetic},
	{TFName: "locked", Kind: Synthetic},
	{TFName: "open_in_fullscreen", JSONName: "openInFullscreen", Kind: Scalar},
	{TFName: "tools", Kind: Synthetic},
	{TFName: "content_version", ReadJSONName: "latestVersion", Kind: Synthetic},
	{TFName: "backing_server_id", ReadJSONName: "mcpServerId", Kind: Synthetic},
}

func (r *AppResource) AttrSpecs() []AttrSpec { return appAttrSpec }
func (r *AppResource) APIShape() any         { return client.GetAppResponse{} }
func (r *AppResource) KnownIntentionallySkipped() []string {
	// spec is authored through the refine tool and has no REST setter. Other
	// fields describe provenance, soft deletion or per-viewer presentation.
	return []string{"scope", "templateId", "spec", "deletedAt", "createdBy", "teams", "users"}
}
func (r *AppResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_app"
}
func (r *AppResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	tools := agentToolsAttribute()
	tools.PlanModifiers = nil
	tools.MarkdownDescription = "Complete upstream tool assignments, including server pins and credential modes. Omit to preserve assignments; [] clears them. Each tool UUID may appear once."
	resp.Schema = schema.Schema{MarkdownDescription: "An Archestra App with its HTML, settings, sharing and upstream tools. Content versions are immutable revisions of this App. Omit HTML to preserve the existing content or use the platform starter on creation. Updates use content-version preconditions. Locked Apps must be explicitly unlocked before content changes or destruction. Creating an App does not open a conversation. Requires API 1.4.0-rc.50 or later.", Attributes: map[string]schema.Attribute{
		"id":                  schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
		"name":                schema.StringAttribute{Required: true, Validators: []validator.String{stringvalidator.LengthBetween(1, 100)}},
		"slug":                schema.StringAttribute{Optional: true, Computed: true, MarkdownDescription: "URL segment, generated from the name when omitted. Changing it breaks links that use the old slug; the UUID remains stable.", Validators: []validator.String{stringvalidator.LengthBetween(1, 100), stringvalidator.RegexMatches(regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`), "Use lowercase letters, numbers and single hyphens."), stringvalidator.NoneOf("catalog"), appSlugValidator{}}},
		"description":         schema.StringAttribute{Optional: true, Validators: []validator.String{stringvalidator.LengthAtMost(500)}},
		"html":                schema.StringAttribute{Optional: true, Computed: true, Sensitive: true, MarkdownDescription: "Complete UTF-8 HTML document, at most 512 KiB. Omit to preserve content. Stored in sensitive Terraform state; use file() to load a local document.", Validators: []validator.String{stringvalidator.LengthAtLeast(1), appHTMLValidator{}}},
		"browser_permissions": schema.SetAttribute{Optional: true, Computed: true, ElementType: types.StringType, MarkdownDescription: "Versioned browser permissions: camera, microphone, geolocation and clipboard_write. Omit to preserve; [] clears. A permission-only edit creates a content revision using the current HTML.", Validators: []validator.Set{setvalidator.ValueStringsAre(stringvalidator.OneOf("camera", "microphone", "geolocation", "clipboard_write"))}},
		"environment_id":      schema.StringAttribute{Optional: true, MarkdownDescription: "Deployment environment UUID. Omission or removal explicitly selects the organization default environment.", Validators: []validator.String{stringvalidator.RegexMatches(canonicalUUIDRegexp, "Use a lowercase canonical environment UUID.")}},
		"icon":                schema.StringAttribute{Optional: true, Validators: []validator.String{stringvalidator.LengthAtMost(1024 * 1024)}},
		"labels":              schema.SetNestedAttribute{Optional: true, Computed: true, MarkdownDescription: "Key/value labels; omission preserves them and [] clears them.", NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{"key": schema.StringAttribute{Required: true}, "value": schema.StringAttribute{Required: true}}}},
		"enabled":             schema.BoolAttribute{Optional: true, Computed: true, MarkdownDescription: "Whether the App is live. Omission preserves the observed value and creation honors organization defaults. Explicitly enable before editing disabled content."},
		"locked":              schema.BoolAttribute{Optional: true, Computed: true, MarkdownDescription: "Whether content edits and deletion are locked. Omission preserves the observed value. Explicitly unlock before content edits or destruction."},
		"open_in_fullscreen":  schema.BoolAttribute{Optional: true, Computed: true, MarkdownDescription: "Default initial display mode; omission preserves the observed setting."},
		"tools":               tools,
		"permissions":         initialGrantsAttribute(),
		"content_version":     schema.Int64Attribute{Computed: true, MarkdownDescription: "Observed immutable content revision; used as an optimistic concurrency precondition."},
		"organization_id":     schema.StringAttribute{Computed: true}, "author_id": schema.StringAttribute{Computed: true}, "backing_server_id": schema.StringAttribute{Computed: true, MarkdownDescription: "Platform-managed backing MCP server. The App owns its lifecycle."},
		"author_name": schema.StringAttribute{Computed: true}, "viewer_role": schema.StringAttribute{Computed: true}, "created_by_service_account_id": schema.StringAttribute{Computed: true},
		"created_at": schema.StringAttribute{Computed: true}, "updated_at": schema.StringAttribute{Computed: true},
	}}
}
func (r *AppResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.ClientWithResponses)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Resource Configure Type", fmt.Sprintf("Expected API client, got %T", req.ProviderData))
		return
	}
	r.client = c
}
func (r *AppResource) ModifyPlan(_ context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	preserveManagedNoopPlan(req, resp)
}
func (r *AppResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	validateInlineTools(ctx, req.Config, &resp.Diagnostics)
}

func (r *AppResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	defer finishParentCreate(ctx, r.client, "app", "id", resp)
	var desired AppResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &desired)...)
	if resp.Diagnostics.HasError() {
		return
	}
	patch := MergePatch(ctx, req.Plan.Raw, tftypes.NewValue(req.Plan.Raw.Type(), nil), appAttrSpec, &resp.Diagnostics)
	delete(patch, "openInFullscreen") // This setting is update-only.
	patch["openInChat"] = false
	// Explicit null selects the organization default; omitted would use a
	// potentially different configured landing environment for new Apps.
	if desired.EnvironmentID.IsNull() {
		patch["environmentId"] = nil
	}
	if !desired.HTML.IsNull() && !desired.HTML.IsUnknown() {
		patch["html"] = desired.HTML.ValueString()
	}
	if !desired.BrowserPermissions.IsNull() && !desired.BrowserPermissions.IsUnknown() {
		patch["uiPermissions"] = appPermissionRequest(ctx, desired.BrowserPermissions, &resp.Diagnostics)
	}
	if resp.Diagnostics.HasError() {
		return
	}
	body, err := json.Marshal(patch)
	if err != nil {
		resp.Diagnostics.AddError("Request encoding failed", err.Error())
		return
	}
	created, err := r.client.CreateAppWithBodyWithResponse(ctx, "application/json", bytes.NewReader(body))
	if err != nil {
		resp.Diagnostics.AddError("App creation failed", err.Error())
		return
	}
	if created.StatusCode() != 200 && created.StatusCode() != 201 {
		appRequestError(&resp.Diagnostics, "App creation", created.StatusCode())
		return
	}
	var row appMetadata
	if err := json.Unmarshal(created.Body, &row); err != nil || row.ID == "" || row.Version < 1 {
		resp.Diagnostics.AddError("Invalid App creation response", "The API did not return the created App identity and version.")
		return
	}
	data := desired
	applyAppMetadata(&data, row)
	if data.HTML.IsUnknown() {
		data.HTML = types.StringNull()
	}
	if data.BrowserPermissions.IsUnknown() {
		data.BrowserPermissions = types.SetNull(types.StringType)
	}
	if data.Tools.IsUnknown() {
		data.Tools = types.SetNull(agentInlineToolType)
	}
	resolveInitialGrants(&data.Permissions)
	// Retain the identity before any version, tool or settings follow-up.
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !r.refresh(ctx, &data, &resp.Diagnostics) {
		if !resp.Diagnostics.HasError() {
			resp.Diagnostics.AddError("App disappeared during creation", "The created identity remains in state for normal cleanup.")
		}
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !r.applyLifecycle(ctx, &desired, &data, true, &resp.Diagnostics) {
		return
	}
	if !desired.OpenInFullscreen.IsNull() && !desired.OpenInFullscreen.IsUnknown() && !desired.OpenInFullscreen.Equal(data.OpenInFullscreen) {
		if !r.patch(ctx, data.ID.ValueString(), map[string]any{"openInFullscreen": desired.OpenInFullscreen.ValueBool(), "baseVersion": data.ContentVersion.ValueInt64()}, &resp.Diagnostics) {
			return
		}
	}
	if !r.writeTools(ctx, &desired, &data, &resp.Diagnostics) {
		return
	}
	if !r.applyLifecycle(ctx, &desired, &data, false, &resp.Diagnostics) {
		return
	}
	if !r.refresh(ctx, &data, &resp.Diagnostics) {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
func (r *AppResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data AppResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !r.refresh(ctx, &data, &resp.Diagnostics) {
		if !resp.Diagnostics.HasError() {
			resp.State.RemoveResource(ctx)
		}
		return
	}
	readParentPermissions(ctx, r.client, "app", data.ID.ValueString(), &data.Permissions, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
func (r *AppResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	finish := beginParentPermissionUpdate(ctx, r.client, "app", "id", req, resp)
	defer finish()
	var desired, prior AppResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &desired)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &prior)...)
	if resp.Diagnostics.HasError() {
		return
	}
	observed := prior
	if !r.refresh(ctx, &observed, &resp.Diagnostics) {
		if !resp.Diagnostics.HasError() {
			resp.Diagnostics.AddError("App no longer exists", "Refresh before applying again.")
		}
		return
	}
	if !observed.ContentVersion.Equal(prior.ContentVersion) {
		resp.Diagnostics.AddError("App content version conflict", "App content changed after planning. Refresh and review the current version before applying again.")
		return
	}
	manageTools := !desired.Tools.IsNull() && !desired.Tools.IsUnknown() && !desired.Tools.Equal(prior.Tools)
	if manageTools && !prior.Tools.IsNull() && !prior.Tools.IsUnknown() && !observed.Tools.Equal(prior.Tools) {
		resp.Diagnostics.AddError("App tool assignments changed", "The assignments changed after planning. Refresh before replacing them.")
		return
	}
	patch := MergePatch(ctx, req.Plan.Raw, req.State.Raw, appAttrSpec, &resp.Diagnostics)
	if !desired.HTML.IsNull() && !desired.HTML.IsUnknown() && !desired.HTML.Equal(prior.HTML) {
		patch["html"] = desired.HTML.ValueString()
	}
	if !desired.BrowserPermissions.IsNull() && !desired.BrowserPermissions.IsUnknown() && !desired.BrowserPermissions.Equal(prior.BrowserPermissions) {
		patch["uiPermissions"] = appPermissionRequest(ctx, desired.BrowserPermissions, &resp.Diagnostics)
		if _, supplied := patch["html"]; !supplied {
			patch["html"] = observed.HTML.ValueString()
		}
	}
	if resp.Diagnostics.HasError() {
		return
	}
	if !r.applyLifecycle(ctx, &desired, &observed, true, &resp.Diagnostics) {
		return
	}
	if len(patch) > 0 {
		patch["baseVersion"] = prior.ContentVersion.ValueInt64()
		if !r.patch(ctx, prior.ID.ValueString(), patch, &resp.Diagnostics) {
			return
		}
	}
	if manageTools && !r.writeTools(ctx, &desired, &observed, &resp.Diagnostics) {
		return
	}
	if !r.applyLifecycle(ctx, &desired, &observed, false, &resp.Diagnostics) {
		return
	}
	if !r.refresh(ctx, &observed, &resp.Diagnostics) {
		return
	}
	observed.Permissions = desired.Permissions
	resp.Diagnostics.Append(resp.State.Set(ctx, &observed)...)
}
func (r *AppResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data AppResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	id, err := uuid.Parse(data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid App UUID", err.Error())
		return
	}
	if resp.Diagnostics.HasError() {
		return
	}
	deleted, err := r.client.DeleteAppWithResponse(ctx, id)
	if err != nil {
		resp.Diagnostics.AddError("App deletion failed", err.Error())
		return
	}
	if deleted.StatusCode() == 409 {
		resp.Diagnostics.AddError("App deletion blocked", "The App may be locked. Apply locked=false explicitly before destroying it. No implicit unlock was performed.")
		return
	}
	if deleted.StatusCode() != 200 && deleted.StatusCode() != 204 && deleted.StatusCode() != 404 {
		appRequestError(&resp.Diagnostics, "App deletion", deleted.StatusCode())
	}
}
func (r *AppResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	if !canonicalUUIDRegexp.MatchString(req.ID) {
		resp.Diagnostics.AddError("Invalid App UUID", "Use a lowercase canonical App UUID, not a name or slug.")
		return
	}
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

type appMetadata struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`
	Slug          *string `json:"slug"`
	Description   *string `json:"description"`
	EnvironmentID *string `json:"environmentId"`
	Icon          *string `json:"icon"`
	Labels        []struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	} `json:"labels"`
	Enabled                   bool    `json:"enabled"`
	Locked                    bool    `json:"locked"`
	Fullscreen                bool    `json:"openInFullscreen"`
	Version                   int64   `json:"latestVersion"`
	OrganizationID            string  `json:"organizationId"`
	AuthorID                  *string `json:"authorId"`
	AuthorName                *string `json:"authorName"`
	ViewerRole                *string `json:"viewerRole"`
	CreatedByServiceAccountID *string `json:"createdByServiceAccountId"`
	BackingServerID           *string `json:"mcpServerId"`
	CreatedAt                 string  `json:"createdAt"`
	UpdatedAt                 string  `json:"updatedAt"`
}

func applyAppMetadata(data *AppResourceModel, row appMetadata) {
	data.ID = types.StringValue(row.ID)
	data.Name = types.StringValue(row.Name)
	data.Slug = types.StringPointerValue(row.Slug)
	data.Description = types.StringPointerValue(row.Description)
	data.EnvironmentID = types.StringPointerValue(row.EnvironmentID)
	data.Icon = types.StringPointerValue(row.Icon)
	data.Labels = make([]AgentLabelModel, 0, len(row.Labels))
	for _, label := range row.Labels {
		data.Labels = append(data.Labels, AgentLabelModel{Key: types.StringValue(label.Key), Value: types.StringValue(label.Value)})
	}
	data.Enabled = types.BoolValue(row.Enabled)
	data.Locked = types.BoolValue(row.Locked)
	data.OpenInFullscreen = types.BoolValue(row.Fullscreen)
	data.ContentVersion = types.Int64Value(row.Version)
	data.OrganizationID = types.StringValue(row.OrganizationID)
	data.AuthorID = types.StringPointerValue(row.AuthorID)
	data.BackingServerID = types.StringPointerValue(row.BackingServerID)
	data.AuthorName = types.StringPointerValue(row.AuthorName)
	data.ViewerRole = types.StringPointerValue(row.ViewerRole)
	data.CreatedByServiceAccountID = types.StringPointerValue(row.CreatedByServiceAccountID)
	data.CreatedAt = types.StringValue(row.CreatedAt)
	data.UpdatedAt = types.StringValue(row.UpdatedAt)
}
func (r *AppResource) refresh(ctx context.Context, data *AppResourceModel, diags *diag.Diagnostics) bool {
	if data.ID.IsNull() || data.ID.IsUnknown() || data.ID.ValueString() == "" {
		return false
	}
	id, err := uuid.Parse(data.ID.ValueString())
	if err != nil {
		diags.AddError("Invalid App UUID", err.Error())
		return false
	}
	meta, err := r.client.GetAppWithResponse(ctx, id.String())
	if err != nil {
		diags.AddError("App read failed", err.Error())
		return false
	}
	if meta.StatusCode() == 404 {
		return false
	}
	if meta.StatusCode() != 200 {
		appRequestError(diags, "App read", meta.StatusCode())
		return false
	}
	var row appMetadata
	if err := json.Unmarshal(meta.Body, &row); err != nil || row.ID != id.String() || row.Version < 1 {
		diags.AddError("Invalid App response", "The API returned an unexpected identity or content version.")
		return false
	}
	content, err := r.client.GetAppVersionWithResponse(ctx, id, int(row.Version))
	if err != nil {
		diags.AddError("App content read failed", err.Error())
		return false
	}
	if content.StatusCode() != 200 {
		appRequestError(diags, "App content read; identity retained", content.StatusCode())
		return false
	}
	var version struct {
		AppID       string                     `json:"appId"`
		Version     int64                      `json:"version"`
		HTML        string                     `json:"html"`
		Permissions map[string]json.RawMessage `json:"uiPermissions"`
	}
	if err := json.Unmarshal(content.Body, &version); err != nil || version.AppID != row.ID || version.Version != row.Version || version.HTML == "" {
		diags.AddError("Invalid App content response", "The API did not return the requested immutable content version.")
		return false
	}
	permissions := make([]string, 0, len(version.Permissions))
	for key := range version.Permissions {
		switch key {
		case "camera", "microphone", "geolocation":
			permissions = append(permissions, key)
		case "clipboardWrite":
			permissions = append(permissions, "clipboard_write")
		default:
			diags.AddError("Unsupported App browser permission", "The API returned an unknown permission. Upgrade the provider before editing this App.")
			return false
		}
	}
	tools := r.readTools(ctx, id, diags)
	if diags.HasError() {
		return false
	}
	applyAppMetadata(data, row)
	data.HTML = types.StringValue(version.HTML)
	data.BrowserPermissions = FlattenStringSet(ctx, permissions, diags)
	data.Tools = tools
	return !diags.HasError()
}
func (r *AppResource) patch(ctx context.Context, rawID string, patch map[string]any, diags *diag.Diagnostics) bool {
	id, err := uuid.Parse(rawID)
	if err != nil {
		diags.AddError("Invalid App UUID", err.Error())
		return false
	}
	body, err := json.Marshal(patch)
	if err != nil {
		diags.AddError("App request encoding failed", err.Error())
		return false
	}
	updated, err := r.client.UpdateAppWithBodyWithResponse(ctx, id, "application/json", bytes.NewReader(body))
	if err != nil {
		diags.AddError("App update failed", err.Error())
		return false
	}
	if updated.StatusCode() != 200 {
		appRequestError(diags, "App update; refresh before retrying", updated.StatusCode())
		return false
	}
	return true
}
func appRequestError(diags *diag.Diagnostics, action string, status int) {
	diags.AddError(action+" failed", fmt.Sprintf("API returned status %d. Refresh and review the current App before retrying.", status))
}
func appPermissionRequest(ctx context.Context, value types.Set, diags *diag.Diagnostics) map[string]any {
	var values []string
	diags.Append(value.ElementsAs(ctx, &values, false)...)
	result := map[string]any{}
	for _, name := range values {
		if name == "clipboard_write" {
			name = "clipboardWrite"
		}
		result[name] = map[string]any{}
	}
	return result
}

type appSlugValidator struct{}

func (appSlugValidator) Description(context.Context) string               { return "An App slug cannot be a UUID." }
func (v appSlugValidator) MarkdownDescription(ctx context.Context) string { return v.Description(ctx) }
func (appSlugValidator) ValidateString(_ context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	if _, err := uuid.Parse(req.ConfigValue.ValueString()); err == nil {
		resp.Diagnostics.AddAttributeError(req.Path, "Invalid App slug", "Use a URL segment that is not a UUID.")
	}
}

type appHTMLValidator struct{}

func (appHTMLValidator) Description(context.Context) string {
	return "HTML must fit within 512 KiB of UTF-8 bytes."
}
func (v appHTMLValidator) MarkdownDescription(ctx context.Context) string { return v.Description(ctx) }
func (appHTMLValidator) ValidateString(_ context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	if len([]byte(req.ConfigValue.ValueString())) > 512*1024 {
		resp.Diagnostics.AddAttributeError(req.Path, "App HTML too large", "The UTF-8 document must be at most 512 KiB.")
	}
}
