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

The API client is generated from the Archestra platform's OpenAPI spec and is pinned to a specific version of Archestra. To regenerate/bump the client:

1. Run the Archestra platform locally at the desired version. The easiest way is using Docker:

   ```bash
   docker run -p 9000:9000 -p 3000:3000 \
     -v archestra-postgres-data:/var/lib/postgresql/data \
     -v archestra-app-data:/app/data \
     archestra/platform:<version-tag>
   ```

   See the [Archestra Platform Quickstart](https://archestra.ai/docs/platform-quickstart) for more details.

2. Generate the client from the running platform's OpenAPI spec (served at <http://localhost:9000/openapi.json>):

   ```bash
   make codegen-api-client
   ```

3. Update the `ARCHESTRA_VERSION` env var in `.github/workflows/on-pull-request.yml` to match the version you generated the client from. This ensures CI acceptance tests run against the same version.

### Code Style

Run the formatter before committing:

```bash
make lint  # requires golangci-lint installed (see https://golangci-lint.run/docs/welcome/install/)
make fmt
```

## Release Process

Releases are automated via GitHub Actions using [`release-please`](https://github.com/googleapis/release-please)
