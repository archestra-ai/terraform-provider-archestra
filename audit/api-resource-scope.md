# Provider resource scope and API review

The current registry contains 45 families, all configured for both Crossplane
scopes. The historical 29-family proof remains in `crossplane-rc48-candidate.json`;
the six-family expansion and two existing-resource control probes pass in both
scopes in `crossplane-persistent-extensions.json`. Those evidence sets have distinct
recorded candidate hashes. `crossplane-35-family-candidate.json` records the full
35-family repetition; `crossplane-a2a-environment-bindings.json` adds four verified
families in both scopes. `crossplane-credential-routing.json` adds two credential
families, their mode variants and creation grants in both scopes. Both actual Linux runtime images and their embedded
xpkgs also pass `runtime-package-validation.json`. Final full-family repetition,
normal protected review and publication remain open.

The [lifecycle decisions](api-lifecycle-decisions.md) are the authoritative current
implementation queue. They classify all 15 additional API review areas, including
consolidated OAuth client facades and the platform-owned LLM proxy singleton.
The original 90 raw unmapped request-field occurrences are fully classified in
`request-field-review.md`; they were not 90 confirmed missing capabilities. The
current 45-family inventory has 144 raw occurrences, all classified in
`current-request-field-review.json`. Additional routes and shared profile schemas
increase that raw count. Gateway subagent controls and installation personal-user
selection/hibernation now have native and both-scope runtime regressions.

## Current release resource families

| Area | Registered Terraform resource families |
|---|---|
| Agents | Agent, MCP gateway, delegation, individual tool binding, authoritative tool batch, tool exclusions, skill activation policy, subagent exclusions, knowledge-source exclusions, gateway skill publication and exclusions |
| Organization and access | Organization settings, environment, team, external-group mapping, role, object permission policy |
| MCP | Registry catalog item, server installation |
| LLM | Provider key, discovered-model overrides, limit |
| Work configuration | Project, knowledge base, schedule trigger, versioned skill content and files |
| Credentials | Runtime credential definition, identity provider, platform API key |

The detailed schema, import, sensitivity, request-field and executed-runtime records
are linked from `README.md`. Schema guards do not replace runtime proof. Provider
keys now include shared ownership and SigV4 inputs, teams include role assignments,
models include embedding dimensions, and runtime definitions include GitLab host
management. Creation-time grants are configurable on fifteen families. The historical
29-family rc.48 Terraform suite passed 118 acceptance tests with zero skips, and
the 35-family expansion passed 126; the 39-family suite passed 131
with zero failures or skips (`a2a-environment-native-acceptance.json`). The
41-family suite passed all 137 (`credential-routing-full-native-acceptance.json`);
the 43-family suite passed all 140 (`github-registrations-full-native-acceptance.json`);
all 16 retained v2.4.1 resource families preserve identity with no-op migration
plans. The full 35-family Crossplane sweep passes 102 records, including 26 grant cases;
the four-family A2A/environment binding wave adds ten passing records with normal
cleanup, including the eleventh grant-bearing family in both scopes. Credential routing
adds twelve records including both mode variants and the twelfth/thirteenth
grant-bearing families. Stored GitHub registrations and version-constraint rejection/recovery
add six verified records (`crossplane-github-registrations-compatibility.json`).
Registered-family lifecycle coverage is complete; full platform API extensions and final candidate repetition remain release gates.

## Additional persistent API lifecycles

This original review queue is retained for traceability; use [current decisions](api-lifecycle-decisions.md) for implemented status and exclusions.

These are remaining requested persistent API lifecycles, not counted as covered.
Completing the full platform coverage requires resource design, implementation and
runtime validation; generation of types would not close them.

| Area | API examples | Remaining implementation and validation |
|---|---|---|
| LLM proxies and legacy profiles | `/api/agents` with `agentType=llm_proxy` or `profile` | Distinct subtype lifecycle, current replacement for legacy profiles, and subtype-specific fields; current Agent/Gateway resources cover `agent`/`mcp_gateway` only |
| Plugins and externally synchronized skills | `/api/skills`, `/api/plugins` | Manual skill content/files are covered; plugin packages and external source synchronization remain open |
| Apps and tool bindings | `/api/apps`, `/api/apps/{appId}/tools/{toolId}` | Versioned source, enable/lock behavior, bindings and deletion |
| Project files and apps | `/api/projects/{id}/files`, `/api/projects/{id}/apps/{appId}` | Attachment and binding ownership; project instructions are implemented and tested as an attribute |
| Connectors and knowledge associations | `/api/connectors`, `/api/connectors/{id}/knowledge-bases`, `/api/connectors/{id}/member-overrides` | Secret preservation, provider-specific configuration, permission sync and associations |
| Knowledge documents and directories | `/api/knowledge-files`, `/api/knowledge-directories` | Binary/content transport, hashes, async indexing, hierarchy and deletion |
| Remote A2A agents | `/api/a2a/remote-agents` | Persistent declaration vs inspection, credentials and visibility |
| Service accounts and tokens | `/api/service-accounts`, `/api/service-accounts/{id}/tokens` | Identity, permission scope, one-time tokens, rotation and explicit credential-creation authorization |
| Virtual and OAuth clients | `/api/llm-virtual-keys`, `/api/oauth-clients`, `/api/mcp-oauth-clients`, `/api/llm-oauth-clients` | Three different client lifecycles, secret-once fields, token exposure and rotation |
| GitHub configurations | `/api/github-app-configs`, `/api/github-pats` | Existing external installation vs registration, secret preservation and import |
| Guardrails and OpenAPPA | `/api/guardrails-deployment`, `/api/guardrails-policy`, `/api/openappa/battery-packages/{name}`, `/api/openappa/battery-installs`, `/api/openappa/credential-bindings/{variable}`, `/api/openappa/policy-tests` | Versioned policy artifacts, deployment, binding and execution boundaries |
| ChatOps configuration and bindings | `/api/chatops/bindings`, `/api/chatops/config/slack`, `/api/chatops/config/ms-teams`, `/api/chatops/config/telegram` | Configuration vs external app provisioning, existing grants and test isolation |
| Hooks and site notifications | `/api/hooks`, `/api/site-notification` | Persistent configuration vs sending a notification or executing a hook |
| Default limits, environment defaults and Vault bindings | `/api/default-user-limits`, `/api/environments/defaults`, `/api/teams/{teamId}/vault-folder` | Singleton/full-set authority, enterprise prerequisites and secret-manager ownership |
| Alert mutes and skill share links | `/api/internal_mcp_catalog/{id}/alert-mutes/{kind}`, `/api/mcp_server/{id}/alert-mutes/{kind}`, `/api/skill-share-links` | Expiration, public-access secrets and revocation |

This table is a lifecycle review queue, not a numerical denominator inferred from
path counts. Some rows contain several resources; others may belong as attributes or
bindings on an existing family. A release claiming all declarative Archestra API
lifecycles must resolve every row rather than divide by 29.

## Specific existing-resource follow-ups

The raw request audit also needs a field-by-field review. Two concrete gaps are
already identified in the existing MCP installation family: selection of `userId` for administrative personal installations and persistent per-install hibernation configuration.
Catalog `hibernationMode` is a bulk cascade onto installation rows, so it must not
be mistaken for a readable catalog attribute. These are separate from manually
encoded fields such as catalog OAuth configuration, installation secret inputs,
skill frontmatter, permission revisions, and team membership updates, which the
static AttrSpec-only mapping count does not recognize.

## Justified operation exclusions

Inference requests, chat messages, individual agent runs, telemetry, render jobs,
model-sync commands, tool invocation, indexing runs and connection tests are
operations or transient results rather than standalone declarative resources. They
can validate an existing resource but do not increase a coverage count.

OAuth/device-login callbacks, consent grants, credential sign-in and token
redemption require an interactive authorization lifecycle. A provider can refer to
an authorized connection, but applying configuration must not silently grant
external account access. Session/browser-storage management and UI pin/read
preferences are user activity, not organization infrastructure configuration.

Bulk APIs, clone, restore and permanent-delete endpoints are alternate operations on
existing resource lifecycles. Count the underlying managed identity once; record the
chosen soft-delete, retention and adoption behavior explicitly.

Manual plugin packages add one family in both scopes, including creation grants, authoritative file replacement and executable-mode preservation ([runtime proof](crossplane-manual-plugins.json)). The full 44-family native suite passes all 142 acceptance cases with zero failures or skips ([native proof](manual-plugins-full-native-acceptance.json)). Both records retain their exact frozen candidate hashes; current Linux packaging and the final combined sweep remain release gates.

Pinned repository packages add one family in both scopes, including creation grants, scheduled candidate configuration, credential-reference clearing and approved-byte preservation ([runtime proof](crossplane-repository-plugins.json)). The full 45-family native suite passes all 145 acceptance cases ([native proof](repository-plugins-full-native-acceptance.json)). Final combined-candidate and Linux package repetition remain release gates.
