# Archestra Terraform Provider

Thank you for your interest in contributing to the Archestra Terraform Provider!

## Development

### Prerequisites

- [Go](https://golang.org/doc/install) >= 1.24
- [Terraform](https://www.terraform.io/downloads.html) >= 1.0
- Make (optional, for convenience)

### Local Development

To use a locally-built provider, you'll need to configure Terraform's development overrides. Create or edit `~/.terraformrc`:

```hcl
provider_installation {
  dev_overrides {
    "archestra-ai/archestra" = "/path/to/your/terraform-provider-archestra"
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

### Generating API Client + docs

The API client, and docs, can be codegen'd:

```bash
make generate
```

### Code Style

Run the formatter before committing:

```bash
make fmt
```

## Release Process

Releases are automated via GitHub Actions using [`release-please`](https://github.com/googleapis/release-please)
