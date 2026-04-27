# Archestra Terraform Provider

Thank you for your interest in contributing to the Archestra Terraform Provider!

## Development

### Prerequisites

- [Go](https://golang.org/doc/install) >= 1.25
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

#### Unit tests

```bash
make test
```

#### Acceptance tests

Run against a local Archestra platform at `http://localhost:9000`. The repo
ships a set of helper scripts under [`scripts/`](scripts/) that mirror what
CI does, so local and CI share the exact same bootstrap path.

**Always-needed:**

```bash
export ARCHESTRA_BASE_URL="http://localhost:9000"

# Sign in and mint an API key (uses the seeded admin@example.com / password
# credentials by default; override via ARCHESTRA_ADMIN_EMAIL /
# ARCHESTRA_ADMIN_PASSWORD).
export ARCHESTRA_API_KEY=$(./scripts/bootstrap-api-key.sh)

make testacc
```

**For BYOS / Vault / EMC tests** (CI runs all of these):

```bash
# Throwaway OIDC IdP for the EMC acceptance test. Idempotent: reuses an
# existing IdP with the same providerId if present.
export ARCHESTRA_TEST_IDP_ID=$(./scripts/bootstrap-test-idp.sh)

# Kind-cluster only: deploy an Ollama mock so the backend's testProviderApiKey
# passes for Ollama BYOS keys without a real Ollama install.
./scripts/bootstrap-ollama-mock.sh

# Kind-cluster only: deploy dev-mode Vault and seed secret/data/test/ollama,
# then restart the Archestra backend so it picks up Vault on startup.
./scripts/bootstrap-vault.sh

export ARCHESTRA_READONLY_VAULT_ENABLED=true
make testacc
```

The Vault and Ollama-mock scripts use `kubectl` against your current context
and apply manifests from [`scripts/k8s/`](scripts/k8s/). They assume the
backend is deployed via the Archestra Helm chart in a Kind cluster (the same
shape CI uses); local-only Docker setups can skip them and run only the
non-BYOS test subset.

A few test gates worth knowing:

- `ARCHESTRA_READONLY_VAULT_ENABLED=true` — gates the vault-ref test suite (`TestAccMcpRegistryCatalogItemResourceWithVaultRefs` and all `TestAccChatLLMProviderApiKeyResource*`). Backend must run with `ARCHESTRA_SECRETS_MANAGER=READONLY_VAULT` + an enterprise license.
- `ARCHESTRA_TEST_IDP_ID=<uuid>` — gates `TestAccMcpRegistryCatalogItemResourceWithEnterpriseManagedConfig`. `bootstrap-test-idp.sh` provisions one and prints its UUID.

CI calls the same `scripts/bootstrap-*.sh` helpers — see [`.github/workflows/on-pull-request.yml`](.github/workflows/on-pull-request.yml).

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
     -e ARCHESTRA_ENTERPRISE_LICENSE_ACTIVATED=true \
     -v archestra-postgres-data:/var/lib/postgresql/data \
     -v archestra-app-data:/app/data \
     archestra/platform:<version-tag>
   ```

   **Note:** The `-e ARCHESTRA_ENTERPRISE_LICENSE_ACTIVATED=true` flag is required to ensure the OpenAPI spec includes all routes and types.

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
