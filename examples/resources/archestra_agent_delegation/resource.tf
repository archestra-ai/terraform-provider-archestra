# A general-purpose channel agent...
resource "archestra_agent" "dispatcher" {
  name          = "dispatcher"
  system_prompt = "You route incoming requests. Hand deployment tasks to the deployer subagent."
}

# ...and a specialist it can hand tasks to.
resource "archestra_agent" "deployer" {
  name          = "deployer"
  system_prompt = "You execute deployment tasks handed to you by other agents."
}

# One resource per delegation edge: the dispatcher surfaces the deployer as a
# subagent delegation tool it can invoke mid-conversation. An agent's whole
# delegation surface is just several of these edges.
resource "archestra_agent_delegation" "dispatcher_to_deployer" {
  agent_id        = archestra_agent.dispatcher.id
  target_agent_id = archestra_agent.deployer.id
}
