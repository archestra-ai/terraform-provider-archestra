package provider

import (
	"context"
	"strings"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

// teamVaultFolderAttrSpec covers the body wire field for the
// SetTeamVaultFolder endpoint. `team_id` is a URL path param (not
// body), so Synthetic keeps it out of MergePatch while still
// declaring the schema↔wire mapping for TestApiCoverage.
var teamVaultFolderAttrSpec = []AttrSpec{
	{TFName: "team_id", JSONName: "teamId", Kind: Synthetic},
	{TFName: "vault_path", JSONName: "vaultPath", Kind: Scalar},
}

func (r *TeamVaultFolderResource) AttrSpecs() []AttrSpec {
	return teamVaultFolderAttrSpec
}

func (r *TeamVaultFolderResource) APIShape() any {
	return client.GetTeamVaultFolderResponse{}
}

// No backend bookkeeping to skip — the Get response is just
// {id, teamId, vaultPath, createdAt, updatedAt}. All carried in
// the schema.
func (r *TeamVaultFolderResource) KnownIntentionallySkipped() []string {
	return nil
}

// vaultPathFormatValidator pre-empts the backend's 400 on paths
// containing `..`, starting with `/`, or ending with `/`. Mirrors
// the validation in `platform/backend/src/routes/team-vault-folder.ee.ts`.
type vaultPathFormatValidator struct{}

func (vaultPathFormatValidator) Description(_ context.Context) string {
	return "vault_path must not contain '..', start with '/', or end with '/'"
}

func (v vaultPathFormatValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (vaultPathFormatValidator) ValidateString(_ context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	p := req.ConfigValue.ValueString()
	switch {
	case strings.Contains(p, ".."):
		resp.Diagnostics.AddAttributeError(req.Path, "Invalid vault_path", "vault_path must not contain '..'")
	case strings.HasPrefix(p, "/"):
		resp.Diagnostics.AddAttributeError(req.Path, "Invalid vault_path", "vault_path must not start with '/'")
	case strings.HasSuffix(p, "/"):
		resp.Diagnostics.AddAttributeError(req.Path, "Invalid vault_path", "vault_path must not end with '/'")
	}
}
