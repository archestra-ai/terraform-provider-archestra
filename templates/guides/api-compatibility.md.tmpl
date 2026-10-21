---
page_title: "API Compatibility - archestra Provider"
---

# API compatibility

Terraform provider and Archestra API releases have independent version numbers.
Pin both in deployment configuration. Crossplane packages embed the matching
Terraform provider release and inherit the same API requirements.

| Provider | API capability floor | Exact API versions tested | Status |
|---|---|---|---|
| Proposed 3.0.0 | 1.4.0-rc.51 | The release in `platform-version.txt` (native acceptance) | Development candidate; unreleased |

The floor includes atomic delegation, the current service-account team ownership API, the App tool-assignment read contract and the organization setting for unauthenticated LLM proxy requests (introduced in 1.4.0-rc.51) used by v3. It is a minimum required
capability level, not evidence that every later release is compatible. This
candidate has not established compatibility with all future 1.4.x versions or
with the eventual 1.4.0 stable release. Prereleases can change contracts between
release candidates; record each tested release explicitly.

`platform-version.txt` at the provider release tag names the tested release.
Acceptance CI runs the native suite against that release and records the image
digest it used in the job summary. The provider embeds the same file for the
runtime check below.

Crossplane reconciliation is not part of CI. The recorded evidence for
1.4.0-rc.55 (image
`sha256:ead9d30000531fe77f2bfe8db103dc19426701234a2f97d88b59fdd7a1952e3c`)
covers App and AppInit in both scopes ([evidence](audit/crossplane-apps-rc55.json)). The other
30 families were not re-run on rc.55: APIKey, Agent, Credential, Directory,
Environment, knowledge File, project File, Gateway, GitHubApp, GitHubPAT,
KnowledgeBase, Limit, Model, ModelProvider, OAuthClient, Plugin, Project,
identity Provider, RegistryEntry, RemoteAgent, Role, Schedule, Server,
ServiceAccount, ServiceAccountAPIKey, Settings, ShareLink, Skill, Team and
VirtualKey. Their Crossplane evidence used 1.4.0-rc.49. Native acceptance, state migration, Crossplane
reconciliation and runtime package evidence are linked from [the audit](audit/README.md).

For Terraform, select an approved provider release with `required_providers` and
commit `.terraform.lock.hcl`. That constraint selects a provider version; Terraform
has no native constraint that selects or validates the remote Archestra server.
For Crossplane, pin an approved package version and its corresponding API version.

## Establishing a supported range

Publish a supported API interval only after compatibility CI passes against its
oldest supported release and newest released member. Both ends must exercise
native acceptance and both Crossplane scopes. Add a targeted regression when an
intermediate release changes a contract; passing the endpoints cannot prove every
intermediate prerelease. Advance the tested release through reviewed provider PRs
as upstream releases arrive. The Update Terraform Provider workflow in
archestra-ai/archestra opens them on the `automation/platform-api` branch: it
regenerates the client from the release's spec and updates
`platform-version.txt`. It never merges. Merging after acceptance passes makes
that release the tested one.

## Runtime compatibility check

Before configuring resource and data-source clients, the provider reads the
public `GET /health` endpoint. It rejects semantic versions below `1.4.0-rc.51`,
which lack required v3 API capabilities. Versions other than the tested release
in `platform-version.txt` produce a warning. Newer does not automatically mean
compatible.
Custom versions or unreadable version metadata produce a warning by default.
No resource client is configured when compatibility validation produces an error.

Use `api_version_constraint` to enforce your deployment's approved version or
range independently of the Terraform provider constraint:

```hcl
provider "archestra" {
  # An approved API release. Credentials stay in environment variables.
  api_version_constraint = "= 1.4.0-rc.55"
}
```

After a stable interval has been validated, a deployment can use a constraint
such as `>= 1.4.0, < 1.5.0`. That is constraint syntax, not a claim that this
candidate has validated that interval. With an explicit constraint, missing,
unreadable or non-semantic server versions are errors because the constraint
cannot be verified. A user constraint cannot lower the provider's minimum floor.

The probe sends no API key, follows no redirects, reads at most 4 KiB and has a
five-second deadline (a shorter configured HTTP timeout still applies). Its
errors do not expose the health response body. The same check runs inside the
Terraform provider embedded in Crossplane. To constrain a Crossplane deployment,
include the same `api_version_constraint` key in the credentials JSON referenced
by its ProviderConfig, for example:

```json
{
  "api_key": "arch_...",
  "base_url": "https://archestra.example",
  "api_version_constraint": "= 1.4.0-rc.55"
}
```

This works with cluster, namespaced, and ClusterProviderConfig credentials.

The range applies to version metadata, not license entitlements, permissions,
backend-specific feature flags or an incompatible custom build using the same
version label. Lifecycle CI remains necessary.
