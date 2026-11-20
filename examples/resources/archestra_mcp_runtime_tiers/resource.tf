# The CPU and memory tiers that locally hosted MCP servers choose from with
# `local_config.runtime_tier`. This resource owns the whole list: tiers left
# out are removed, which fails while an MCP registry entry, even a deleted
# one, still names it. Renaming a used tier fails the same way.
#
# Memory left unset is derived from the CPU request. Destroying the resource
# leaves the tiers as they are.
resource "archestra_mcp_runtime_tiers" "this" {
  tiers = [
    { name = "S", cpu_request = 0.1 },
    { name = "M", cpu_request = 0.5, memory_request_mib = 512, memory_limit_mib = 1024 },
    { name = "L", cpu_request = 1, cpu_limit = 2, memory_request_mib = 1024, memory_limit_mib = 2048 },
  ]
}
