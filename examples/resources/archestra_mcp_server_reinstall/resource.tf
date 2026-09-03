# One-shot resource that re-deploys the K8s pod backing an MCP server
# installation without recreating the install row. Run this when the
# parent installation's `reinstall_required` flips to true (e.g. after
# a catalog-item template change).

resource "archestra_mcp_server_reinstall" "refresh" {
  mcp_server_id = archestra_mcp_server_installation.api.id
  trigger       = "image-pull-2026-05-19"
}
