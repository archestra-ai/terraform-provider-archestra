---
name: user-facing-text
description: Use when writing or editing any text a provider user reads - schema MarkdownDescription strings, examples/, templates/ (guides and index), the generated docs/, README.md and COMPATIBILITY.md. Keeps that text version-neutral, because the Terraform Registry already shows the docs of the provider version the reader selected.
---

# User-facing text

## Where the text lives

| Published as | Edit here |
|---|---|
| Resource and data-source reference (`docs/resources`, `docs/data-sources`) | `MarkdownDescription` in `internal/provider/*.go`, plus `examples/resources/*`, `examples/data-sources/*` |
| Guides and the provider index (`docs/guides`, `docs/index.md`) | `templates/guides/*.md.tmpl`, `templates/index.md.tmpl`, `examples/provider/` |
| Repository pages | `README.md`, `COMPATIBILITY.md` |

Never edit `docs/` by hand. Run `make generate` after changing a source above
and commit the regenerated `docs/`. `COMPATIBILITY.md` mirrors the body of
`templates/guides/api-compatibility.md.tmpl` (repository-relative links); change
both together.

## Describe this release, not its history

The Registry publishes a separate copy of the docs for every provider release,
and each release is tested against the Archestra release in
`platform-version.txt`. A reader who picks a provider version already sees text
for that version. So write what the provider does now, against its tested
Archestra release. Do not write:

- Archestra version gates: "Requires API 1.5.0-rc.29 or later", "(API X or
  later)", "needs X or later", "leave unset on older releases".
- Behavior of older releases: "Null on API releases before X", "older releases
  ignore it", "before X also set `SOME_FLAG=true`". If the tested release no
  longer needs a flag, drop the flag.
- Provider history: "new in v1.2.20", "Provider 3.0.4 does not contain this
  resource", "removed in v2".
- Upstream links used as version markers: "the API introduced in PR #1234".
- Test-evidence narratives in guides: dates, the rc a run used, image digests.
  Records go in `audit/` and are linked from `audit/README.md`.

Conditions that are not versions stay: "ALPHA", "Requires an Enterprise
license", "Requires auto routing to be enabled", "Requires OpenAPPA guardrails
to be enabled".

## Where version facts go instead

- Runtime diagnostics. An error that names the release a feature needs is
  shown only to someone on an older release, so it is useful there (see
  `identityBindingsMinimumAPIVersion` in `agent_identity_bindings.go`).
  `DeprecationMessage` is runtime output too.
- Acceptance-test skips (`testAccAPIVersionAtLeast`) and code comments.
- `CHANGELOG.md`, `MIGRATION-*.md` and the PR description.

## Versions that may appear

- The provider's API capability floor (`minimumAPIVersion`), stated once in the
  API compatibility guide and `COMPATIBILITY.md`.
- Syntax examples for `api_version_constraint`.
- A major-version provider constraint in examples, such as `version = "~> 3.0"`.
- Pinned third-party tools in contributor procedures, matching the scripts they
  describe (Terraform, Crossplane chart, Vault image).
- Data that happens to contain a version: model IDs, image tags, plugin
  manifests.

## Check before you finish

Run from the repository root; every hit must fit "Versions that may appear":

```shell
grep -rnE 'rc\.[0-9]+|or later|releases? before|older releases|new in v?[0-9]|PR #[0-9]+|[Pp]rovider [0-9]+\.[0-9]+' \
  docs templates examples README.md COMPATIBILITY.md \
  $(grep -rlE 'MarkdownDescription' internal/provider --include='*.go' | grep -v _test.go)
```

Matches in `.go` files outside `MarkdownDescription` strings (errors, comments)
are fine.
