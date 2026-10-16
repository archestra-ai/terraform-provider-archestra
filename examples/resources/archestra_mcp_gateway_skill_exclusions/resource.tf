# Auto publication excludes these skills. The exclusion set persists while Auto
# mode is disabled; destroy clears it while preserving the publication policy.
resource "archestra_mcp_gateway_skill_exclusions" "restricted" {
  gateway_id         = archestra_mcp_gateway.operations.id
  excluded_skill_ids = [archestra_skill.restricted.id]
}
