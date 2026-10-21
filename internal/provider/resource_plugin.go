package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework-validators/mapvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"sort"
	"strings"
)

func NewPluginResource() resource.Resource { return &PluginResource{} }

type PluginResource struct {
	client *client.ClientWithResponses
}

var _ resource.ResourceWithImportState = &PluginResource{}

type PluginResourceModel struct {
	GitSource                   types.Object `tfsdk:"git_source"`
	ID                          types.String `tfsdk:"id"`
	DisplayName                 types.String `tfsdk:"display_name"`
	Description                 types.String `tfsdk:"description"`
	Icon                        types.String `tfsdk:"icon"`
	ClientType                  types.String `tfsdk:"client_type"`
	SupportedPlatforms          types.Set    `tfsdk:"supported_platforms"`
	Files                       types.Map    `tfsdk:"files"`
	FileDigests                 types.Map    `tfsdk:"file_digests"`
	Labels                      types.Set    `tfsdk:"labels"`
	Enabled                     types.Bool   `tfsdk:"enabled"`
	InitialGrants               types.Set    `tfsdk:"permissions"`
	OrganizationID              types.String `tfsdk:"organization_id"`
	AuthorID                    types.String `tfsdk:"author_id"`
	Scope                       types.String `tfsdk:"scope"`
	PluginSlug                  types.String `tfsdk:"plugin_slug"`
	ContentHash                 types.String `tfsdk:"content_hash"`
	SourceKind                  types.String `tfsdk:"source_kind"`
	SourceRef                   types.String `tfsdk:"source_ref"`
	SourceMarketplaceRepo       types.String `tfsdk:"source_marketplace_repo"`
	SourceMarketplacePath       types.String `tfsdk:"source_marketplace_path"`
	SourceMarketplacePluginName types.String `tfsdk:"source_marketplace_plugin_name"`
	SourceOrigin                types.String `tfsdk:"source_origin"`
	LastSyncedAt                types.String `tfsdk:"last_synced_at"`
	LastSyncError               types.String `tfsdk:"last_sync_error"`
	PendingSourceSHA            types.String `tfsdk:"pending_source_sha"`
	PendingContentHash          types.String `tfsdk:"pending_content_hash"`
	PendingDetectedAt           types.String `tfsdk:"pending_detected_at"`
	SourceID                    types.String `tfsdk:"source_id"`
	ApprovedContentHash         types.String `tfsdk:"approved_content_hash"`
	ApprovedAt                  types.String `tfsdk:"approved_at"`
	ApprovedBy                  types.String `tfsdk:"approved_by"`
	CreatedByServiceAccountID   types.String `tfsdk:"created_by_service_account_id"`
	CreatedAt                   types.String `tfsdk:"created_at"`
	UpdatedAt                   types.String `tfsdk:"updated_at"`
	DeletedAt                   types.String `tfsdk:"deleted_at"`
	Teams                       types.String `tfsdk:"teams"`
	Users                       types.String `tfsdk:"users"`
	CreatedBy                   types.String `tfsdk:"created_by"`
}

var pluginFileObjectType = types.ObjectType{AttrTypes: map[string]attr.Type{"content": types.StringType, "encoding": types.StringType, "mode": types.StringType}}

type pluginFileModel struct {
	Content  types.String `tfsdk:"content"`
	Encoding types.String `tfsdk:"encoding"`
	Mode     types.String `tfsdk:"mode"`
}

var pluginAttrSpec = []AttrSpec{
	{TFName: "display_name", JSONName: "displayName", Kind: Scalar},
	{TFName: "description", JSONName: "description", Kind: Scalar},
	{TFName: "icon", JSONName: "icon", Kind: Scalar},
	{TFName: "client_type", JSONName: "clientType", Kind: Scalar},
	{TFName: "supported_platforms", JSONName: "supportedPlatforms", Kind: Set},
	{TFName: "enabled", JSONName: "enabled", Kind: Scalar, OmitOnNull: true},
	{TFName: "files", JSONName: "files", Kind: Map, Encoder: encodePluginFiles, Children: []AttrSpec{
		{TFName: "content", JSONName: "content", Kind: Scalar}, {TFName: "encoding", JSONName: "encoding", Kind: Scalar}, {TFName: "mode", JSONName: "mode", Kind: Scalar},
	}}, credentialLabelsSpec, initialGrantsAttrSpec,
	{TFName: "git_source", JSONName: "githubSource", Kind: AtomicObject, Children: pluginGitSourceSpecs},
}

func (r *PluginResource) AttrSpecs() []AttrSpec               { return pluginAttrSpec }
func (r *PluginResource) APIShape() any                       { return client.GetPluginResponse{} }
func (r *PluginResource) KnownIntentionallySkipped() []string { return nil }
func (r *PluginResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_plugin"
}
func (r *PluginResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{MarkdownDescription: "Plugin package for a supported client. Choose authored `files` or a pinned `git_source`. Files are stored as opaque bytes without executing hooks. Changing Git tracking settings preserves the approved package; changing the approved commit or selection replaces it. Derived skills remain owned by the package.", Attributes: map[string]schema.Attribute{
		"id":                  schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, MarkdownDescription: "Plugin UUID."},
		"display_name":        schema.StringAttribute{Required: true, Validators: []validator.String{stringvalidator.LengthBetween(1, 120), pluginDisplayNameValidator{}}, MarkdownDescription: "Plugin display name."},
		"description":         schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""), MarkdownDescription: "Package description."},
		"icon":                schema.StringAttribute{Optional: true, MarkdownDescription: "Emoji or base64 image URL; omission clears the override."},
		"client_type":         schema.StringAttribute{Required: true, Validators: []validator.String{stringvalidator.OneOf("claude-code", "claude-desktop", "copilot-cli", "codex", "cursor")}, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}, MarkdownDescription: "Immutable target client format."},
		"supported_platforms": schema.SetAttribute{Required: true, ElementType: types.StringType, Validators: []validator.Set{setvalidator.SizeAtLeast(1), setvalidator.ValueStringsAre(stringvalidator.OneOf("posix", "windows"))}, MarkdownDescription: "Supported operating-system families."},
		"files": schema.MapNestedAttribute{Optional: true, Computed: true, Validators: []validator.Map{mapvalidator.ExactlyOneOf(path.MatchRoot("git_source")), mapvalidator.SizeBetween(1, 100), pluginPathsValidator{}}, MarkdownDescription: "Complete plugin file set keyed by safe relative path.", NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
			"content":  schema.StringAttribute{Required: true, MarkdownDescription: "UTF-8 text or canonical base64 bytes."},
			"encoding": schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("utf8"), Validators: []validator.String{stringvalidator.OneOf("utf8", "base64")}, MarkdownDescription: "Content encoding."},
			"mode":     schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("100644"), Validators: []validator.String{stringvalidator.OneOf("100644", "100755")}, MarkdownDescription: "Git file mode, including executable 100755."},
		}}},
		"file_digests":                   schema.MapAttribute{Computed: true, ElementType: types.StringType, MarkdownDescription: "Observed per-file digests keyed by path."},
		"labels":                         credentialLabelsAttribute(),
		"enabled":                        schema.BoolAttribute{Optional: true, Computed: true, PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()}, MarkdownDescription: "Whether the package is enabled; omission preserves the current setting."},
		"permissions":                    initialGrantsAttribute(),
		"organization_id":                schema.StringAttribute{Computed: true, MarkdownDescription: "Observed organization id."},
		"author_id":                      schema.StringAttribute{Computed: true, MarkdownDescription: "Observed author id."},
		"scope":                          schema.StringAttribute{Computed: true, MarkdownDescription: "Observed scope."},
		"plugin_slug":                    schema.StringAttribute{Computed: true, MarkdownDescription: "Observed plugin slug."},
		"content_hash":                   schema.StringAttribute{Computed: true, MarkdownDescription: "Observed content hash."},
		"source_kind":                    schema.StringAttribute{Computed: true, MarkdownDescription: "Observed source kind."},
		"source_ref":                     schema.StringAttribute{Computed: true, MarkdownDescription: "Observed source ref."},
		"source_marketplace_repo":        schema.StringAttribute{Computed: true, MarkdownDescription: "Observed source marketplace repo."},
		"source_marketplace_path":        schema.StringAttribute{Computed: true, MarkdownDescription: "Observed source marketplace path."},
		"source_marketplace_plugin_name": schema.StringAttribute{Computed: true, MarkdownDescription: "Observed source marketplace plugin name."},
		"source_origin":                  schema.StringAttribute{Computed: true, MarkdownDescription: "Observed source origin."},
		"last_synced_at":                 schema.StringAttribute{Computed: true, MarkdownDescription: "Observed last synced at."},
		"last_sync_error":                schema.StringAttribute{Computed: true, MarkdownDescription: "Observed last sync error."},
		"pending_source_sha":             schema.StringAttribute{Computed: true, MarkdownDescription: "Observed pending source sha."},
		"pending_content_hash":           schema.StringAttribute{Computed: true, MarkdownDescription: "Observed pending content hash."},
		"pending_detected_at":            schema.StringAttribute{Computed: true, MarkdownDescription: "Observed pending detected at."},
		"source_id":                      schema.StringAttribute{Computed: true, MarkdownDescription: "Observed source id."},
		"approved_content_hash":          schema.StringAttribute{Computed: true, MarkdownDescription: "Observed approved content hash."},
		"approved_at":                    schema.StringAttribute{Computed: true, MarkdownDescription: "Observed approved at."},
		"approved_by":                    schema.StringAttribute{Computed: true, MarkdownDescription: "Observed approved by."},
		"created_by_service_account_id":  schema.StringAttribute{Computed: true, MarkdownDescription: "Observed created by service account id."},
		"created_at":                     schema.StringAttribute{Computed: true, MarkdownDescription: "Observed created at."},
		"updated_at":                     schema.StringAttribute{Computed: true, MarkdownDescription: "Observed updated at."},
		"deleted_at":                     schema.StringAttribute{Computed: true, MarkdownDescription: "Observed deleted at."},
		"teams":                          schema.StringAttribute{Computed: true, MarkdownDescription: "teams metadata as JSON."},
		"users":                          schema.StringAttribute{Computed: true, MarkdownDescription: "users metadata as JSON."},
		"created_by":                     schema.StringAttribute{Computed: true, MarkdownDescription: "createdBy metadata as JSON."},
	}}
	resp.Schema.Attributes["git_source"] = schema.SingleNestedAttribute{Optional: true, Attributes: gitPluginAttributes(), PlanModifiers: []planmodifier.Object{pluginSourceKindPlanModifier{}}, MarkdownDescription: "Pinned Git content source. Configure this instead of files. Candidate synchronization never approves new executable bytes."}
}
func (r *PluginResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
func (r *PluginResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	defer finishParentCreate(ctx, r.client, "plugin", "id", resp)

	var data PluginResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !data.GitSource.IsNull() {
		r.createGit(ctx, req, resp)
		return
	}
	patch := MergePatch(ctx, req.Plan.Raw, tftypes.NewValue(req.Plan.Raw.Type(), nil), pluginAttrSpec, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	delete(patch, "enabled")
	LogPatch(ctx, "archestra_plugin Create", patch, pluginAttrSpec)
	body, err := json.Marshal(patch)
	if err != nil {
		resp.Diagnostics.AddError("Request Encoding Failed", err.Error())
		return
	}
	result, err := r.client.CreatePluginWithBodyWithResponse(ctx, "application/json", bytes.NewReader(body))
	if err != nil {
		resp.Diagnostics.AddError("API Error", err.Error())
		return
	}
	if result.JSON200 == nil {
		resp.Diagnostics.AddError("Creation Failed", fmt.Sprintf("API returned status %d", result.StatusCode()))
		return
	}
	wantedEnabled := data.Enabled
	r.flatten(ctx, &data, result.Body, &resp.Diagnostics)
	resolveInitialGrants(&data.InitialGrants)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !wantedEnabled.IsNull() && !wantedEnabled.IsUnknown() && !wantedEnabled.Equal(data.Enabled) {
		if !r.update(ctx, &data, map[string]any{"enabled": wantedEnabled.ValueBool()}, &resp.Diagnostics) {
			return
		}
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
	}
}
func (r *PluginResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data PluginResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if data.ID.IsNull() || data.ID.ValueString() == "" {
		resp.State.RemoveResource(ctx)
		return
	}
	id, err := uuid.Parse(data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid ID", err.Error())
		return
	}
	result, err := r.client.GetPluginWithResponse(ctx, id)
	if err != nil {
		resp.Diagnostics.AddError("API Error", err.Error())
		return
	}
	if IsNotFound(result) {
		resp.State.RemoveResource(ctx)
		return
	}
	if result.JSON200 == nil {
		resp.Diagnostics.AddError("Read Failed", fmt.Sprintf("API returned status %d", result.StatusCode()))
		return
	}
	r.flatten(ctx, &data, result.Body, &resp.Diagnostics)
	readParentPermissions(ctx, r.client, "plugin", data.ID.ValueString(), &data.InitialGrants, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
func (r *PluginResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	finishPermissions := beginParentPermissionUpdate(ctx, r.client, "plugin", "id", req, resp)
	if resp.Diagnostics.HasError() {
		return
	}
	defer finishPermissions()

	var data PluginResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !data.GitSource.IsNull() {
		r.updateGit(ctx, req, resp)
		return
	}
	patch := MergePatch(ctx, req.Plan.Raw, req.State.Raw, pluginAttrSpec, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	delete(patch, "initialGrants")
	delete(patch, "clientType")
	if len(patch) == 0 {
		resp.State = req.State
		return
	}
	if !r.update(ctx, &data, patch, &resp.Diagnostics) {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
func (r *PluginResource) update(ctx context.Context, data *PluginResourceModel, patch map[string]any, diags *diag.Diagnostics) bool {
	id, err := uuid.Parse(data.ID.ValueString())
	if err != nil {
		diags.AddError("Invalid ID", err.Error())
		return false
	}
	LogPatch(ctx, "Plugin Update", patch, r.AttrSpecs())
	body, err := json.Marshal(patch)
	if err != nil {
		diags.AddError("Request Encoding Failed", err.Error())
		return false
	}
	result, err := r.client.UpdatePluginWithBodyWithResponse(ctx, id, "application/json", bytes.NewReader(body))
	if err != nil {
		diags.AddError("API Error", err.Error())
		return false
	}
	if result.JSON200 == nil {
		diags.AddError("Update Failed", fmt.Sprintf("API returned status %d", result.StatusCode()))
		return false
	}
	r.flatten(ctx, data, result.Body, diags)
	return !diags.HasError()
}
func (r *PluginResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data PluginResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	id, err := uuid.Parse(data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid ID", err.Error())
		return
	}
	result, err := r.client.DeletePluginWithResponse(ctx, id)
	if err != nil {
		resp.Diagnostics.AddError("API Error", err.Error())
		return
	}
	if result.StatusCode() != 200 && result.StatusCode() != 204 && !IsNotFound(result) {
		resp.Diagnostics.AddError("Delete Failed", fmt.Sprintf("API returned status %d", result.StatusCode()))
	}
}
func (r *PluginResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
func (r *PluginResource) flatten(ctx context.Context, data *PluginResourceModel, body []byte, diags *diag.Diagnostics) {
	var value struct {
		ID                 string   `json:"id"`
		DisplayName        string   `json:"displayName"`
		Description        string   `json:"description"`
		Icon               *string  `json:"icon"`
		ClientType         string   `json:"clientType"`
		SupportedPlatforms []string `json:"supportedPlatforms"`
		Enabled            bool     `json:"enabled"`
		Files              []struct {
			Path     string `json:"path"`
			Content  string `json:"content"`
			Encoding string `json:"encoding"`
			Mode     string `json:"mode"`
			Digest   string `json:"digest"`
		} `json:"files"`
		Labels []struct {
			Key   string `json:"key"`
			Value string `json:"value"`
		} `json:"labels"`
		OrganizationID              *string         `json:"organizationId"`
		AuthorID                    *string         `json:"authorId"`
		Scope                       *string         `json:"scope"`
		PluginSlug                  *string         `json:"pluginSlug"`
		ContentHash                 *string         `json:"contentHash"`
		SourceKind                  *string         `json:"sourceKind"`
		SourceRepo                  *string         `json:"sourceRepo"`
		SourceRef                   *string         `json:"sourceRef"`
		SourceSHA                   *string         `json:"sourceSha"`
		SourceSubdir                *string         `json:"sourceSubdir"`
		SourceExclude               []string        `json:"sourceExclude"`
		SourceMarketplaceRepo       *string         `json:"sourceMarketplaceRepo"`
		SourceMarketplacePath       *string         `json:"sourceMarketplacePath"`
		SourceMarketplacePluginName *string         `json:"sourceMarketplacePluginName"`
		GithubSyncInterval          *string         `json:"githubSyncInterval"`
		GithubSyncRef               *string         `json:"githubSyncRef"`
		GithubAppConfigID           *string         `json:"githubAppConfigId"`
		GithubPatID                 *string         `json:"githubPatId"`
		SourceProvider              *string         `json:"sourceProvider"`
		SourceOrigin                *string         `json:"sourceOrigin"`
		GitlabTokenID               *string         `json:"gitlabTokenId"`
		LastSyncedAt                *string         `json:"lastSyncedAt"`
		LastSyncError               *string         `json:"lastSyncError"`
		PendingSourceSHA            *string         `json:"pendingSourceSha"`
		PendingContentHash          *string         `json:"pendingContentHash"`
		PendingDetectedAt           *string         `json:"pendingDetectedAt"`
		SourceID                    *string         `json:"sourceId"`
		ApprovedContentHash         *string         `json:"approvedContentHash"`
		ApprovedAt                  *string         `json:"approvedAt"`
		ApprovedBy                  *string         `json:"approvedBy"`
		CreatedByServiceAccountID   *string         `json:"createdByServiceAccountId"`
		CreatedAt                   *string         `json:"createdAt"`
		UpdatedAt                   *string         `json:"updatedAt"`
		DeletedAt                   *string         `json:"deletedAt"`
		Teams                       json.RawMessage `json:"teams"`
		Users                       json.RawMessage `json:"users"`
		CreatedBy                   json.RawMessage `json:"createdBy"`
	}
	if err := json.Unmarshal(body, &value); err != nil {
		diags.AddError("Invalid API Response", err.Error())
		return
	}
	if value.SourceKind == nil || (*value.SourceKind != "manual" && *value.SourceKind != "github") {
		diags.AddError("Unexpected plugin source", "Only authored files and Git sources are supported for plugin management.")
		return
	}
	data.ID = types.StringValue(value.ID)
	data.DisplayName = types.StringValue(value.DisplayName)
	data.Description = types.StringValue(value.Description)
	data.Icon = types.StringPointerValue(value.Icon)
	data.ClientType = types.StringValue(value.ClientType)
	data.Enabled = types.BoolValue(value.Enabled)
	var d diag.Diagnostics
	data.SupportedPlatforms, d = types.SetValueFrom(ctx, types.StringType, value.SupportedPlatforms)
	diags.Append(d...)
	files := map[string]pluginFileModel{}
	digests := map[string]string{}
	for _, file := range value.Files {
		files[file.Path] = pluginFileModel{Content: types.StringValue(file.Content), Encoding: types.StringValue(file.Encoding), Mode: types.StringValue(file.Mode)}
		digests[file.Path] = file.Digest
	}
	data.Files, d = types.MapValueFrom(ctx, pluginFileObjectType, files)
	diags.Append(d...)
	data.FileDigests, d = types.MapValueFrom(ctx, types.StringType, digests)
	diags.Append(d...)
	labels := make([]AgentLabelModel, len(value.Labels))
	for i, label := range value.Labels {
		labels[i] = AgentLabelModel{Key: types.StringValue(label.Key), Value: types.StringValue(label.Value)}
	}
	data.Labels, d = types.SetValueFrom(ctx, credentialLabelType, labels)
	diags.Append(d...)
	data.OrganizationID = types.StringPointerValue(value.OrganizationID)
	data.AuthorID = types.StringPointerValue(value.AuthorID)
	data.Scope = types.StringPointerValue(value.Scope)
	data.PluginSlug = types.StringPointerValue(value.PluginSlug)
	data.ContentHash = types.StringPointerValue(value.ContentHash)
	data.SourceKind = types.StringPointerValue(value.SourceKind)
	data.GitSource = types.ObjectNull(pluginGitSourceType.AttrTypes)
	if *value.SourceKind == "github" {
		exclude, d := types.ListValueFrom(ctx, types.StringType, value.SourceExclude)
		diags.Append(d...)
		source := pluginGitSourceModel{Repo: types.StringPointerValue(value.SourceRepo), CommitSHA: types.StringPointerValue(value.SourceSHA), Subdir: types.StringPointerValue(value.SourceSubdir), Exclude: exclude, Provider: types.StringPointerValue(value.SourceProvider), TrackingRef: types.StringPointerValue(value.GithubSyncRef), SyncInterval: types.StringPointerValue(value.GithubSyncInterval), GitHubAppID: types.StringPointerValue(value.GithubAppConfigID), GitHubPATID: types.StringPointerValue(value.GithubPatID), GitLabTokenID: types.StringPointerValue(value.GitlabTokenID)}
		data.GitSource, d = types.ObjectValueFrom(ctx, pluginGitSourceType.AttrTypes, source)
		diags.Append(d...)
	}
	data.SourceRef = types.StringPointerValue(value.SourceRef)
	data.SourceMarketplaceRepo = types.StringPointerValue(value.SourceMarketplaceRepo)
	data.SourceMarketplacePath = types.StringPointerValue(value.SourceMarketplacePath)
	data.SourceMarketplacePluginName = types.StringPointerValue(value.SourceMarketplacePluginName)
	data.SourceOrigin = types.StringPointerValue(value.SourceOrigin)
	data.LastSyncedAt = canonicalTokenTime(value.LastSyncedAt)
	data.LastSyncError = types.StringPointerValue(value.LastSyncError)
	data.PendingSourceSHA = types.StringPointerValue(value.PendingSourceSHA)
	data.PendingContentHash = types.StringPointerValue(value.PendingContentHash)
	data.PendingDetectedAt = canonicalTokenTime(value.PendingDetectedAt)
	data.SourceID = types.StringPointerValue(value.SourceID)
	data.ApprovedContentHash = types.StringPointerValue(value.ApprovedContentHash)
	data.ApprovedAt = canonicalTokenTime(value.ApprovedAt)
	data.ApprovedBy = types.StringPointerValue(value.ApprovedBy)
	data.CreatedByServiceAccountID = types.StringPointerValue(value.CreatedByServiceAccountID)
	data.CreatedAt = canonicalTokenTime(value.CreatedAt)
	data.UpdatedAt = canonicalTokenTime(value.UpdatedAt)
	data.DeletedAt = canonicalTokenTime(value.DeletedAt)
	data.Teams = credentialJSONValue(value.Teams, diags)
	data.Users = credentialJSONValue(value.Users, diags)
	data.CreatedBy = credentialJSONValue(value.CreatedBy, diags)
}
func encodePluginFiles(value any) any {
	files, ok := value.(map[string]any)
	if !ok {
		return value
	}
	paths := make([]string, 0, len(files))
	for path := range files {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	result := make([]map[string]any, 0, len(paths))
	for _, path := range paths {
		file, ok := files[path].(map[string]any)
		if !ok {
			continue
		}
		file["path"] = path
		result = append(result, file)
	}
	return result
}

type pluginPathsValidator struct{}

func (pluginPathsValidator) Description(context.Context) string {
	return "Plugin files use safe relative paths without traversal or case-insensitive duplicates."
}
func (v pluginPathsValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}
func (pluginPathsValidator) ValidateMap(_ context.Context, req validator.MapRequest, resp *validator.MapResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	seen := map[string]bool{}
	for key := range req.ConfigValue.Elements() {
		bad := key == "" || strings.ContainsAny(key, "\\\x00") || strings.HasPrefix(key, "/") || strings.HasSuffix(key, "/") || strings.Contains(key, "//")
		for _, part := range strings.Split(key, "/") {
			bad = bad || part == "." || part == ".."
		}
		if bad || seen[strings.ToLower(key)] {
			resp.Diagnostics.AddAttributeError(req.Path.AtMapKey(key), "Invalid plugin file path", "Use a unique safe relative path without traversal, backslashes or case-insensitive duplicates.")
		}
		seen[strings.ToLower(key)] = true
	}
}

type pluginDisplayNameValidator struct{}

func (pluginDisplayNameValidator) Description(context.Context) string {
	return "Display names omit leading and trailing whitespace."
}
func (v pluginDisplayNameValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}
func (pluginDisplayNameValidator) ValidateString(_ context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	value := req.ConfigValue.ValueString()
	if value != strings.TrimSpace(value) {
		resp.Diagnostics.AddAttributeError(req.Path, "Noncanonical display name", "Use a name without leading or trailing whitespace so the API can return it unchanged.")
	}
}

func (r *PluginResource) ModifyPlan(_ context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	preserveManagedNoopPlan(req, resp)
}
