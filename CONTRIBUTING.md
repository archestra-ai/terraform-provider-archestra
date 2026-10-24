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

Pull requests always run lint, unit tests, generation, both-architecture builds
and Terraform/Crossplane coverage checks. Documentation and Marketplace asset
changes skip integration stacks; provider, dependency and harness changes run
the native acceptance matrix and Crossplane lifecycle suites. Release Please
PRs always run the full matrix. `scripts/ci_integration_scope.py` owns that
routing, with regression tests for runtime changes, renames and release PRs.

Native jobs cache the root and tools Go modules together. Crossplane jobs use
a separate root/Crossplane dependency key, keeping cache contents and
invalidation aligned with the modules each job builds. Fresh lifecycle stacks
pull the platform directly into Kind instead of pulling and transferring a
second copy from Docker. Lifecycle controllers poll at two seconds in CI to
exercise the same drift assertions with less idle waiting.

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

3. Write the same version to `platform-version.txt`. Acceptance CI,
   `scripts/bootstrap-local-stack.sh` and the provider's untested-version
   warning all read it.

For new platform releases, the Update Terraform Provider workflow in
archestra-ai/archestra does steps 2 and 3 and opens a pull request on the
`automation/platform-api` branch.

Pull request CI also runs both acceptance suites against the oldest supported
release (`minimumAPIVersion`) and the newest stable release, which
`scripts/supported-platform-versions.sh` selects. When a test uses an API
feature that those releases do not have, check `testAccAPIVersionAtLeast` and
leave that part out on older releases.

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
    <name>_shared.go       # ONLY when consumed by 2+ resource files (e.g. agent_shared.go feeds agent/mcp_gateway)
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
- Mark the change as breaking in its pull request title (`feat!:` or `fix!:`). Release Please writes [CHANGELOG.md](CHANGELOG.md) from merged titles and lists it under `⚠ BREAKING CHANGES`; do not edit the changelog by hand. For a type rename, implement and test the Plugin Framework state-move contract or provide a verified import/adoption procedure. Plain `terraform state mv` rejects a move between different resource types; do not publish that as a migration step. Test prior-version state, no-op plans, and destroy behavior before releasing.
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

Brings up the platform image + dev Vault + Ollama mock + EE license + provisioned IdP. `ARCHESTRA_VERSION` comes from `platform-version.txt` so local tracks CI without re-hardcoding.

When another stack is already running, isolate the test cluster and host ports:

```bash
export KIND_CLUSTER=archestra-provider-tests
export ARCHESTRA_TEST_BACKEND_PORT=19000
export ARCHESTRA_TEST_FRONTEND_PORT=13000
export ARCHESTRA_TEST_METRICS_PORT=19050
scripts/bootstrap-local-stack.sh
eval "$(scripts/bootstrap-local-stack.sh --print-env)"
# Run acceptance tests, then remove only this selected test cluster.
scripts/bootstrap-local-stack.sh --down
```

The bootstrap uses a private kubeconfig for its commands and child scripts, leaving
the user's current Kubernetes context unchanged. `ARCHESTRA_NAMESPACE` selects the
platform's namespace inside this test cluster (default `default`).

## Crossplane lifecycle probes

The Crossplane coverage workflow reports resource mappings and checks that the
README badge matches them. When provider registrations or Crossplane mappings
change, regenerate the badge and include it in the same pull request:

```bash
scripts/crossplane-coverage.sh --badge .github/badges/crossplane-coverage.json
scripts/crossplane-coverage.sh --check-badge .github/badges/crossplane-coverage.json
```

`scripts/bootstrap-crossplane.sh` adds Crossplane to the stack above for
both-scope reconciliation tests. Run it with the same `KIND_CLUSTER` and
`ARCHESTRA_TEST_*_PORT` values as the stack:

```bash
scripts/bootstrap-local-stack.sh
scripts/bootstrap-crossplane.sh                       # also needs go, goimports, make, helm
eval "$(scripts/bootstrap-local-stack.sh --print-env)"
W=~/.cache/archestra-provider-crossplane/archestra-ci-cluster
python3 scripts/integration/crossplane-lifecycle.py --cases apps \
  --api-url "$ARCHESTRA_BASE_URL" --key-file "$W/api-key" --readonly-vault \
  --llm-base-url "$ARCHESTRA_TEST_LLM_BASE_URL" \
  --mcp-url http://provider-mcp-mock.default.svc.cluster.local:19006/mcp \
  --kubeconfig "$W/kubeconfig" --provider-config default \
  --terraform-binary terraform-provider-archestra \
  --controller-binary crossplane/_output/bin/linux_amd64/provider \
  --evidence /tmp/crossplane-apps.json
scripts/bootstrap-crossplane.sh --down                # remove Crossplane, CRDs and fixtures
```

The script installs Crossplane 2.2.1 with Helm, builds the provider and both
generated scopes, applies their CRDs, and creates a credentials Secret from
`--print-env`. It names the cluster `ProviderConfig` and the namespaced
`ClusterProviderConfig` and `ProviderConfig` `default`. The controller runs on
the host with Terraform 1.5.7, the locally built provider in a filesystem
mirror and a 10-second drift poll; its log and private kubeconfig live in the
work directory printed at the end. Set `CROSSPLANE_SKIP_BUILD=1` to reuse
existing binaries.

[`scripts/k8s/provider-mcp-mock.yaml`](scripts/k8s/provider-mcp-mock.yaml)
runs [`scripts/integration/mcp-mock.py`](scripts/integration/mcp-mock.py) in the
cluster, so the backend pod can install it as a remote MCP server at
`http://provider-mcp-mock.default.svc.cluster.local:19006/mcp`. The bootstrap
creates its script ConfigMap and applies it; App lifecycle cases use it for
upstream tools.

## Validating the examples

`make validate-examples` builds the provider from the working tree and runs
`terraform init` and `terraform validate` on every example directory with a
development override, so a schema change that breaks an example fails the
"Linting and Tests" job. It needs network access for the other providers the
examples use.

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
