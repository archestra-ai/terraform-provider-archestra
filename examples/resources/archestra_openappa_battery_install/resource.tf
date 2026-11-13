# Requires Archestra 1.5.0-rc.1 or later with OpenAPPA guardrails enabled.
# Installs edit the organization's policy text, so they fail while a GitHub
# repository owns that text.
resource "archestra_mcp_registry_entry" "github" {
  name = "github"
  remote_config = {
    url = "https://api.githubcopilot.com/mcp/"
  }
}

# A battery that governs MCP servers attaches to one registry entry. The
# entry needs synced tools first: install the server before the battery.
resource "archestra_openappa_battery_install" "github" {
  battery_name = "github"
  catalog_id   = archestra_mcp_registry_entry.github.id
}

# Detach a battery from an entry but keep it included with enabled = false.
# A battery made of annotators alone governs the whole organization: install
# it without catalog_id. Pin an uploaded battery package with package_hash.
