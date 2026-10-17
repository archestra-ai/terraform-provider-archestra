# Provider v3 migration (release candidate work in progress)

## Public names in the clean v3 surface

The following resource names replace their former names; v3 does not register aliases. Review the new schema and import existing objects under the new resource type. Save state before removing obsolete declarations or addresses.

| Former type | v3 type |
|---|---|
| `archestra_marketplace_share_link` | `archestra_share_link` |
| `archestra_mcp_registry_catalog_item` | `archestra_mcp_registry_entry` |
| `archestra_mcp_server_installation` | `archestra_mcp_server` |
| `archestra_llm_provider_api_key` | `archestra_model_provider` |
| `archestra_llm_model` | `archestra_model` |
| `archestra_virtual_api_key` | `archestra_virtual_key` |
| `archestra_schedule_trigger` | `archestra_schedule` |
| `archestra_service_account_token` | `archestra_service_account_api_key` |
| `archestra_runtime_credential` | `archestra_credential` |
| `archestra_github_app_config` | `archestra_github_app` |

Agent/gateway relationships now belong to their parent fields. Import the parent and configure its subagents, remote agents, tools, skills, exclusions and hooks there. The obsolete combined proxy-profile type is not available for new creation. Earlier migration notes below describe prior schemas; they do not add v3 aliases.

Do not upgrade production to this branch. The complete resource matrix, live
reconciliation suite, and release packaging are still being verified. Pin the
provider and Crossplane package to the tested release until that work is complete.

## Ownership changes

Agent and gateway declarations now contain their subagents, remote agents,
tools, skills, exclusions and hooks. Omit an optional relationship field to
preserve it; an explicit empty collection clears it. Declare each relationship
in one parent only. `archestra_remote_agent` still owns the external registration
and authentication, separately from the agents that reference it.

Team `external_group_ids` and `vault_folder` replace the standalone SSO-group and
Vault-folder binding types. Team lookup exposes the same bindings.

Every shareable object exposes `permissions` for its additional direct grants.
Omit it to preserve the current grants; `[]` clears additional grants. Updates
keep the object's identity and existing human-owner access. Inherited access and
ownership remain managed by Archestra. Revision conflicts stop the write so a
concurrent edit can be refreshed and reviewed. Passthrough virtual keys remain
personal; Archestra rejects sharing them with other principals.

`archestra_organization_settings` owns `environment_defaults`,
`default_user_limits` and `default_permissions`. The corresponding standalone
resources are retired. Omitted default-policy/environment slots remain unmanaged;
empty sets clear a policy and an empty environment string restores the implicit
Default. A declared spending-default map owns its complete set of rows; `{}`
clears the set. Destroying organization settings leaves the singleton and its
settings intact.

`archestra_plugin` supports either `files` or a nested `git_source` object.
The separate Git plugin type is retired. Changes to the approved commit or source
selection require replacement; metadata, tracking and existing credential
references update in place. Import adopts either source mode. Git source controls
do not execute plugin hooks.

The read-only surface is `agent`, `team`, `tool` and `mcp_tool_calls`. Agent tool
lookups are included in the agent lookup, team group mappings in team lookup,
and server-specific tool lookup uses `tool.mcp_server_id`.

## Upgrade order

1. Save Terraform state and Crossplane manifests, external names and connection
   secret references securely.
2. Install the API capability floor in [COMPATIBILITY.md](COMPATIBILITY.md).
3. Rewrite declarations to the current names and parent-owned fields. Do not apply
   the old and new declarations against the same relationships concurrently.
4. Adopt existing identities under the new resource types using the procedure
   below, then inspect refresh-only and normal plans before applying changes.

## State and CRD identity

A gateway already represented by `archestra_mcp_gateway` keeps its resource type.
Moving an `archestra_agent` that represents a gateway to the gateway resource is
an adoption operation. Terraform 1.13 rejects ordinary cross-type `state mv` with
“resource types don't match.” Do not edit the raw state's resource type by hand.

For such a change, record its remote ID and configuration, back up state, remove
only the old binding, then import the same ID into the new configured resource.
Keep the remote object throughout. Review a no-op plan before any apply. This
procedure must be exercised against a disposable workspace for the exact source
and destination schemas before applying it to an existing workspace.

Some clean-major names also change Crossplane group/kind. Preserve external
names and use orphan/adoption when moving an existing binding to a new kind. For a new kind or scope, first orphan the old managed
resource and verify that deleting its Kubernetes binding preserves the backend
object. Create the new resource with the recorded `crossplane.io/external-name`
and the existing provider config. Verify Observe and a no-op reconciliation
before enabling update/delete management. Namespaced and cluster-scoped resources
cannot be moved merely by changing a manifest's namespace.

## LLM key primary flag

Rename `is_organization_default` to `is_primary` in HCL. The current API flag
selects the provider primary key; its old name incorrectly implied an
organization-only setting. Resource schema version 1 upgrades v2 state in place,
preserving the remote ID and stored sensitive value. This upgrade makes no API
writes and does not rotate the key. `is_agent_key` now exposes platform ownership.
Review a no-op plan after renaming HCL.

## Redacted identity provider secrets

Current Archestra returns `configured_secret_paths` instead of SSO credential
values. Refresh preserves previously managed secret values only when their paths
remain configured. Import cannot recover plaintext: supply a value explicitly
to manage rotation. Out-of-band replacement of a redacted value cannot be
detected by comparing plaintext; presence is observable. Dropping an entire
credential parent clears its scope according to the API contract; removing an
individual secret leaf means stop managing it, because the API preserves omitted
credentials on update. Sensitive values retained in Terraform state still require
protected state storage.

## Other configuration changes

- Removed organization settings whose APIs or columns were retired, including
  compression/TOON flags, global tool policy, default LLM name/provider writes,
  the old limit cleanup interval, old two-factor visibility flag and MCP OAuth
  token lifetime field. Map required settings to their current replacement and
  verify the resulting backend behavior; do not copy an obsolete value blindly.
- Team TOON settings and MCP Kubernetes `service_account` overrides were retired.
- MCP tool-call queries use cursor pagination. Replace the unsupported `search`
  input with `mcp_server_name` where an exact server filter is intended. Replace
  `total` with `returned_count`; it is the number returned under `max_records`,
  not the backend's total matching log count. Check `truncated` explicitly.
- Audit arguments, results and complete tool payloads are sensitive outputs.
  Terraform state still stores sensitive values; use protected state storage.

## Release gates

The release is blocked until upgrade/adoption fixtures, all meaningful-resource
coverage decisions, real Terraform and both Crossplane-scope lifecycle tests,
permission failure paths, documentation examples and package artifacts pass.
Generated CRDs alone do not satisfy these gates. Daily sync PRs require manual
review and must never automatically merge or publish breaking changes.

## Server-owned built-in metadata

Remove the `built_in_agent_config` block from HCL. Current Archestra reserves this
configuration for its seeder. The provider now exposes it as computed JSON metadata.
Schema-1 upgrades preserve the existing object ID, drop retired sharing/compression
attributes, and retain sensitive state. Agent metadata changes type from the old
block to JSON; refresh then reads the server-owned value. State upgrades do not
perform API writes. Back up state and inspect a refresh-only plan before apply.

## Runtime credential removal

Clear an agent's runtime credential references and apply that change while keeping
the credential definition. Remove the credential in a subsequent apply. Deleting
a referenced credential is rejected; the provider does not detach consumers.

## Historical migration validation

These probes were executed before the clean-major name and ownership
consolidation. Their recorded binaries remain the evidence boundary. Their CLI
fixtures need actual v2 type names and explicit v3 orphan/adoption for renamed
types before they can be reused; the old retained-relationship fixture also
predates parent-field ownership. They are historical evidence, not a promise of
automatic compatibility or current-candidate migration proof. Do not edit raw state resource types to manufacture an upgrade.


The local Terraform CLI probe imports through the v2 schema, upgrades an LLM key
from schema 0 to 1, preserves its ID and false primary flag, produces two no-change
plans, and destroys the fixture. True primary flags and stored secret preservation
are separately covered by unit migration tests. The executable probe is
`scripts/integration/verify-llm-state-upgrade.py`; it refuses non-loopback backends.

The local Kubernetes probe replaced actual v2 Agent CRDs with v3 CRDs, verified
retired-field pruning with storage version `v1alpha1`, orphaned the old binding,
adopted the same ID in namespaced Observe mode, then verified update and explicit
delete. No CRD version conversion was required for this schema change. Other
resource migrations still require their own runtime evidence before release.

## Current labels, key endpoints and limit bindings

LLM provider keys now manage sensitive `extra_headers`, `inference_base_url` and
labels. Headers remain sensitive in Terraform output but are stored in state.
Imports recover server-visible headers; inline API keys and unreturned vault
references cannot be reconstructed. The API must include the label-read fixes
from archestra-ai/archestra#9032 before using key or knowledge-base labels.

Teams support `parent_id` and labels; removing `parent_id` detaches the team from
its parent. Limits support `cleanup_interval` and labels. Changing a limit's
`entity_id` or `entity_type` now requires replacement because the API cannot move
an existing limit to another entity. Inspect replacement plans before applying.

## Catalog tenancy and model adoption

Catalog `multitenant` is immutable after creation, so changing it now requires
replacement. `cloned_from` selects the source during creation and also requires
replacement when changed. `environment_id` changes update the existing catalog;
inspect effects on installed runtimes before moving a live catalog.

Set a model's `llm_provider` when more than one provider exposes the same
`model_id`. Ambiguous adoption now fails before writing. Prefer importing the
model UUID; a bare model name works only when unique. Existing model IDs remain
stable when `llm_provider` is omitted.

Model resources manage cache prices, custom context/output limits, labels and
native Ollama generation parameters. Removing an override clears it. The
`input_modalities` and `output_modalities` inputs describe overrides;
`effective_input_modalities` and `effective_output_modalities` expose the current
server values, including discovered defaults. An import cannot infer which
modality values were explicitly configured, so re-declare intended overrides in
HCL and inspect the first plan. Destroy clears managed overrides and labels and
retains the discovered model. Failed resets remain errors instead of silently
dropping the Terraform binding.

## Crossplane nested object representation

Framework single blocks such as `spec.forProvider.oidcConfig` and
`spec.forProvider.runtime` are native objects in v3 CRDs, matching Terraform
configuration and state. Use `oidcConfig: {clientId: ...}` rather than a
one-element array. Real list blocks, such as role-mapping rules, remain arrays.
Identity-provider CRDs were not in the released v2 five-resource inventory.
For an experimental manifest using an array, export it before upgrading and
change it to an object before applying the new schema; verify the stored
manifest was not pruned and that its external-name annotation is preserved.

## Current role and credential capabilities

Teams accept `roles` as a set of role identifiers, such as
`[archestra_role.reader.role]`. Omit it to leave existing assignments unmanaged;
set `roles = []` to revoke the team's assigned roles. Team membership roles and
organization roles are separate controls.

Provider keys accept `shared = true` to create a key owned by the organization.
Its audience is controlled by its permission policy. Ownership is immutable;
changing `shared` requires replacement. Omitting it after import preserves the
current ownership, including keys created before this attribute existed.

Bedrock keys accept sensitive `aws_access_key_id`, `aws_secret_access_key` and
optional `aws_session_token` inputs. They cannot be combined with an inline API
key or Vault reference. Token rotation resends the pair because Archestra
replaces the complete SigV4 secret; ordinary metadata updates do not resend it.
Removing all three attributes stops managing these write-only inputs and clears
them from Terraform state. It does not revoke the credentials stored remotely;
delete the provider key to remove its stored credential. Import cannot recover
secret values: redeclare them to resume managing rotation.

Models accept `embedding_dimensions` using the dimensions supported by the
current API. `effective_embedding_dimensions` reports the live API value even
when the override is unmanaged. Removing a declared dimension clears it; destroy
clears it only if Terraform was managing it. Models used by knowledge-base
embedding configuration cannot change dimensions until that configuration is
removed; the provider surfaces the API refusal. As with modality overrides,
import cannot infer whether an existing dimension was explicitly managed.

### Model modality state upgrade

V2 stored provider-discovered modalities in the same state fields as explicit
modality overrides. V3 upgrades that state to schema version 1, preserving those
values as `effective_input_modalities` / `effective_output_modalities` and making
undeclared override inputs null. This state-only conversion does not clear the
live model. Explicit overrides in configuration are reapplied to establish their
management intent; review that plan before applying. Verify a second plan is a
no-op. The real CLI probe is recorded in `audit/model-state-upgrade.json`.

Identity-provider saved-secret migration is recorded separately in
`audit/identity-state-upgrade.json`. V2 cannot collect redacted secrets from the
current API, so that probe restores a synthetic saved secret into actual v2
imported state to model pre-redaction saved state. The v3 upgrade preserves it
and the remote identity; it does not recover a lost secret from the API.

### Agent policy fields

`archestra_agent.skills` owns activation mode and stable native, external-MCP or
plugin references. Hidden or unavailable references require restored visibility
or explicit removal before management. `excluded_subagent_ids` owns automatic
subagent exclusions; an empty set clears them. Both fields preserve the agent.

### Versioned skill content

`archestra_skill` owns manifest metadata, the Markdown instruction body and the
complete file map. Set `files = {}` to remove resource files. Explicit empty
`environment_ids` removes environment restrictions; `labels = []` clears labels.
Files declare `utf8` or `base64` encoding. Import by skill UUID.

The backend trims outer manifest/body whitespace; equivalent configured body
formatting is retained. Import cannot recover original formatting. Content/file
updates send the last observed `latest_version` and reject stale versions;
frontmatter-only changes do not create content versions, so that counter does not
provide concurrency protection for metadata-only edits. Destroy uses soft delete
and does not invoke the separate permanent-purge API. Git-synchronized content
remains owned by its source, and the API rejects conflicting content edits.
