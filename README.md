# Archestra Terraform Provider

Thank you for your interest in contributing to the Archestra Terraform Provider!

## Development

### Prerequisites

- [Go](https://golang.org/doc/install) >= 1.24
- [Terraform](https://www.terraform.io/downloads.html)
  - We recommend using [`tenv`](https://github.com/tofuutils/tenv) to manage your `terraform` installations
- Make (optional, for convenience)

### Local Development

To use a locally-built provider, you'll need to configure Terraform's development overrides. Create or edit `~/.terraformrc`:

```hcl
provider_installation {
  dev_overrides {
    "archestra-ai/archestra" = "<your-path-to-this-repo-on-your-machine>"
  }

  direct {}
}
```

Then you can run Terraform commands in the `examples/` directory.

### Building the Provider

```bash
make build
```

### Testing

Run the provider tests:

```bash
make test
```

### Codegen

#### Terraform docs

```bash
make generate
```

#### Archestra API Client

Ensure that you have the Archestra platform running locally (and the backend's openapi spec is consumable at <http://localhost:9000/openapi.json>)

```bash
make codegen-api-client
```

### Code Style

Run the formatter before committing:

```bash
make lint  # requires golangci-lint installed (see https://golangci-lint.run/docs/welcome/install/)
make fmt
```

## Release Process

Releases are automated via GitHub Actions using [`release-please`](https://github.com/googleapis/release-please)
