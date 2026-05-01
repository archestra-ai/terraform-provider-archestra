---
page_title: "Common Issues - archestra Provider"
subcategory: ""
description: |-
  Verbatim error strings users hit, with cause and fix.
---

# Common Issues

Each section starts with the literal error string the provider or
backend emits. Search for the error you're seeing.

## `Either apiKey or both vaultSecretPath and vaultSecretKey must be provided`

**Where:** wire-level 400 from the backend, surfaces on Create/Update of
`archestra_llm_provider_api_key` or `archestra_mcp_server_installation`
(when `is_byos_vault = true`).

**Cause:** the backend rejects the *shape* of your config before
validating the provider — either you sent `api_key` to a backend running
in `READONLY_VAULT` (BYOS) mode, or `vault_secret_path` to a backend
running in `DB` mode.

**Fix:** check the backend's `ARCHESTRA_SECRETS_MANAGER` env var. Match
the form on your resource accordingly. See the
[BYOS Vault guide](./byos-vault) for the full shape table.

## `Precondition Not Met` on `archestra_team`

**Search-key:** `convert_tool_results_to_toon = true on a team requires archestra_organization_settings.compression_scope = "team"`

**Where:** plan-time error on `archestra_team` when setting
`convert_tool_results_to_toon = true`.

**Cause:** provider pre-flight catches the backend behavior where
team-level TOON is silently dropped unless org `compression_scope` is
`"team"`. The check exists *before* apply because the backend's silent
ignore previously surfaced as Terraform's "Provider produced inconsistent
result after apply" mid-apply, leaving partial state.

**Fix:** apply the org-level scope first, in a separate apply or via
`-target`:

```hcl
resource "archestra_organization_settings" "this" {
  compression_scope = "team"
}
```

…then re-plan the team change.

## `Provider produced inconsistent result after apply`

**Where:** Terraform-framework error, raised when the resource's Read
returns a value different from what the resource's Create/Update planned.

**Cause:** in this provider, almost always means the backend silently
ignored a write the provider expected to take effect. The historic
trigger was team-level `convert_tool_results_to_toon` without org
`compression_scope = "team"`; current versions catch that at plan time
via pre-flight, so seeing this error today indicates a *new* silent-write
case the pre-flight doesn't cover.

**Fix:** the error message names the offending field. Read that field's
schema description for any gating rule on another resource's setting.
If you can't find one, file a bug — silent-write cases are pre-flight
gaps that should be caught at plan time.

## `Optimization rule not found in list response`

**Where:** warning (not error) on Read of `archestra_optimization_rule`.

**Cause:** the backend's list endpoint doesn't return a rule the provider
expects from state. Most often the rule was deleted out-of-band (UI,
API, another Terraform workspace).

**Fix:** `terraform refresh` to drop it from state, or re-apply to
recreate. Check for concurrent edits if this recurs.

## `Tool '<name>' not found for agent <id>` / `Invalid Tool ID`

**Where:** error from `data.archestra_agent_tool` or 400 from the policy
endpoints.

**Cause:** `tool_id` confusion. Two distinct UUIDs exist:
- The **bare tool UUID** (from the `tools` table) — what
  `archestra_tool_invocation_policy.tool_id` /
  `archestra_trusted_data_policy.tool_id` expect.
- The **agent-tool assignment composite UUID** (from the join table) —
  what `data.archestra_agent_tool.id` exposes.

**Fix:** for policy resources, prefer the one-line lookup:

```hcl
tool_id = archestra_mcp_server_installation.fs.tool_id_by_name["filesystem__read_text_file"]
```

If you need a data source, use `data.archestra_mcp_server_tool` (returns
the bare UUID via `id`), not `data.archestra_agent_tool` (returns the
composite UUID via `id` and the bare UUID via `tool_id`).

## Changing `archestra_agent` to `archestra_llm_proxy` (or `archestra_mcp_gateway`) destroys and recreates

**Where:** plan diff shows the resource going away and a new one being
added when you swap resource types in HCL.

**Cause:** the backend models all three on a single `agents` table with an
`agentType` discriminator, but the provider exposes them as three
separate resource types for 1:1 parity with the UI. Changing the HCL
resource type means deleting one resource and creating another in
Terraform's view.

**Caveat:** the new resource gets a fresh UUID. Anything that referenced
the old agent's id —
`archestra_tool_invocation_policy.tool_id` /
`archestra_trusted_data_policy.tool_id`,
`archestra_agent_tool` / `archestra_agent_tool_batch`,
`archestra_limit` scoped to it — also needs updating, otherwise those
resources still point at a deleted UUID.

**Fix (preserve identity):** `terraform state mv archestra_agent.X
archestra_llm_proxy.X` *and* flip the backend record's `agentType`
first (UI or API). Without the backend flip, the next Read will diff
and Terraform will plan a recreate again.
