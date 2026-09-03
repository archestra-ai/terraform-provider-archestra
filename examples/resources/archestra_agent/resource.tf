# Externals (declare elsewhere): archestra_team.engineering, archestra_llm_provider_api_key.vault_backed, archestra_runtime_credential.github.

# NOTE: `llm_model` is the model_id you'd pass to the upstream LLM API
# (e.g. "gpt-4o" for OpenAI, "claude-sonnet-4-5" for Anthropic). The
# platform auto-discovers models from your `archestra_llm_provider_api_key`,
# so the corresponding key resource must exist for the chosen provider.
# Use `archestra_llm_model` only when you need to override pricing or
# per-model settings on a discovered model.

# Customer-support agent — the most common shape: a system prompt, an LLM,
# a couple of suggested prompts, and a few labels for filtering in the UI.
resource "archestra_agent" "support" {
  name          = "customer-support"
  description   = "Tier-1 customer support agent"
  icon          = "headphones"
  system_prompt = "You are a friendly customer-support agent. Be concise."
  llm_model     = "gpt-4o"
  scope         = "org"

  suggested_prompts = [
    {
      summary_title = "Refund a charge"
      prompt        = "Help me refund a customer charge."
    },
    {
      summary_title = "Look up an order"
      prompt        = "Find the most recent order for customer {{email}}."
    },
  ]

  labels = [
    { key = "team", value = "support" },
    { key = "tier", value = "1" },
  ]
}

# Team-scoped agent — visible only to members of the listed teams. The
# `archestra_team` reference enforces ordering so the team exists first.
resource "archestra_agent" "engineering_qa" {
  name          = "eng-qa"
  description   = "Engineering QA assistant"
  system_prompt = "You triage bug reports against the eng playbook."
  llm_model     = "claude-sonnet-4-5"
  scope         = "team"
  teams         = [archestra_team.engineering.id]

  # Wire a specific provider key (e.g. an Anthropic key from BYOS Vault) instead
  # of relying on the org-wide default.
  llm_api_key_id = archestra_llm_provider_api_key.vault_backed.id
}

# Email-triggered agent — receive new chats via incoming email. The agent
# accepts mail from anyone in `acme.com` thanks to `internal` security mode.
resource "archestra_agent" "intake" {
  name          = "support-intake"
  description   = "Routes incoming email into chats"
  system_prompt = "Triage incoming customer emails into the right team."
  llm_model     = "gpt-4o-mini"

  incoming_email_enabled        = true
  incoming_email_security_mode  = "internal"
  incoming_email_allowed_domain = "acme.com"

  consider_context_untrusted = true # Treat email content as untrusted by default.
}

# Agent Runtime worker — delegated work runs in its own container via the
# platform's runtime backend (e.g. the Kubernetes orchestrator).
# Auto-tool mode gives it every reachable tool; pair with
# `archestra_agent_tool_exclusions` to carve some out.
resource "archestra_agent" "worker" {
  name          = "repo-worker"
  description   = "Runs delegated repository work in a dedicated runtime"
  system_prompt = "You run delegated repository work."

  access_all_tools            = true
  missing_credential_behavior = "warn"

  runtime = {
    image              = "registry.example.com/repo-worker:latest"
    command            = ["run-worker"]
    inference_protocol = "anthropic"
    steer_mode         = "tmux_keys"
    privileged         = true

    resources = {
      cpu_request    = "2"
      memory_request = "4Gi"
      memory_limit   = "20Gi"
    }

    # Secrets never go in `environment` — declare them as credentials and
    # deposit values via `archestra_runtime_credential` (shared) or per user.
    credentials = [
      {
        key           = "GITHUB_TOKEN" # env var inside the run
        scope         = "per_user"
        credential_id = archestra_runtime_credential.github.key
        label         = "GitHub token"
        description   = "A personal token that can clone the repository and open pull requests."
        required      = true
      },
    ]

    max_cost_usd = 25
  }
}
