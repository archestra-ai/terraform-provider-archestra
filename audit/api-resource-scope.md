# Provider resource scope and API review

The current Terraform registry contains 29 families, all configured for both
Crossplane scopes and backed by accumulated real lifecycle evidence. The latest
candidate adds gateway skill publication/exclusions, project instructions and
knowledge-source exclusions. Atomic delegation additionally passes a two-process
Terraform lost-update regression and both-scope drift/deletion probes against the
published API 1.4.0-rc.48, following Archestra PR #9038. These are accumulated proofs,
not final-candidate or complete Archestra API coverage.

## Current release resource families

| Area | Registered Terraform resource families |
|---|---|
| Agents | Agent, MCP gateway, delegation, individual tool binding, authoritative tool batch, tool exclusions, skill activation policy, subagent exclusions, knowledge-source exclusions, gateway skill publication and exclusions |
| Organization and access | Organization settings, environment, team, external-group mapping, role, object permission policy |
| MCP | Registry catalog item, server installation |
| LLM | Provider key, discovered-model overrides, limit |
| Work configuration | Project, knowledge base, schedule trigger, versioned skill content and files |
| Credentials | Runtime credential definition, identity provider, platform API key |

The detailed schema, import, sensitivity, request-field and executed-runtime
records are linked from `README.md`. Schema guards do not replace runtime proof.
Provider keys now include shared ownership and SigV4 inputs, teams include role
assignments, models include embedding dimensions, and runtime definitions include
GitLab host management. Creation-time grants are now configurable on eight families. The published
rc.48 full Terraform suite passes 118 acceptance tests with zero skips; eight
distinct v2.4.1 schema upgrades preserve identity with no-op plans. Crossplane
creation-grant repetition and complete-candidate packaging remain release gates.

## Additional persistent API lifecycles

These are concrete additional API areas, not counted as covered and not silently
excluded. Treating them as part of the v3 release boundary requires additional
resource design and runtime validation; generation of types would not close them.

| Area | API examples | Decision still required |
|---|---|---|
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
path counts. Some rows contain several resources; others may belong as attributes
or bindings on an existing family. A release claiming all declarative Archestra
API lifecycles must resolve every row rather than divide by 29.

## Justified operation exclusions

Inference requests, chat messages, individual agent runs, telemetry, render jobs,
model-sync commands, tool invocation, indexing runs and connection tests are
operations or transient results rather than standalone declarative resources.
They can validate an existing resource but do not increase a coverage count.

OAuth/device-login callbacks, consent grants, credential sign-in and token
redemption require an interactive authorization lifecycle. A provider can refer
to an authorized connection, but applying configuration must not silently grant
external account access. Session/browser-storage management and UI pin/read
preferences are user activity, not organization infrastructure configuration.

Bulk APIs, clone, restore and permanent-delete endpoints are alternate operations
on existing resource lifecycles. Count the underlying managed identity once;
record the chosen soft-delete, retention and adoption behavior explicitly.
