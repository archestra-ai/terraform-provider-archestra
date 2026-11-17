# One person's connected Slack account: an installation made in the app.
# People's own installations of one registry entry share a name, so the owner
# selects it.
data "archestra_mcp_server" "slack" {
  catalog_name = "Slack"
  owner_email  = "release-manager@example.com"
}

data "archestra_tool" "slack_send_message" {
  name = "slack__slack_send_message"
}

# Give an agent that one tool, always run through that installation.
resource "archestra_agent" "announcer" {
  name          = "Release announcer"
  system_prompt = "Post a short note about each release."

  tools = [{
    tool_id                    = data.archestra_tool.slack_send_message.id
    mcp_server_id              = data.archestra_mcp_server.slack.id
    credential_resolution_mode = "static"
  }]
}
