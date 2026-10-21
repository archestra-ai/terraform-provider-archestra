# Provider modernization audit


The reviewed milestone 411935f had **29 Terraform resources and four Terraform data sources**. That milestone generated all 29 resources in both Crossplane scopes. Agent/gateway relationships, team bindings, object sharing and organization defaults belong to their parents. Manual and Git-backed plugins share one resource. The [migration guide](../MIGRATION-v3.md) describes the clean-major names; there are no aliases for retired types. Additional durable API areas, final runtime/package validation, protected review and publication remain in progress. The [full native acceptance proof](consolidated-native-acceptance.json) records 147 passing cases with zero failures or skips against API 1.4.0-rc.49.

Current request-field review classifies **172 raw occurrences across 29 owners**,
with zero unclassified rows. These include manually encoded parent fields and
server-owned values; they are not a count of missing APIs. Native and Crossplane
results below retain their original binary boundaries.

The [current organization-defaults runtime proof](crossplane-organization-defaults.json) and [manifest/sharing regression proof](crossplane-skill-manifest-sharing.json) pass in both scopes with normal cleanup.

The [current consolidated runtime proof](consolidated-crossplane-runtime.json)
verifies 29 families in both scopes: 58 family/scope pairs and 124 runtime
records. Global test settings are restored and all owned fixtures clean up normally.
This evidence uses the same native provider bytes as the 147-case native proof.

The pre-consolidation checkpoint covered **47 registered resources in both scopes**.
The [current inventory](provider-v3-current-inventory.md) records the schema and
API call coverage. The [29-family baseline](crossplane-rc48-candidate.json) and
[six-family expansion](crossplane-persistent-extensions.json) have separate recorded
binary hashes and real both-scope lifecycle evidence. The [full 35-family sweep](crossplane-35-family-candidate.json) passes 102 records with normal cleanup. [Four additional families](crossplane-a2a-environment-bindings.json) and remote-agent creation grants pass in both scopes on separate recorded binaries. [Two credential-routing families](crossplane-credential-routing.json), their mode variants and creation grants add twelve passing both-scope records, including in-place OAuth rotation and nullable/empty clears. [Stored GitHub registrations and compatibility constraints](crossplane-github-registrations-compatibility.json) add six verified records in both scopes. Both Linux runtime images
and embedded xpkgs pass [packaging and actual image reconciliation](runtime-package-validation.json).

The [API lifecycle decisions](api-lifecycle-decisions.md) track every broader
persistent API area. [Request-field review](request-field-review.md) classifies all
90 baseline raw occurrences; `current-request-field-review.json` classifies all
172 current occurrences. These raw counts include manual encoding, other agent
subtypes and server-owned state. They are not counts of missing capabilities.
The completed 39-family full native suite passes 131 acceptance tests with zero failures or skips (`a2a-environment-native-acceptance.json`). The historical expansion has its own 126-test record (`persistent-extensions-native-acceptance.json`). New profile/service-account creation grants pass in both scopes (`crossplane-persistent-extension-grants.json`). The 41-family full native suite passed all 137 tests (`credential-routing-full-native-acceptance.json`); the 43-family suite passed all 140 with zero failures or skips (`github-registrations-full-native-acceptance.json`). Its six targeted cases are also recorded separately (`credential-routing-native-acceptance.json`). Final full-candidate repetition, remaining justified API implementations, normal
protected review and publication remain open. The sections below retain the
original audit baseline for traceability.


Audit inputs: Terraform `2e34cde000894e9c96793da0ec1f2c90b9193636` (v2.4.0),
Archestra `35e36afd3f8a29cd568bc486a9e448bb29028414` (main, platform 1.4.0-rc.43).

The current candidate exposes **31 resources and four data sources**, adding App. Against published API 1.4.0-rc.55, with the client regenerated from that release, [the full native suite](rc55-native-acceptance.json) passes all 163 cases in one run, including seven App cases, with zero failures or skips. Acceptance CI now pins that image digest. With the floor at rc.51, [the full native suite](rc55-floor-rc51-native-acceptance.json) passes all 164 cases against the same digest, adding the unauthenticated LLM proxy setting. Virtual key values are write-once: import adopts metadata and never retrieves the bearer credential. The provider's API floor is 1.4.0-rc.51, the release that introduced the organization setting for unauthenticated LLM proxy requests; rc.55 is the tested release. [Both-scope Crossplane reconciliation for App and AppInit](crossplane-apps-rc55.json) passes all four records against the same rc.55 digest with normal cleanup, using the reproducible `scripts/bootstrap-crossplane.sh` harness. It includes the fix that keeps initProvider-only Secret inputs creation-only; AppInit failed in both scopes before it. The other 30 families were not re-run on rc.55; their Crossplane evidence remains on rc.49 and earlier binaries.

The Knowledge File checkpoint exposed **30 resources and four data sources**. Its [full native suite](knowledge-files-native-acceptance.json) passes 156 cases with zero failures or skips. [Six current file records](crossplane-knowledge-files.json) pass both scopes and normal cleanup, including initProvider Secret inputs. The 29-family evidence below remains a historical reviewed milestone; the final expanded runtime/package sweep remains open.

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

## Historical baseline API-domain review

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

Manual plugin packages add one family in both scopes, including creation grants, authoritative file replacement and executable-mode preservation ([runtime proof](crossplane-manual-plugins.json)). The full 44-family native suite passes all 142 acceptance cases with zero failures or skips ([native proof](manual-plugins-full-native-acceptance.json)). Both records retain their exact frozen candidate hashes; current Linux packaging and the final combined sweep remain release gates.

Pinned repository packages add one family in both scopes, including creation grants, scheduled candidate configuration, credential-reference clearing and approved-byte preservation ([runtime proof](crossplane-repository-plugins.json)). The full 45-family native suite passes all 145 acceptance cases ([native proof](repository-plugins-full-native-acceptance.json)). Final combined-candidate and Linux package repetition remain release gates.

Revocable marketplace share links add one family and six both-scope records for skill/plugin membership, secret publication, anonymous read-only access, revocation drift and expiry ([runtime proof](crossplane-marketplace-share-links.json)). The full 46-family native suite passes all 148 acceptance cases ([native proof](marketplace-share-links-full-native-acceptance.json)). Final combined-candidate and Linux package repetition remain release gates.

Project-file content adds one family and four both-scope records for exact text/binary bytes, secret input and project references, drift repair, empty text updates, replacement refusal and normal deletion ([runtime proof](crossplane-project-files.json)). All152 native acceptance cases pass ([native proof](project-files-full-native-acceptance.json)). This is a pre-consolidation checkpoint; the major-version surface is being reviewed against product navigation.
