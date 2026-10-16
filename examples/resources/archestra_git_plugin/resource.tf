# Review the exact commit before declaring the approved source bytes.
# A new source SHA or file selection replaces this package; periodic checks
# only stage candidates and never approve new executable content.
resource "archestra_git_plugin" "example" {
  display_name        = "Pinned repository package"
  client_type         = "claude-code"
  supported_platforms = ["posix"]
  source_repo         = "archestra-ai/terraform-provider-archestra"
  source_sha          = "9e6a43f5347f6f3730ffeb75946f6e162791851d"
  source_subdir       = "scripts/integration/fixtures/plugins/synthetic"
  source_exclude      = ["excluded.txt"]
  # Omit github_sync_interval to disable periodic candidate checks.
  # An existing stored credential can be selected with github_pat_id,
  # github_app_config_id or gitlab_token_id; the latter uses source_provider="gitlab".
  enabled = false
}
