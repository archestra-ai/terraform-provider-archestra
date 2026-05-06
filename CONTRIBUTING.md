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

Then `make install` and run `terraform plan` / `apply` from any HCL
directory. Note: `dev_overrides` skips `terraform init` — Terraform
prints a warning each run reminding you that override is in effect.
That warning is normal during local development; remove the override
before testing release behaviour.

## Building

```bash
make build      # build the provider binary
make install    # build and install into $GOPATH/bin for dev_overrides
```

## Testing

### Unit tests

```bash
make test
```

### Acceptance tests

Acceptance tests run against a real Archestra backend at
`http://localhost:9000` (or wherever `ARCHESTRA_BASE_URL` points). The
`scripts/` directory ships helper bootstrappers that mirror exactly
what CI runs.

**For the full suite, prefer the one-shot wrapper** documented under
[Local stack for full testacc](#local-stack-for-full-testacc) below —
it bundles every script into a single `bootstrap-local-stack.sh` call
plus an `eval` of its env output. The step-by-step path here is for
when you want to run a subset of the suite or understand what each
piece does.

**Minimum setup:**

```bash
export ARCHESTRA_BASE_URL="http://localhost:9000"
export ARCHESTRA_API_KEY=$(./scripts/bootstrap-api-key.sh)  # uses seeded admin@example.com / password by default
make testacc
```

**Full suite (BYOS / Vault / EMC subsets):**

```bash
export ARCHESTRA_TEST_IDP_ID=$(./scripts/bootstrap-test-idp.sh)  # OIDC IdP for the EMC test
./scripts/bootstrap-ollama-mock.sh                                # kind-cluster only
./scripts/bootstrap-vault.sh                                      # kind-cluster only
export ARCHESTRA_READONLY_VAULT_ENABLED=true
make testacc
```

The Vault and Ollama-mock scripts apply manifests under
[`scripts/k8s/`](scripts/k8s/) against your current `kubectl` context;
they assume the Helm chart is deployed in a Kind cluster (the same
shape CI uses). For local-only Docker setups, skip them and run the
non-BYOS subset.

Test gates worth knowing:

- `ARCHESTRA_READONLY_VAULT_ENABLED=true` — gates the vault-ref test suite (`TestAccMcpRegistryCatalogItemResourceWithVaultRefs` and all `TestAccLLMProviderApiKeyResource*`). Backend must run with `ARCHESTRA_SECRETS_MANAGER=READONLY_VAULT` + an enterprise license.
- `ARCHESTRA_TEST_IDP_ID=<uuid>` — gates `TestAccMcpRegistryCatalogItemResourceWithEnterpriseManagedConfig`. `bootstrap-test-idp.sh` provisions one and prints its UUID.

CI invokes the same `scripts/bootstrap-*.sh` helpers — see
[`.github/workflows/on-pull-request.yml`](.github/workflows/on-pull-request.yml).

## Codegen

### Terraform docs

```bash
make generate
```

Renders `examples/**/*.tf` into `docs/` via `tfplugindocs`. Run this
after any schema or example change.

### Archestra API client

The API client is generated from the platform's OpenAPI spec and pinned
to a specific platform version. To bump:

1. Run the platform locally at the desired version:

   ```bash
   docker run -p 9000:9000 -p 3000:3000 \
     -e ARCHESTRA_ENTERPRISE_LICENSE_ACTIVATED=true \
     -v archestra-postgres-data:/var/lib/postgresql/data \
     -v archestra-app-data:/app/data \
     archestra/platform:<version-tag>
   ```

   The `ARCHESTRA_ENTERPRISE_LICENSE_ACTIVATED=true` flag is required so
   the OpenAPI spec includes all routes and types. See the [platform quickstart](https://archestra.ai/docs/platform-quickstart).

2. Regenerate from the running platform's spec at <http://localhost:9000/openapi.json>:

   ```bash
   make codegen-api-client
   ```

3. Update `ARCHESTRA_VERSION` in `.github/workflows/on-pull-request.yml`
   so CI runs against the same backend version.

## Code style

- `gofmt -s -w` + `terraform fmt` (run via `make fmt`).
- `golangci-lint v2` (run via `make lint`).
- Comments only when WHY is non-obvious. Don't comment WHAT — well-named identifiers do that. Don't reference current-task or fix-history in comments — that belongs in the commit message and rots otherwise.
- Don't add error handling, fallbacks, or validation for scenarios that can't happen. Trust internal code and framework guarantees. Validate at system boundaries (user input, external APIs).

## Release process

Releases are automated via GitHub Actions using
[`release-please`](https://github.com/googleapis/release-please).
Conventional-commit messages drive version bumps and changelog
generation.

## Repo layout

```
internal/
  client/                  # generated from the Archestra OpenAPI spec — do NOT edit
  provider/
    provider.go            # registers all resources + data sources
    mergepatch.go          # JSON Merge Patch (RFC 7396) helper
    flatten.go             # API → state mapping helpers
    specdrift_test.go      # schema ↔ AttrSpec drift check
    apicoverage_test.go    # API ↔ schema coverage check
    resource_<name>.go     # one resource per file
    <name>_helpers.go      # split-out helpers for a SINGLE resource (AttrSpec, response mappers, etc.)
    <name>_shared.go       # ONLY when consumed by 2+ resource files (e.g. agent_shared.go feeds agent/llm_proxy/mcp_gateway)
examples/                  # HCL examples — `make generate` renders these into docs/
docs/                      # generated; do NOT edit by hand
scripts/                   # CI + local bootstrap (api-key, vault, IdP, full stack)
tools/oapi-patch/          # OpenAPI-spec normalizer run before oapi-codegen; see the file header in tools/oapi-patch/main.go for what it rewrites and the removal criterion.
```

## Architecture

The wire-shape strategy (merge-patch + per-resource `AttrSpec`), the
drift-check tests that gate every PR, and the schema-attr-vs-skip-list
convention are documented in [ARCHITECTURE.md](ARCHITECTURE.md). Read
that before adding or modifying a resource.

## Adding a new resource — checklist

For each new mutable resource:

- [ ] **Schema** — Define `<Name>ResourceModel` struct + `Schema()` method. Use `Sensitive: true` on every secret field. Use `Computed: true` on backend-derived values; `Optional + Computed` if the user can set or omit (with a backend default), `Required` for must-be-set.
- [ ] **AttrSpec** — Declare `<name>AttrSpec []AttrSpec` matching every Optional/Required schema attr to its wire JSONName. Mark sensitive children. Use `OmitOnNull: true` if the backend zod is `.optional()` rather than `.nullable()`. Use `Synthetic` for URL-path fields and HCL-only ergonomic groupings.
- [ ] **`AttrSpecs()` method** — `func (r *FooResource) AttrSpecs() []AttrSpec { return fooAttrSpec }`. Activates `TestSpecDrift` for the resource.
- [ ] **`APIShape()` + `KnownIntentionallySkipped()` methods** — Activates `TestApiCoverage`. Run `go test -run TestApiCoverage ./internal/provider/` after adding to triage every flagged wire field.
- [ ] **Create/Read/Update/Delete** — Use `MergePatch` for Create + Update (Create's prior is a typed-null; Update's prior is `req.State.Raw`). Read populates state from the API response (drift-honest). Delete calls the typed client method.
- [ ] **`ImportState`** — Pass through the resource ID; the framework will populate the rest via Read.
- [ ] **Register** — Add `New<Name>Resource` to the slice in [provider.go](internal/provider/provider.go) `Resources()`.
- [ ] **Acceptance tests** — In `<resource>_test.go`. Cover Create, Update, ImportState, and the resource's edge cases. For BYOS / EE / EMC paths, use `testAccRequireByosEnabled(t)` so missing setup fails loudly under `make testacc` rather than silently skipping.
- [ ] **Examples** — Drop a minimal HCL example in `examples/resources/archestra_<name>/resource.tf`. Reference it from your `Schema()` MarkdownDescription if helpful.
- [ ] **`make generate`** — Regenerates docs from schema + examples.
- [ ] **Verify all 4 test gates green** — `make test` (unit + drift checks), `make testacc` (against the local stack), `go vet ./...`, `make lint`.

## Renaming a resource (breaking change)

Order of operations to avoid double-renames:

1. **PascalCase** Go identifiers first (`FooResource` → `BarResource`). Catches struct, constructor, receiver, and any `MapFooResponse`-style helpers.
2. **camelCase** Go identifiers (`fooAttrSpec` → `barAttrSpec`, `fooApiBody` → `barApiBody`).
3. **snake_case** wire/HCL refs (`archestra_foo` → `archestra_bar`, `_foo` TypeName concat).
4. **English-language strings** (descriptions, error messages).
5. `git mv` files only after content edits — keeps git's rename detection happy.

After:

- Update [provider.go](internal/provider/provider.go) registration.
- Update cross-references in other resources' MarkdownDescriptions.
- Add a `## Unreleased` section to [CHANGELOG.md](CHANGELOG.md) with a `### ⚠ BREAKING CHANGES` block. Include the `terraform state mv archestra_foo.<n> archestra_bar.<n>` migration step.
- Run `make generate` to regenerate docs at the new name.
- `go test ./...`, `make testacc` to verify.

The provider takes a clean-break stance: no deprecation aliases. Precedent: `profile → agent` (`ba24561`), `sso_provider → identity_provider` and `chat_llm_provider_api_key → llm_provider_api_key` (`536ec7f`).

## Local stack for full testacc

```bash
scripts/bootstrap-local-stack.sh                      # ~90s, idempotent
eval "$(scripts/bootstrap-local-stack.sh --print-env)"
make testacc                                          # 75/75 PASS
scripts/bootstrap-local-stack.sh --down               # tear down
```

Brings up the platform image + dev Vault + Ollama mock + EE license + provisioned IdP. `ARCHESTRA_VERSION` is pulled from `.github/workflows/on-pull-request.yml` so local tracks CI without re-hardcoding.

## Manual smoke testing with `examples/complete/`

For PR reviewers and contributors who want to *see* a change in action
without writing their own Terraform module, [examples/complete/](examples/complete/)
ships a runnable root module that wires the full bring-up chain. Apply
it against the local stack to verify behavior end-to-end:

```bash
scripts/bootstrap-local-stack.sh
eval "$(scripts/bootstrap-local-stack.sh --print-env)"

cd examples/complete
cp terraform.tfvars.example terraform.tfvars         # fill in real keys / IdP IDs as needed
terraform init
terraform apply

# poke around at $ARCHESTRA_BASE_URL in a browser, exercise the resources

terraform destroy
cd -
scripts/bootstrap-local-stack.sh --down
```

Use this for "does my schema change still apply cleanly against a real
backend?" and "does the user-visible flow still feel right?" The
acceptance tests verify resource correctness in isolation; the demo
verifies the connected story.

Two CI implications worth knowing:

- A failing `make generate` after touching schemas usually surfaces in
  the demo first — `terraform validate` against `examples/complete/` is the
  cheapest fast-feedback check (no backend required).
- Schema renames or breaking attribute changes will break the demo.
  Update it in the same PR; reviewers shouldn't have to fix the demo
  themselves to test your change.
