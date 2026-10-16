# One authoritative publication policy per gateway. Publishing a skill makes
# its content available to the gateway's callers.
resource "archestra_mcp_gateway_skill_publication" "procedures" {
  gateway_id        = archestra_mcp_gateway.operations.id
  access_all_skills = false
  skill_ids         = [archestra_skill.procedure.id]
}

# Auto mode retains explicit assignments; destroy disables Auto mode and clears
# those assignments while preserving the gateway and its independent exclusions.
