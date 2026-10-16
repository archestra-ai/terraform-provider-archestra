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
