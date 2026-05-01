# Architecture

How this provider is built. Read [CONTRIBUTING.md](CONTRIBUTING.md) first for
setup and contribution workflow; this document is the *why* behind the design
choices the contributing guide tells you to follow.

## Merge-patch + AttrSpec

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
