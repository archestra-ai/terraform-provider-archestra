resource "archestra_remote_agent" "external" {
  name    = "external-agent"
  enabled = false
  source = jsonencode({
    type = "inline_card"
    agentCard = {
      name        = "External agent"
      description = "Existing external A2A service."
      version     = "1.0.0"
      supportedInterfaces = [{
        url             = "https://a2a.example.invalid/agent"
        protocolBinding = "JSONRPC"
        protocolVersion = "1.0"
      }]
      capabilities         = { streaming = false }
      defaultInputModes    = ["text/plain"]
      defaultOutputModes   = ["text/plain"]
      skills               = []
      securitySchemes      = {}
      securityRequirements = []
    }
  })
  auth = jsonencode({ type = "none" })
}
