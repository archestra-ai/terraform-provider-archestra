# Archestra Provider

[![Crossplane coverage](https://img.shields.io/endpoint?url=https://raw.githubusercontent.com/archestra-ai/terraform-provider-archestra/main/.github/badges/crossplane-coverage.json)](.github/workflows/crossplane-coverage.yml)

Source for two artifacts, both regenerated from a single Terraform
schema and shipped together on every `vX.Y.Z` tag:

- `archestra-ai/archestra` on the [Terraform Registry](https://registry.terraform.io/providers/archestra-ai/archestra/latest) — end-user install path.
- `xpkg.upbound.io/archestra/provider-archestra` on Upbound — same resources, packaged for Crossplane v1/v2.

End users install from those — this README is for contributors
adding or extending resources.

## Adding a Terraform resource

Source lives in [`internal/provider/`](internal/provider/) — one
`resource_<name>.go` per resource, registered in `provider.go`'s
`Resources()` function. The dev loop:

1. Add `internal/provider/resource_<name>.go` + register it in `provider.go`.
2. Add an acceptance test (`resource_<name>_test.go`) and an example under `examples/resources/archestra_<name>/`. Every bug fix needs a regression test too — see [`CLAUDE.md`](CLAUDE.md).
3. `make generate` — regenerates `docs/` from schema + examples via tfplugindocs.
4. `make test` (unit + drift checks) → `make testacc` (against `$ARCHESTRA_BASE_URL`).

Architecture (merge-patch + AttrSpec design, the two drift-check
tests, the resource opt-in interfaces) and the full new-resource
checklist live in
[`ARCHITECTURE.md`](ARCHITECTURE.md) and
[`CONTRIBUTING.md`](CONTRIBUTING.md). Read these before touching
the schema — skipping them leads to phantom plan diffs and silent
backend-drift bugs.

## Adding a Crossplane mapping

The Crossplane subtree is a pure code-gen target — generated Go
and CRDs are gitignored. To expose an existing TF resource as a
Crossplane MR:

1. Whitelist it in `ExternalNameConfigs` in [`crossplane/config/external_name.go`](crossplane/config/external_name.go).
2. Add a configurator block in **both** `crossplane/config/cluster/<group>/config.go` and `.../namespaced/<group>/config.go` (Crossplane v1 + v2 scopes).
3. `make -C crossplane generate` — runs upjet over the TF schema to (re)produce `apis/**/zz_*.go`, `internal/controller/**/zz_*.go`, and `package/crds/*.yaml`.

Local setup, the add-a-resource walkthrough, and `make e2e` are
in [`crossplane/README.md`](crossplane/README.md). The badge above
links to the live coverage gap between this list and the TF
resource set.

## Release process

```
conventional commit on main
        │
        ▼
release-please opens "chore(main): release vX.Y.Z"
        │  merge
        ▼
tag vX.Y.Z   ──┬──▶  goreleaser   ──▶  Terraform Registry
               │
               └──▶  make generate + crank xpkg push  ──▶  xpkg.upbound.io
```
