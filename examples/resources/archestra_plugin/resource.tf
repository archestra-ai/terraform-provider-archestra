resource "archestra_plugin" "example" {
  display_name        = "Operational helpers"
  client_type         = "claude-code"
  supported_platforms = ["posix"]
  enabled             = true
  files = {
    ".claude-plugin/plugin.json" = {
      content = jsonencode({ name = "operational-helpers", version = "1.0.0" })
    }
    "README.md" = {
      content = "Operational helper package."
    }
    "scripts/check.sh" = {
      content = "#!/bin/sh\nprintf 'ready\n'\n"
      mode    = "100755"
    }
  }
}

# Alternatively, declare a pinned Git source instead of files:
# git_source = {
#   repo       = "archestra-ai/terraform-provider-archestra"
#   commit_sha = "9e6a43f5347f6f3730ffeb75946f6e162791851d"
#   subdir     = "scripts/integration/fixtures/plugins/synthetic"
#   exclude    = []
# }
