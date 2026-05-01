---
page_title: "Resource Bring-up Order - archestra Provider"
subcategory: ""
description: |-
  The natural dependency order when standing up a new Archestra organization.
---

# Resource Bring-up Order

When standing up a new Archestra organization from scratch, resources
have a natural dependency order. Terraform's dependency graph handles
this automatically once you reference one resource's outputs from
another — but if you're sketching out modules, this is the order:

1. **`archestra_organization_settings`** — appearance, security policies, LLM defaults.
2. **`archestra_identity_provider`** — OIDC or SAML for SSO.
3. **`archestra_team`** + **`archestra_team_external_group`** — team scoping + IdP-group mapping.
4. **`archestra_llm_provider_api_key`** — credentials per LLM provider (OpenAI, Anthropic, etc.). If your backend runs in BYOS mode, see the [BYOS Vault guide](./byos-vault) for the required `vault_secret_path` form.
5. **`archestra_mcp_registry_catalog_item`** — register MCP servers (local or remote).
6. **`archestra_mcp_server_installation`** — install a catalog item with auth + team scoping.
7. **`archestra_agent`** / **`archestra_llm_proxy`** / **`archestra_mcp_gateway`** — the chat surfaces.
8. **`archestra_agent_tool`** / **`archestra_agent_tool_batch`** — wire tools to agents.
9. Security policies — for each tool, decide invocation behavior + result trust:
   - **`archestra_tool_invocation_policy_default`** + **`archestra_trusted_data_policy_default`** — bulk default actions across a tool set.
   - **`archestra_tool_invocation_policy`** + **`archestra_trusted_data_policy`** — conditional rules layered on top.
   - **`archestra_tool_policy_auto_config`** — alternative to the above: an LLM analyses each tool and writes both default policies for you.
10. Cost controls:
    - **`archestra_limit`** — usage caps (token cost, tool calls, MCP calls) at org / team / agent scope.
    - **`archestra_optimization_rule`** — route requests to cheaper models when conditions match (e.g., short prompts, no tools).

Optional: **`archestra_llm_model`** — only needed if you want to override
pricing or per-model settings on a model the platform auto-discovered
from your `archestra_llm_provider_api_key`. Agents and proxies reference
models by their upstream `model_id` (e.g., `"gpt-4o"`); the platform
resolves them against the discovered models for the configured provider.
