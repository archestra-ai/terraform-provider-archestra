# Provider v3 migration (release candidate work in progress)

Do not upgrade production to this branch. The complete resource matrix, live
reconciliation suite, and release packaging are still being verified. Pin the
provider and Crossplane package to the tested release until that work is complete.

## Upgrade order

1. Back up Terraform state securely and export the managed Crossplane manifests,
   external-name annotations, connection secret references and provider settings.
   State and connection secrets can contain credentials even when marked sensitive.
2. Upgrade Archestra first to the release named in the final compatibility matrix.
   Environment resources require the environment-by-ID read endpoint introduced
   by archestra-ai/archestra#9021. Do not infer compatibility from a generated SDK.
3. Review retired resources and sharing changes below while still on v2. Remove
   dependencies on retired configuration before changing provider constraints.
4. Upgrade Terraform in a non-production copy of the workspace. Run refresh-only
   planning, then a normal plan. Review every deletion, replacement and grant
   revocation before applying. A version upgrade does not authorize recreation.
5. Upgrade Crossplane only after its bundled Terraform release is published and
   verified. Keep the old ProviderRevision available until adoption and rollback
   checks complete. Do not uninstall CRDs containing live managed resources.

## Retired API resources

The current Archestra API no longer implements these v2 resources:

- `archestra_optimization_rule`
- `archestra_tool_invocation_policy`
- `archestra_tool_invocation_policy_default`
- `archestra_tool_policy_auto_config`
- `archestra_trusted_data_policy`
- `archestra_trusted_data_policy_default`

There is no safe automatic translation of their semantics to current guardrails
or OpenAPPA policy. Inventory each old rule and design and test its replacement
against the current policy engine. A resource rename alone does not preserve
security behavior. Do not run a legacy destroy against an API that has removed
the route. After recording the old policy and its approved replacement, remove
only the stale Terraform binding with `terraform state rm ADDRESS`. This forgets
the binding; it does not recreate policy enforcement or prove an old rule was
removed server-side.

The removed Crossplane ToolInvocationPolicy CRD cannot be converted mechanically
to the new permission Policy: invocation decisions and object sharing are different
controls. Preserve its manifest, set and verify an orphan deletion policy before
removing the old managed resource, and verify replacement enforcement separately.
Do not delete the CRD until every instance has been accounted for.

## Sharing and permission policies

`scope` and `teams` are no longer creation/update inputs on agents, gateways and
catalog items. Current Archestra rejects these legacy inputs. Define direct grants
with `archestra_resource_permission_policy`, with one resource per target. Import
an existing policy before taking ownership so the plan shows any revoked grants:

```sh
terraform import archestra_resource_permission_policy.existing environment/ENVIRONMENT_UUID
```

Set `grants` to the complete desired direct-grant set. Inherited grants remain
owned by their source. Destroying a policy revokes **all direct grants**, not the
target object. Revision conflicts stop the operation so a concurrent change can
be refreshed and reviewed; the provider never blindly retries the write.

LLM key `scope` and `team_id` are now computed metadata. Remove them from writable
configuration. Catalog and key grants require the API's resource type and exact
object ID; resource-type tokens are API values (for example `mcpRegistry`).

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

Keep existing Crossplane group/kind and external names when a resource still
represents the same object. For a new kind or scope, first orphan the old managed
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
