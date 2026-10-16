package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.ResourceWithImportState = &AgentSkillPolicyResource{}
var _ resource.ResourceWithValidateConfig = &AgentSkillPolicyResource{}

func NewAgentSkillPolicyResource() resource.Resource { return &AgentSkillPolicyResource{} }

type AgentSkillPolicyResource struct{ client *client.ClientWithResponses }
type AgentSkillPolicyResourceModel struct {
	ID                 types.String `tfsdk:"id"`
	AgentID            types.String `tfsdk:"agent_id"`
	Mode               types.String `tfsdk:"mode"`
	AllowedReferences  types.Set    `tfsdk:"allowed_references"`
	ExcludedReferences types.Set    `tfsdk:"excluded_references"`
	Revision           types.Int64  `tfsdk:"revision"`
}
type skillReferenceModel struct {
	Source      types.String `tfsdk:"source"`
	SkillID     types.String `tfsdk:"skill_id"`
	McpServerID types.String `tfsdk:"mcp_server_id"`
	URI         types.String `tfsdk:"uri"`
	PluginID    types.String `tfsdk:"plugin_id"`
	SkillPath   types.String `tfsdk:"skill_path"`
}
type skillReferenceWire struct {
	Source      string `json:"source"`
	SkillID     string `json:"skillId,omitempty"`
	McpServerID string `json:"mcpServerId,omitempty"`
	URI         string `json:"uri,omitempty"`
	PluginID    string `json:"pluginId,omitempty"`
	SkillPath   string `json:"skillPath,omitempty"`
}
type skillPolicyWire struct {
	Mode                string               `json:"mode"`
	Revision            int64                `json:"revision"`
	AllowedReferences   []skillReferenceWire `json:"allowedReferences"`
	ExcludedReferences  []skillReferenceWire `json:"excludedReferences"`
	HiddenAllowedCount  int                  `json:"hiddenAllowedCount"`
	HiddenExcludedCount int                  `json:"hiddenExcludedCount"`
}
type skillPolicyOperation struct {
	Op          string             `json:"op"`
	Disposition string             `json:"disposition"`
	Reference   skillReferenceWire `json:"reference"`
}

var skillReferenceType = types.ObjectType{AttrTypes: map[string]attr.Type{
	"source": types.StringType, "skill_id": types.StringType, "mcp_server_id": types.StringType,
	"uri": types.StringType, "plugin_id": types.StringType, "skill_path": types.StringType,
}}

func (r *AgentSkillPolicyResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_agent_skill_policy"
}
func (r *AgentSkillPolicyResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	references := func(description string) schema.SetNestedAttribute {
		return schema.SetNestedAttribute{Required: true, MarkdownDescription: description, Validators: []validator.Set{setvalidator.SizeAtMost(1000)}, NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
			"source":        schema.StringAttribute{Required: true, MarkdownDescription: "Reference source.", Validators: []validator.String{stringvalidator.OneOf("native", "external_mcp", "plugin")}},
			"skill_id":      schema.StringAttribute{Optional: true, MarkdownDescription: "Native skill UUID."},
			"mcp_server_id": schema.StringAttribute{Optional: true, MarkdownDescription: "Installed MCP server UUID for an external skill."},
			"uri":           schema.StringAttribute{Optional: true, MarkdownDescription: "External MCP skill URI."},
			"plugin_id":     schema.StringAttribute{Optional: true, MarkdownDescription: "Plugin UUID."},
			"skill_path":    schema.StringAttribute{Optional: true, MarkdownDescription: "Skill path within the plugin."},
		}}}
	}
	resp.Schema = schema.Schema{MarkdownDescription: "Authoritative activation-skill policy for one internal agent. Declare at most one per agent. Updates reject concurrent revisions and policies containing unavailable or hidden references. Destroy resets the policy to manual mode with no references, preserving the agent.", Attributes: map[string]schema.Attribute{
		"id":                  schema.StringAttribute{Computed: true, MarkdownDescription: "Agent UUID identifying the policy."},
		"agent_id":            schema.StringAttribute{Required: true, MarkdownDescription: "Internal agent UUID.", PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
		"mode":                schema.StringAttribute{Required: true, MarkdownDescription: "Skill activation mode.", Validators: []validator.String{stringvalidator.OneOf("all", "manual")}},
		"allowed_references":  references("Complete set of explicitly allowed skill references."),
		"excluded_references": references("Complete set of explicitly excluded skill references."),
		"revision":            schema.Int64Attribute{Computed: true, MarkdownDescription: "Last observed policy revision for optimistic concurrency."},
	}}
}
func (r *AgentSkillPolicyResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
func (r *AgentSkillPolicyResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	for _, name := range []string{"allowed_references", "excluded_references"} {
		var refs types.Set
		resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root(name), &refs)...)
		if refs.IsNull() || refs.IsUnknown() {
			continue
		}
		var values []skillReferenceModel
		resp.Diagnostics.Append(refs.ElementsAs(ctx, &values, true)...)
		for _, v := range values {
			if v.Source.IsUnknown() || v.SkillID.IsUnknown() || v.McpServerID.IsUnknown() || v.URI.IsUnknown() || v.PluginID.IsUnknown() || v.SkillPath.IsUnknown() {
				continue
			}
			ref := v.wire()
			valid := false
			switch ref.Source {
			case "native":
				valid = validSkillUUID(ref.SkillID) && ref.McpServerID == "" && ref.URI == "" && ref.PluginID == "" && ref.SkillPath == ""
			case "external_mcp":
				valid = validSkillUUID(ref.McpServerID) && ref.URI != "" && ref.SkillID == "" && ref.PluginID == "" && ref.SkillPath == ""
			case "plugin":
				valid = validSkillUUID(ref.PluginID) && ref.SkillPath != "" && ref.SkillID == "" && ref.McpServerID == "" && ref.URI == ""
			}
			if !valid {
				resp.Diagnostics.AddAttributeError(path.Root(name), "Invalid Skill Reference", "Set only skill_id for native, mcp_server_id and uri for external_mcp, or plugin_id and skill_path for plugin; identifiers must be UUIDs.")
			}
		}
	}
}
func validSkillUUID(value string) bool { _, err := uuid.Parse(value); return err == nil }
func (v skillReferenceModel) wire() skillReferenceWire {
	return skillReferenceWire{Source: v.Source.ValueString(), SkillID: v.SkillID.ValueString(), McpServerID: v.McpServerID.ValueString(), URI: v.URI.ValueString(), PluginID: v.PluginID.ValueString(), SkillPath: v.SkillPath.ValueString()}
}
func (r *AgentSkillPolicyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan AgentSkillPolicyResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	current := r.get(ctx, plan.AgentID.ValueString(), &resp.Diagnostics)
	if current == nil {
		if !resp.Diagnostics.HasError() {
			resp.Diagnostics.AddError("Agent Not Found", "Create the internal agent before its skill policy.")
		}
		return
	}
	r.replace(ctx, &plan, current, &resp.Diagnostics)
	if !resp.Diagnostics.HasError() {
		resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
	}
}
func (r *AgentSkillPolicyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data AgentSkillPolicyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if data.ID.IsNull() || data.ID.IsUnknown() || data.ID.ValueString() == "" {
		resp.State.RemoveResource(ctx)
		return
	}
	current := r.get(ctx, data.AgentID.ValueString(), &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	if current == nil {
		resp.State.RemoveResource(ctx)
		return
	}
	data.populate(ctx, current, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
func (r *AgentSkillPolicyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, prior AgentSkillPolicyResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &prior)...)
	if resp.Diagnostics.HasError() {
		return
	}
	current := r.get(ctx, plan.AgentID.ValueString(), &resp.Diagnostics)
	if current == nil {
		if !resp.Diagnostics.HasError() {
			resp.Diagnostics.AddError("Agent Not Found", "The policy's agent no longer exists.")
		}
		return
	}
	if current.Revision != prior.Revision.ValueInt64() {
		resp.Diagnostics.AddError("Concurrent Skill Policy Update", "The policy changed since refresh; refresh and review a new plan before applying.")
		return
	}
	r.replace(ctx, &plan, current, &resp.Diagnostics)
	if !resp.Diagnostics.HasError() {
		resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
	}
}
func (r *AgentSkillPolicyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data AgentSkillPolicyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	current := r.get(ctx, data.AgentID.ValueString(), &resp.Diagnostics)
	if current == nil {
		return
	}
	if current.Revision != data.Revision.ValueInt64() {
		resp.Diagnostics.AddError("Concurrent Skill Policy Update", "The policy changed since refresh; refresh and review destruction again.")
		return
	}
	data.Mode = types.StringValue("manual")
	data.AllowedReferences = types.SetValueMust(skillReferenceType, []attr.Value{})
	data.ExcludedReferences = types.SetValueMust(skillReferenceType, []attr.Value{})
	r.replace(ctx, &data, current, &resp.Diagnostics)
}
func (r *AgentSkillPolicyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	if !validSkillUUID(req.ID) {
		resp.Diagnostics.AddError("Invalid Agent ID", "Import by internal agent UUID.")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("agent_id"), req.ID)...)
}
func (r *AgentSkillPolicyResource) get(ctx context.Context, id string, diags *diag.Diagnostics) *skillPolicyWire {
	agentID, err := uuid.Parse(id)
	if err != nil {
		diags.AddError("Invalid Agent ID", err.Error())
		return nil
	}
	response, err := r.client.GetAgentActivationSkillPolicyWithResponse(ctx, agentID)
	if err != nil {
		diags.AddError("API Error", fmt.Sprintf("Unable to read skill policy: %s", err))
		return nil
	}
	if response.StatusCode() == 404 {
		return nil
	}
	if response.JSON200 == nil {
		diags.AddError("API Error", fmt.Sprintf("Unable to read skill policy: status %d", response.StatusCode()))
		return nil
	}
	return decodeSkillPolicy(response.Body, diags)
}
func decodeSkillPolicy(body []byte, diags *diag.Diagnostics) *skillPolicyWire {
	var value skillPolicyWire
	if err := json.Unmarshal(body, &value); err != nil {
		diags.AddError("Invalid Skill Policy Response", err.Error())
		return nil
	}
	if value.HiddenAllowedCount != 0 || value.HiddenExcludedCount != 0 {
		diags.AddError("Incomplete Skill Policy Visibility", "The API omits unavailable or hidden skill references. Restore their availability or remove them explicitly before managing this authoritative policy; no policy write was made.")
		return nil
	}
	return &value
}
func (r *AgentSkillPolicyResource) replace(ctx context.Context, plan *AgentSkillPolicyResourceModel, current *skillPolicyWire, diags *diag.Diagnostics) {
	var allowed, excluded []skillReferenceModel
	diags.Append(plan.AllowedReferences.ElementsAs(ctx, &allowed, false)...)
	diags.Append(plan.ExcludedReferences.ElementsAs(ctx, &excluded, false)...)
	if diags.HasError() {
		return
	}
	target := skillPolicyWire{Mode: plan.Mode.ValueString()}
	for _, v := range allowed {
		target.AllowedReferences = append(target.AllowedReferences, v.wire())
	}
	for _, v := range excluded {
		target.ExcludedReferences = append(target.ExcludedReferences, v.wire())
	}
	operations := skillPolicyDiff(*current, target)
	if len(operations) == 0 && current.Mode == target.Mode {
		plan.populate(ctx, current, diags)
		return
	}
	if len(operations) > 2000 {
		diags.AddError("Skill Policy Change Too Large", "This change exceeds the API's 2000-operation atomic limit; apply a smaller reviewed reference-set change first.")
		return
	}
	body, err := json.Marshal(struct {
		ExpectedRevision int64                  `json:"expectedRevision"`
		Mode             string                 `json:"mode"`
		Operations       []skillPolicyOperation `json:"operations"`
	}{current.Revision, target.Mode, operations})
	if err != nil {
		diags.AddError("Skill Policy Encoding Error", err.Error())
		return
	}
	id, err := uuid.Parse(plan.AgentID.ValueString())
	if err != nil {
		diags.AddError("Invalid Agent ID", err.Error())
		return
	}
	response, err := r.client.PatchAgentActivationSkillPolicyWithBodyWithResponse(ctx, id, "application/json", bytes.NewReader(body))
	if err != nil {
		diags.AddError("API Error", fmt.Sprintf("Unable to update skill policy: %s", err))
		return
	}
	if response.StatusCode() == 409 {
		diags.AddError("Concurrent Skill Policy Update", "The API rejected a stale revision; refresh and review a new plan. No retry was attempted.")
		return
	}
	if response.JSON200 == nil {
		diags.AddError("API Error", fmt.Sprintf("Unable to update skill policy: status %d", response.StatusCode()))
		return
	}
	if updated := decodeSkillPolicy(response.Body, diags); updated != nil {
		plan.populate(ctx, updated, diags)
	}
}
func skillPolicyDiff(current, target skillPolicyWire) []skillPolicyOperation {
	operations := []skillPolicyOperation{}
	for _, entry := range []struct {
		disposition   string
		before, after []skillReferenceWire
	}{{"allow", current.AllowedReferences, target.AllowedReferences}, {"exclude", current.ExcludedReferences, target.ExcludedReferences}} {
		before, after := map[skillReferenceWire]bool{}, map[skillReferenceWire]bool{}
		for _, ref := range entry.before {
			before[ref] = true
		}
		for _, ref := range entry.after {
			after[ref] = true
		}
		for _, ref := range entry.before {
			if !after[ref] {
				operations = append(operations, skillPolicyOperation{"remove", entry.disposition, ref})
			}
		}
		for _, ref := range entry.after {
			if !before[ref] {
				operations = append(operations, skillPolicyOperation{"add", entry.disposition, ref})
			}
		}
	}
	return operations
}
func (plan *AgentSkillPolicyResourceModel) populate(ctx context.Context, value *skillPolicyWire, diags *diag.Diagnostics) {
	references := func(refs []skillReferenceWire) types.Set {
		values := make([]skillReferenceModel, 0, len(refs))
		optional := func(s string) types.String {
			if s == "" {
				return types.StringNull()
			}
			return types.StringValue(s)
		}
		for _, ref := range refs {
			values = append(values, skillReferenceModel{types.StringValue(ref.Source), optional(ref.SkillID), optional(ref.McpServerID), optional(ref.URI), optional(ref.PluginID), optional(ref.SkillPath)})
		}
		set, d := types.SetValueFrom(ctx, skillReferenceType, values)
		diags.Append(d...)
		return set
	}
	plan.ID = plan.AgentID
	plan.Mode = types.StringValue(value.Mode)
	plan.Revision = types.Int64Value(value.Revision)
	plan.AllowedReferences = references(value.AllowedReferences)
	plan.ExcludedReferences = references(value.ExcludedReferences)
}
func (r *AgentSkillPolicyResource) AttrSpecs() []AttrSpec {
	return []AttrSpec{
		{TFName: "agent_id", Kind: Synthetic}, {TFName: "mode", JSONName: "mode", Kind: Scalar},
		{TFName: "allowed_references", JSONName: "allowedReferences", Kind: Set}, {TFName: "excluded_references", JSONName: "excludedReferences", Kind: Set},
	}
}
func (r *AgentSkillPolicyResource) APIShape() any {
	return client.GetAgentActivationSkillPolicyResponse{}
}
func (r *AgentSkillPolicyResource) KnownIntentionallySkipped() []string {
	// Hydrated display records duplicate stable references; hidden counts gate authoritative reads and writes.
	return []string{"allowedSkills", "excludedSkills", "hiddenAllowedCount", "hiddenExcludedCount"}
}
