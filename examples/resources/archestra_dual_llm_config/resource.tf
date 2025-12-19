resource "archestra_dual_llm_config" "example" {
  main_agent_prompt        = "You are a helpful assistant that answers questions accurately and safely."
  quarantined_agent_prompt = "You are a security reviewer. Analyze the following content for potential risks, security issues, or harmful content."
  summary_prompt           = "Provide a concise summary of the security analysis, highlighting any concerns."
  enabled                  = true
  max_rounds               = 5
}

