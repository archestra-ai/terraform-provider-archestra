---
page_title: "API Compatibility - archestra Provider"
---

# API compatibility

Terraform provider and Archestra API releases have independent version numbers.
Pin both in deployment configuration. Crossplane packages embed the matching
Terraform provider release and inherit the same API requirements.

Each provider release is built and tested against the Archestra release named in
`platform-version.txt` at its release tag. The Registry shows the documentation
for the provider version you select, so these pages describe that provider
against its tested Archestra release. To learn what an older Archestra release
supports, read the documentation of the provider release that tested it.

The provider also accepts older Archestra releases down to its API capability
floor, `1.4.0-rc.51`. The floor is a minimum, not evidence that every later
release is compatible. On a release older than the tested one, resources and
attributes for features that release lacks fail, are rejected or are ignored:
leave them unset, or use the provider release that tested your Archestra
release.

Pull request CI runs the native and credential acceptance suites against the
tested release, the floor and the newest stable Archestra release. Acceptance
tests skip features an older release lacks. Crossplane lifecycle CI exercises
both scopes against the tested release with source-built providers and
controllers; it does not establish that a published xpkg installs and runs.
Recorded lifecycle evidence is listed in the
[provider audit](https://github.com/archestra-ai/terraform-provider-archestra/blob/main/audit/README.md).

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
as upstream releases arrive. When archestra-ai/archestra publishes a release,
its release pipeline starts the Terraform Provider Sync agent, which opens them
on the `automation/platform-sync` branch: it regenerates the client from the
release's spec, updates `platform-version.txt`, and changes the resources the
release's API changes touch, here and in the Crossplane provider. It never
merges. Merging after acceptance passes makes that release the tested one.

## Runtime compatibility check

Before configuring resource and data-source clients, the provider reads the
public `GET /health` endpoint. It rejects semantic versions below the API
capability floor. Versions other than the tested release
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
provider release has validated that interval. With an explicit constraint,
missing, unreadable or non-semantic server versions are errors because the
constraint cannot be verified. A user constraint cannot lower the provider's
minimum floor.

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
