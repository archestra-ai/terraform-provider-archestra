package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
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

var _ resource.Resource = &KnowledgeConnectorResource{}
var _ resource.ResourceWithImportState = &KnowledgeConnectorResource{}
var _ resource.ResourceWithValidateConfig = &KnowledgeConnectorResource{}

func NewKnowledgeConnectorResource() resource.Resource {
	return &KnowledgeConnectorResource{}
}

type KnowledgeConnectorResource struct {
	client *client.ClientWithResponses
}

// KnowledgeConnectorCredentialsModel is the SingleNestedAttribute
// projection. Both fields land in state; api_token is the sensitive
// write-only field (backend stores in a secret manager and never
// echoes it back) — Read preserves whatever is in prior state.
type KnowledgeConnectorCredentialsModel struct {
	Email    types.String `tfsdk:"email"`
	APIToken types.String `tfsdk:"api_token"`
}

type KnowledgeConnectorResourceModel struct {
	ID               types.String         `tfsdk:"id"`
	Name             types.String         `tfsdk:"name"`
	Description      types.String         `tfsdk:"description"`
	Visibility       types.String         `tfsdk:"visibility"`
	TeamIDs          types.List           `tfsdk:"team_ids"`
	ConnectorType    types.String         `tfsdk:"connector_type"`
	Config           jsontypes.Normalized `tfsdk:"config"`
	Credentials      types.Object         `tfsdk:"credentials"`
	Schedule         types.String         `tfsdk:"schedule"`
	Enabled          types.Bool           `tfsdk:"enabled"`
	KnowledgeBaseIDs types.List           `tfsdk:"knowledge_base_ids"`
	SecretID         types.String         `tfsdk:"secret_id"`
	LastSyncAt       types.String         `tfsdk:"last_sync_at"`
	LastSyncStatus   types.String         `tfsdk:"last_sync_status"`
	CreatedAt        types.String         `tfsdk:"created_at"`
	UpdatedAt        types.String         `tfsdk:"updated_at"`
}

// connectorCredentialsAttrTypes mirrors the schema's nested object —
// kept in one place so flatten and the nested-default path agree.
var connectorCredentialsAttrTypes = map[string]attr.Type{
	"email":     types.StringType,
	"api_token": types.StringType,
}

// knowledgeConnectorAPIResponse is the JSON-roundtrip mirror used by
// every Create/Read/Update response projection. The generated client
// emits a broken `union json.RawMessage` for the polymorphic config
// field (it's a `discriminatedUnion` of named schemas, which oapi-
// codegen can't handle correctly), so we deserialise the raw bytes
// into this neutral shape instead of trying to use the typed struct.
type knowledgeConnectorAPIResponse struct {
	Checkpoint     json.RawMessage `json:"checkpoint"`
	Config         json.RawMessage `json:"config"`
	ConnectorType  string          `json:"connectorType"`
	CreatedAt      time.Time       `json:"createdAt"`
	Description    *string         `json:"description"`
	Enabled        bool            `json:"enabled"`
	ID             string          `json:"id"`
	LastSyncAt     *time.Time      `json:"lastSyncAt"`
	LastSyncError  *string         `json:"lastSyncError"`
	LastSyncStatus *string         `json:"lastSyncStatus"`
	Name           string          `json:"name"`
	Schedule       *string         `json:"schedule"`
	SecretID       *string         `json:"secretId"`
	TeamIDs        []string        `json:"teamIds"`
	UpdatedAt      time.Time       `json:"updatedAt"`
	Visibility     string          `json:"visibility"`
	// KnowledgeBaseIds is NOT in the singular GET response — it lives
	// on the m2m table and is fetched via a separate endpoint. Field
	// retained here as `omitempty` so future backend additions land
	// without a code change.
	KnowledgeBaseIds []string `json:"knowledgeBaseIds,omitempty"`
}

func (r *KnowledgeConnectorResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_knowledge_connector"
}

func (r *KnowledgeConnectorResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "External data source that syncs documents into one or more `archestra_knowledge_base` records on a schedule.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Connector identifier.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Human-readable name of the connector.",
				Validators:          []validator.String{stringvalidator.LengthAtLeast(1)},
			},
			"description": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Free-form description.",
			},
			"visibility": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("org-wide"),
				MarkdownDescription: "Who can see the connector and the documents it syncs: `org-wide` or `team-scoped`.",
				Validators: []validator.String{
					stringvalidator.OneOf(validVisibilityValues...),
				},
			},
			"team_ids": schema.ListAttribute{
				Optional:            true,
				Computed:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Team IDs the connector is scoped to; required when `visibility = \"team-scoped\"`.",
				PlanModifiers:       []planmodifier.List{EmptyListOnConfigNull()},
			},
			"connector_type": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Type of upstream system. Changing this forces a new resource because the wire `config` shape changes with type.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
				Validators:          []validator.String{stringvalidator.OneOf(validConnectorTypes...)},
			},
			"config": schema.StringAttribute{
				Required:            true,
				CustomType:          jsontypes.NormalizedType{},
				MarkdownDescription: "Connector-type-specific configuration as a JSON object. The schema is the corresponding `<Type>ConfigSchema` from the backend (e.g. `JiraConfigSchema`, `GithubConfigSchema`). Do NOT include the `type` field — Terraform injects it from `connector_type`.",
			},
			"credentials": schema.SingleNestedAttribute{
				Required:            true,
				Sensitive:           true,
				MarkdownDescription: "Credentials the connector uses to authenticate against the upstream system.",
				Attributes: map[string]schema.Attribute{
					"email": schema.StringAttribute{
						Optional:            true,
						MarkdownDescription: "Account email (required for some connector types, e.g. Jira Cloud).",
					},
					"api_token": schema.StringAttribute{
						Required:            true,
						Sensitive:           true,
						MarkdownDescription: "API token / personal access token. Stored in the backend secret manager and never echoed back; rotating it requires updating the value here.",
					},
				},
			},
			"schedule": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Cron expression controlling sync cadence (backend-validated). Backend assigns a default (`0 */6 * * *`) when omitted; the value is round-tripped into state.",
			},
			"enabled": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(true),
				MarkdownDescription: "Whether the connector runs on its schedule. Disable to pause without deleting.",
			},
			"knowledge_base_ids": schema.ListAttribute{
				Optional:            true,
				Computed:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "IDs of `archestra_knowledge_base` records this connector syncs into. Reconciled in-place on update via the backend's assign/unassign endpoints.",
				PlanModifiers:       []planmodifier.List{EmptyListOnConfigNull()},
			},
			"secret_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Internal secret-manager handle for the stored credentials.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"last_sync_at": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Timestamp of the most recent sync attempt (RFC 3339), or null if never run.",
			},
			"last_sync_status": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Status of the most recent sync run, or null if never run.",
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

func (r *KnowledgeConnectorResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// ValidateConfig pins all the cross-field and config-shape rules the
// backend enforces at apply time. Catching these in plan saves an
// API round-trip and gives sharper error messages.
func (r *KnowledgeConnectorResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var data KnowledgeConnectorResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// visibility / team_ids cross-field
	if !data.Visibility.IsNull() && !data.Visibility.IsUnknown() {
		visibility := data.Visibility.ValueString()
		teamsSet := !data.TeamIDs.IsNull() && !data.TeamIDs.IsUnknown() && len(data.TeamIDs.Elements()) > 0

		if visibility == "team-scoped" && !teamsSet {
			resp.Diagnostics.AddAttributeError(
				path.Root("team_ids"),
				"Missing Required Attribute",
				`team_ids must contain at least one team ID when visibility = "team-scoped"`,
			)
		}
		if visibility != "team-scoped" && teamsSet {
			resp.Diagnostics.AddAttributeError(
				path.Root("team_ids"),
				"Invalid Attribute Value",
				fmt.Sprintf(`team_ids must be empty when visibility = %q; assignments only apply when visibility = "team-scoped"`, visibility),
			)
		}
	}

	// config shape: must be a JSON object, must not carry a `type` key
	// (we inject from connector_type), and must include the required
	// keys for the declared connector_type.
	if !data.Config.IsNull() && !data.Config.IsUnknown() &&
		!data.ConnectorType.IsNull() && !data.ConnectorType.IsUnknown() {
		var cfg map[string]any
		if err := json.Unmarshal([]byte(data.Config.ValueString()), &cfg); err != nil {
			resp.Diagnostics.AddAttributeError(
				path.Root("config"),
				"Invalid config JSON",
				fmt.Sprintf("config must be a JSON object: %s", err),
			)
		} else if cfg == nil {
			resp.Diagnostics.AddAttributeError(
				path.Root("config"),
				"Invalid config JSON",
				"config must be a JSON object, not null",
			)
		} else {
			if _, hasType := cfg["type"]; hasType {
				resp.Diagnostics.AddAttributeError(
					path.Root("config"),
					"Reserved key in config",
					"`type` is set automatically from `connector_type`; remove it from your config object.",
				)
			}
			connectorType := data.ConnectorType.ValueString()
			for _, requiredKey := range connectorRequiredConfigFields[connectorType] {
				if _, ok := cfg[requiredKey]; !ok {
					resp.Diagnostics.AddAttributeError(
						path.Root("config"),
						"Missing required config key",
						fmt.Sprintf("connector_type %q requires `config.%s` to be set.", connectorType, requiredKey),
					)
				}
			}
		}
	}
}

func (r *KnowledgeConnectorResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
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
	if err := finalizeKnowledgeConnectorPatch(patch, &data, true); err != nil {
		resp.Diagnostics.AddError("Patch finalize error", err.Error())
		return
	}
	LogPatch(ctx, "archestra_knowledge_connector Create", patch, knowledgeConnectorAttrSpec)

	bodyBytes, err := json.Marshal(patch)
	if err != nil {
		resp.Diagnostics.AddError("Marshal Error", err.Error())
		return
	}
	apiResp, err := r.client.CreateConnectorWithBodyWithResponse(ctx, "application/json", bytes.NewReader(bodyBytes))
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to create knowledge connector: %s", err))
		return
	}
	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Expected 200 OK, got status %d: %s", apiResp.StatusCode(), string(apiResp.Body)),
		)
		return
	}

	if !r.flattenConnectorBody(ctx, &data, apiResp.Body, &resp.Diagnostics) {
		return
	}
	// `knowledge_base_ids` doesn't round-trip through the connector
	// response — fetch separately and refresh state. (Create's inline
	// handler already attached them; this is just to populate state.)
	if !r.refreshKnowledgeBaseIDs(ctx, &data, &resp.Diagnostics) {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *KnowledgeConnectorResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data KnowledgeConnectorResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := r.client.GetConnectorWithResponse(ctx, data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to read knowledge connector: %s", err))
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

	if !r.flattenConnectorBody(ctx, &data, apiResp.Body, &resp.Diagnostics) {
		return
	}
	if !r.refreshKnowledgeBaseIDs(ctx, &data, &resp.Diagnostics) {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *KnowledgeConnectorResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var planData, stateData KnowledgeConnectorResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &planData)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &stateData)...)
	if resp.Diagnostics.HasError() {
		return
	}

	patch := MergePatch(ctx, req.Plan.Raw, req.State.Raw, knowledgeConnectorAttrSpec, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	// `knowledge_base_ids` enters the patch when it changes (Kind:
	// List) — finalize strips it because Update's wire body doesn't
	// accept it. We reconcile via assign/unassign below. Look up
	// whether KB membership actually changed *before* finalize wipes
	// the field, so the no-op short-circuit can still trigger when
	// only KBs change.
	_, kbsChanged := patch["knowledgeBaseIds"]

	if err := finalizeKnowledgeConnectorPatch(patch, &planData, false); err != nil {
		resp.Diagnostics.AddError("Patch finalize error", err.Error())
		return
	}

	if len(patch) == 0 {
		// No wire diff on the main PUT body. Reconcile KB membership
		// in case the only change was knowledge_base_ids, then
		// refresh Computed fields from the API so `last_sync_at` etc.
		// land concrete instead of as Unknown.
		if kbsChanged {
			if !r.reconcileKnowledgeBaseIDs(ctx, &planData, &stateData, &resp.Diagnostics) {
				return
			}
		}
		if !r.refreshFromBackend(ctx, &planData, &resp.Diagnostics) {
			return
		}
		if !r.refreshKnowledgeBaseIDs(ctx, &planData, &resp.Diagnostics) {
			return
		}
		resp.Diagnostics.Append(resp.State.Set(ctx, &planData)...)
		return
	}
	LogPatch(ctx, "archestra_knowledge_connector Update", patch, knowledgeConnectorAttrSpec)

	bodyBytes, err := json.Marshal(patch)
	if err != nil {
		resp.Diagnostics.AddError("Marshal Error", err.Error())
		return
	}
	apiResp, err := r.client.UpdateConnectorWithBodyWithResponse(ctx, planData.ID.ValueString(), "application/json", bytes.NewReader(bodyBytes))
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to update knowledge connector: %s", err))
		return
	}
	if IsNotFound(apiResp) {
		resp.Diagnostics.AddError(
			"Resource Deleted Outside Terraform",
			"The knowledge connector was deleted on the backend between refresh and apply. Re-run `terraform apply`.",
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

	if !r.flattenConnectorBody(ctx, &planData, apiResp.Body, &resp.Diagnostics) {
		return
	}
	if !r.reconcileKnowledgeBaseIDs(ctx, &planData, &stateData, &resp.Diagnostics) {
		return
	}
	if !r.refreshKnowledgeBaseIDs(ctx, &planData, &resp.Diagnostics) {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &planData)...)
}

func (r *KnowledgeConnectorResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data KnowledgeConnectorResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := r.client.DeleteConnectorWithResponse(ctx, data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to delete knowledge connector: %s", err))
		return
	}
	if apiResp.JSON200 == nil && !IsNotFound(apiResp) {
		resp.Diagnostics.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Expected 200 OK or 404 Not Found, got status %d: %s", apiResp.StatusCode(), string(apiResp.Body)),
		)
	}
}

// ImportState passes the connector ID through. The next refresh
// populates everything from the API — except `api_token`, which the
// backend never returns. Users importing must `terraform apply` after
// import to set credentials.api_token; the apply produces a diff that
// rewrites credentials with the user's value.
func (r *KnowledgeConnectorResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// finalizeKnowledgeConnectorPatch reconciles the merge-patch shape
// with the backend wire shape:
//
//  1. Whenever `config` is in the patch (changed), inject the
//     discriminator `type` field from the plan's `connector_type`
//     so the backend's `discriminatedUnion` routes correctly. The
//     encoder set on the AttrSpec already parsed the user's JSON
//     string into a real object; we just stamp `type` onto it.
//
//  2. On Update, drop `knowledgeBaseIds` from the patch — the wire
//     Update body has no such field; reconcile happens through the
//     assign/unassign endpoints in `reconcileKnowledgeBaseIDs`. On
//     Create the wire body DOES accept it, so leave it alone.
func finalizeKnowledgeConnectorPatch(patch map[string]any, data *KnowledgeConnectorResourceModel, isCreate bool) error {
	if !isCreate {
		delete(patch, "knowledgeBaseIds")
	}

	if cfgRaw, present := patch["config"]; present {
		cfg, ok := cfgRaw.(map[string]any)
		if !ok {
			// Encoder fell back to a string (parse failure). Shouldn't
			// happen because ValidateConfig rejected non-JSON at plan
			// time, but tolerate by parsing here.
			s, isString := cfgRaw.(string)
			if !isString {
				return fmt.Errorf("config patch is neither object nor string: %T", cfgRaw)
			}
			cfg = map[string]any{}
			if s != "" {
				if err := json.Unmarshal([]byte(s), &cfg); err != nil {
					return fmt.Errorf("config JSON parse error: %w", err)
				}
			}
		}
		if data.ConnectorType.IsNull() || data.ConnectorType.IsUnknown() {
			return fmt.Errorf("connector_type required to assemble config wire body")
		}
		cfg["type"] = data.ConnectorType.ValueString()
		patch["config"] = cfg
	}
	return nil
}

// flattenConnectorBody projects the raw API response body into state.
// `api_token` is preserved from prior state (backend never echoes it
// back), so Read/Update don't blank the user's secret.
func (r *KnowledgeConnectorResource) flattenConnectorBody(ctx context.Context, data *KnowledgeConnectorResourceModel, body []byte, diags *diag.Diagnostics) bool {
	var src knowledgeConnectorAPIResponse
	if err := json.Unmarshal(body, &src); err != nil {
		diags.AddError("API Response Decode Error", "Unable to decode connector response: "+err.Error())
		return false
	}

	priorAPIToken := types.StringNull()
	priorEmail := types.StringNull()
	if !data.Credentials.IsNull() && !data.Credentials.IsUnknown() {
		attrs := data.Credentials.Attributes()
		if t, ok := attrs["api_token"].(types.String); ok {
			priorAPIToken = t
		}
		if e, ok := attrs["email"].(types.String); ok {
			priorEmail = e
		}
	}

	data.ID = types.StringValue(src.ID)
	data.Name = types.StringValue(src.Name)
	optionalStringFromAPI(&data.Description, src.Description)
	data.Visibility = types.StringValue(src.Visibility)
	data.ConnectorType = types.StringValue(src.ConnectorType)
	data.Enabled = types.BoolValue(src.Enabled)
	// Backend returns schedule as a (possibly empty) string. Normalise
	// "" to null so HCL absence and backend absence match.
	if src.Schedule != nil && *src.Schedule != "" {
		data.Schedule = types.StringValue(*src.Schedule)
	} else if !data.Schedule.IsNull() {
		data.Schedule = types.StringNull()
	}
	if src.SecretID != nil {
		data.SecretID = types.StringValue(*src.SecretID)
	} else {
		data.SecretID = types.StringNull()
	}
	data.CreatedAt = types.StringValue(src.CreatedAt.Format(time.RFC3339))
	data.UpdatedAt = types.StringValue(src.UpdatedAt.Format(time.RFC3339))
	if src.LastSyncAt != nil {
		data.LastSyncAt = types.StringValue(src.LastSyncAt.Format(time.RFC3339))
	} else if !data.LastSyncAt.IsNull() {
		data.LastSyncAt = types.StringNull()
	}
	if src.LastSyncStatus != nil {
		data.LastSyncStatus = types.StringValue(*src.LastSyncStatus)
	} else if !data.LastSyncStatus.IsNull() {
		data.LastSyncStatus = types.StringNull()
	}

	teamIDs, d := types.ListValueFrom(ctx, types.StringType, src.TeamIDs)
	diags.Append(d...)
	if diags.HasError() {
		return false
	}
	data.TeamIDs = teamIDs

	// Project the polymorphic config object back to a JSON string.
	// Strip the wire-only `type` field so the round-trip matches what
	// the user wrote in HCL (we re-inject `type` on the next write).
	if len(src.Config) > 0 {
		var cfg map[string]any
		if err := json.Unmarshal(src.Config, &cfg); err != nil {
			diags.AddError("Config decode error", "Unable to decode connector config from response: "+err.Error())
			return false
		}
		delete(cfg, "type")
		encoded, err := json.Marshal(cfg)
		if err != nil {
			diags.AddError("Config encode error", err.Error())
			return false
		}
		data.Config = jsontypes.NewNormalizedValue(string(encoded))
	}

	// Credentials are never echoed back — preserve from prior state.
	credObj, d := types.ObjectValue(connectorCredentialsAttrTypes, map[string]attr.Value{
		"email":     priorEmail,
		"api_token": priorAPIToken,
	})
	diags.Append(d...)
	if diags.HasError() {
		return false
	}
	data.Credentials = credObj

	return true
}

// refreshFromBackend re-reads the connector and overwrites the
// projected fields. Used in Update's no-op branch to lift Computed
// fields out of Unknown without sending a redundant PATCH.
func (r *KnowledgeConnectorResource) refreshFromBackend(ctx context.Context, data *KnowledgeConnectorResourceModel, diags *diag.Diagnostics) bool {
	apiResp, err := r.client.GetConnectorWithResponse(ctx, data.ID.ValueString())
	if err != nil {
		diags.AddError("API Error", fmt.Sprintf("Unable to refresh knowledge connector: %s", err))
		return false
	}
	if IsNotFound(apiResp) {
		diags.AddError(
			"Resource Deleted Outside Terraform",
			"The knowledge connector disappeared between refresh and apply. Re-run `terraform apply`.",
		)
		return false
	}
	if apiResp.JSON200 == nil {
		diags.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Expected 200 OK, got status %d: %s", apiResp.StatusCode(), string(apiResp.Body)),
		)
		return false
	}
	return r.flattenConnectorBody(ctx, data, apiResp.Body, diags)
}

// refreshKnowledgeBaseIDs fetches the m2m KB list and projects it
// into state. Called after every Create/Read/Update so the attribute
// stays drift-honest.
func (r *KnowledgeConnectorResource) refreshKnowledgeBaseIDs(ctx context.Context, data *KnowledgeConnectorResourceModel, diags *diag.Diagnostics) bool {
	apiResp, err := r.client.GetConnectorKnowledgeBasesWithResponse(ctx, data.ID.ValueString())
	if err != nil {
		diags.AddError("API Error", fmt.Sprintf("Unable to list connector knowledge bases: %s", err))
		return false
	}
	// No IsNotFound branch: list endpoints don't 404 on missing
	// records — they return empty `data` — so a 404 here is
	// infrastructure (proxy/auth). Treating it as "no KBs assigned"
	// would silently clobber the user's state. Same lesson as
	// findVirtualKeyByID's pagination loop.
	if apiResp.JSON200 == nil {
		diags.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Expected 200 OK from KB list, got status %d: %s", apiResp.StatusCode(), string(apiResp.Body)),
		)
		return false
	}

	ids := make([]string, 0, len(apiResp.JSON200.Data))
	for _, kb := range apiResp.JSON200.Data {
		ids = append(ids, kb.Id.String())
	}
	sort.Strings(ids)
	list, d := types.ListValueFrom(ctx, types.StringType, ids)
	diags.Append(d...)
	if diags.HasError() {
		return false
	}
	data.KnowledgeBaseIDs = list
	return true
}

// reconcileKnowledgeBaseIDs computes the additions/removals between
// the plan and state KB lists and calls the assign/unassign endpoints
// accordingly. No-ops if the lists are equal.
func (r *KnowledgeConnectorResource) reconcileKnowledgeBaseIDs(ctx context.Context, plan, state *KnowledgeConnectorResourceModel, diags *diag.Diagnostics) bool {
	planIDs := stringSetFromList(plan.KnowledgeBaseIDs)
	stateIDs := stringSetFromList(state.KnowledgeBaseIDs)

	toAdd := diffStringSet(planIDs, stateIDs)
	toRemove := diffStringSet(stateIDs, planIDs)

	if len(toAdd) > 0 {
		body := client.AssignConnectorToKnowledgeBasesJSONRequestBody{KnowledgeBaseIds: toAdd}
		bodyBytes, err := json.Marshal(body)
		if err != nil {
			diags.AddError("Marshal Error", err.Error())
			return false
		}
		apiResp, err := r.client.AssignConnectorToKnowledgeBasesWithBodyWithResponse(ctx, plan.ID.ValueString(), "application/json", bytes.NewReader(bodyBytes))
		if err != nil {
			diags.AddError("API Error", fmt.Sprintf("Unable to assign knowledge bases to connector: %s", err))
			return false
		}
		if apiResp.JSON200 == nil {
			diags.AddError(
				"Unexpected API Response",
				fmt.Sprintf("Expected 200 from KB assign, got status %d: %s", apiResp.StatusCode(), string(apiResp.Body)),
			)
			return false
		}
	}

	for _, kbID := range toRemove {
		apiResp, err := r.client.UnassignConnectorFromKnowledgeBaseWithResponse(ctx, plan.ID.ValueString(), kbID)
		if err != nil {
			diags.AddError("API Error", fmt.Sprintf("Unable to unassign knowledge base %s from connector: %s", kbID, err))
			return false
		}
		if apiResp.JSON200 == nil && !IsNotFound(apiResp) {
			diags.AddError(
				"Unexpected API Response",
				fmt.Sprintf("Expected 200 from KB unassign, got status %d: %s", apiResp.StatusCode(), string(apiResp.Body)),
			)
			return false
		}
	}

	return true
}

func stringSetFromList(l types.List) map[string]struct{} {
	out := map[string]struct{}{}
	if l.IsNull() || l.IsUnknown() {
		return out
	}
	for _, e := range l.Elements() {
		if s, ok := e.(types.String); ok && !s.IsNull() && !s.IsUnknown() {
			out[s.ValueString()] = struct{}{}
		}
	}
	return out
}

func diffStringSet(a, b map[string]struct{}) []string {
	var out []string
	for k := range a {
		if _, found := b[k]; !found {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}
