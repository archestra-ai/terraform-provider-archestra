package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/google/uuid"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

var _ resource.ResourceWithValidateConfig = &SkillResource{}
var _ resource.ResourceWithModifyPlan = &SkillResource{}

type skillGitSourceModel struct {
	RepoURL       types.String `tfsdk:"repo_url"`
	SkillPath     types.String `tfsdk:"skill_path"`
	Provider      types.String `tfsdk:"provider"`
	SyncInterval  types.String `tfsdk:"sync_interval"`
	GitHubAppID   types.String `tfsdk:"github_app_id"`
	GitHubPATID   types.String `tfsdk:"github_pat_id"`
	GitLabTokenID types.String `tfsdk:"gitlab_token_id"`
}

var skillGitSourceType = types.ObjectType{AttrTypes: map[string]attr.Type{
	"repo_url": types.StringType, "skill_path": types.StringType, "provider": types.StringType,
	"sync_interval": types.StringType, "github_app_id": types.StringType,
	"github_pat_id": types.StringType, "gitlab_token_id": types.StringType,
}}

var skillManifestFields = []string{"name", "description", "content", "license", "compatibility", "agent_name", "templated", "metadata", "allowed_tools", "files"}

func configureSkillGitSchema(attributes map[string]schema.Attribute) error {
	for _, name := range []string{"name", "description", "content", "license", "compatibility", "agent_name"} {
		value, ok := attributes[name].(schema.StringAttribute)
		if !ok {
			return fmt.Errorf("unexpected Skill schema attribute %s", name)
		}
		value.Required = false
		value.Optional = true
		value.Computed = true
		attributes[name] = value
	}
	templated, ok := attributes["templated"].(schema.BoolAttribute)
	if !ok {
		return fmt.Errorf("unexpected Skill schema attribute %s", "templated")
	}
	templated.Default = nil
	attributes["templated"] = templated
	metadata, ok := attributes["metadata"].(schema.MapAttribute)
	if !ok {
		return fmt.Errorf("unexpected Skill schema attribute %s", "metadata")
	}
	metadata.Computed = true
	attributes["metadata"] = metadata
	tools, ok := attributes["allowed_tools"].(schema.SetAttribute)
	if !ok {
		return fmt.Errorf("unexpected Skill schema attribute %s", "allowed_tools")
	}
	tools.Computed = true
	attributes["allowed_tools"] = tools
	files, ok := attributes["files"].(schema.MapNestedAttribute)
	if !ok {
		return fmt.Errorf("unexpected Skill schema attribute %s", "files")
	}
	files.Required = false
	files.Optional = true
	files.Computed = true
	attributes["files"] = files
	attributes["git_source"] = schema.SingleNestedAttribute{Optional: true,
		MarkdownDescription: "GitHub or GitLab source for one Skill. Configure this instead of manifest fields and files. Scheduled pulls own content; icon, environments, labels and permissions remain editable here. Changing between authored and Git content replaces the Skill.",
		PlanModifiers:       []planmodifier.Object{skillSourceKindPlanModifier{}}, Attributes: skillGitAttributes()}
	permissions := initialGrantsAttribute()
	permissions.MarkdownDescription = "Additional direct sharing grants. Authored Skills apply these atomically on creation. Git Skills are imported privately, then environment/settings updates and sharing are applied. Omit to preserve grants; [] clears additional grants while preserving the owner's access."
	attributes["permissions"] = permissions
	return nil
}

func skillGitAttributes() map[string]schema.Attribute {
	result := map[string]schema.Attribute{
		"repo_url":      schema.StringAttribute{Required: true, MarkdownDescription: "Repository path or URL, optionally with /tree/<ref>. Select one repository-root-relative folder using skill_path. Use an immutable commit in the URL when content must stay pinned.", Validators: []validator.String{stringvalidator.LengthBetween(1, 2048), skillGitURLValidator{}}},
		"skill_path":    schema.StringAttribute{Required: true, MarkdownDescription: "Safe relative directory containing SKILL.md, measured from the repository root; use an empty string for the root.", Validators: []validator.String{gitPluginSubdirValidator{}}},
		"provider":      schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("github"), Validators: []validator.String{stringvalidator.OneOf("github", "gitlab")}, MarkdownDescription: "Git provider, matching the selected stored credential."},
		"sync_interval": schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("1d"), Validators: []validator.String{stringvalidator.OneOf("15m", "1h", "1d")}, MarkdownDescription: "Scheduled pull interval. Git content stays read-only in Archestra while connected."},
	}
	for _, name := range []string{"github_app_id", "github_pat_id", "gitlab_token_id"} {
		others := []path.Expression{}
		for _, other := range []string{"github_app_id", "github_pat_id", "gitlab_token_id"} {
			if name != other {
				others = append(others, path.MatchRoot("git_source").AtName(other))
			}
		}
		result[name] = schema.StringAttribute{Optional: true, Validators: []validator.String{stringvalidator.RegexMatches(canonicalUUIDRegexp, "Use a lowercase canonical stored credential UUID."), stringvalidator.ConflictsWith(others...)}, MarkdownDescription: "Saved Git credential UUID; omit for a public repository. Only one credential may be selected. GitLab references use Credential.definition_id."}
	}
	return result
}

func (r *SkillResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	git := lookupOrNull(req.Config.Raw, "git_source")
	if !git.IsKnown() {
		return
	}
	if git.IsNull() {
		for _, name := range []string{"name", "description", "content", "files"} {
			value := lookupOrNull(req.Config.Raw, name)
			if value.IsKnown() && value.IsNull() {
				resp.Diagnostics.AddAttributeError(path.Root(name), "Missing authored Skill configuration", "Configure name, description, content and files together, or configure git_source instead.")
			}
		}
		return
	}
	for _, name := range skillManifestFields {
		value := lookupOrNull(req.Config.Raw, name)
		if value.IsKnown() && !value.IsNull() {
			resp.Diagnostics.AddAttributeError(path.Root(name), "Git Skill content is repository-owned", "Omit manifest fields and files when git_source is configured. Archestra reads these values from the repository.")
		}
	}
	var source skillGitSourceModel
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("git_source"), &source)...)
	if resp.Diagnostics.HasError() {
		return
	}
	provider := source.Provider.ValueString()
	if source.Provider.IsNull() {
		provider = "github"
	}
	if !source.Provider.IsUnknown() {
		hasGitlab := !source.GitLabTokenID.IsNull() && !source.GitLabTokenID.IsUnknown()
		hasGithub := (!source.GitHubAppID.IsNull() && !source.GitHubAppID.IsUnknown()) || (!source.GitHubPATID.IsNull() && !source.GitHubPATID.IsUnknown())
		if (hasGitlab && provider != "gitlab") || (hasGithub && provider != "github") {
			resp.Diagnostics.AddAttributeError(path.Root("git_source").AtName("provider"), "Git credential provider mismatch", "Set provider to match the selected GitHub or GitLab credential.")
		}
	}
}

// Authored content keeps explicit clearing/default behavior. The same fields
// become observed values when Git owns the manifest and files.
func modifySkillContentPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) bool {
	if req.Plan.Raw.IsNull() {
		return false
	}
	git := lookupOrNull(req.Config.Raw, "git_source")
	if !git.IsKnown() || !git.IsNull() {
		return false
	}
	changed := false
	defaults := map[string]attr.Value{"license": types.StringNull(), "compatibility": types.StringNull(), "agent_name": types.StringNull(), "templated": types.BoolValue(false), "metadata": types.MapNull(types.StringType), "allowed_tools": types.SetNull(types.StringType)}
	for name, value := range defaults {
		if lookupOrNull(req.Config.Raw, name).IsNull() {
			resp.Diagnostics.Append(resp.Plan.SetAttribute(ctx, path.Root(name), value)...)
			if !req.State.Raw.IsNull() && !lookupOrNull(resp.Plan.Raw, name).Equal(lookupOrNull(req.State.Raw, name)) {
				changed = true
			}
		}
	}
	if changed {
		for name, attribute := range resp.Plan.Schema.GetAttributes() {
			if name == "id" || !attribute.IsComputed() || attribute.IsOptional() {
				continue
			}
			kind := attribute.GetType()
			value, err := kind.ValueFromTerraform(ctx, tftypes.NewValue(kind.TerraformType(ctx), tftypes.UnknownValue))
			if err != nil {
				resp.Diagnostics.AddError("Invalid Skill output plan", err.Error())
				return changed
			}
			resp.Diagnostics.Append(resp.Plan.SetAttribute(ctx, path.Root(name), value)...)
		}
	}
	return changed
}

type skillSourceKindPlanModifier struct{}

func (skillSourceKindPlanModifier) Description(context.Context) string {
	return "Changing between authored and Git content replaces the Skill."
}
func (m skillSourceKindPlanModifier) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}
func (skillSourceKindPlanModifier) PlanModifyObject(_ context.Context, req planmodifier.ObjectRequest, resp *planmodifier.ObjectResponse) {
	if !req.State.Raw.IsNull() && !req.Plan.Raw.IsNull() && !req.PlanValue.IsUnknown() {
		resp.RequiresReplace = req.PlanValue.IsNull() != req.StateValue.IsNull()
	}
}

type skillGitURLValidator struct{}

func (skillGitURLValidator) Description(context.Context) string {
	return "Use a repository path or HTTP(S) Git URL without embedded credentials, query or fragment."
}
func (v skillGitURLValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}
func (skillGitURLValidator) ValidateString(_ context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	raw := req.ConfigValue.ValueString()
	parsed, err := url.Parse(raw)
	invalid := err != nil || strings.TrimSpace(raw) != raw
	if err == nil {
		invalid = invalid || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || (parsed.Scheme != "" && parsed.Scheme != "https" && parsed.Scheme != "http") || strings.Contains(raw, "@")
	}
	if invalid {
		resp.Diagnostics.AddAttributeError(req.Path, "Invalid Git repository URL", "Use a repository path or HTTP(S) URL without embedded credentials, query or fragment. Select authentication with a saved credential reference.")
	}
}

func skillGitSource(ctx context.Context, value types.Object, diags *diag.Diagnostics) skillGitSourceModel {
	var source skillGitSourceModel
	diags.Append(value.As(ctx, &source, basetypes.ObjectAsOptions{})...)
	return source
}
func (source skillGitSourceModel) request() map[string]any {
	result := map[string]any{"repoUrl": source.RepoURL.ValueString(), "skillPath": source.SkillPath.ValueString(), "provider": source.Provider.ValueString()}
	for name, value := range map[string]types.String{"githubAppConfigId": source.GitHubAppID, "githubPatId": source.GitHubPATID, "gitlabTokenId": source.GitLabTokenID} {
		if !value.IsNull() && !value.IsUnknown() {
			result[name] = value.ValueString()
		}
	}
	return result
}

func skillGitSettingsSpecs() []AttrSpec {
	result := []AttrSpec{}
	for _, spec := range skillAttrSpec {
		if spec.TFName == "icon" || spec.TFName == "environment_ids" || spec.TFName == "labels" {
			result = append(result, spec)
		}
	}
	return result
}

// Preserve metadata's JSON key order: the published API compares its serialized
// representation when enforcing Git-owned content. Files must never be echoed.
func skillGitSettingsBody(detail []byte, patch map[string]any, diags *diag.Diagnostics) []byte {
	var wire skillAPIWire
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(detail, &wire); err != nil {
		diags.AddError("Invalid Skill response", err.Error())
		return nil
	}
	if err := json.Unmarshal(detail, &fields); err != nil {
		diags.AddError("Invalid Skill response", err.Error())
		return nil
	}
	header := map[string]any{"name": wire.Name, "description": wire.Description, "templated": wire.Templated}
	for name, value := range map[string]*string{"license": wire.License, "compatibility": wire.Compatibility, "agent": wire.AgentName, "allowed-tools": wire.AllowedTools} {
		if value != nil {
			header[name] = *value
		}
	}
	if raw, ok := fields["metadata"]; ok && string(raw) != "null" {
		header["metadata"] = raw
	}
	encoded, err := json.Marshal(header)
	if err != nil {
		diags.AddError("Skill settings encoding failed", err.Error())
		return nil
	}
	delete(patch, "files")
	patch["content"] = "---\n" + string(encoded) + "\n---\n" + wire.Content
	patch["baseVersion"] = wire.LatestVersion
	result, err := json.Marshal(patch)
	if err != nil {
		diags.AddError("Skill settings encoding failed", err.Error())
	}
	return result
}

func (r *SkillResource) gitDetail(ctx context.Context, data *SkillResourceModel, diags *diag.Diagnostics) []byte {
	id, err := parseResourceUUID(data.ID, "Skill", diags)
	if err {
		return nil
	}
	result, e := r.client.GetSkillWithResponse(ctx, id)
	if e != nil {
		diags.AddError("Read Git Skill failed", e.Error())
		return nil
	}
	if result.JSON200 == nil {
		diags.AddError("Read Git Skill failed", fmt.Sprintf("API returned status %d. The imported identity remains in state for recovery.", result.StatusCode()))
		return nil
	}
	data.populate(ctx, result.Body, diags)
	return result.Body
}

func (r *SkillResource) gitSettings(ctx context.Context, data *SkillResourceModel, detail []byte, patch map[string]any, diags *diag.Diagnostics) {
	if len(patch) == 0 {
		return
	}
	body := skillGitSettingsBody(detail, patch, diags)
	if diags.HasError() {
		return
	}
	id, bad := parseResourceUUID(data.ID, "Skill", diags)
	if bad {
		return
	}
	result, err := r.client.UpdateSkillWithBodyWithResponse(ctx, id, "application/json", bytes.NewReader(body))
	if err != nil {
		diags.AddError("Git Skill settings update failed", err.Error())
		return
	}
	if result.JSON200 == nil {
		diags.AddError("Git Skill settings update failed", fmt.Sprintf("API returned status %d; no retry was attempted. Refresh and review a new plan.", result.StatusCode()))
		return
	}
	data.populate(ctx, result.Body, diags)
}

func (r *SkillResource) createGitSkill(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse, wanted SkillResourceModel) {
	source := skillGitSource(ctx, wanted.GitSource, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	patch := source.request()
	delete(patch, "skillPath")
	patch["skillPaths"] = []string{source.SkillPath.ValueString()}
	patch["sync"] = map[string]string{"interval": source.SyncInterval.ValueString()}
	// Keep a partially configured import private. Sharing is reconciled by
	// finishParentCreate only after environment and presentation settings succeed.
	encoded, err := json.Marshal(patch)
	if err != nil {
		resp.Diagnostics.AddError("Git Skill request encoding failed", err.Error())
		return
	}
	result, err := r.client.ImportGithubSkillsWithBodyWithResponse(ctx, "application/json", bytes.NewReader(encoded))
	if err != nil {
		resp.Diagnostics.AddError("Git Skill import failed", err.Error())
		return
	}
	if result.JSON200 == nil {
		resp.Diagnostics.AddError("Git Skill import failed", fmt.Sprintf("API returned status %d", result.StatusCode()))
		return
	}
	var imported struct {
		Created      []json.RawMessage `json:"created"`
		Skipped      []string          `json:"skipped"`
		SkippedFiles []struct {
			Files []string `json:"files"`
		} `json:"skippedFiles"`
	}
	if err := json.Unmarshal(result.Body, &imported); err != nil {
		resp.Diagnostics.AddError("Invalid Git Skill import response", err.Error())
		return
	}
	if len(imported.Created) != 1 {
		resp.Diagnostics.AddError("Git Skill was not created", fmt.Sprintf("Expected one newly created Skill, received %d created and %d skipped. Existing Skills are never adopted implicitly; import the intended UUID explicitly.", len(imported.Created), len(imported.Skipped)))
		return
	}
	data := wanted
	data.populate(ctx, imported.Created[0], &resp.Diagnostics)
	data.InitialGrants = types.SetNull(permissionGrantObjectType)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	for _, skipped := range imported.SkippedFiles {
		if len(skipped.Files) > 0 {
			resp.Diagnostics.AddError("Git Skill import omitted resource files", "The API omitted oversized, unfetchable or over-limit resource files. The created identity is retained in state for normal recovery and cleanup; review the repository and platform limits before retrying.")
			return
		}
	}
	detail := r.gitDetail(ctx, &data, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
	patch = MergePatch(ctx, req.Plan.Raw, resp.State.Raw, skillGitSettingsSpecs(), &resp.Diagnostics)
	if !resp.Diagnostics.HasError() {
		r.gitSettings(ctx, &data, detail, patch, &resp.Diagnostics)
	}
	if !resp.Diagnostics.HasError() {
		data.InitialGrants = wanted.InitialGrants
		resolveInitialGrants(&data.InitialGrants)
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *SkillResource) updateGitSkill(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse, wanted, prior SkillResourceModel) {
	source := skillGitSource(ctx, wanted.GitSource, &resp.Diagnostics)
	settings := MergePatch(ctx, req.Plan.Raw, req.State.Raw, skillGitSettingsSpecs(), &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	sourceChanged := !wanted.GitSource.Equal(prior.GitSource)
	if !sourceChanged && len(settings) == 0 {
		resp.State = req.State
		return
	}
	data := prior
	detail := r.gitDetail(ctx, &data, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	if !data.GitSource.Equal(prior.GitSource) {
		resp.Diagnostics.AddError("Git Skill source changed after planning", "Refresh and review a new plan before changing the source or settings. No update was attempted.")
		return
	}
	if sourceChanged {
		patch := map[string]any{"interval": source.SyncInterval.ValueString()}
		before := skillGitSource(ctx, prior.GitSource, &resp.Diagnostics)
		intervalOnly := before
		intervalOnly.SyncInterval = source.SyncInterval
		candidate, d := types.ObjectValueFrom(ctx, skillGitSourceType.AttrTypes, intervalOnly)
		resp.Diagnostics.Append(d...)
		if !candidate.Equal(wanted.GitSource) || data.GithubSyncInterval.IsNull() {
			patch["source"] = source.request()
		}
		if resp.Diagnostics.HasError() {
			return
		}
		encoded, err := json.Marshal(patch)
		if err != nil {
			resp.Diagnostics.AddError("Git Skill request encoding failed", err.Error())
			return
		}
		id, bad := parseResourceUUID(data.ID, "Skill", &resp.Diagnostics)
		if bad {
			return
		}
		result, err := r.client.UpdateSkillGithubSyncWithBodyWithResponse(ctx, id, "application/json", bytes.NewReader(encoded))
		if err != nil {
			resp.Diagnostics.AddError("Git Skill source update failed", err.Error())
			return
		}
		if result.JSON200 == nil {
			resp.Diagnostics.AddError("Git Skill source update failed", fmt.Sprintf("API returned status %d; no retry was attempted.", result.StatusCode()))
			return
		}
		data.GitSource = wanted.GitSource
		data.populate(ctx, result.Body, &resp.Diagnostics)
		detail = result.Body
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
		if resp.Diagnostics.HasError() {
			return
		}
	}
	r.gitSettings(ctx, &data, detail, settings, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (data *SkillResourceModel) populateGitSource(ctx context.Context, wire skillAPIWire, diags *diag.Diagnostics) {
	hadSource := !data.GitSource.IsNull() && !data.GitSource.IsUnknown()
	if wire.GithubSyncInterval == nil && !hadSource {
		data.GitSource = types.ObjectNull(skillGitSourceType.AttrTypes)
		return
	}
	if wire.SourceType == nil || *wire.SourceType != "github" || wire.SourceRef == nil {
		diags.AddError("Unsupported Git Skill source", "The API did not report a standalone Git-imported Skill. No content or source mutation was attempted.")
		return
	}
	repo, tail, ok := strings.Cut(*wire.SourceRef, "@")
	_, skillPath, hasPath := strings.Cut(tail, ":")
	if !ok || !hasPath || repo == "" {
		diags.AddError("Invalid Git Skill source", "The API source reference does not identify a repository and Skill path.")
		return
	}
	provider := "github"
	if wire.SourceProvider != nil {
		provider = *wire.SourceProvider
	}
	origin := "https://github.com"
	if provider == "gitlab" {
		origin = "https://gitlab.com"
	}
	if wire.SourceOrigin != nil {
		origin = strings.TrimRight(*wire.SourceOrigin, "/")
	}
	canonical := origin + "/" + repo
	if wire.GithubSyncRef != nil {
		if provider == "gitlab" {
			canonical += "/-"
		}
		canonical += "/tree/" + url.PathEscape(*wire.GithubSyncRef)
	}
	source := skillGitSourceModel{RepoURL: types.StringValue(canonical), SkillPath: types.StringValue(skillPath), Provider: types.StringValue(provider), SyncInterval: types.StringPointerValue(wire.GithubSyncInterval), GitHubAppID: types.StringPointerValue(wire.GithubAppConfigID), GitHubPATID: types.StringPointerValue(wire.GithubPatID), GitLabTokenID: types.StringPointerValue(wire.GitlabTokenID)}
	if hadSource {
		old := skillGitSource(ctx, data.GitSource, diags)
		if skillGitURLMatches(old.RepoURL.ValueString(), repo, wire.GithubSyncRef, provider, origin) {
			source.RepoURL = old.RepoURL
		}
	}
	var d diag.Diagnostics
	data.GitSource, d = types.ObjectValueFrom(ctx, skillGitSourceType.AttrTypes, source)
	diags.Append(d...)
}

func skillGitURLMatches(raw, repo string, ref *string, provider, origin string) bool {
	value := strings.Trim(raw, "/")
	base, err := url.Parse(origin)
	if err != nil {
		return false
	}
	first, _, _ := strings.Cut(value, "/")
	if strings.Contains(value, "://") || strings.ContainsAny(first, ".:") || strings.EqualFold(first, base.Host) {
		if !strings.Contains(value, "://") {
			value = base.Scheme + "://" + value
		}
		parsed, err := url.Parse(value)
		if err != nil || parsed.User != nil || !strings.EqualFold(parsed.Host, base.Host) {
			return false
		}
		value = strings.TrimPrefix(parsed.EscapedPath(), strings.TrimRight(base.EscapedPath(), "/")+"/")
	}
	marker := "/tree/"
	blob := "/blob/"
	if provider == "gitlab" {
		marker = "/-/tree/"
		blob = "/-/blob/"
	}
	before, after, hasRef := strings.Cut(value, marker)
	if !hasRef {
		before, after, hasRef = strings.Cut(value, blob)
	}
	before = strings.TrimSuffix(before, ".git")
	if before != repo {
		return false
	}
	if !hasRef {
		return ref == nil
	}
	desired, _, _ := strings.Cut(after, "/")
	decoded, err := url.PathUnescape(desired)
	return err == nil && ref != nil && decoded == *ref
}

// The Git operations use the same canonical UUID identity as the base resource.
func parseResourceUUID(value types.String, label string, diags *diag.Diagnostics) (id uuid.UUID, bad bool) {
	parsed, err := uuid.Parse(value.ValueString())
	if err != nil {
		diags.AddError("Invalid "+label+" ID", err.Error())
		return id, true
	}
	return parsed, false
}
