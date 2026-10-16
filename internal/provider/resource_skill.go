package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

var _ resource.ResourceWithImportState = &SkillResource{}

func NewSkillResource() resource.Resource { return &SkillResource{} }

type SkillResource struct{ client *client.ClientWithResponses }
type SkillResourceModel struct {
	GitSource                 types.Object      `tfsdk:"git_source"`
	InitialGrants             types.Set         `tfsdk:"permissions"`
	ID                        types.String      `tfsdk:"id"`
	Name                      types.String      `tfsdk:"name"`
	Description               types.String      `tfsdk:"description"`
	Content                   types.String      `tfsdk:"content"`
	Icon                      types.String      `tfsdk:"icon"`
	License                   types.String      `tfsdk:"license"`
	Compatibility             types.String      `tfsdk:"compatibility"`
	AgentName                 types.String      `tfsdk:"agent_name"`
	Templated                 types.Bool        `tfsdk:"templated"`
	Metadata                  types.Map         `tfsdk:"metadata"`
	AllowedTools              types.Set         `tfsdk:"allowed_tools"`
	Files                     types.Map         `tfsdk:"files"`
	FileDigests               types.Map         `tfsdk:"file_digests"`
	EnvironmentIDs            types.Set         `tfsdk:"environment_ids"`
	Labels                    []AgentLabelModel `tfsdk:"labels"`
	LatestVersion             types.Int64       `tfsdk:"latest_version"`
	UsageCount                types.Int64       `tfsdk:"usage_count"`
	CreatedBy                 types.String      `tfsdk:"created_by"`
	OrganizationID            types.String      `tfsdk:"organization_id"`
	AuthorID                  types.String      `tfsdk:"author_id"`
	Scope                     types.String      `tfsdk:"scope"`
	Digest                    types.String      `tfsdk:"digest"`
	SourceType                types.String      `tfsdk:"source_type"`
	SourceRef                 types.String      `tfsdk:"source_ref"`
	SourceOrigin              types.String      `tfsdk:"source_origin"`
	SourceCommit              types.String      `tfsdk:"source_commit"`
	GithubSyncInterval        types.String      `tfsdk:"github_sync_interval"`
	GithubSyncRef             types.String      `tfsdk:"github_sync_ref"`
	GithubAppConfigID         types.String      `tfsdk:"github_app_config_id"`
	GithubPatID               types.String      `tfsdk:"github_pat_id"`
	SourceProvider            types.String      `tfsdk:"source_provider"`
	GitlabTokenID             types.String      `tfsdk:"gitlab_token_id"`
	LastSyncedAt              types.String      `tfsdk:"last_synced_at"`
	LastSyncError             types.String      `tfsdk:"last_sync_error"`
	LastUsedAt                types.String      `tfsdk:"last_used_at"`
	CreatedByServiceAccountID types.String      `tfsdk:"created_by_service_account_id"`
	CreatedAt                 types.String      `tfsdk:"created_at"`
	UpdatedAt                 types.String      `tfsdk:"updated_at"`
	DeletedAt                 types.String      `tfsdk:"deleted_at"`
}
type skillFileModel struct {
	Content  types.String `tfsdk:"content"`
	Encoding types.String `tfsdk:"encoding"`
}
type skillAPIWire struct {
	ID            string            `json:"id"`
	Name          string            `json:"name"`
	Description   string            `json:"description"`
	Content       string            `json:"content"`
	Icon          *string           `json:"icon"`
	License       *string           `json:"license"`
	Compatibility *string           `json:"compatibility"`
	AgentName     *string           `json:"agentName"`
	Templated     bool              `json:"templated"`
	Metadata      map[string]string `json:"metadata"`
	AllowedTools  *string           `json:"allowedTools"`
	Files         []struct {
		Path     string  `json:"path"`
		Content  string  `json:"content"`
		Encoding string  `json:"encoding"`
		Digest   *string `json:"digest"`
	} `json:"files"`
	Environments []struct {
		ID string `json:"id"`
	} `json:"environments"`
	Labels []struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	} `json:"labels"`
	LatestVersion             int64           `json:"latestVersion"`
	UsageCount                int64           `json:"usageCount"`
	CreatedBy                 json.RawMessage `json:"createdBy"`
	OrganizationID            *string         `json:"organizationId"`
	AuthorID                  *string         `json:"authorId"`
	Scope                     *string         `json:"scope"`
	Digest                    *string         `json:"digest"`
	SourceType                *string         `json:"sourceType"`
	SourceRef                 *string         `json:"sourceRef"`
	SourceOrigin              *string         `json:"sourceOrigin"`
	SourceCommit              *string         `json:"sourceCommit"`
	GithubSyncInterval        *string         `json:"githubSyncInterval"`
	GithubSyncRef             *string         `json:"githubSyncRef"`
	GithubAppConfigID         *string         `json:"githubAppConfigId"`
	GithubPatID               *string         `json:"githubPatId"`
	SourceProvider            *string         `json:"sourceProvider"`
	GitlabTokenID             *string         `json:"gitlabTokenId"`
	LastSyncedAt              *string         `json:"lastSyncedAt"`
	LastSyncError             *string         `json:"lastSyncError"`
	LastUsedAt                *string         `json:"lastUsedAt"`
	CreatedByServiceAccountID *string         `json:"createdByServiceAccountId"`
	CreatedAt                 *string         `json:"createdAt"`
	UpdatedAt                 *string         `json:"updatedAt"`
	DeletedAt                 *string         `json:"deletedAt"`
}

var skillFileType = types.ObjectType{AttrTypes: map[string]attr.Type{"content": types.StringType, "encoding": types.StringType}}
var skillAttrSpec = []AttrSpec{
	{TFName: "git_source", Kind: Synthetic},
	initialGrantsAttrSpec,
	{TFName: "name", Kind: Synthetic}, {TFName: "description", Kind: Synthetic},
	{TFName: "license", Kind: Synthetic}, {TFName: "compatibility", Kind: Synthetic}, {TFName: "agent_name", Kind: Synthetic},
	{TFName: "templated", Kind: Synthetic}, {TFName: "metadata", Kind: Synthetic}, {TFName: "allowed_tools", Kind: Synthetic},
	{TFName: "content", JSONName: "content", Kind: Scalar},
	{TFName: "icon", JSONName: "icon", Kind: Scalar},
	{TFName: "files", JSONName: "files", Kind: Map, Encoder: encodeSkillFiles, Children: []AttrSpec{{TFName: "content", JSONName: "content", Kind: Scalar}, {TFName: "encoding", JSONName: "encoding", Kind: Scalar}}},
	{TFName: "environment_ids", JSONName: "environmentIds", Kind: Set},
	{TFName: "labels", JSONName: "labels", Kind: Set, Children: []AttrSpec{{TFName: "key", JSONName: "key", Kind: Scalar}, {TFName: "value", JSONName: "value", Kind: Scalar}}},
}

func (r *SkillResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_skill"
}
func (r *SkillResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	attributes := map[string]schema.Attribute{
		"permissions":   initialGrantsAttribute(),
		"id":            schema.StringAttribute{Computed: true, MarkdownDescription: "Skill UUID.", PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
		"name":          schema.StringAttribute{Required: true, MarkdownDescription: "Skill name rendered into manifest frontmatter.", Validators: []validator.String{stringvalidator.LengthAtLeast(1)}},
		"description":   schema.StringAttribute{Required: true, MarkdownDescription: "Skill description rendered into manifest frontmatter.", Validators: []validator.String{stringvalidator.LengthAtLeast(1)}},
		"content":       schema.StringAttribute{Required: true, MarkdownDescription: "Markdown instruction body without frontmatter."},
		"icon":          schema.StringAttribute{Optional: true, MarkdownDescription: "Display emoji or image data URL."},
		"license":       schema.StringAttribute{Optional: true, MarkdownDescription: "Manifest license."},
		"compatibility": schema.StringAttribute{Optional: true, MarkdownDescription: "Manifest compatibility information."},
		"agent_name":    schema.StringAttribute{Optional: true, MarkdownDescription: "Agent name used when activating this skill delegates execution."},
		"templated":     schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), MarkdownDescription: "Whether the instruction body uses Handlebars rendering."},
		"metadata":      schema.MapAttribute{Optional: true, ElementType: types.StringType, MarkdownDescription: "Manifest metadata entries."},
		"allowed_tools": schema.SetAttribute{Optional: true, ElementType: types.StringType, MarkdownDescription: "Tools pre-approved by the skill manifest.", Validators: []validator.Set{setvalidator.ValueStringsAre(stringvalidator.RegexMatches(regexp.MustCompile(`^\S+$`), "tool names cannot contain whitespace"))}},
		"files": schema.MapNestedAttribute{Required: true, MarkdownDescription: "Complete resource files keyed by relative path; an empty map clears files.", NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
			"content":  schema.StringAttribute{Required: true, MarkdownDescription: "Resource file content in the specified encoding."},
			"encoding": schema.StringAttribute{Required: true, MarkdownDescription: "Resource file content encoding.", Validators: []validator.String{stringvalidator.OneOf("utf8", "base64")}},
		}}},
		"file_digests":    schema.MapAttribute{Computed: true, ElementType: types.StringType, MarkdownDescription: "Backend content digests keyed by resource file path."},
		"environment_ids": schema.SetAttribute{Required: true, ElementType: types.StringType, MarkdownDescription: "Complete environment restrictions; an empty set allows every environment."},
		"labels":          schema.SetNestedAttribute{Required: true, MarkdownDescription: "Complete set of resource labels.", NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{"key": schema.StringAttribute{Required: true}, "value": schema.StringAttribute{Required: true}}}},
		"latest_version":  schema.Int64Attribute{Computed: true, MarkdownDescription: "Latest immutable content version used for optimistic updates."},
		"usage_count":     schema.Int64Attribute{Computed: true, MarkdownDescription: "Recorded activation count."},
		"created_by":      schema.StringAttribute{Computed: true, MarkdownDescription: "Creator identity metadata as JSON."},
	}
	attributes["organization_id"] = schema.StringAttribute{Computed: true, MarkdownDescription: "Organization id reported by the API."}
	attributes["author_id"] = schema.StringAttribute{Computed: true, MarkdownDescription: "Author id reported by the API."}
	attributes["scope"] = schema.StringAttribute{Computed: true, MarkdownDescription: "Scope reported by the API."}
	attributes["digest"] = schema.StringAttribute{Computed: true, MarkdownDescription: "Digest reported by the API."}
	attributes["source_type"] = schema.StringAttribute{Computed: true, MarkdownDescription: "Source type reported by the API."}
	attributes["source_ref"] = schema.StringAttribute{Computed: true, MarkdownDescription: "Source ref reported by the API."}
	attributes["source_origin"] = schema.StringAttribute{Computed: true, MarkdownDescription: "Source origin reported by the API."}
	attributes["source_commit"] = schema.StringAttribute{Computed: true, MarkdownDescription: "Source commit reported by the API."}
	attributes["github_sync_interval"] = schema.StringAttribute{Computed: true, MarkdownDescription: "Github sync interval reported by the API."}
	attributes["github_sync_ref"] = schema.StringAttribute{Computed: true, MarkdownDescription: "Github sync ref reported by the API."}
	attributes["github_app_config_id"] = schema.StringAttribute{Computed: true, MarkdownDescription: "Github app config id reported by the API."}
	attributes["github_pat_id"] = schema.StringAttribute{Computed: true, MarkdownDescription: "Github pat id reported by the API."}
	attributes["source_provider"] = schema.StringAttribute{Computed: true, MarkdownDescription: "Source provider reported by the API."}
	attributes["gitlab_token_id"] = schema.StringAttribute{Computed: true, MarkdownDescription: "Gitlab token id reported by the API."}
	attributes["last_synced_at"] = schema.StringAttribute{Computed: true, MarkdownDescription: "Last synced at reported by the API."}
	attributes["last_sync_error"] = schema.StringAttribute{Computed: true, MarkdownDescription: "Last sync error reported by the API."}
	attributes["last_used_at"] = schema.StringAttribute{Computed: true, MarkdownDescription: "Last used at reported by the API."}
	attributes["created_by_service_account_id"] = schema.StringAttribute{Computed: true, MarkdownDescription: "Created by service account id reported by the API."}
	attributes["created_at"] = schema.StringAttribute{Computed: true, MarkdownDescription: "Created at reported by the API."}
	attributes["updated_at"] = schema.StringAttribute{Computed: true, MarkdownDescription: "Updated at reported by the API."}
	attributes["deleted_at"] = schema.StringAttribute{Computed: true, MarkdownDescription: "Deleted at reported by the API."}
	if err := configureSkillGitSchema(attributes); err != nil {
		resp.Diagnostics.AddError("Unexpected Skill schema", err.Error())
		return
	}
	resp.Schema = schema.Schema{MarkdownDescription: "A versioned skill with declarative manifest metadata, instruction body and complete resource files. Import by UUID. Updates reject stale versions. Destroy soft-deletes the skill; immutable historical versions remain under platform retention. Configure sharing through permissions on this skill. Git-synchronized content remains owned by its external source and the API rejects conflicting edits.", Attributes: attributes}
}
func (r *SkillResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
func (r *SkillResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	defer finishParentCreate(ctx, r.client, "skill", "id", resp)

	var plan SkillResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !plan.GitSource.IsNull() && !plan.GitSource.IsUnknown() {
		r.createGitSkill(ctx, req, resp, plan)
		return
	}
	patch := MergePatch(ctx, req.Plan.Raw, tftypes.NewValue(req.Plan.Raw.Type(), nil), skillAttrSpec, &resp.Diagnostics)
	body := plan.request(ctx, patch, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	result, err := r.client.CreateSkillWithBodyWithResponse(ctx, "application/json", bytes.NewReader(body))
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to create skill: %s", err))
		return
	}
	if result.JSON200 == nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to create skill: status %d", result.StatusCode()))
		return
	}
	plan.populate(ctx, result.Body, &resp.Diagnostics)
	if !resp.Diagnostics.HasError() {
		resolveInitialGrants(&plan.InitialGrants)
		resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
	}
}
func (r *SkillResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data SkillResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if data.ID.IsNull() || data.ID.IsUnknown() || data.ID.ValueString() == "" {
		resp.State.RemoveResource(ctx)
		return
	}
	id, err := uuid.Parse(data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid Skill ID", err.Error())
		return
	}
	result, err := r.client.GetSkillWithResponse(ctx, id)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to read skill: %s", err))
		return
	}
	if result.StatusCode() == 404 {
		resp.State.RemoveResource(ctx)
		return
	}
	if result.JSON200 == nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to read skill: status %d", result.StatusCode()))
		return
	}
	data.populate(ctx, result.Body, &resp.Diagnostics)
	if !resp.Diagnostics.HasError() {
		resolveInitialGrants(&data.InitialGrants)
		readParentPermissions(ctx, r.client, "skill", data.ID.ValueString(), &data.InitialGrants, &resp.Diagnostics)
		if resp.Diagnostics.HasError() {
			return
		}
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
	}
}
func (r *SkillResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	finishPermissions := beginParentPermissionUpdate(ctx, r.client, "skill", "id", req, resp)
	if resp.Diagnostics.HasError() {
		return
	}
	defer finishPermissions()

	var plan, prior SkillResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &prior)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !plan.GitSource.IsNull() && !plan.GitSource.IsUnknown() {
		r.updateGitSkill(ctx, req, resp, plan, prior)
		return
	}
	patch := MergePatch(ctx, req.Plan.Raw, req.State.Raw, skillAttrSpec, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	if len(patch) == 0 && !skillManifestChanged(req.Plan.Raw, req.State.Raw) {
		resp.State = req.State
		return
	}
	if prior.LatestVersion.IsNull() || prior.LatestVersion.IsUnknown() {
		resp.Diagnostics.AddError("Missing Skill Version", "Refresh the skill before updating so the API can reject concurrent edits.")
		return
	}
	patch["baseVersion"] = prior.LatestVersion.ValueInt64()
	body := plan.request(ctx, patch, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	id, err := uuid.Parse(prior.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid Skill ID", err.Error())
		return
	}
	result, err := r.client.UpdateSkillWithBodyWithResponse(ctx, id, "application/json", bytes.NewReader(body))
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to update skill: %s", err))
		return
	}
	if result.StatusCode() == 409 {
		resp.Diagnostics.AddError("Skill Update Conflict", "The skill version changed or its name conflicts with another skill; refresh and review a new plan. No retry was attempted.")
		return
	}
	if result.JSON200 == nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to update skill: status %d", result.StatusCode()))
		return
	}
	plan.populate(ctx, result.Body, &resp.Diagnostics)
	if !resp.Diagnostics.HasError() {
		resolveInitialGrants(&plan.InitialGrants)
		resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
	}
}
func (r *SkillResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data SkillResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	id, err := uuid.Parse(data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid Skill ID", err.Error())
		return
	}
	result, err := r.client.DeleteSkillWithResponse(ctx, id)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to delete skill: %s", err))
		return
	}
	if result.StatusCode() != 200 && result.StatusCode() != 404 {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to soft-delete skill: status %d", result.StatusCode()))
	}
}
func (r *SkillResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	if _, err := uuid.Parse(req.ID); err != nil {
		resp.Diagnostics.AddError("Invalid Skill ID", "Import by skill UUID.")
		return
	}
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
func (r *SkillResource) AttrSpecs() []AttrSpec { return skillAttrSpec }
func (r *SkillResource) APIShape() any         { return client.GetSkillResponse{} }
func (r *SkillResource) KnownIntentionallySkipped() []string {
	// Environment identities map to environment_ids. Sharing projections belong to the separate permission policy.
	return []string{"environments", "teams", "users"}
}
func encodeSkillFiles(value any) any {
	files, ok := value.(map[string]any)
	if !ok {
		return value
	}
	paths := make([]string, 0, len(files))
	for path := range files {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	result := make([]any, 0, len(paths))
	for _, path := range paths {
		entry, ok := files[path].(map[string]any)
		if !ok {
			continue
		}
		result = append(result, map[string]any{"path": path, "content": entry["content"], "encoding": entry["encoding"]})
	}
	return result
}

// Manifest attributes are synthetic wire inputs. Their edits still require
// rebuilding content when the generic field patch is otherwise empty.
func skillManifestChanged(plan, prior tftypes.Value) bool {
	for _, spec := range skillAttrSpec {
		if spec.Kind == Synthetic && spec.TFName != "git_source" && !lookupOrNull(plan, spec.TFName).Equal(lookupOrNull(prior, spec.TFName)) {
			return true
		}
	}
	return false
}

func (plan *SkillResourceModel) request(ctx context.Context, patch map[string]any, diags *diag.Diagnostics) []byte {
	header := map[string]any{"name": plan.Name.ValueString(), "description": plan.Description.ValueString(), "templated": plan.Templated.ValueBool()}
	for key, value := range map[string]types.String{"license": plan.License, "compatibility": plan.Compatibility, "agent": plan.AgentName} {
		if !value.IsNull() && !value.IsUnknown() {
			header[key] = value.ValueString()
		}
	}
	if !plan.Metadata.IsNull() && !plan.Metadata.IsUnknown() {
		var metadata map[string]string
		diags.Append(plan.Metadata.ElementsAs(ctx, &metadata, false)...)
		header["metadata"] = metadata
	}
	if !plan.AllowedTools.IsNull() && !plan.AllowedTools.IsUnknown() {
		var tools []string
		diags.Append(plan.AllowedTools.ElementsAs(ctx, &tools, false)...)
		sort.Strings(tools)
		header["allowed-tools"] = strings.Join(tools, " ")
	}
	if diags.HasError() {
		return nil
	}
	encoded, err := json.Marshal(header)
	if err != nil {
		diags.AddError("Skill Manifest Encoding Error", err.Error())
		return nil
	}
	// JSON is a YAML mapping, so frontmatter retains arbitrary Unicode and quoting without a second parser.
	patch["content"] = "---\n" + string(encoded) + "\n---\n" + plan.Content.ValueString()
	body, err := json.Marshal(patch)
	if err != nil {
		diags.AddError("Skill Request Encoding Error", err.Error())
	}
	return body
}
func skillTextFromAPI(prior types.String, value string) types.String {
	// The manifest parser trims outer whitespace; preserve equivalent configured formatting only.
	if !prior.IsNull() && !prior.IsUnknown() && strings.TrimSpace(prior.ValueString()) == value {
		return prior
	}
	return types.StringValue(value)
}
func skillOptionalTextFromAPI(prior types.String, value *string) types.String {
	if value == nil {
		if !prior.IsNull() && !prior.IsUnknown() && strings.TrimSpace(prior.ValueString()) == "" {
			return prior
		}
		return types.StringNull()
	}
	return skillTextFromAPI(prior, *value)
}
func (plan *SkillResourceModel) populate(ctx context.Context, body []byte, diags *diag.Diagnostics) {
	var wire skillAPIWire
	if err := json.Unmarshal(body, &wire); err != nil {
		diags.AddError("Invalid Skill Response", err.Error())
		return
	}
	plan.ID = types.StringValue(wire.ID)
	plan.Name = skillTextFromAPI(plan.Name, wire.Name)
	plan.Description = skillTextFromAPI(plan.Description, wire.Description)
	plan.Content = skillTextFromAPI(plan.Content, wire.Content)
	plan.Icon = types.StringPointerValue(wire.Icon)
	plan.License = skillOptionalTextFromAPI(plan.License, wire.License)
	plan.Compatibility = skillOptionalTextFromAPI(plan.Compatibility, wire.Compatibility)
	plan.AgentName = skillOptionalTextFromAPI(plan.AgentName, wire.AgentName)
	plan.Templated = types.BoolValue(wire.Templated)
	var d diag.Diagnostics
	if len(wire.Metadata) == 0 && plan.Metadata.IsNull() {
		plan.Metadata = types.MapNull(types.StringType)
	} else {
		plan.Metadata, d = types.MapValueFrom(ctx, types.StringType, wire.Metadata)
		diags.Append(d...)
	}
	if wire.AllowedTools == nil && plan.AllowedTools.IsNull() {
		plan.AllowedTools = types.SetNull(types.StringType)
	} else {
		tools := []string{}
		if wire.AllowedTools != nil {
			tools = strings.Fields(*wire.AllowedTools)
		}
		plan.AllowedTools, d = types.SetValueFrom(ctx, types.StringType, tools)
		diags.Append(d...)
	}
	files := map[string]skillFileModel{}
	digests := map[string]attr.Value{}
	for _, file := range wire.Files {
		files[file.Path] = skillFileModel{types.StringValue(file.Content), types.StringValue(file.Encoding)}
		digests[file.Path] = types.StringPointerValue(file.Digest)
	}
	plan.Files, d = types.MapValueFrom(ctx, skillFileType, files)
	diags.Append(d...)
	plan.FileDigests, d = types.MapValue(types.StringType, digests)
	diags.Append(d...)
	environments := make([]string, 0, len(wire.Environments))
	for _, env := range wire.Environments {
		environments = append(environments, env.ID)
	}
	plan.EnvironmentIDs, d = types.SetValueFrom(ctx, types.StringType, environments)
	diags.Append(d...)
	plan.Labels = make([]AgentLabelModel, 0, len(wire.Labels))
	for _, label := range wire.Labels {
		plan.Labels = append(plan.Labels, AgentLabelModel{Key: types.StringValue(label.Key), Value: types.StringValue(label.Value)})
	}
	plan.LatestVersion = types.Int64Value(wire.LatestVersion)
	plan.UsageCount = types.Int64Value(wire.UsageCount)
	plan.CreatedBy = types.StringValue(string(wire.CreatedBy))
	plan.OrganizationID = types.StringPointerValue(wire.OrganizationID)
	plan.AuthorID = types.StringPointerValue(wire.AuthorID)
	plan.Scope = types.StringPointerValue(wire.Scope)
	plan.Digest = types.StringPointerValue(wire.Digest)
	plan.SourceType = types.StringPointerValue(wire.SourceType)
	plan.SourceRef = types.StringPointerValue(wire.SourceRef)
	plan.SourceOrigin = types.StringPointerValue(wire.SourceOrigin)
	plan.SourceCommit = types.StringPointerValue(wire.SourceCommit)
	plan.GithubSyncInterval = types.StringPointerValue(wire.GithubSyncInterval)
	plan.GithubSyncRef = types.StringPointerValue(wire.GithubSyncRef)
	plan.GithubAppConfigID = types.StringPointerValue(wire.GithubAppConfigID)
	plan.GithubPatID = types.StringPointerValue(wire.GithubPatID)
	plan.SourceProvider = types.StringPointerValue(wire.SourceProvider)
	plan.GitlabTokenID = types.StringPointerValue(wire.GitlabTokenID)
	plan.LastSyncedAt = types.StringPointerValue(wire.LastSyncedAt)
	plan.LastSyncError = types.StringPointerValue(wire.LastSyncError)
	plan.LastUsedAt = types.StringPointerValue(wire.LastUsedAt)
	plan.CreatedByServiceAccountID = types.StringPointerValue(wire.CreatedByServiceAccountID)
	plan.CreatedAt = types.StringPointerValue(wire.CreatedAt)
	plan.UpdatedAt = types.StringPointerValue(wire.UpdatedAt)
	plan.DeletedAt = types.StringPointerValue(wire.DeletedAt)
	plan.populateGitSource(ctx, wire, diags)
}
