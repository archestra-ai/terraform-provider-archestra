package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

var _ resource.Resource = &KnowledgeConnectorResource{}
var _ resource.ResourceWithImportState = &KnowledgeConnectorResource{}
var _ resource.ResourceWithValidateConfig = &KnowledgeConnectorResource{}

func NewKnowledgeConnectorResource() resource.Resource {
	return &KnowledgeConnectorResource{}
}

type KnowledgeConnectorResource struct {
	api        *client.ClientWithResponses
	connectors *client.KnowledgeConnectorClient
}

type KnowledgeConnectorResourceModel struct {
	InitialGrants                 types.Set            `tfsdk:"permissions"`
	Labels                        []AgentLabelModel    `tfsdk:"labels"`
	ID                            types.String         `tfsdk:"id"`
	Name                          types.String         `tfsdk:"name"`
	Description                   types.String         `tfsdk:"description"`
	ConnectorType                 types.String         `tfsdk:"connector_type"`
	Config                        jsontypes.Normalized `tfsdk:"config"`
	Credentials                   types.Object         `tfsdk:"credentials"`
	Schedule                      types.String         `tfsdk:"schedule"`
	FtsLanguage                   types.String         `tfsdk:"fts_language"`
	PermissionSyncIntervalSeconds types.Int64          `tfsdk:"permission_sync_interval_seconds"`
	Enabled                       types.Bool           `tfsdk:"enabled"`
	SyncPermissionsFromSource     types.Bool           `tfsdk:"sync_permissions_from_source"`
	EnvironmentID                 types.String         `tfsdk:"environment_id"`
	KnowledgeBaseIDs              types.Set            `tfsdk:"knowledge_base_ids"`
	CreatedAt                     types.String         `tfsdk:"created_at"`
}

// knowledgeConnectorTypes are the types the API lets clients create.
// `file_upload` is internal: the knowledge-files page creates it implicitly.
var knowledgeConnectorTypes = []string{
	"jira", "confluence", "github", "gitlab", "servicenow", "notion", "sharepoint", "gdrive",
	"dropbox", "onedrive", "asana", "linear", "outline", "salesforce", "web_crawler", "perforce", "mfiles",
}

var knowledgeConnectorFtsLanguages = []string{
	"simple", "arabic", "armenian", "basque", "catalan", "danish", "dutch", "english", "finnish",
	"french", "german", "greek", "hindi", "hungarian", "indonesian", "irish", "italian", "lithuanian",
	"nepali", "norwegian", "portuguese", "romanian", "russian", "serbian", "spanish", "swedish",
	"tamil", "turkish", "yiddish",
}

var knowledgeConnectorCredentialAttrTypes = map[string]attr.Type{
	"email":         types.StringType,
	"api_token":     types.StringType,
	"admin_api_key": types.StringType,
}

// knowledgeConnectorAttrSpec declares the create/update wire fields. `config`
// gets its `type` discriminator from connector_type after MergePatch; the
// connector type, initial knowledge bases and permission sync are create
// inputs (later knowledge-base changes use the assignment endpoints, and the
// update API takes permission sync as `visibility`).
var knowledgeConnectorAttrSpec = []AttrSpec{
	initialGrantsAttrSpec,
	{TFName: "labels", JSONName: "labels", Kind: Set, OmitOnNull: true, Children: []AttrSpec{{TFName: "key", JSONName: "key", Kind: Scalar}, {TFName: "value", JSONName: "value", Kind: Scalar}}},
	{TFName: "name", JSONName: "name", Kind: Scalar},
	{TFName: "description", JSONName: "description", Kind: Scalar},
	{TFName: "connector_type", JSONName: "connectorType", Kind: Scalar, CreateOnly: true},
	{TFName: "config", JSONName: "config", Kind: Scalar, Encoder: organizationJSONSetting},
	{TFName: "credentials", JSONName: "credentials", Kind: AtomicObject, OmitOnNull: true, Children: []AttrSpec{
		{TFName: "email", JSONName: "email", Kind: Scalar, OmitOnNull: true},
		{TFName: "api_token", JSONName: "apiToken", Kind: Scalar, Sensitive: true},
		{TFName: "admin_api_key", JSONName: "adminApiKey", Kind: Scalar, OmitOnNull: true, Sensitive: true},
	}},
	{TFName: "schedule", JSONName: "schedule", Kind: Scalar, OmitOnNull: true},
	{TFName: "fts_language", JSONName: "ftsLanguage", Kind: Scalar, OmitOnNull: true},
	{TFName: "permission_sync_interval_seconds", JSONName: "permissionSyncIntervalSeconds", Kind: Scalar, OmitOnNull: true},
	{TFName: "enabled", JSONName: "enabled", Kind: Scalar, OmitOnNull: true},
	{TFName: "sync_permissions_from_source", JSONName: "syncPermissionsFromSource", Kind: Scalar, OmitOnNull: true, CreateOnly: true},
	{TFName: "environment_id", JSONName: "environmentId", Kind: Scalar, OmitOnNull: true},
	{TFName: "knowledge_base_ids", JSONName: "knowledgeBaseIds", Kind: Set, OmitOnNull: true, CreateOnly: true},
}

func (r *KnowledgeConnectorResource) AttrSpecs() []AttrSpec { return knowledgeConnectorAttrSpec }
func (r *KnowledgeConnectorResource) APIShape() any {
	return client.GetKnowledgeConnectorResponse{}
}

// KnownIntentionallySkipped — wire fields not modeled on this resource:
//   - organizationId: implied by the API key.
//   - secretId: storage pointer for the write-only credentials.
//   - visibility/teamIds: legacy access projection; `permissions` owns access
//     and `sync_permissions_from_source` the permission-sync switch.
//   - checkpoint, aclConfigEpoch, lastSync*, lastPermissionSync*,
//     totalDocsIngested: sync bookkeeping that changes on every run.
//   - updatedAt: bumped by that bookkeeping, so it would churn every refresh.
//   - createdBy, createdByServiceAccountId: audit metadata.
//   - deletedAt: a deleted connector reads as gone (404).
func (r *KnowledgeConnectorResource) KnownIntentionallySkipped() []string {
	return []string{
		"organizationId", "secretId", "visibility", "teamIds", "checkpoint", "aclConfigEpoch",
		"lastSyncAt", "lastSyncStatus", "lastSyncError", "lastPermissionSyncAt", "lastPermissionSyncStatus",
		"totalDocsIngested", "updatedAt", "createdBy", "createdByServiceAccountId", "deletedAt",
	}
}

func (r *KnowledgeConnectorResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_knowledge_connector"
}

func (r *KnowledgeConnectorResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	sticky := "Omit to keep the current value; the API has no reset, so removing it from configuration leaves the server value in place."
	resp.Schema = schema.Schema{
		MarkdownDescription: "A knowledge connector syncs documents from an external source (Jira, Confluence, GitHub, Google Drive, a website, ...) into knowledge bases. " +
			"Terraform manages its configuration; documents, sync runs and per-member overrides stay in the UI. " +
			"Google Drive in individual (`oauth`) mode is created without credentials and must then be authorized in the UI. " +
			"Destroying the connector moves it to the trash, where an administrator can restore or purge it.",
		Attributes: map[string]schema.Attribute{
			"permissions": initialGrantsAttribute(),
			"labels":      schema.SetNestedAttribute{Optional: true, MarkdownDescription: "Connector labels.", NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{"key": schema.StringAttribute{Required: true}, "value": schema.StringAttribute{Required: true}}}},
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Connector identifier.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Connector name.",
				Validators:          []validator.String{stringvalidator.LengthAtLeast(1)},
			},
			"description": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Free-form description.",
			},
			"connector_type": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Source type: `" + strings.Join(knowledgeConnectorTypes, "`, `") + "`. Changing it replaces the connector.",
				Validators:          []validator.String{stringvalidator.OneOf(knowledgeConnectorTypes...)},
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"config": schema.StringAttribute{
				Required:   true,
				CustomType: jsontypes.NormalizedType{},
				MarkdownDescription: "Type-specific settings as a JSON object, usually written with `jsonencode()`, using the API's camelCase keys " +
					"(for example `{ startUrl = \"https://docs.example.com\", maxPages = 200 }` for `web_crawler`). " +
					"Leave out the `type` key: the provider adds it from `connector_type`. " +
					"Keys you leave out keep their server defaults and are not tracked; URL normalization by the API is not reported as drift.",
			},
			"credentials": schema.SingleNestedAttribute{
				Optional: true,
				MarkdownDescription: "Inline credentials for the source. Write-only: the API never returns them, so Terraform cannot detect out-of-band changes. " +
					"Required by most types; not allowed for `web_crawler`, GitHub App connectors or Google Drive in `oauth` mode. Removing the block leaves the stored credentials in place.",
				Attributes: map[string]schema.Attribute{
					"email":         schema.StringAttribute{Optional: true, MarkdownDescription: "Account email, for sources that authenticate with email plus token (Jira, Confluence)."},
					"api_token":     schema.StringAttribute{Required: true, Sensitive: true, MarkdownDescription: "API token, password or private key, depending on the source."},
					"admin_api_key": schema.StringAttribute{Optional: true, Sensitive: true, MarkdownDescription: "Atlassian organization admin API key, used to resolve managed-account emails during permission sync."},
				},
			},
			"schedule": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Cron schedule for content syncs (server default `0 */6 * * *`). " + sticky,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"fts_language": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Full-text search language for indexed documents. " + sticky,
				Validators:          []validator.String{stringvalidator.OneOf(knowledgeConnectorFtsLanguages...)},
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"permission_sync_interval_seconds": schema.Int64Attribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "How often permissions are re-synced, in seconds, when `sync_permissions_from_source` is on. " + sticky,
				Validators:          []validator.Int64{int64validator.AtLeast(0)},
				PlanModifiers:       []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
			},
			"enabled": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Whether scheduled syncs run (server default `true`). " + sticky,
				PlanModifiers:       []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"sync_permissions_from_source": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Mirror each document's access control from the source, so searches return only what the caller could open there. Needs an enterprise license and a source type that supports it. " + sticky,
				PlanModifiers:       []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"environment_id": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Deployment environment the connector syncs in. The API picks the default environment when omitted. " + sticky,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"knowledge_base_ids": schema.SetAttribute{
				Optional:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Knowledge bases that receive this connector's documents. Omit to leave assignments unmanaged; an empty set removes every assignment.",
			},
			"created_at": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Creation timestamp (RFC 3339).",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
		},
	}
}

func (r *KnowledgeConnectorResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	api, ok := req.ProviderData.(*client.ClientWithResponses)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Resource Configure Type", fmt.Sprintf("Expected *client.ClientWithResponses, got: %T. Please report this issue to the provider developers.", req.ProviderData))
		return
	}
	connectors, err := client.NewKnowledgeConnectorClient(api)
	if err != nil {
		resp.Diagnostics.AddError("Knowledge Connector Client Configuration Failed", err.Error())
		return
	}
	r.api, r.connectors = api, connectors
}

func (r *KnowledgeConnectorResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var data KnowledgeConnectorResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() || data.Config.IsNull() || data.Config.IsUnknown() {
		return
	}
	var config map[string]any
	if err := json.Unmarshal([]byte(data.Config.ValueString()), &config); err != nil || config == nil {
		resp.Diagnostics.AddAttributeError(path.Root("config"), "Invalid Connector Config", "config must be a JSON object.")
		return
	}
	if kind, ok := config["type"]; ok && !data.ConnectorType.IsUnknown() && !data.ConnectorType.IsNull() && kind != data.ConnectorType.ValueString() {
		resp.Diagnostics.AddAttributeError(path.Root("config"), "Conflicting Connector Type",
			fmt.Sprintf("config.type is %q but connector_type is %q. Leave `type` out of config; the provider adds it.", kind, data.ConnectorType.ValueString()))
	}
	if data.ConnectorType.ValueString() == "web_crawler" && !data.Credentials.IsNull() && !data.Credentials.IsUnknown() {
		resp.Diagnostics.AddAttributeError(path.Root("credentials"), "Credentials Not Allowed", "Web crawler connectors take no credentials.")
	}
}

func (r *KnowledgeConnectorResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	defer finishParentCreate(ctx, r.api, "knowledgeConnector", "id", resp)

	var data KnowledgeConnectorResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	priorNull := tftypes.NewValue(req.Plan.Schema.Type().TerraformType(ctx), nil)
	patch := MergePatch(ctx, req.Plan.Raw, priorNull, knowledgeConnectorAttrSpec, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	body := r.encodeBody(patch, data.ConnectorType.ValueString(), &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	connector, err := r.connectors.Create(ctx, body)
	if err != nil {
		resp.Diagnostics.AddError("Knowledge Connector Creation Failed", err.Error())
		return
	}
	plannedKnowledgeBases := data.KnowledgeBaseIDs
	mapKnowledgeConnector(&data, connector, &resp.Diagnostics)
	data.KnowledgeBaseIDs = plannedKnowledgeBases
	r.readKnowledgeBaseIDs(ctx, &data, &resp.Diagnostics)
	resolveInitialGrants(&data.InitialGrants)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *KnowledgeConnectorResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data KnowledgeConnectorResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if data.ID.IsNull() || data.ID.ValueString() == "" {
		resp.State.RemoveResource(ctx)
		return
	}
	connector, err := r.connectors.Get(ctx, data.ID.ValueString())
	if client.IsKnowledgeConnectorNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Knowledge Connector Read Failed", err.Error())
		return
	}
	mapKnowledgeConnector(&data, connector, &resp.Diagnostics)
	r.readKnowledgeBaseIDs(ctx, &data, &resp.Diagnostics)
	resolveInitialGrants(&data.InitialGrants)
	readParentPermissions(ctx, r.api, "knowledgeConnector", data.ID.ValueString(), &data.InitialGrants, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *KnowledgeConnectorResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	finishPermissions := beginParentPermissionUpdate(ctx, r.api, "knowledgeConnector", "id", req, resp)
	if resp.Diagnostics.HasError() {
		return
	}
	defer finishPermissions()

	var data, prior KnowledgeConnectorResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &prior)...)
	if resp.Diagnostics.HasError() {
		return
	}
	id := prior.ID.ValueString()

	patch := MergePatch(ctx, req.Plan.Raw, req.State.Raw, knowledgeConnectorAttrSpec, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	// The update API switches permission sync through the legacy visibility
	// input; access itself is owned by `permissions`.
	if !data.SyncPermissionsFromSource.IsUnknown() && !data.SyncPermissionsFromSource.IsNull() &&
		!data.SyncPermissionsFromSource.Equal(prior.SyncPermissionsFromSource) {
		patch["visibility"] = "org-wide"
		if data.SyncPermissionsFromSource.ValueBool() {
			patch["visibility"] = "auto-sync-permissions"
		}
	}
	var connector *client.KnowledgeConnector
	var err error
	if len(patch) > 0 {
		body := r.encodeBody(patch, prior.ConnectorType.ValueString(), &resp.Diagnostics)
		if resp.Diagnostics.HasError() {
			return
		}
		connector, err = r.connectors.Update(ctx, id, body)
		if err != nil {
			resp.Diagnostics.AddError("Knowledge Connector Update Failed", err.Error())
			return
		}
	}

	r.syncKnowledgeBases(ctx, id, prior.KnowledgeBaseIDs, data.KnowledgeBaseIDs, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	if connector == nil {
		connector, err = r.connectors.Get(ctx, id)
		if err != nil {
			resp.Diagnostics.AddError("Knowledge Connector Read Failed", err.Error())
			return
		}
	}
	// Credentials are write-only: state keeps what the plan sent.
	data.Credentials = planOrPrior(data.Credentials, prior.Credentials)
	mapKnowledgeConnector(&data, connector, &resp.Diagnostics)
	r.readKnowledgeBaseIDs(ctx, &data, &resp.Diagnostics)
	resolveInitialGrants(&data.InitialGrants)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *KnowledgeConnectorResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data KnowledgeConnectorResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.connectors.Delete(ctx, data.ID.ValueString()); err != nil && !client.IsKnowledgeConnectorNotFound(err) {
		resp.Diagnostics.AddError("Knowledge Connector Deletion Failed", err.Error())
	}
}

func (r *KnowledgeConnectorResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// encodeBody marshals a MergePatch result, putting the `type` discriminator
// that the API's config union requires into config.
func (r *KnowledgeConnectorResource) encodeBody(patch map[string]any, connectorType string, diags *diag.Diagnostics) []byte {
	if raw, ok := patch["config"]; ok && raw != nil {
		var config map[string]any
		encoded, err := json.Marshal(raw)
		if err == nil {
			err = json.Unmarshal(encoded, &config)
		}
		if err != nil || config == nil {
			diags.AddAttributeError(path.Root("config"), "Invalid Connector Config", "config must be a JSON object.")
			return nil
		}
		config["type"] = connectorType
		patch["config"] = config
	}
	LogPatch(context.Background(), "archestra_knowledge_connector write", patch, knowledgeConnectorAttrSpec)
	body, err := json.Marshal(patch)
	if err != nil {
		diags.AddError("Marshal Error", fmt.Sprintf("Unable to marshal request body: %s", err))
		return nil
	}
	return body
}

// readKnowledgeBaseIDs refreshes managed assignments. A null set means the
// configuration leaves assignments unmanaged, so they are not read.
func (r *KnowledgeConnectorResource) readKnowledgeBaseIDs(ctx context.Context, data *KnowledgeConnectorResourceModel, diags *diag.Diagnostics) {
	if data.KnowledgeBaseIDs.IsNull() || data.KnowledgeBaseIDs.IsUnknown() || diags.HasError() {
		return
	}
	ids, err := r.connectors.KnowledgeBaseIDs(ctx, data.ID.ValueString())
	if err != nil {
		diags.AddError("Knowledge Connector Assignments Read Failed", err.Error())
		return
	}
	value, d := types.SetValueFrom(ctx, types.StringType, ids)
	diags.Append(d...)
	data.KnowledgeBaseIDs = value
}

// syncKnowledgeBases applies the difference between two managed assignment
// sets. A null desired set leaves assignments unmanaged.
func (r *KnowledgeConnectorResource) syncKnowledgeBases(ctx context.Context, id string, prior, desired types.Set, diags *diag.Diagnostics) {
	if desired.IsNull() || desired.IsUnknown() {
		return
	}
	want := setStrings(ctx, desired, diags)
	var have []string
	if prior.IsNull() || prior.IsUnknown() {
		current, err := r.connectors.KnowledgeBaseIDs(ctx, id)
		if err != nil {
			diags.AddError("Knowledge Connector Assignments Read Failed", err.Error())
			return
		}
		have = current
	} else {
		have = setStrings(ctx, prior, diags)
	}
	if diags.HasError() {
		return
	}
	add, remove := stringSetDifference(want, have), stringSetDifference(have, want)
	if len(add) > 0 {
		if err := r.connectors.AssignKnowledgeBases(ctx, id, add); err != nil {
			diags.AddError("Knowledge Base Assignment Failed", err.Error())
			return
		}
	}
	for _, kb := range remove {
		if err := r.connectors.UnassignKnowledgeBase(ctx, id, kb); err != nil && !client.IsKnowledgeConnectorNotFound(err) {
			diags.AddError("Knowledge Base Unassignment Failed", err.Error())
			return
		}
	}
}

// mapKnowledgeConnector copies the API response into state. Credentials and
// knowledge_base_ids are left to the caller: the API never returns the first,
// and the second comes from its own endpoint.
func mapKnowledgeConnector(data *KnowledgeConnectorResourceModel, connector *client.KnowledgeConnector, diags *diag.Diagnostics) {
	data.ID = types.StringValue(connector.ID)
	data.Name = types.StringValue(connector.Name)
	data.Description = types.StringPointerValue(connector.Description)
	data.ConnectorType = types.StringValue(connector.ConnectorType)
	data.Schedule = types.StringValue(connector.Schedule)
	data.FtsLanguage = types.StringValue(connector.FtsLanguage)
	data.PermissionSyncIntervalSeconds = types.Int64Value(connector.PermissionSyncIntervalSeconds)
	data.Enabled = types.BoolValue(connector.Enabled)
	data.SyncPermissionsFromSource = types.BoolValue(connector.SyncPermissionsFromSource)
	data.EnvironmentID = types.StringPointerValue(connector.EnvironmentID)
	data.CreatedAt = types.StringValue(connector.CreatedAt)
	if data.Credentials.IsUnknown() {
		data.Credentials = types.ObjectNull(knowledgeConnectorCredentialAttrTypes)
	}

	data.Labels = knowledgeConnectorLabels(data.Labels, connector)

	config, err := knowledgeConnectorConfigFromAPI(data.Config, connector.Config)
	if err != nil {
		diags.AddError("Invalid Connector Config Response", err.Error())
		return
	}
	data.Config = config
}

// knowledgeConnectorConfigFromAPI keeps the configured JSON while every key it
// sets matches the server's copy. The API adds defaults for omitted keys and
// normalizes URLs (adds https://, drops trailing slashes), which is not drift.
// A changed configured key returns the server config so the plan shows it.
func knowledgeConnectorConfigFromAPI(prior jsontypes.Normalized, raw json.RawMessage) (jsontypes.Normalized, error) {
	var server map[string]any
	if err := json.Unmarshal(raw, &server); err != nil {
		return prior, fmt.Errorf("connector config is not a JSON object: %w", err)
	}
	delete(server, "type")
	if !prior.IsNull() && !prior.IsUnknown() {
		var configured map[string]any
		if err := json.Unmarshal([]byte(prior.ValueString()), &configured); err == nil && configured != nil {
			delete(configured, "type")
			if connectorConfigCovers(configured, server) {
				return prior, nil
			}
		}
	}
	encoded, err := json.Marshal(server)
	if err != nil {
		return prior, err
	}
	return jsontypes.NewNormalizedValue(string(encoded)), nil
}

// connectorConfigCovers reports whether every key in configured has a
// matching value in server; nested objects follow the same rule.
func connectorConfigCovers(configured, server map[string]any) bool {
	for key, want := range configured {
		got, ok := server[key]
		if !ok {
			if want == nil {
				continue
			}
			return false
		}
		if !connectorConfigValueEqual(want, got) {
			return false
		}
	}
	return true
}

func connectorConfigValueEqual(want, got any) bool {
	switch w := want.(type) {
	case map[string]any:
		g, ok := got.(map[string]any)
		return ok && connectorConfigCovers(w, g)
	case []any:
		g, ok := got.([]any)
		if !ok || len(g) != len(w) {
			return false
		}
		for i := range w {
			if !connectorConfigValueEqual(w[i], g[i]) {
				return false
			}
		}
		return true
	case string:
		g, ok := got.(string)
		return ok && normalizeConnectorString(w) == normalizeConnectorString(g)
	default:
		return reflect.DeepEqual(want, got)
	}
}

// normalizeConnectorString mirrors the API's URL transforms: it adds https://
// to scheme-less URLs and strips trailing slashes (and Perforce's `/...`).
func normalizeConnectorString(value string) string {
	value = strings.TrimPrefix(value, "https://")
	value = strings.TrimSuffix(value, "/...")
	return strings.TrimRight(value, "/")
}

func knowledgeConnectorLabels(prior []AgentLabelModel, connector *client.KnowledgeConnector) []AgentLabelModel {
	if len(connector.Labels) == 0 {
		if prior != nil {
			return []AgentLabelModel{}
		}
		return nil
	}
	out := make([]AgentLabelModel, len(connector.Labels))
	for i, l := range connector.Labels {
		out[i] = AgentLabelModel{Key: types.StringValue(l.Key), Value: types.StringValue(l.Value)}
	}
	return out
}

func planOrPrior(plan, prior types.Object) types.Object {
	if plan.IsUnknown() {
		return prior
	}
	return plan
}

func setStrings(ctx context.Context, value types.Set, diags *diag.Diagnostics) []string {
	var out []string
	diags.Append(value.ElementsAs(ctx, &out, false)...)
	return out
}

func stringSetDifference(a, b []string) []string {
	seen := make(map[string]bool, len(b))
	for _, v := range b {
		seen[v] = true
	}
	var out []string
	for _, v := range a {
		if !seen[v] {
			out = append(out, v)
		}
	}
	sort.Strings(out)
	return out
}
