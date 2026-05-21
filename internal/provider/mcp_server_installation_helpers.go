package provider

import (
	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
)

// APIShape opts archestra_mcp_server_installation into TestApiCoverage.
// Returning GetMcpServerResponse{} lets the drift test reflect over the
// JSON200 body to ensure every wire field is either surfaced as a schema
// attribute or named in KnownIntentionallySkipped.
func (r *MCPServerResource) APIShape() any {
	return client.GetMcpServerResponse{}
}

// KnownIntentionallySkipped — wire fields not modeled on this resource:
//   - createdAt/updatedAt: audit timestamps.
//   - catalogName: backend display sugar (catalog item is referenced by
//     catalog_id; users already control the name via the catalog item).
//   - ownerEmail/ownerId/userDetails/users/teamDetails: ownership and
//     membership metadata; surfacing them would mirror data already
//     reachable via archestra_organization_members / archestra_team.
//   - secretStorageType: BYOS/READONLY_VAULT bookkeeping signaling which
//     secret-manager backend stored the secret; users control storage
//     via is_byos_vault.
//   - serverType: catalog-item-derived discriminator (local/remote/etc);
//     fixed at install time by the chosen catalog item.
func (r *MCPServerResource) KnownIntentionallySkipped() []string {
	return []string{
		"createdAt", "updatedAt", "catalogName",
		"ownerEmail", "ownerId", "userDetails", "users", "teamDetails",
		"secretStorageType", "serverType",
	}
}
