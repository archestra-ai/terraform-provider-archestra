# End-to-end demo

Runnable root module that wires the full Archestra bring-up chain
together: organisation settings, identity provider, teams, an Ollama
LLM provider key, an MCP server with discovered tools, an agent with a
tool wired to it, security policies, a usage limit, and an
optimisation rule.

`terraform apply` against a real or local Archestra backend stands all
of it up in one shot — useful for trying the provider end-to-end
without writing your own module first.

## Prerequisites

- A running Archestra backend you can reach.
- An API key minted from that backend (UI → Settings → API Keys, or
  `scripts/bootstrap-api-key.sh` against a local stack).
- For the LLM provider key resource: the backend must be in BYOS mode
  (`ARCHESTRA_SECRETS_MANAGER=READONLY_VAULT`) with a Vault entry at
  `secret/data/test/ollama` containing key `api_key`. See the
  [BYOS Vault guide](../../docs/guides/byos-vault.md)
  for setup. To run against a DB-mode
  backend instead, swap the `vault_secret_*` fields on
  `archestra_llm_provider_api_key.ollama_vault` for an inline `api_key`.

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
| 4 | `archestra_llm_provider_api_key.ollama_vault` | Ollama credentials read from Vault |
| 5 | `archestra_mcp_registry_catalog_item.filesystem` | Filesystem MCP server in the private catalog |
| 6 | `archestra_mcp_server_installation.filesystem` | Installed instance — backend discovers ~14 tools |
| 7 | `archestra_agent.support` | Customer-support agent backed by Ollama |
| 8 | `archestra_agent_tool.read_text_file` | Wires the discovered `read_text_file` tool to the support agent |
| 9 | `archestra_tool_invocation_policy_default.filesystem_blocked` + `archestra_trusted_data_policy_default.filesystem_sanitize` | Per-tool default policies |
| 10 | `archestra_limit.engineering_filesystem_calls`, `archestra_optimization_rule.support_short_prompts` | Usage cap + cheap-model routing |

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
- **DB-mode backend (no Vault)?** Replace
  `archestra_llm_provider_api_key.ollama_vault`'s `vault_secret_*`
  fields with an inline `api_key`.
- **Want a different LLM provider?** Swap `llm_provider`, `llm_model`,
  and the agent's `llm_model` consistently. The `vault_secret_*` path
  must point at a secret you've seeded.
- **Want SAML instead of OIDC?** Replace the `oidc_config` block on
  `archestra_identity_provider.oidc` with a `saml_config` block — see
  [examples/resources/archestra_identity_provider/](../resources/archestra_identity_provider/)
  for the shape.

## Files

- [main.tf](main.tf) — the full bring-up chain.
- [variables.tf](variables.tf) — variable declarations consumed by `main.tf`.
- [terraform.tfvars.example](terraform.tfvars.example) — copy to `terraform.tfvars` (gitignored) and edit.
- [.gitignore](.gitignore) — keeps state, lock file, and your real tfvars out of source.
