---
page_title: "API Compatibility - archestra Provider"
---

# API compatibility

Terraform provider and Archestra API releases have independent version numbers.
Pin both in deployment configuration. Crossplane packages embed the matching
Terraform provider release and inherit the same API requirements.

| Provider | API capability floor | Exact API versions tested | Status |
|---|---|---|---|
| Proposed 3.0.0 | 1.4.0-rc.48 | 1.4.0-rc.48, pinned image digest below | Development candidate; unreleased |

The floor includes the atomic delegation API used by v3. It is a minimum required
capability level, not evidence that every later release is compatible. This
candidate has not established compatibility with all future 1.4.x versions or
with the eventual 1.4.0 stable release. Prereleases can change contracts between
release candidates; record each tested release explicitly.

The tested platform image digest is
`sha256:fdd17cf0e4361034c39d49d88ba367c980a576f7f7e93f438facb495c64bffb4`.
Acceptance CI pins that digest. Native acceptance, state migration, Crossplane
reconciliation and runtime package evidence are linked from [the audit](audit/README.md).
An exact digest makes that result reproducible even if an image tag is republished.

For Terraform, select an approved provider release with `required_providers` and
commit `.terraform.lock.hcl`. That constraint selects a provider version; Terraform
has no native constraint that selects or validates the remote Archestra server.
For Crossplane, pin an approved package version and its corresponding API version.

## Establishing a supported range

Publish a supported API interval only after compatibility CI passes against its
oldest supported release and newest released member. Both ends must exercise
native acceptance and both Crossplane scopes. Add a targeted regression when an
intermediate release changes a contract; passing the endpoints cannot prove every
intermediate prerelease. Advance this matrix through reviewed provider PRs as
upstream releases arrive. Daily upstream sync must open manually reviewed PRs and
must not silently widen support or merge updates.

## Runtime compatibility check

Before configuring resource and data-source clients, the provider reads the
public `GET /health` endpoint. It rejects semantic versions below `1.4.0-rc.48`,
which lack required v3 API capabilities. Versions outside the tested interval
`= 1.4.0-rc.48` produce a warning. Newer does not automatically mean compatible.
Custom versions or unreadable version metadata produce a warning by default.
No resource client is configured when compatibility validation produces an error.

Use `api_version_constraint` to enforce your deployment's approved version or
range independently of the Terraform provider constraint:

```hcl
provider "archestra" {
  # The current tested API release. Credentials stay in environment variables.
  api_version_constraint = "= 1.4.0-rc.48"
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
  "api_version_constraint": "= 1.4.0-rc.48"
}
```

This works with cluster, namespaced, and ClusterProviderConfig credentials.

The range applies to version metadata, not license entitlements, permissions,
backend-specific feature flags or an incompatible custom build using the same
version label. Lifecycle CI remains necessary.
