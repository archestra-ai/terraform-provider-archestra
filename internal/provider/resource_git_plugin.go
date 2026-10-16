package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"regexp"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func gitPluginAttributes() map[string]schema.Attribute {
	a := map[string]schema.Attribute{}
	replace := []planmodifier.String{stringplanmodifier.RequiresReplace()}
	a["repo"] = schema.StringAttribute{Required: true, PlanModifiers: replace, Validators: []validator.String{stringvalidator.RegexMatches(regexp.MustCompile(`^[A-Za-z0-9_.-]+(?:/[A-Za-z0-9_.-]+)+$`), "Use the canonical owner/repository path without URL, ref, or trailing slash."), stringvalidator.LengthAtMost(2048), gitPluginRepositoryValidator{}}, MarkdownDescription: "Canonical repository path on the selected Git host."}
	a["commit_sha"] = schema.StringAttribute{Required: true, PlanModifiers: replace, Validators: []validator.String{stringvalidator.RegexMatches(regexp.MustCompile(`^[a-f0-9]{40}$`), "Use the approved lowercase full 40-character commit SHA.")}, MarkdownDescription: "Explicitly approved immutable commit; changing it replaces the package."}
	a["subdir"] = schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""), PlanModifiers: replace, Validators: []validator.String{gitPluginSubdirValidator{}}, MarkdownDescription: "Canonical safe relative subtree, or empty for the repository root."}
	a["exclude"] = schema.ListAttribute{Required: true, ElementType: types.StringType, Validators: []validator.List{listvalidator.SizeAtMost(50), listvalidator.ValueStringsAre(stringvalidator.RegexMatches(regexp.MustCompile(`^\S(?:.*\S)?$`), "Exclusions must be nonempty without surrounding whitespace."))}, PlanModifiers: []planmodifier.List{listplanmodifier.RequiresReplace()}, MarkdownDescription: "Ordered exclusion patterns applied to approved source bytes; changes replace the package."}
	a["provider"] = schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("github"), PlanModifiers: replace, Validators: []validator.String{stringvalidator.OneOf("github", "gitlab")}, MarkdownDescription: "Source Git provider, matching any selected stored credential."}
	a["tracking_ref"] = schema.StringAttribute{Optional: true, Validators: []validator.String{stringvalidator.LengthBetween(1, 1024), stringvalidator.RegexMatches(regexp.MustCompile(`^\S(?:.*\S)?$`), "Use a ref without surrounding whitespace.")}, MarkdownDescription: "Branch or tag tracked for candidate updates; omission tracks the default branch."}
	a["sync_interval"] = schema.StringAttribute{Optional: true, Validators: []validator.String{stringvalidator.OneOf("15m", "1h", "1d")}, MarkdownDescription: "Periodic candidate-check interval; omission disables scheduled checks."}
	for _, name := range []string{"github_app_id", "github_pat_id", "gitlab_token_id"} {
		others := []path.Expression{}
		for _, other := range []string{"github_app_id", "github_pat_id", "gitlab_token_id"} {
			if other != name {
				others = append(others, path.MatchRoot("git_source").AtName(other))
			}
		}
		a[name] = schema.StringAttribute{Optional: true, Validators: []validator.String{stringvalidator.RegexMatches(canonicalUUIDRegexp, "Use a lowercase canonical stored credential UUID."), stringvalidator.ConflictsWith(others...)}, MarkdownDescription: "Existing stored Git credential reference; omission clears this authentication method."}
	}
	return a
}

func (r *PluginResource) createGit(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data PluginResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	patch := MergePatch(ctx, req.Plan.Raw, tftypes.NewValue(req.Plan.Raw.Type(), nil), pluginAttrSpec, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	source, ok := patch["githubSource"].(map[string]any)
	if !ok {
		resp.Diagnostics.AddError("Invalid Git source", "A complete git_source is required to import a plugin.")
		return
	}
	delete(patch, "githubSource")
	delete(patch, "files")
	for key, value := range source {
		patch[key] = value
	}
	for _, key := range []string{"enabled", "icon", "labels", "syncInterval"} {
		delete(patch, key)
	}
	for _, key := range []string{"githubAppConfigId", "githubPatId", "gitlabTokenId"} {
		if patch[key] == nil {
			delete(patch, key)
		}
	}
	LogPatch(ctx, "archestra_plugin Create (Git source)", patch, pluginAttrSpec)
	body, err := json.Marshal(patch)
	if err != nil {
		resp.Diagnostics.AddError("Request Encoding Failed", err.Error())
		return
	}
	result, err := r.client.ImportGithubPluginWithBodyWithResponse(ctx, "application/json", bytes.NewReader(body))
	if err != nil {
		resp.Diagnostics.AddError("API Error", err.Error())
		return
	}
	if result.JSON200 == nil {
		resp.Diagnostics.AddError("Creation Failed", fmt.Sprintf("API returned status %d", result.StatusCode()))
		return
	}
	wanted := data
	r.flatten(ctx, &data, result.Body, &resp.Diagnostics)
	resolveInitialGrants(&data.InitialGrants)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	// Persist the identity before applying controls absent from the import contract.
	follow := map[string]any{"githubSource": gitPluginSource(ctx, wanted, &resp.Diagnostics)}
	if !wanted.Enabled.IsNull() && !wanted.Enabled.IsUnknown() {
		follow["enabled"] = wanted.Enabled.ValueBool()
	}
	if !wanted.Icon.IsNull() && !wanted.Icon.IsUnknown() {
		follow["icon"] = wanted.Icon.ValueString()
	}
	if !wanted.Labels.IsNull() && !wanted.Labels.IsUnknown() {
		follow["labels"] = encodeGitPluginLabels(ctx, wanted.Labels, &resp.Diagnostics)
	}
	if resp.Diagnostics.HasError() {
		return
	}
	if !r.update(ctx, &data, follow, &resp.Diagnostics) {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *PluginResource) updateGit(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data PluginResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	patch := MergePatch(ctx, req.Plan.Raw, req.State.Raw, pluginAttrSpec, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	if _, changed := patch["githubSource"]; changed {
		patch["githubSource"] = gitPluginSource(ctx, data, &resp.Diagnostics)
	}
	delete(patch, "initialGrants")
	delete(patch, "clientType")
	if resp.Diagnostics.HasError() {
		return
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

type pluginGitSourceModel struct {
	Repo          types.String `tfsdk:"repo"`
	CommitSHA     types.String `tfsdk:"commit_sha"`
	Subdir        types.String `tfsdk:"subdir"`
	Exclude       types.List   `tfsdk:"exclude"`
	Provider      types.String `tfsdk:"provider"`
	TrackingRef   types.String `tfsdk:"tracking_ref"`
	SyncInterval  types.String `tfsdk:"sync_interval"`
	GitHubAppID   types.String `tfsdk:"github_app_id"`
	GitHubPATID   types.String `tfsdk:"github_pat_id"`
	GitLabTokenID types.String `tfsdk:"gitlab_token_id"`
}

var pluginGitSourceType = types.ObjectType{AttrTypes: map[string]attr.Type{
	"repo": types.StringType, "commit_sha": types.StringType, "subdir": types.StringType,
	"exclude": types.ListType{ElemType: types.StringType}, "provider": types.StringType,
	"tracking_ref": types.StringType, "sync_interval": types.StringType,
	"github_app_id": types.StringType, "github_pat_id": types.StringType, "gitlab_token_id": types.StringType,
}}
var pluginGitSourceSpecs = []AttrSpec{
	{TFName: "repo", ReadJSONName: "sourceRepo", JSONName: "repoUrl", Kind: Scalar},
	{TFName: "commit_sha", ReadJSONName: "sourceSha", JSONName: "approvedCommitSha", Kind: Scalar},
	{TFName: "subdir", ReadJSONName: "sourceSubdir", JSONName: "subdir", Kind: Scalar},
	{TFName: "exclude", ReadJSONName: "sourceExclude", JSONName: "exclude", Kind: List},
	{TFName: "provider", ReadJSONName: "sourceProvider", JSONName: "provider", Kind: Scalar},
	{TFName: "tracking_ref", ReadJSONName: "githubSyncRef", JSONName: "trackingRef", Kind: Scalar},
	{TFName: "sync_interval", ReadJSONName: "githubSyncInterval", JSONName: "syncInterval", Kind: Scalar},
	{TFName: "github_app_id", JSONName: "githubAppConfigId", Kind: Scalar},
	{TFName: "github_pat_id", JSONName: "githubPatId", Kind: Scalar},
	{TFName: "gitlab_token_id", JSONName: "gitlabTokenId", Kind: Scalar},
}

func gitPluginSource(ctx context.Context, data PluginResourceModel, diags *diag.Diagnostics) map[string]any {
	var source pluginGitSourceModel
	diags.Append(data.GitSource.As(ctx, &source, basetypes.ObjectAsOptions{})...)
	return map[string]any{"repoUrl": source.Repo.ValueString(), "ref": gitPluginNullableString(source.TrackingRef), "syncInterval": gitPluginNullableString(source.SyncInterval), "authentication": map[string]any{"githubAppConfigId": gitPluginNullableString(source.GitHubAppID), "githubPatId": gitPluginNullableString(source.GitHubPATID), "gitlabTokenId": gitPluginNullableString(source.GitLabTokenID)}}
}

type pluginSourceKindPlanModifier struct{}

func (pluginSourceKindPlanModifier) Description(context.Context) string {
	return "Changing between authored files and a Git source replaces the plugin."
}
func (m pluginSourceKindPlanModifier) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}
func (pluginSourceKindPlanModifier) PlanModifyObject(_ context.Context, req planmodifier.ObjectRequest, resp *planmodifier.ObjectResponse) {
	if req.State.Raw.IsNull() || req.Plan.Raw.IsNull() || req.PlanValue.IsUnknown() {
		return
	}
	resp.RequiresReplace = req.PlanValue.IsNull() != req.StateValue.IsNull()
}

func gitPluginNullableString(value types.String) any {
	if value.IsNull() || value.IsUnknown() {
		return nil
	}
	return value.ValueString()
}
func encodeGitPluginLabels(ctx context.Context, value types.Set, diags *diag.Diagnostics) any {
	var labels []AgentLabelModel
	diags.Append(value.ElementsAs(ctx, &labels, false)...)
	result := make([]map[string]string, 0, len(labels))
	for _, label := range labels {
		result = append(result, map[string]string{"key": label.Key.ValueString(), "value": label.Value.ValueString()})
	}
	return result
}

type gitPluginSubdirValidator struct{}

func (gitPluginSubdirValidator) Description(context.Context) string {
	return "Use an empty string or a safe relative directory without trailing slash."
}
func (v gitPluginSubdirValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}
func (gitPluginSubdirValidator) ValidateString(_ context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	value := req.ConfigValue.ValueString()
	if value == "" {
		return
	}
	m, _ := types.MapValueFrom(context.Background(), types.StringType, map[string]string{value: ""})
	v := validator.MapResponse{}
	pluginPathsValidator{}.ValidateMap(context.Background(), validator.MapRequest{ConfigValue: m, Path: req.Path}, &v)
	if v.Diagnostics.HasError() || value != strings.TrimSpace(value) {
		resp.Diagnostics.AddAttributeError(req.Path, "Invalid source subtree", "Use an empty string or a canonical safe relative directory.")
	}
}

type gitPluginRepositoryValidator struct{}

func (gitPluginRepositoryValidator) Description(context.Context) string {
	return "Use the canonical repository path without .git suffix or traversal segments."
}
func (v gitPluginRepositoryValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}
func (gitPluginRepositoryValidator) ValidateString(_ context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	value := req.ConfigValue.ValueString()
	bad := strings.HasSuffix(value, ".git")
	for _, part := range strings.Split(value, "/") {
		bad = bad || part == "." || part == ".."
	}
	if bad {
		resp.Diagnostics.AddAttributeError(req.Path, "Noncanonical repository path", "Use the canonical repository path without .git suffix or traversal segments.")
	}
}
