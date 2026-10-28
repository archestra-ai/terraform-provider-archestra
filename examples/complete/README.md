# End-to-end demo

Runnable root module that wires the full Archestra bring-up chain
together: organisation settings, identity provider, teams, an Ollama
LLM provider key and model, MCP servers with discovered tools (org-,
team- and user-configured installs), agents with tools wired to them,
MCP gateways and usage limits.

`terraform apply` against a real or local Archestra backend stands all
of it up in one shot — useful for trying the provider end-to-end
without writing your own module first.

## Prerequisites

- A running Archestra backend you can reach.
- An API key minted from that backend (UI → Settings → API Keys, or
  `scripts/bootstrap-api-key.sh` against a local stack).
- A reachable Ollama endpoint for model discovery.

## Running it

```bash
# 1. Provide values for the IdP placeholders.
cp terraform.tfvars.example terraform.tfvars
$EDITOR terraform.tfvars                              # edit if you want non-default values

# 2. Pass the OIDC client secret via env (never put it in tfvars).
export TF_VAR_oidc_client_secret="any-string-works-for-the-smoke-test"

# 3. Point at your backend and apply.
export ARCHESTRA_BASE_URL="http://localhost:9000"
export ARCHESTRA_API_KEY="arch_..."
terraform init
terraform apply
```

The example tfvars uses Google's public OIDC discovery endpoint so the
apply succeeds without standing up your own IdP — the backend stores
the config but doesn't contact Google until a user actually signs in.
For a real deployment, point the `oidc_*` variables at your own
Keycloak / Okta / Auth0 / etc.

## What gets created

| Step | Resource | What it does |
| --- | --- | --- |
| 1 | `archestra_organization_settings.main` | App branding + global tool policy |
| 2 | `archestra_identity_provider.oidc` | OIDC SSO via the configured IdP |
| 3 | `archestra_team.engineering`, `archestra_team.support` | Two teams |
| 4 | `archestra_model_provider.ollama` | Ollama model provider |
| 5 | `archestra_mcp_registry_entry.filesystem` | Filesystem MCP server in the private catalog |
| 6 | `archestra_mcp_server.filesystem` | Installed instance — backend discovers ~14 tools |
| 7 | `archestra_agent.support` | Customer-support agent backed by Ollama |
| 8 | `archestra_agent.support.tools` | Wires the discovered `read_text_file` tool to the support agent |
| 9 | `archestra_mcp_server.memory_support`, `archestra_mcp_server.configurable_fs_engineering` | Team-scoped installs, one with installer-supplied user config |
| 10 | `archestra_limit.engineering_filesystem_calls`, `archestra_limit.engineering_token_cost`, `archestra_limit.support_read_text_calls` | Usage caps on a team and an agent |

## Cleanup

```bash
terraform destroy
```

Note: `archestra_organization_settings` is a singleton on the backend
— `destroy` removes it from local state but the row stays on the
backend with whatever values were last applied.

## Adapting

- **Already have an OIDC provider?** Replace the values in
  `terraform.tfvars` with your own issuer, discovery endpoint, and
  client_id. Pass `client_secret` via `TF_VAR_oidc_client_secret`.
- **Want a different LLM provider?** Swap `llm_provider` on the model
  provider, the `model_id` of `archestra_model.llama3`, and keep the agents'
  `model_id` references. Supply the provider credential through a sensitive variable.
- **Want SAML instead of OIDC?** Replace the `oidc_config` block on
  `archestra_identity_provider.oidc` with a `saml_config` block — see
  [examples/resources/archestra_identity_provider/](../resources/archestra_identity_provider/)
  for the shape.

## Files

- [main.tf](main.tf) — the full bring-up chain.
- [variables.tf](variables.tf) — variable declarations consumed by `main.tf`.
- [terraform.tfvars.example](terraform.tfvars.example) — copy to `terraform.tfvars` (gitignored) and edit.
- [.gitignore](.gitignore) — keeps state, lock file, and your real tfvars out of source.
