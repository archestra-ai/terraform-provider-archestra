resource "archestra_skill" "procedure" {
  name        = "repository-procedure"
  description = "Instructions for repository maintenance"
  content     = "Inspect the repository instructions before making changes."
  files = {
    "references/checklist.md" = {
      content  = "Run the relevant checks and report their results."
      encoding = "utf8"
    }
  }
  environment_ids = []
  labels          = []
}

# Files, environment restrictions and labels are authoritative collections.
# Updates use the last observed immutable version; a conflict requires a new plan.
# Destroy soft-deletes the skill instead of permanently purging its history.

# Use the same resource for repository-owned content. This pinned public fixture
# contains a harmless synthetic Skill; replace its URL/path with your source.
resource "archestra_skill" "git_procedure" {
  git_source = {
    repo_url      = "https://github.com/archestra-ai/terraform-provider-archestra/tree/411935fb1648ca812dff1f3503c0be77db65bcf3"
    skill_path    = "scripts/integration/fixtures/skills/synthetic"
    sync_interval = "1d"
  }
  environment_ids = []
  labels          = []
  permissions     = []
}

# Git owns name, description, instructions, manifest options and files. Omit those
# inputs in Git mode. Icon, labels, environment restrictions and sharing remain
# editable. Switching between authored and Git content replaces the Skill.
