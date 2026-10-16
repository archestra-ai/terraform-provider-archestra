# Provider modernization audit

Current v3 configuration covers **35 registered resources in both scopes**.
The [current inventory](provider-v3-current-inventory.md) records the schema and
API call coverage. The [29-family baseline](crossplane-rc48-candidate.json) and
[six-family expansion](crossplane-persistent-extensions.json) have separate recorded
binary hashes and real both-scope lifecycle evidence. Both Linux runtime images
and embedded xpkgs pass [packaging and actual image reconciliation](runtime-package-validation.json).

The [API lifecycle decisions](api-lifecycle-decisions.md) track every broader
persistent API area. [Request-field review](request-field-review.md) classifies all
90 baseline raw occurrences; `current-request-field-review.json` classifies all
122 expansion occurrences. These raw counts include manual encoding, other agent
subtypes and server-owned state. They are not counts of missing capabilities.
The expansion full native suite passes 126 acceptance tests with zero failures or skips (`persistent-extensions-native-acceptance.json`). New profile/service-account creation grants pass in both scopes (`crossplane-persistent-extension-grants.json`). Final full-candidate repetition, remaining justified API implementations, normal
protected review and publication remain open. The sections below retain the
original audit baseline for traceability.


Audit inputs: Terraform `2e34cde000894e9c96793da0ec1f2c90b9193636` (v2.4.0),
Archestra `35e36afd3f8a29cd568bc486a9e448bb29028414` (main, platform 1.4.0-rc.43).

## What is verified

- Crossplane is in `crossplane/` in this repository, not a separate provider repository.
  It has a separate Go module (`github.com/archestra-ai/provider-archestra`). Upjet
  consumes the executable Terraform schema; generated types, controllers and CRDs
  are ignored and rebuilt for both cluster and namespaced scopes. The build
  submodule is Crossplane's shared build system, not the Archestra provider.
- The baseline registry had **22 Terraform resources and seven data sources**.
  Five Terraform resources are configured for Crossplane: **5/22 = 22.7%**.
  This is configuration inclusion only. One included policy kind calls removed APIs.
  Generation/build/module checks pass for those five kinds in both scopes (10 managed-resource CRDs, plus five provider/configuration CRDs). The module has no behavioral tests. No reconciliation or runtime coverage percentage is established.
- `go test ./...` passes at the Terraform baseline. Actual Terraform acceptance
  against an isolated 1.4.0-rc.43 instance produces 15 failing tests in the agent,
  team, optimization and policy subsets. The test database and cluster were
  disposable; no existing development context was switched.
- Current API request schemas explicitly reject `scope` and `teams` on agent
  creation. The real backend returns 400 and directs callers to `initialGrants`.
  Presence of a route is insufficient evidence of provider compatibility.
- All six legacy policy/optimization resources have missing direct API calls.
  Organization settings also uses removed `PATCH /api/organization/llm-settings`.
- All 22 state schema versions are zero. Terraform `sensitive` values remain in
  state; none of these resources declares native Terraform write-only attributes.
  Nested secret flags and attribute modes are recorded in the JSON inventory.

`provider-api-inventory.md` is the compact existing-resource matrix. The JSON
includes individual methods, paths, request fields, retired fields, query
parameters, Terraform attribute modes, sensitivity, import implementation and test
source counts. Test declarations are explicitly distinguished from executed tests.
Direct calls are inventoried; shared helper calls, every request/response field,
permissions, import formats and delete behavior still need manual reconciliation.

## Current API domains and meaningful-resource review

The API has 683 paths, not 683 managed resources. This is the review queue for a
complete supported-resource denominator; it is not a claim of completed coverage.

| Domain | Existing Terraform support | Work required before claiming coverage |
|---|---|---|
| Agents, gateways | Resources exist | Replace retired audience fields with grants; audit runtime fields, soft-delete and ownership |
| Agent tools, exclusions, delegations | Resources exist | Authoritative vs additive behavior, pagination, composite imports and revocation |
| MCP catalog and installations | Resources exist | New catalog/deployment fields, credentials, async readiness, soft-delete, paging |
| Teams and external groups | Resources exist | Current member response shape, permission semantics, retired TOON settings |
| Identity providers | Resource exists | Current non-UUID IDs, secret round trips, OIDC/SAML, immutable fields |
| Runtime credentials | Resource exists | Personal/organization binding lifecycle, masked fields and secure imports |
| LLM provider keys, model overrides, limits | Resources exist | Discovery/adoption, secrets, list filters, reset-on-delete and current settings |
| Organization settings | Resource exists | Split retired LLM settings from current connection/security/knowledge/agent settings |
| Legacy invocation/trusted-data/default/autoconfig policies | Five resources | Removed APIs; migrate policy intent to current guardrails/OpenAPPA, with behavioral policy tests |
| Optimization rules | Resource exists | Removed API; document semantic replacement or explicit retirement |
| Environments and resource defaults | Missing | By-ID API, namespace validation, dependencies/409 deletion, network-policy effects |
| Resource permission grants | Missing | Revision conflicts, principal types, ownership protection, revocation and authoritative semantics |
| Roles, service accounts and tokens | Missing | Read/write permissions, secret-once tokens, rotation, revocation and import limits |
| Projects, instructions, app bindings | Missing | Ownership, environment, files, soft/permanent deletion and attachment relationships |
| Schedules | Missing | Project/agent scope, timezone, disabled creation, runs excluded from configuration |
| Skills and plugins | Missing | Versioned content, GitHub sync, ownership, restore and deletion semantics |
| Knowledge bases and connectors | Missing | Authentication prerequisites, indexed lifecycle, associations and permissions |
| Knowledge files/directories | Missing | Binary/content transport, async indexing, soft deletion and size limits |
| Remote A2A agents | Missing | Inspection vs persistent declaration, owner/team grants and secrets |
| Apps | Missing | Versioned source, environment, enable/disable, tool bindings and ownership |
| Virtual keys and OAuth clients | Missing | Secret creation/rotation, credential bindings, read-redaction and caller permissions |
| GitHub App configs and PAT registrations | Missing | Secret preservation, organization/personal access, externally approved installations |
| Guardrails deployment/policy and OpenAPPA configuration | Missing | Distinguish declarative packages/bindings/tests/sync from transient execution |
| Hooks and site notifications | Missing | Persistence and privileged scope; notification delivery is separate authorization |
| Per-environment default user limits and vault bindings | Missing | Enterprise/read-only-vault gating, no implicit credential creation |
| Inference, chat messages, telemetry, runtime executions, OAuth handshakes, bulk RPCs | Exclude as standalone managed resources | Transient operations or alternate operations on an existing lifecycle; retain justified data sources as appropriate |

Do not drop a supported lifecycle from the denominator because generation fails.
Do not count generated types or configuration mappings as CRUD/reconciliation.

## Release packaging defects

The Crossplane Makefile defaults `TERRAFORM_PROVIDER_VERSION` to 1.0.0, while
release CI passes only the package `VERSION`. Thus generated CRDs can describe the
current schema while the image downloads a different Terraform provider. The
Crossplane publication job also runs in parallel with publication of the Terraform
release assets it needs. The release fix must pass the tag's version explicitly
and depend on completed Terraform asset publication, then verify both packages.

## Breaking release proposal and dependency order

1. Complete the supported-resource and field inventory, with explicit exclusions.
2. Land missing API reads, update semantics and RBAC tests. Release API prerequisites.
3. Prepare Terraform v3.0.0: remove obsolete resource implementations and retired
   audience fields, expose current grants with unambiguous semantics, and update
   examples and client generation against the released API snapshot.
4. Test v2 state upgrade, import/adoption, no-op plans, out-of-band drift, failed
   updates, permission denial, 404, pagination and deletion for every supported resource.
5. Generate and compile both Crossplane scopes from that exact Terraform binary.
   Reconcile create/update/delete against the same isolated backend. Compare CRD
   schemas and conversion/storage versions with the last published package.
6. Open coherent dependent PRs. Review changes and migrations, then follow ordinary
   protected merge and release processes. Verify signed Terraform assets, registry
   availability and both Crossplane package architectures before announcing release.
7. Enable the daily agent only after scoped credentials and its deployment are
   approved and a real end-to-end dry run passes. Every daily PR requires manual review.

No modernization release or live scheduler is established by this audit.

## Migration safety

A name-only move between different Terraform resource types cannot be implemented
with plain `terraform state mv`: Terraform 1.13.4 rejects it with "resource types
don't match." Add and test the Plugin Framework cross-type state mover, or use
explicit import/adoption with a saved state backup and plans proving no recreation.
Do not repeat the older contributor-guide cross-type `state mv` example untested.

For removed policy APIs there is no safe mechanical rename to OpenAPPA. Translate
policy intent and compare decisions with representative allowed/denied fixtures.
Remove obsolete state only after policy migration has been verified; deleting a
state entry does not migrate or delete the remote policy. Keep sensitive backups
private and out of Git.

For Crossplane kinds that change group/version/name, preserve the external ID,
provider reference and secrets; reconcile an orphan deletion policy before removing
an old managed resource. Adopt through an observe-only phase, compare desired and
observed configuration, then enable writes. Test this with the actual prior CRDs;
recreating a CR with a new Kubernetes name is not a migration test.
