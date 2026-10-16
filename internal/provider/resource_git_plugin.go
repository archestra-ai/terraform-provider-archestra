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
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func NewGitPluginResource() resource.Resource { return &PluginResource{gitSource: true} }

func gitPluginAttrSpecs() []AttrSpec {
	specs := make([]AttrSpec, 0, len(pluginAttrSpec)+9)
	for _, spec := range pluginAttrSpec {
		if spec.TFName != "files" {
			specs = append(specs, spec)
		}
	}
	return append(specs,
		AttrSpec{TFName: "source_repo", JSONName: "repoUrl", Kind: Scalar},
		AttrSpec{TFName: "source_sha", JSONName: "approvedCommitSha", Kind: Scalar},
		AttrSpec{TFName: "source_subdir", JSONName: "subdir", Kind: Scalar},
		AttrSpec{TFName: "source_exclude", JSONName: "exclude", Kind: List},
		AttrSpec{TFName: "source_provider", JSONName: "provider", Kind: Scalar},
		AttrSpec{TFName: "github_sync_ref", JSONName: "trackingRef", Kind: Scalar},
		AttrSpec{TFName: "github_sync_interval", JSONName: "syncInterval", Kind: Scalar},
		AttrSpec{TFName: "github_app_config_id", JSONName: "githubAppConfigId", Kind: Scalar},
		AttrSpec{TFName: "github_pat_id", JSONName: "githubPatId", Kind: Scalar},
		AttrSpec{TFName: "gitlab_token_id", JSONName: "gitlabTokenId", Kind: Scalar},
	)
}

func gitPluginSchema(s *schema.Schema) {
	s.MarkdownDescription = "Git-sourced plugin package pinned to an explicitly approved immutable commit, with repository tracking and synchronization configuration. Changing approved bytes replaces the package. Periodic checks may stage a candidate; they do not approve new executable content."
	a := s.Attributes
	files := a["files"].(schema.MapNestedAttribute) //nolint:forcetypeassert // The shared schema constructs this map immediately before adaptation.
	files.Required = false
	files.Computed = true
	files.Validators = nil
	for name, attr := range files.NestedObject.Attributes {
		v := attr.(schema.StringAttribute) //nolint:forcetypeassert // Every file member is constructed as a string by the shared schema.
		v.Required = false
		v.Optional = false
		v.Computed = true
		v.Default = nil
		v.Validators = nil
		files.NestedObject.Attributes[name] = v
	}
	files.MarkdownDescription = "Observed files from the approved repository commit."
	a["files"] = files
	replace := []planmodifier.String{stringplanmodifier.RequiresReplace()}
	a["source_repo"] = schema.StringAttribute{Required: true, PlanModifiers: replace, Validators: []validator.String{stringvalidator.RegexMatches(regexp.MustCompile(`^[A-Za-z0-9_.-]+(?:/[A-Za-z0-9_.-]+)+$`), "Use the canonical owner/repository path without URL, ref, or trailing slash."), stringvalidator.LengthAtMost(2048), gitPluginRepositoryValidator{}}, MarkdownDescription: "Canonical repository path on the selected Git host."}
	a["source_sha"] = schema.StringAttribute{Required: true, PlanModifiers: replace, Validators: []validator.String{stringvalidator.RegexMatches(regexp.MustCompile(`^[a-f0-9]{40}$`), "Use the approved lowercase full 40-character commit SHA.")}, MarkdownDescription: "Explicitly approved immutable commit; changing it replaces the package."}
	a["source_subdir"] = schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""), PlanModifiers: replace, Validators: []validator.String{gitPluginSubdirValidator{}}, MarkdownDescription: "Canonical safe relative subtree, or empty for the repository root."}
	a["source_exclude"] = schema.ListAttribute{Required: true, ElementType: types.StringType, Validators: []validator.List{listvalidator.SizeAtMost(50), listvalidator.ValueStringsAre(stringvalidator.RegexMatches(regexp.MustCompile(`^\S(?:.*\S)?$`), "Exclusions must be nonempty without surrounding whitespace."))}, PlanModifiers: []planmodifier.List{listplanmodifier.RequiresReplace()}, MarkdownDescription: "Ordered exclusion patterns applied to approved source bytes; changes replace the package."}
	a["source_provider"] = schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("github"), PlanModifiers: replace, Validators: []validator.String{stringvalidator.OneOf("github", "gitlab")}, MarkdownDescription: "Source Git provider, matching any selected stored credential."}
	a["github_sync_ref"] = schema.StringAttribute{Optional: true, Validators: []validator.String{stringvalidator.LengthBetween(1, 1024), stringvalidator.RegexMatches(regexp.MustCompile(`^\S(?:.*\S)?$`), "Use a ref without surrounding whitespace.")}, MarkdownDescription: "Branch or tag tracked for candidate updates; omission tracks the default branch."}
	a["github_sync_interval"] = schema.StringAttribute{Optional: true, Validators: []validator.String{stringvalidator.OneOf("15m", "1h", "1d")}, MarkdownDescription: "Periodic candidate-check interval; omission disables scheduled checks."}
	for _, name := range []string{"github_app_config_id", "github_pat_id", "gitlab_token_id"} {
		others := []path.Expression{}
		for _, other := range []string{"github_app_config_id", "github_pat_id", "gitlab_token_id"} {
			if other != name {
				others = append(others, path.MatchRoot(other))
			}
		}
		a[name] = schema.StringAttribute{Optional: true, Validators: []validator.String{stringvalidator.RegexMatches(canonicalUUIDRegexp, "Use a lowercase canonical stored credential UUID."), stringvalidator.ConflictsWith(others...)}, MarkdownDescription: "Existing stored Git credential reference; omission clears this authentication method."}
	}
}

func (r *PluginResource) createGit(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data PluginResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	patch := MergePatch(ctx, req.Plan.Raw, tftypes.NewValue(req.Plan.Raw.Type(), nil), r.AttrSpecs(), &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	for _, key := range []string{"enabled", "icon", "labels", "syncInterval"} {
		delete(patch, key)
	}
	for _, key := range []string{"githubAppConfigId", "githubPatId", "gitlabTokenId"} {
		if patch[key] == nil {
			delete(patch, key)
		}
	}
	LogPatch(ctx, "archestra_git_plugin Create", patch, r.AttrSpecs())
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
	follow := map[string]any{"githubSource": gitPluginSource(wanted)}
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
	patch := MergePatch(ctx, req.Plan.Raw, req.State.Raw, r.AttrSpecs(), &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	sourceChanged := false
	for _, key := range []string{"trackingRef", "syncInterval", "githubAppConfigId", "githubPatId", "gitlabTokenId"} {
		if _, ok := patch[key]; ok {
			sourceChanged = true
		}
		delete(patch, key)
	}
	if sourceChanged {
		patch["githubSource"] = gitPluginSource(data)
	}
	for _, key := range []string{"initialGrants", "clientType", "repoUrl", "approvedCommitSha", "subdir", "exclude", "provider"} {
		delete(patch, key)
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
func gitPluginSource(data PluginResourceModel) map[string]any {
	return map[string]any{"repoUrl": data.SourceRepo.ValueString(), "ref": gitPluginNullableString(data.GithubSyncRef), "syncInterval": gitPluginNullableString(data.GithubSyncInterval), "authentication": map[string]any{"githubAppConfigId": gitPluginNullableString(data.GithubAppConfigID), "githubPatId": gitPluginNullableString(data.GithubPatID), "gitlabTokenId": gitPluginNullableString(data.GitlabTokenID)}}
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
