# Changelog

## Unreleased

### ⚠ BREAKING CHANGES

* **`archestra_profile` replaced by three type-specific resources.** Backend collapses `agent`, `llm_proxy`, and `mcp_gateway` onto a single `agents` table with an `agentType` discriminator; the previous single `archestra_profile` mixed every variant's fields and required mode-aware validation in HCL. No deprecation alias.

  Migration: split each `resource "archestra_profile" "..."` block into one of `archestra_agent`, `archestra_llm_proxy`, or `archestra_mcp_gateway` (matching the original `agent_type`), then run `terraform state mv archestra_profile.<n> archestra_<new_type>.<n>` for each. The agent-tool resource and data source were renamed in the same pass: `archestra_profile_tool` → `archestra_agent_tool`, `data.archestra_profile_tool` → `data.archestra_agent_tool`, with `profile_id` → `agent_id`. The `tool_id` attribute on `archestra_tool_invocation_policy` and `archestra_trusted_data_policy` was renamed from `profile_tool_id` (was a misnomer — the backend stores a bare `toolId` referencing `tools.id`, not an agent-tool assignment ID).

* **resources renamed to match backend + frontend naming.** No deprecation aliases; HCL must migrate.

  * `archestra_sso_provider` → `archestra_identity_provider`. Backend table is `identity_providers`, route is `/api/identity-providers`, frontend page is `/settings/identity-providers/`. The legacy `sso_provider` name is the provider-side outlier.
  * `archestra_chat_llm_provider_api_key` → `archestra_llm_provider_api_key`. Backend route is `/api/llm-provider-api-keys`. The `chat_` prefix was misleading — the same key is consumed by both Chat and the LLM Proxy.

  Migration: rename the `resource "archestra_sso_provider" "..."` and `resource "archestra_chat_llm_provider_api_key" "..."` blocks in HCL, then run `terraform state mv archestra_sso_provider.<n> archestra_identity_provider.<n>` and `terraform state mv archestra_chat_llm_provider_api_key.<n> archestra_llm_provider_api_key.<n>` for each instance. Schema, attribute names, and import IDs are unchanged.

* **`archestra_mcp_registry_catalog_item.local_config.environment`** changed from `Map<string, string>` + sibling `mounted_env_keys: Set<string>` to `SetNested<{key, type, value, default, description, required, prompt_on_installation, mounted}>` matching the wire shape. Existing HCL must be rewritten one entry per element.

* **Policy `conditions`** on `archestra_tool_invocation_policy` and `archestra_trusted_data_policy` changed from scalar fan-out (`argument_name` / `attribute_path` / `operator` / `value`) to `conditions = [{ key, operator, value }, ...]` matching the backend's array semantics. Single-condition policies must be wrapped in a one-element list; multi-condition policies (previously impossible) are now supported.

* **`archestra_mcp_server_installation` import requires composite ID `<uuid>:<name>`.** The backend rewrites `name` on insert (local installs get an `-<ownerId|teamId>` suffix), so the user's original base name can't be recovered from the API response. Bare-UUID import is rejected with an actionable error pointing at the composite format.

* **Validators tightened** — plan-time `OneOf`/`Between`/numeric-bounds added across enum and numeric attributes. Mainly catches typos earlier; configurations that previously round-tripped values the backend would 400 on now fail at plan time. **Type tightening**: `local_config.node_port` and `oauth_config.streamable_http_port` are now `Int64` (were `Float64`); fractional values are no longer accepted.

### Features

* **JSON Merge Patch architecture.** Update emits only fields whose plan value differs from state; sensitive sub-fields are masked in debug logs. Closes the structural class of bugs where unchanged values were re-sent on every Update — sensitive fields no longer leak back onto the wire, and `labels` / `teams` no longer clobber backend defaults or external edits. Documented in the new `ARCHITECTURE.md`.
* **New resources:** `archestra_agent_tool_batch` (bulk-assign N tools onto an agent in one round-trip), `archestra_tool_invocation_policy_default` and `archestra_trusted_data_policy_default` (the UI's `DEFAULT` row, with drift-detecting Read), `archestra_tool_policy_auto_config` (LLM-driven policy generator), `archestra_llm_model` (replaces the removed `archestra_token_price`).
* **New data sources:** `data.archestra_agent_tools` (plural), `data.archestra_mcp_tool_calls` (audit log), `data.archestra_tool` (lookup any tool by name).
* **`archestra_mcp_server_installation.tools`** is now a Computed list with `{id, name, description, parameters, assigned_agent_count, assigned_agents, created_at}` per element — `for_each` over an install's tools without separate data-source blocks.
* **`archestra_mcp_server_installation.tool_id_by_name`** is now a Computed map for one-line tool-id lookups (`installation.tool_id_by_name["<server>__<short>"]`).
* **5 Registry guides**: Getting Started, Authentication, Resource Bring-up Order, BYOS Vault, Common Issues. Plus a Support block on the Registry index page.
* **Per-resource `import.sh`** — every importable resource auto-renders an `## Import` section in its docs page.
* **`scripts/bootstrap-local-stack.sh`** — one-command full-suite local setup with EE license + BYOS Vault + Ollama mock.

### Bug Fixes

* **`archestra_llm_model` apply crashed with `cannot unmarshal number into Go struct field .embeddingDimensions`** when the backend had any embedding model configured. Worked around at the OpenAPI patcher (`tools/oapi-patch`); the backend zod is being fixed upstream so the patcher can be removed.
* **`archestra_mcp_server_installation` Create halted with "Provider returned invalid result object after apply"** on every install regardless of config — Create never assigned `secret_id` from the API response. Defensive same-shape fix applied to `archestra_identity_provider` (`domain_verified` / `organization_id` / `user_id`).
* **`archestra_mcp_server_installation.secret_id` non-idempotent on Update** — schema was `Optional`-only; the backend auto-creates a secret on installs with `user_config_values` / `is_byos_vault`, and the next plan diffed `secret_id = "<uuid>" -> null` and triggered destroy+recreate. Schema is now `Optional + Computed + UseStateForUnknown + RequiresReplace`.
* **`archestra_organization_settings` produced "non-refresh plan was not empty" failures** on any apply against a populated singleton — 26 `Optional+Computed` fields lacked `UseStateForUnknown`. Plan modifiers added; the documented sticky-from-state contract is now enforced by the schema.
* **`terraform import` round-trip** on `archestra_agent_tool` and `archestra_agent_tool_batch` now restores `agent_id` / `tool_id` and `credential_resolution_mode` from state correctly (previously triggered destroy+recreate on the next plan).
* **`archestra_team.convert_tool_results_to_toon`** now pre-flights the org-level `compression_scope` at plan time and fails with an actionable error rather than producing partial state via the framework's "inconsistent result after apply".
* **`archestra_limit.ValidateConfig`** no longer fires false-positive "X is required when X is set" errors when required-when fields reference another resource's value (Unknown at plan time).
* **`archestra_optimization_rule.llm_provider`** enum widened from 3 values to the backend's full 17-provider list.
* **`archestra_optimization_rule.conditions`** Read now parses the response back into typed conditions; out-of-band edits were previously invisible to Terraform.
* **`archestra_agent.labels`** preserves the empty-vs-null distinction across read; backend `[]` no longer becomes a perma-diff against HCL `labels = []`.
* **`data.archestra_tool`** now retries when the tool isn't found yet, eliminating a race against `archestra_mcp_server_installation` tool registration in the same plan.
* **Schema preservation defaults** added on `oauth_config.supports_resource_metadata` (false) and `image_pull_secrets[].source` ("existing") to stop perma-diffs when HCL omits them.
* **`user_config.default` and `local_config.environment[].default`** are now type-gated against the sibling `type` before send (was blind JSON-decoding HCL strings).

## [0.6.0](https://github.com/archestra-ai/terraform-provider-archestra/compare/v0.5.0...v0.6.0) (2026-04-23)


### Features

* update provider for Archestra v1.2.20 API ([#82](https://github.com/archestra-ai/terraform-provider-archestra/issues/82)) ([f658cd4](https://github.com/archestra-ai/terraform-provider-archestra/commit/f658cd45f4a9e96a07607b387718cf019635b3f0))


### Dependencies

* **terraform:** bump github.com/hashicorp/terraform-plugin-sdk/v2 from 2.38.1 to 2.38.2 in the terraform-go-dependencies group ([#77](https://github.com/archestra-ai/terraform-provider-archestra/issues/77)) ([cc21eec](https://github.com/archestra-ai/terraform-provider-archestra/commit/cc21eec3cc4141d8de5ca1390ecbbed3a5371c91))

## [0.5.0](https://github.com/archestra-ai/terraform-provider-archestra/compare/v0.4.1...v0.5.0) (2026-01-06)


### Features

* add `archestra_prompt` resource + data sources ([#67](https://github.com/archestra-ai/terraform-provider-archestra/issues/67)) ([8b1f024](https://github.com/archestra-ai/terraform-provider-archestra/commit/8b1f024a286f9bfe22b3633d747c2f33f99188ca))
* add `archestra_sso_provider` resource ([#65](https://github.com/archestra-ai/terraform-provider-archestra/issues/65)) ([37bf7ac](https://github.com/archestra-ai/terraform-provider-archestra/commit/37bf7ac08c90f9dc33bb4be8f17ad3b4bde956e6))

## [0.4.1](https://github.com/archestra-ai/terraform-provider-archestra/compare/v0.4.0...v0.4.1) (2025-12-29)


### Bug Fixes

* address `docker_image` without `command` + env var bugs in `archestra_mcp_registry_catalog_item` resource ([#62](https://github.com/archestra-ai/terraform-provider-archestra/issues/62)) ([4f5c02b](https://github.com/archestra-ai/terraform-provider-archestra/commit/4f5c02be8590af3915cd48d11b7bef995d6fe470))

## [0.4.0](https://github.com/archestra-ai/terraform-provider-archestra/compare/v0.3.0...v0.4.0) (2025-12-29)


### Features

* Add `remote_config` support to `archestra_mcp_registry_catalog_item` resource ([#60](https://github.com/archestra-ai/terraform-provider-archestra/issues/60)) ([836cb78](https://github.com/archestra-ai/terraform-provider-archestra/commit/836cb78d14c1917f00ca944e05a0e46c85576174))

## [0.3.0](https://github.com/archestra-ai/terraform-provider-archestra/compare/v0.2.0...v0.3.0) (2025-12-19)


### Features

* add `archestra_dual_llm_config` resource ([#50](https://github.com/archestra-ai/terraform-provider-archestra/issues/50)) ([9e55ec8](https://github.com/archestra-ai/terraform-provider-archestra/commit/9e55ec860fde2cd4dd14f0d4582d0a30290bb2b6))
* add `archestra_profile_tool` resource + rename `archestra_agent_tool` datasource -&gt; `archestra_profile_tool` ([#47](https://github.com/archestra-ai/terraform-provider-archestra/issues/47)) ([e2345ec](https://github.com/archestra-ai/terraform-provider-archestra/commit/e2345ec0436ae2b12159bb3aba907c55cb687a7d))
* after mcp server installation, wait for tools to be available ([#54](https://github.com/archestra-ai/terraform-provider-archestra/issues/54)) ([2b69232](https://github.com/archestra-ai/terraform-provider-archestra/commit/2b6923253b1bd07c7b7e099856f929e5ac2d1262))
* rename `archestra_mcp_server` resource to `archestra_mcp_registry_catalog_item` ([#46](https://github.com/archestra-ai/terraform-provider-archestra/issues/46)) ([baf01b6](https://github.com/archestra-ai/terraform-provider-archestra/commit/baf01b64bd1b8018379e0d54428f8451aafeb0e7))

## [0.2.0](https://github.com/archestra-ai/terraform-provider-archestra/compare/v0.1.0...v0.2.0) (2025-12-17)


### Features

* add `archestra_chat_llm_provider_api_key` resource ([#43](https://github.com/archestra-ai/terraform-provider-archestra/issues/43)) ([cefcfca](https://github.com/archestra-ai/terraform-provider-archestra/commit/cefcfcae3c7ae4e9fcb37cdc8159c6d9c2608776))

## [0.1.0](https://github.com/archestra-ai/terraform-provider-archestra/compare/v0.0.5...v0.1.0) (2025-12-17)


### Features

* add `archestra_mcp_server` Resource ([#15](https://github.com/archestra-ai/terraform-provider-archestra/issues/15)) ([8528aba](https://github.com/archestra-ai/terraform-provider-archestra/commit/8528aba32a1f5bf207204f2fad37fe860a591c10))
* add `archestra_organization_settings` resource ([#37](https://github.com/archestra-ai/terraform-provider-archestra/issues/37)) ([d54e0ac](https://github.com/archestra-ai/terraform-provider-archestra/commit/d54e0ac50e207aeac9a935b7b087f0f94b9bff74))
* add `archestra_team_external_group` resource and `archestra_team_external_groups` data source ([#34](https://github.com/archestra-ai/terraform-provider-archestra/issues/34)) ([aa7b286](https://github.com/archestra-ai/terraform-provider-archestra/commit/aa7b2861179bdff8bf1e39ff9fb52731989dd2a5))
* Add cost-saving resources for token pricing, limits, and optimization ([#22](https://github.com/archestra-ai/terraform-provider-archestra/issues/22)) ([8129190](https://github.com/archestra-ai/terraform-provider-archestra/commit/81291907126fdfdc163a91f2821976cf84a078aa))


### Bug Fixes

* add retry mechanism for async tool assignment in agent_tool data source ([#33](https://github.com/archestra-ai/terraform-provider-archestra/issues/33)) ([b41c866](https://github.com/archestra-ai/terraform-provider-archestra/commit/b41c866aeef0bbd62b7120be63c155f48338527a))


### Dependencies

* **terraform:** bump the terraform-go-dependencies group with 2 updates ([#24](https://github.com/archestra-ai/terraform-provider-archestra/issues/24)) ([a9c3e85](https://github.com/archestra-ai/terraform-provider-archestra/commit/a9c3e8556e0335e6a297f8f01580d21e9827cfcd))

## [0.0.5](https://github.com/archestra-ai/terraform-provider-archestra/compare/v0.0.4...v0.0.5) (2025-11-01)


### Features

* add `labels` to `archestra_agent` resource ([#12](https://github.com/archestra-ai/terraform-provider-archestra/issues/12)) ([acf2847](https://github.com/archestra-ai/terraform-provider-archestra/commit/acf28476cfbee55cdae551383c60bc4ec9de972e))

## [0.0.4](https://github.com/archestra-ai/terraform-provider-archestra/compare/v0.0.3...v0.0.4) (2025-10-27)


### Documentation

* remove `is_demo` and `is_default` from `archestra_agent` example ([147a05e](https://github.com/archestra-ai/terraform-provider-archestra/commit/147a05eb123f36c0f989ba44629dc08b1f1d6202))

## [0.0.3](https://github.com/archestra-ai/terraform-provider-archestra/compare/v0.0.2...v0.0.3) (2025-10-27)


### Documentation

* improve/clarify resource argument documentation + remove `is_default` + `is_demo` from agent resource ([#9](https://github.com/archestra-ai/terraform-provider-archestra/issues/9)) ([16fa690](https://github.com/archestra-ai/terraform-provider-archestra/commit/16fa69009ea967376a2a14c2b6dc51dcc3dcec41))

## [0.0.2](https://github.com/archestra-ai/terraform-provider-archestra/compare/v0.0.1...v0.0.2) (2025-10-27)


### Bug Fixes

* outstanding provider issues ([#7](https://github.com/archestra-ai/terraform-provider-archestra/issues/7)) ([c33e1ec](https://github.com/archestra-ai/terraform-provider-archestra/commit/c33e1ec1160976dce6434a4866594c066e9d0162))

## 0.0.1 (2025-10-26)


### Features

* Archestra Terraform provider (hello world) ([#1](https://github.com/archestra-ai/terraform-provider-archestra/issues/1)) ([e1ff1e4](https://github.com/archestra-ai/terraform-provider-archestra/commit/e1ff1e482d93bfa4562c0eeb2bcc5d311fe09fae))
