# Contributing to terraform-provider-archestra

This guide is for contributors and reviewers. For setup, building, and testing,
see [README.md](README.md). This document covers the **internal architecture**
and the **conventions a new resource needs to follow**.

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
    <name>_shared.go       # AttrSpec + opt-in interface methods (when split out)
examples/                  # HCL examples — `make generate` renders these into docs/
docs/                      # generated; do NOT edit by hand
scripts/                   # CI + local bootstrap (api-key, vault, IdP, full stack)
```

## Architecture: merge-patch + AttrSpec

Every mutable resource sends Update bodies via JSON Merge Patch (RFC 7396).
The merge-patch is computed from a plan-vs-prior-state diff using a per-resource
`AttrSpec` table. This pattern closes four bug classes simultaneously:

- Sensitive fields (passwords, secrets) never leave the user's machine unless
  changed.
- Empty collections (`labels = []`, `teams = []`) aren't sent on every Update,
  preventing wipes of out-of-band changes.
- JSONB columns can be field-level diffed (`RecursiveObject`) or replaced
  wholesale (`AtomicObject`) depending on backend storage semantics.
- Backend defaults (computed values like `id = "sub"` for OIDC mapping) don't
  get clobbered by re-sending the user-omitted field as null.

### The `AttrSpec` declaration

Each resource declares a `<resource>AttrSpec` slice describing every wire field
it manages. Example from [identity_provider_shared.go](internal/provider/identity_provider_shared.go):

```go
var identityProviderAttrSpec = []AttrSpec{
    {TFName: "provider_id",  JSONName: "providerId",  Kind: Scalar},
    {TFName: "domain",       JSONName: "domain",      Kind: Scalar},
    {
        TFName: "oidc_config", JSONName: "oidcConfig", Kind: AtomicObject, OmitOnNull: true,
        Children: []AttrSpec{
            {TFName: "client_id",     JSONName: "clientId",     Kind: Scalar},
            {TFName: "client_secret", JSONName: "clientSecret", Kind: Scalar, Sensitive: true},
            // …
        },
    },
}
```

Fields:

| Field        | Purpose                                                                                                                                                                                       |
|--------------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `TFName`     | snake_case Terraform attribute name (matches the schema's `tfsdk` tag).                                                                                                                       |
| `JSONName`   | camelCase wire field name (matches the backend's zod / OpenAPI spec).                                                                                                                         |
| `Kind`       | See "Kinds" below.                                                                                                                                                                            |
| `Sensitive`  | Mask in `LogPatch` debug output. Should match the schema's `Sensitive: true`.                                                                                                                 |
| `OmitOnNull` | When the plan value is null, omit the field instead of emitting `null`. Use when the backend zod is `.optional()` rather than `.nullable()` — sending null gets rejected with a 400.          |
| `Encoder`    | Post-encode value transform (function `func(any) any`). Used for polymorphic unions, JSON-string round-trips, paginated wrappers, anything that doesn't map cleanly through the generic walker. |
| `Children`   | Nested AttrSpec for `AtomicObject`/`RecursiveObject`/`List`/`Set`/`Map`-of-objects.                                                                                                           |

### Kinds

| Kind              | When to use                                                                                                                                                                                                                                                                                                                         |
|-------------------|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `Scalar`          | Strings, numbers, booleans.                                                                                                                                                                                                                                                                                                         |
| `List` / `Set`    | Ordered / unordered collections of scalars or objects (use `Children` for objects).                                                                                                                                                                                                                                                 |
| `Map`             | `Map<string, T>`. Use `Children` for `Map<string, object>`.                                                                                                                                                                                                                                                                         |
| `AtomicObject`    | A nested object stored as a single JSONB column (Drizzle `text(...)` serialized to JSON, or `jsonb`). Replaced wholesale on update — merge-patch emits the full nested object on any change.                                                                                                                                        |
| `RecursiveObject` | A nested object backed by multiple top-level columns where each sub-field can be patched independently. Rare. Use only when the backend explicitly supports field-level updates within the nested structure.                                                                                                                        |
| `Synthetic`       | A schema attribute that has no wire field — typical examples: URL-path values like `agent_id` / `tool_id` (RequiresReplace), Create-time-only lookup keys, or HCL-only ergonomic groupings (catalog_item's `remote_config` block decomposes into top-level `serverUrl` / `oauthConfig` on the wire). MergePatch skips Synthetic entries. |

### Update flow

1. `MergePatch(ctx, plan.Raw, prior.Raw, spec, &diags)` walks `plan` and `prior` in parallel, emitting only the changed fields per the spec.
2. The patch is `json.Marshal`'d.
3. Resource Update sends the patch via the generated client's `*WithBodyWithResponse` method.
4. `LogPatch` writes the patch to `tflog.Debug` with sensitive values masked.

### Receive flow (Read)

Reads are **drift-honest**: state is set from the API response, not preferred from prior state. This surfaces out-of-band changes as a plan diff so users can decide. The exception is **write-only fields** (e.g. `image_pull_secrets[].password`) where the backend strips the value on write and never echoes it back; those preserve from prior state.

For nested objects with separate Get/Create/Update generated response types, write a single mapping helper (`mapXxxResponse`) that takes a JSON-roundtrip type bridging the three. See [identity_provider_shared.go](internal/provider/identity_provider_shared.go) for the canonical example.

## Drift-check tests

Two unit tests enforce the alignment between schema, AttrSpec, and the API. They run as part of `make test` (no TF_ACC needed) and gate every PR.

### `TestSpecDrift` (schema ↔ AttrSpec)

Source: [specdrift_test.go](internal/provider/specdrift_test.go).

Asserts:

- Every Schema attribute (top-level) on a migrated resource has a matching `AttrSpec` entry, *unless* it's Computed-only.
- Every `AttrSpec` entry has a matching Schema attribute (catches typos and stale entries).
- Every `Sensitive: true` schema attribute is also `Sensitive: true` on its `AttrSpec` entry.

A resource opts in by implementing `resourceWithAttrSpec`:

```go
func (r *FooResource) AttrSpecs() []AttrSpec { return fooAttrSpec }
```

### `TestApiCoverage` (API ↔ schema)

Source: [apicoverage_test.go](internal/provider/apicoverage_test.go).

Asserts: every wire field returned by the resource's `Get` endpoint is covered by either:

1. a Schema attribute (snake-case roundtrip of the JSON name matches a `TFName`), OR
2. an `AttrSpec.JSONName` mapping (catches intentional renames like `customFont ↔ font`, `theme ↔ color_theme`), OR
3. a `KnownIntentionallySkipped()` entry on the resource with a justification comment.

Catches the silent-drift class of bug: backend adds a field, provider keeps shipping without exposing it, users get no plan-time signal and can't read the field via Terraform.

A resource opts in by implementing `resourceWithAPIShape`:

```go
func (r *FooResource) APIShape() any                       { return client.GetFooResponse{} }
func (r *FooResource) KnownIntentionallySkipped() []string { return []string{"createdAt", "updatedAt"} }
```

The walker handles three response shapes:

- `JSON200 *struct{...}` (single record).
- `JSON200 *[]struct{...}` (list endpoint).
- `JSON200 *struct{Data []struct{...}, Pagination ...}` (paginated envelope).

## Convention: schema attr vs. skip-list

When `TestApiCoverage` flags a wire field, you have two choices:

**Add a Computed-only schema attribute** — preferred when the field has user value (drift visibility, debug visibility, or terraform-output usefulness). Example: `created_at`, `slug`, `secret_id`.

**Add to `KnownIntentionallySkipped()`** with a Why-comment — preferred when the field belongs to a different conceptual domain or is duplicated elsewhere. Acceptable categories:

- Audit timestamps you've decided not to surface yet (`createdAt`, `updatedAt`).
- Discriminators the provider uses internally (e.g. `agentType` to split one backend table into three resources).
- Backend bookkeeping (`limit.lastCleanup` cleanup-scheduler state).
- Synthetic-wrapper decomposition (e.g. catalog_item's `oauthConfig` is wire top-level but HCL-nested under `remote_config`).
- m2m-relationship duplicates (e.g. `agent.tools` is managed by `archestra_agent_tool`; surfacing it on the agent would create phantom diffs).
- TFName↔JSONName renames where the AttrSpec doesn't include it (because the field is Computed-only with no merge-patch involvement).

The skip-list comment matters. A future maintainer reading it should be able to decide whether the original reasoning still holds.

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

## Code style

- `gofmt -s -w` + `terraform fmt` (run via `make fmt`).
- `golangci-lint v2` (run via `make lint`).
- Comments only when WHY is non-obvious. Don't comment WHAT — well-named identifiers do that. Don't reference current-task or fix-history in comments — that belongs in the commit message and rots otherwise.
- Don't add error handling, fallbacks, or validation for scenarios that can't happen. Trust internal code and framework guarantees. Validate at system boundaries (user input, external APIs).

## Where decisions live

Project-wide conventions (merge-patch as the wire-shape strategy, drift-honest reads, hand-curated AttrSpec, no deprecation aliases, etc.) are decided once and applied across resources. If you're considering deviating, raise it in the PR.
