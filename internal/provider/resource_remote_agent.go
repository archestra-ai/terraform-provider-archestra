package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

var _ resource.ResourceWithImportState = &RemoteAgentResource{}

func NewRemoteAgentResource() resource.Resource { return &RemoteAgentResource{} }

type RemoteAgentResource struct{ client *client.ClientWithResponses }
type RemoteAgentResourceModel struct {
	ID                        types.String         `tfsdk:"id"`
	Source                    jsontypes.Normalized `tfsdk:"source"`
	Auth                      jsontypes.Normalized `tfsdk:"auth"`
	Name                      types.String         `tfsdk:"name"`
	Description               types.String         `tfsdk:"description"`
	Enabled                   types.Bool           `tfsdk:"enabled"`
	InitialGrants             types.Set            `tfsdk:"permissions"`
	AgentCard                 jsontypes.Normalized `tfsdk:"agent_card"`
	Connection                jsontypes.Normalized `tfsdk:"connection_metadata"`
	CreatedBy                 jsontypes.Normalized `tfsdk:"created_by"`
	ConnectionID              types.String         `tfsdk:"connection_id"`
	AssignmentCount           types.Int64          `tfsdk:"assignment_count"`
	DiscoveryMode             types.String         `tfsdk:"discovery_mode"`
	DiscoveryURL              types.String         `tfsdk:"discovery_url"`
	OrganizationID            types.String         `tfsdk:"organization_id"`
	AuthorID                  types.String         `tfsdk:"author_id"`
	CardHash                  types.String         `tfsdk:"card_hash"`
	LastDiscoveredAt          types.String         `tfsdk:"last_discovered_at"`
	CreatedByServiceAccountID types.String         `tfsdk:"created_by_service_account_id"`
	CreatedAt                 types.String         `tfsdk:"created_at"`
	UpdatedAt                 types.String         `tfsdk:"updated_at"`
	ToolID                    types.String         `tfsdk:"tool_id"`
	LastUsedAt                types.String         `tfsdk:"last_used_at"`
	AuthorName                types.String         `tfsdk:"author_name"`
}

var remoteAgentAttrSpec = []AttrSpec{
	{TFName: "source", JSONName: "source", Kind: Scalar, Encoder: organizationJSONSetting},
	{TFName: "auth", JSONName: "auth", Kind: Scalar, Encoder: organizationJSONSetting, Sensitive: true, OmitOnNull: true},
	{TFName: "name", JSONName: "name", Kind: Scalar, OmitOnNull: true},
	{TFName: "description", JSONName: "description", Kind: Scalar},
	{TFName: "enabled", JSONName: "enabled", Kind: Scalar, OmitOnNull: true},
	initialGrantsAttrSpec,
}

func (r *RemoteAgentResource) AttrSpecs() []AttrSpec { return remoteAgentAttrSpec }
func (r *RemoteAgentResource) APIShape() any         { return client.GetA2aRemoteAgentResponse{} }

// The read-only connection object is exposed as connection_metadata because Terraform reserves connection.
func (r *RemoteAgentResource) KnownIntentionallySkipped() []string { return []string{"connection"} }
func (r *RemoteAgentResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_remote_agent"
}
func (r *RemoteAgentResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	attributes := map[string]schema.Attribute{
		"id":                  schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, MarkdownDescription: "Remote registration UUID."},
		"source":              schema.StringAttribute{Required: true, CustomType: jsontypes.NormalizedType{}, Validators: []validator.String{remoteAgentSourceValidator{}}, MarkdownDescription: "JSON source object using well_known or card_url with url, or inline_card with a canonical JSON Schema agentCard."},
		"auth":                schema.StringAttribute{Optional: true, CustomType: jsontypes.NormalizedType{}, Sensitive: true, MarkdownDescription: "Sensitive JSON authentication object: none, bearer with credential, or api_key with headerName and credential; omit to preserve authentication and set type none to clear it."},
		"name":                schema.StringAttribute{Optional: true, Computed: true, MarkdownDescription: "Registration name, defaulting to the Agent Card name on creation."},
		"description":         schema.StringAttribute{Optional: true, Computed: true, MarkdownDescription: "Description, defaulting to the Agent Card description; an empty string clears it."},
		"enabled":             schema.BoolAttribute{Optional: true, Computed: true, PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()}, MarkdownDescription: "Whether the saved remote connection can be used; omission preserves its current setting."},
		"permissions":         initialGrantsAttribute(),
		"agent_card":          schema.StringAttribute{Computed: true, CustomType: jsontypes.NormalizedType{}, MarkdownDescription: "Normalized saved Agent Card as JSON."},
		"connection_metadata": schema.StringAttribute{Computed: true, CustomType: jsontypes.NormalizedType{}, MarkdownDescription: "Connection metadata as JSON, with credential material omitted by the API."},
		"created_by":          schema.StringAttribute{Computed: true, CustomType: jsontypes.NormalizedType{}, MarkdownDescription: "Creator metadata as JSON."},
		"connection_id":       schema.StringAttribute{Computed: true, MarkdownDescription: "Connection UUID used by archestra_agent_remote_delegations."},
		"assignment_count":    schema.Int64Attribute{Computed: true, MarkdownDescription: "Number of agents assigned to this connection."},
		"discovery_mode":      schema.StringAttribute{Computed: true, MarkdownDescription: "Saved discovery mode."},
		"discovery_url":       schema.StringAttribute{Computed: true, MarkdownDescription: "Saved discovery URL, absent for inline cards."},
	}
	attributes["organization_id"] = schema.StringAttribute{Computed: true, MarkdownDescription: "Organization id."}
	attributes["author_id"] = schema.StringAttribute{Computed: true, MarkdownDescription: "Author id."}
	attributes["card_hash"] = schema.StringAttribute{Computed: true, MarkdownDescription: "Card hash."}
	attributes["last_discovered_at"] = schema.StringAttribute{Computed: true, MarkdownDescription: "Last discovered at."}
	attributes["created_by_service_account_id"] = schema.StringAttribute{Computed: true, MarkdownDescription: "Created by service account id."}
	attributes["created_at"] = schema.StringAttribute{Computed: true, MarkdownDescription: "Created at."}
	attributes["updated_at"] = schema.StringAttribute{Computed: true, MarkdownDescription: "Updated at."}
	attributes["tool_id"] = schema.StringAttribute{Computed: true, MarkdownDescription: "Tool id."}
	attributes["last_used_at"] = schema.StringAttribute{Computed: true, MarkdownDescription: "Last used at."}
	attributes["author_name"] = schema.StringAttribute{Computed: true, MarkdownDescription: "Author name."}
	resp.Schema = schema.Schema{MarkdownDescription: "External A2A agent registration and its saved connection. Registration does not execute an outbound agent run. Source uses the current JSON Schema Agent Card format; protobuf JSON cards must be converted before declaration. Credential values are write-only and preserved on metadata changes; imports require explicit replacement authentication before changing an authenticated discovery source. Destroy removes the registration and assignments, preserving external agent servers.", Attributes: attributes}
}
func (r *RemoteAgentResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
func (r *RemoteAgentResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	defer finishParentCreate(ctx, r.client, "externalAgent", "id", resp)

	var data RemoteAgentResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	patch := MergePatch(ctx, req.Plan.Raw, tftypes.NewValue(req.Plan.Raw.Type(), nil), remoteAgentAttrSpec, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	delete(patch, "enabled")
	LogPatch(ctx, "archestra_remote_agent Create", patch, remoteAgentAttrSpec)
	body, err := json.Marshal(patch)
	if err != nil {
		resp.Diagnostics.AddError("Request Encoding Failed", err.Error())
		return
	}
	result, err := r.client.CreateA2aRemoteAgentWithBodyWithResponse(ctx, "application/json", bytes.NewReader(body))
	if err != nil {
		resp.Diagnostics.AddError("API Error", err.Error())
		return
	}
	if result.JSON200 == nil {
		resp.Diagnostics.AddError("Creation Failed", fmt.Sprintf("API returned status %d", result.StatusCode()))
		return
	}
	desiredEnabled := data.Enabled
	r.flatten(&data, result.Body, &resp.Diagnostics)
	resolveInitialGrants(&data.InitialGrants)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !desiredEnabled.IsNull() && !desiredEnabled.IsUnknown() && desiredEnabled.ValueBool() != data.Enabled.ValueBool() {
		if !r.update(ctx, &data, map[string]any{"enabled": desiredEnabled.ValueBool()}, &resp.Diagnostics) {
			return
		}
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
	}
}
func (r *RemoteAgentResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data RemoteAgentResourceModel
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
	result, err := r.client.GetA2aRemoteAgentWithResponse(ctx, id)
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
	r.flatten(&data, result.Body, &resp.Diagnostics)
	readParentPermissions(ctx, r.client, "externalAgent", data.ID.ValueString(), &data.InitialGrants, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
func (r *RemoteAgentResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	finishPermissions := beginParentPermissionUpdate(ctx, r.client, "externalAgent", "id", req, resp)
	if resp.Diagnostics.HasError() {
		return
	}
	defer finishPermissions()

	var data RemoteAgentResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	patch := MergePatch(ctx, req.Plan.Raw, req.State.Raw, remoteAgentAttrSpec, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	var prior RemoteAgentResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &prior)...)
	if resp.Diagnostics.HasError() {
		return
	}
	for _, field := range []struct {
		key               string
		planned, previous jsontypes.Normalized
	}{{"source", data.Source, prior.Source}, {"auth", data.Auth, prior.Auth}} {
		if field.planned.IsNull() || field.planned.IsUnknown() || field.previous.IsNull() || field.previous.IsUnknown() {
			continue
		}
		equal, d := field.planned.StringSemanticEquals(ctx, field.previous)
		resp.Diagnostics.Append(d...)
		if equal {
			delete(patch, field.key)
		}
	}
	if resp.Diagnostics.HasError() {
		return
	}
	// The backend requires a replacement credential when an authenticated source changes.
	if _, changed := patch["source"]; changed {
		if !data.Auth.IsNull() && !data.Auth.IsUnknown() {
			patch["auth"] = json.RawMessage(data.Auth.ValueString())
		}
	}
	if len(patch) == 0 {
		resp.State = req.State
		return
	}
	if !r.update(ctx, &data, patch, &resp.Diagnostics) {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
func (r *RemoteAgentResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data RemoteAgentResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	id, err := uuid.Parse(data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid ID", err.Error())
		return
	}
	result, err := r.client.DeleteA2aRemoteAgentWithResponse(ctx, id)
	if err != nil {
		resp.Diagnostics.AddError("API Error", err.Error())
		return
	}
	if result.StatusCode() != 200 && result.StatusCode() != 204 && !IsNotFound(result) {
		resp.Diagnostics.AddError("Delete Failed", fmt.Sprintf("API returned status %d", result.StatusCode()))
	}
}
func (r *RemoteAgentResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
func (r *RemoteAgentResource) update(ctx context.Context, data *RemoteAgentResourceModel, patch map[string]any, diags *diag.Diagnostics) bool {
	id, err := uuid.Parse(data.ID.ValueString())
	if err != nil {
		diags.AddError("Invalid ID", err.Error())
		return false
	}
	LogPatch(ctx, "archestra_remote_agent Update", patch, remoteAgentAttrSpec)
	body, err := json.Marshal(patch)
	if err != nil {
		diags.AddError("Request Encoding Failed", err.Error())
		return false
	}
	result, err := r.client.UpdateA2aRemoteAgentWithBodyWithResponse(ctx, id, "application/json", bytes.NewReader(body))
	if err != nil {
		diags.AddError("API Error", err.Error())
		return false
	}
	if result.JSON200 == nil {
		diags.AddError("Update Failed", fmt.Sprintf("API returned status %d", result.StatusCode()))
		return false
	}
	r.flatten(data, result.Body, diags)
	return !diags.HasError()
}
func (r *RemoteAgentResource) flatten(data *RemoteAgentResourceModel, body []byte, diags *diag.Diagnostics) {
	var value struct {
		ID                        string          `json:"id"`
		Name                      string          `json:"name"`
		Description               *string         `json:"description"`
		AgentCard                 json.RawMessage `json:"agentCard"`
		Connection                json.RawMessage `json:"connection"`
		CreatedBy                 json.RawMessage `json:"createdBy"`
		AssignmentCount           int64           `json:"assignmentCount"`
		DiscoveryMode             string          `json:"discoveryMode"`
		DiscoveryURL              *string         `json:"discoveryUrl"`
		OrganizationID            *string         `json:"organizationId"`
		AuthorID                  *string         `json:"authorId"`
		CardHash                  *string         `json:"cardHash"`
		LastDiscoveredAt          *string         `json:"lastDiscoveredAt"`
		CreatedByServiceAccountID *string         `json:"createdByServiceAccountId"`
		CreatedAt                 *string         `json:"createdAt"`
		UpdatedAt                 *string         `json:"updatedAt"`
		ToolID                    *string         `json:"toolId"`
		LastUsedAt                *string         `json:"lastUsedAt"`
		AuthorName                *string         `json:"authorName"`
	}
	if err := json.Unmarshal(body, &value); err != nil {
		diags.AddError("Invalid API Response", err.Error())
		return
	}
	var connection struct {
		ID            string         `json:"id"`
		Enabled       bool           `json:"enabled"`
		AuthType      string         `json:"authType"`
		HasCredential bool           `json:"hasCredential"`
		AuthConfig    map[string]any `json:"authConfig"`
	}
	if err := json.Unmarshal(value.Connection, &connection); err != nil {
		diags.AddError("Invalid Connection Response", err.Error())
		return
	}
	data.ID = types.StringValue(value.ID)
	data.Name = types.StringValue(value.Name)
	data.Description = types.StringPointerValue(value.Description)
	data.ConnectionID = types.StringValue(connection.ID)
	data.Enabled = types.BoolValue(connection.Enabled)
	data.AssignmentCount = types.Int64Value(value.AssignmentCount)
	data.DiscoveryMode = types.StringValue(value.DiscoveryMode)
	data.DiscoveryURL = types.StringPointerValue(value.DiscoveryURL)
	var err error
	data.AgentCard, err = normalizedOrganizationJSON(value.AgentCard)
	if err != nil {
		diags.AddError("Invalid Agent Card Response", err.Error())
		return
	}
	data.Connection, err = normalizedOrganizationJSON(value.Connection)
	if err != nil {
		diags.AddError("Invalid Connection Response", err.Error())
		return
	}
	data.CreatedBy, err = normalizedOrganizationJSON(value.CreatedBy)
	if err != nil {
		diags.AddError("Invalid Creator Response", err.Error())
		return
	}
	source := map[string]any{"type": value.DiscoveryMode}
	if value.DiscoveryMode == "inline_card" {
		source["agentCard"] = json.RawMessage(data.AgentCard.ValueString())
	} else {
		source["url"] = value.DiscoveryURL
	}
	encoded, err := json.Marshal(source)
	if err != nil {
		diags.AddError("Source Encoding Failed", err.Error())
		return
	}
	data.Source = jsontypes.NewNormalizedValue(string(encoded))
	if !data.Auth.IsNull() && !data.Auth.IsUnknown() {
		var auth map[string]any
		if err := json.Unmarshal([]byte(data.Auth.ValueString()), &auth); err != nil {
			diags.AddError("Invalid Authentication State", "Unable to decode saved authentication.")
			return
		}
		auth["type"] = connection.AuthType
		if connection.AuthType == "api_key" {
			auth["headerName"] = connection.AuthConfig["headerName"]
		} else {
			delete(auth, "headerName")
		}
		if connection.AuthType == "none" || !connection.HasCredential {
			delete(auth, "credential")
		}
		encoded, err := json.Marshal(auth)
		if err != nil {
			diags.AddError("Authentication State Encoding Failed", "Unable to encode authentication metadata.")
			return
		}
		data.Auth = jsontypes.NewNormalizedValue(string(encoded))
	}
	data.OrganizationID = types.StringPointerValue(value.OrganizationID)
	data.AuthorID = types.StringPointerValue(value.AuthorID)
	data.CardHash = types.StringPointerValue(value.CardHash)
	data.LastDiscoveredAt = types.StringPointerValue(value.LastDiscoveredAt)
	data.CreatedByServiceAccountID = types.StringPointerValue(value.CreatedByServiceAccountID)
	data.CreatedAt = types.StringPointerValue(value.CreatedAt)
	data.UpdatedAt = types.StringPointerValue(value.UpdatedAt)
	data.ToolID = types.StringPointerValue(value.ToolID)
	data.LastUsedAt = types.StringPointerValue(value.LastUsedAt)
	data.AuthorName = types.StringPointerValue(value.AuthorName)
}

type remoteAgentSourceValidator struct{}

func (remoteAgentSourceValidator) Description(context.Context) string {
	return "Inline cards use the canonical JSON Schema Agent Card representation."
}
func (v remoteAgentSourceValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}
func (remoteAgentSourceValidator) ValidateString(_ context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	var source struct {
		Type      string         `json:"type"`
		AgentCard map[string]any `json:"agentCard"`
	}
	if err := json.Unmarshal([]byte(req.ConfigValue.ValueString()), &source); err != nil {
		return
	}
	var keys map[string]json.RawMessage
	if err := json.Unmarshal([]byte(req.ConfigValue.ValueString()), &keys); err != nil {
		return
	}
	allowed := map[string]bool{"type": true}
	switch source.Type {
	case "well_known", "card_url":
		allowed["url"] = true
	case "inline_card":
		allowed["agentCard"] = true
	default:
		resp.Diagnostics.AddAttributeError(req.Path, "Invalid remote source", "Source type must be well_known, card_url, or inline_card.")
		return
	}
	for key := range keys {
		if !allowed[key] {
			resp.Diagnostics.AddAttributeError(req.Path, "Invalid remote source", "The source object contains a field outside the selected source variant.")
			return
		}
	}
	if source.Type == "inline_card" && remoteCardUsesProtobufJSON(source.AgentCard) {
		resp.Diagnostics.AddAttributeError(req.Path, "Unsupported Agent Card representation", "Use the canonical JSON Schema Agent Card representation rather than protobuf JSON so the declared source can be read back without changing shape.")
	}
}
func remoteCardUsesProtobufJSON(card map[string]any) bool {
	if schemes, ok := card["securitySchemes"].(map[string]any); ok {
		for _, entry := range schemes {
			if scheme, ok := entry.(map[string]any); ok {
				if _, exists := scheme["type"]; !exists {
					return true
				}
			}
		}
	}
	if requirements, ok := card["security"].([]any); ok {
		for _, entry := range requirements {
			if requirement, ok := entry.(map[string]any); ok {
				if _, exists := requirement["schemes"]; exists {
					return true
				}
			}
		}
	}
	if skills, ok := card["skills"].([]any); ok {
		for _, entry := range skills {
			if skill, ok := entry.(map[string]any); ok && remoteCardUsesProtobufJSON(skill) {
				return true
			}
		}
	}
	return false
}
