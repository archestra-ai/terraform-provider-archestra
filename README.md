# Archestra Provider

[![Crossplane coverage](https://img.shields.io/endpoint?url=https://raw.githubusercontent.com/archestra-ai/terraform-provider-archestra/main/.github/badges/crossplane-coverage.json)](.github/workflows/crossplane-coverage.yml)

Source for two artifacts, both regenerated from a single Terraform
schema and shipped together on every `vX.Y.Z` tag:

- `archestra-ai/archestra` on the [Terraform Registry](https://registry.terraform.io/providers/archestra-ai/archestra/latest)
- `xpkg.upbound.io/archestra/provider-archestra` on [Upbound](https://marketplace.upbound.io)

End users install from those. This README walks a new contributor
through extending the provider.

## 1. Build it

```bash
git clone https://github.com/archestra-ai/terraform-provider-archestra
cd terraform-provider-archestra
make install   # compiles + installs to $GOPATH/bin
```

## 2. Point Terraform at your local build

Create `~/.terraformrc`:

```hcl
provider_installation {
  dev_overrides {
    "archestra-ai/archestra" = "/path/to/your/$GOPATH/bin"
  }
  direct {}
}
```

Now any `terraform apply` resolves `archestra-ai/archestra` to your
local binary instead of the published Registry version.

## 3. Smoke-test against a local Archestra

```bash
export ARCHESTRA_BASE_URL=http://localhost:9000
export ARCHESTRA_API_KEY=arch_...      # mint via Settings → API Keys
cd examples/basic && terraform apply
```

If this works, your dev_overrides wiring is right. If it doesn't,
fix that before touching any code.

## 4. Add or modify a Terraform resource

Each resource lives in `internal/provider/resource_<name>.go` and is
registered in `provider.go`'s `Resources()` function. The loop:

1. Implement the resource (or change the existing one) + register it.
2. Add an example under `examples/resources/archestra_<name>/`.
3. Add an acceptance test (`resource_<name>_test.go`) — every bug fix needs a regression test too.
4. `make generate` — refreshes `docs/` from your example via tfplugindocs.
5. `make test && make testacc` — unit + drift checks, then acceptance against `$ARCHESTRA_BASE_URL`.

Architecture (merge-patch + AttrSpec, the two drift-check tests)
and the full new-resource checklist are in
[`ARCHITECTURE.md`](ARCHITECTURE.md) and
[`CONTRIBUTING.md`](CONTRIBUTING.md). Read them before touching the
schema — skipping leads to phantom plan diffs and silent backend drift.

## 5. Expose it as a Crossplane MR (optional)

If the resource should also be reachable from Crossplane:

1. Whitelist the TF resource name in `crossplane/config/external_name.go`.
2. Add a configurator block in **both** `crossplane/config/cluster/<group>/config.go` and `.../namespaced/<group>/config.go` (v1 and v2 scopes).
3. `make -C crossplane generate` — upjet regenerates apis, controllers, and CRDs.

Local controller run, `make e2e`, and the full walkthrough are in
[`crossplane/README.md`](crossplane/README.md).

## 6. Ship it

Open a PR with a conventional-commit title (`feat:`, `fix:`, etc.).
After it merges:

```
commit on main
        │
        ▼
release-please opens "chore(main): release vX.Y.Z"
        │  merge
        ▼
tag vX.Y.Z   ──┬──▶  goreleaser    ──▶  Terraform Registry
               │
               └──▶  crank xpkg push ──▶  xpkg.upbound.io
```

The two publish jobs run in parallel; one failing won't roll back
the other.
