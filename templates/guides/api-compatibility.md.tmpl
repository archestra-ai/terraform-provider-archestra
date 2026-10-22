---
page_title: "API Compatibility - archestra Provider"
---

# API compatibility

Terraform provider and Archestra API releases have independent version numbers.
Pin both in deployment configuration. Crossplane packages embed the matching
Terraform provider release and inherit the same API requirements.

| Provider | API capability floor | Exact API versions tested | Status |
|---|---|---|---|
| 3.0.0 | 1.4.0-rc.51 | The release in `platform-version.txt` at the release tag | Current |

The floor includes atomic delegation, the current service-account team ownership API, the App tool-assignment read contract and the organization setting for unauthenticated LLM proxy requests (introduced in 1.4.0-rc.51) used by v3. It is a minimum required
capability level, not evidence that every later release is compatible. Pull
request CI tests the floor and the newest stable release, not each release
between them. Prereleases can change contracts between release candidates;
record each tested release explicitly.

Two features need 1.5.0-rc.1 or later:

- `default_personal_agent_access_all_tools` on `archestra_organization_settings`.
  Leave it unset on older releases; they do not store it.
- Recovering the `value` of an `archestra_virtual_key` that a service account
  created, on import or refresh. Older releases return 403, so `value` stays
  null. Users can recover keys they created on every supported release.

`platform-version.txt` at the provider release tag names the tested release.
Acceptance CI runs the native suite against that release and records the image
digest it used in the job summary. The provider embeds the same file for the
runtime check below.

Pull request CI also runs the native and credential acceptance suites against
the oldest supported release (`1.4.0-rc.51`) and the newest stable Archestra
release. These checks are required, so a change that breaks a supported older
release cannot merge. The runtime check below still warns on those releases,
because it treats only the release in `platform-version.txt` as tested.

Crossplane reconciliation is not part of CI. The recorded evidence for
1.5.0-rc.3 (image
`sha256:ef2cc471d08b2af05e40cd211bc8b8292752723dff13b8d6c08530079e7167d6`)
covers all 31 resource families in both scopes on one Terraform provider and
controller binary pair: 138 lifecycle records with normal cleanup
([evidence](https://github.com/archestra-ai/terraform-provider-archestra/blob/main/audit/crossplane-release-1.5.0-rc.3.json)). The same run's native
acceptance ([evidence](https://github.com/archestra-ai/terraform-provider-archestra/blob/main/audit/native-acceptance-1.5.0-rc.3.json)) and runtime
package checks for linux/amd64 and linux/arm64
([evidence](https://github.com/archestra-ai/terraform-provider-archestra/blob/main/audit/runtime-package-validation-1.5.0-rc.3.json)) are linked, with
the earlier milestone records, from [the audit](https://github.com/archestra-ai/terraform-provider-archestra/blob/main/audit/README.md).

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
