# One-shot resource that re-runs credential resolution against an
# installed MCP server. Use during PAT/OAuth rotations or when the
# parent installation's `oauth_refresh_error` flips non-null.
#
# Bumping `trigger` is the supported way to force a re-run; all other
# attributes are RequiresReplace so changing them re-runs too.

variable "github_pat" {
  type      = string
  sensitive = true
}

resource "archestra_mcp_server_reauthenticate" "github_rotation" {
  mcp_server_id = archestra_mcp_server_installation.github.id
  access_token  = var.github_pat
  trigger       = "2026-05-19-rotation"
}
