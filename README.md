# Archestra Provider

[![Crossplane coverage](https://img.shields.io/endpoint?url=https://raw.githubusercontent.com/archestra-ai/terraform-provider-archestra/main/.github/badges/crossplane-coverage.json)](.github/workflows/crossplane-coverage.yml)

Manage Archestra agents, MCP servers, identity providers, teams,
LLM keys, and security policies. Two distributions are published
from the same `vX.Y.Z` tag — the Crossplane xpkg is
[upjet](https://github.com/crossplane/upjet)-codegen'd from this
provider's TF schema on every release, so the two stay locked.

## Terraform

Published to [registry.terraform.io/providers/archestra-ai/archestra](https://registry.terraform.io/providers/archestra-ai/archestra/latest).

```hcl
terraform {
  required_providers {
    archestra = {
      source  = "archestra-ai/archestra"
      version = "~> 1.0"
    }
  }
}

provider "archestra" {
  # ARCHESTRA_BASE_URL / ARCHESTRA_API_KEY are read from the env
}
```

```bash
export ARCHESTRA_BASE_URL="https://archestra.your-company.example"
export ARCHESTRA_API_KEY="arch_..."   # mint via Settings → API Keys
terraform init && terraform apply
```

Full walk-through:
[Getting Started](docs/guides/getting-started.md). Other guides
under [`docs/guides/`](docs/guides/), HCL snippets under
[`examples/`](examples/), schema reference in [`docs/`](docs/),
contributor setup in [`CONTRIBUTING.md`](CONTRIBUTING.md).

## Crossplane

Published to `xpkg.upbound.io/archestra/provider-archestra`. The
Crossplane subtree lives in [`crossplane/`](crossplane/) and is a
**pure code-gen target** — `apis/**/zz_*.go`,
`internal/controller/**/zz_*.go`, and `package/crds/*.yaml` are
gitignored. After a fresh clone, regenerate before anything else:

```bash
go build -o terraform-provider-archestra .   # repo root — upjet shells out to this binary
cd crossplane
make schema     # dumps the TF schema to config/schema.json
make generate   # runs upjet -> apis/, internal/controller/, package/crds/
make build      # _output/bin/<host>/provider
```

The TF binary at the repo root is a hard prerequisite — upjet
invokes it to extract the schema. Skipping `make generate`
leaves the Go packages empty and any `go build` fails with
"package not found".

Cluster install, ProviderConfig wiring, the five-step
add-a-resource checklist, and the `make e2e` target are in
[`crossplane/README.md`](crossplane/README.md).
