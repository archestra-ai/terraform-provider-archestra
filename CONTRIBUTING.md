# Contributing to terraform-provider-archestra

This guide is for contributors and reviewers — people modifying the
provider itself. If you're a *user* of the provider, see
[README.md](README.md) instead.

## Prerequisites

- [Go](https://golang.org/doc/install) >= 1.25
- [Terraform](https://www.terraform.io/downloads.html) — [`tenv`](https://github.com/tofuutils/tenv) recommended for managing versions
- `make` (for the helper targets in [Makefile](Makefile))
- `golangci-lint` v2 ([install](https://golangci-lint.run/docs/welcome/install/))

## Local development with `dev_overrides`

To point Terraform at a locally-built provider binary, configure
`~/.terraformrc`:

```hcl
provider_installation {
  dev_overrides {
    "archestra-ai/archestra" = "<absolute-path-to-this-repo>"
  }

  direct {}
}
```

Then run `make install` and use Terraform commands in the `examples/` directory.

### Code Quality

```bash
make fmt                      # Format code with gofmt and terraform fmt
make lint                     # Run golangci-lint v2 (requires golangci-lint installed)
gofmt -s -w -e .             # Direct format command
```
