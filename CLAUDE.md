# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

Terraform provider for Archestra, built on the Terraform Plugin Framework.

> **Adding or modifying a resource? Read [ARCHITECTURE.md](ARCHITECTURE.md) and [CONTRIBUTING.md](CONTRIBUTING.md) first.** ARCHITECTURE covers the merge-patch + AttrSpec design, the two drift-check tests (TestSpecDrift, TestApiCoverage) and the per-resource opt-in interfaces (`resourceWithAttrSpec`, `resourceWithAPIShape`), and the schema-attr-vs-skip-list convention. CONTRIBUTING covers the new-resource checklist plus build/test/codegen workflow. Skipping them leads to phantom plan diffs and silent backend-drift bugs.

## Development Commands

### Building and Installation

```bash
make build                    # Build the provider
make install                  # Build and install locally (creates binary in $GOPATH/bin)
go build -v ./...            # Direct build command
```

### Testing

#### Unit Tests

```bash
make test                     # Run unit tests (timeout=120s, parallel=10)
go test -v -cover -timeout=120s -parallel=10 ./...  # Direct test command
```

#### Acceptance Tests

Acceptance tests require environment configuration:

```bash
export ARCHESTRA_BASE_URL="http://localhost:9000"
export ARCHESTRA_API_KEY="your-api-key"
make testacc                  # Run acceptance tests against hosted environment
```

**One-shot local setup for the *full* suite (EE + BYOS + EMC):**

```bash
scripts/bootstrap-local-stack.sh                          # ~90 s, idempotent
eval "$(scripts/bootstrap-local-stack.sh --print-env)"
make testacc                                              # run full suite
scripts/bootstrap-local-stack.sh --down                   # tear down
```

The script runs the platform image + `hashicorp/vault:1.18` + an Ollama
mock (hashicorp/http-echo) on a shared docker network, with
`ARCHESTRA_ENTERPRISE_LICENSE_ACTIVATED=true` and
`ARCHESTRA_SECRETS_MANAGER=READONLY_VAULT`. It also seeds the BYOS test
secret and provisions the EMC IdP. The platform image tag tracks
`ARCHESTRA_VERSION` (default matches `.github/workflows/on-pull-request.yml`;
override the env var to test against a different release). Without the
script, the BYOS / EMC / LLM-model subsets `t.Fatal` with actionable
messages instead of running against an under-configured backend.

### Code Quality

```bash
make fmt                      # Format code with gofmt and terraform fmt
make lint                     # Run golangci-lint v2 (requires golangci-lint installed)
gofmt -s -w -e .             # Direct format command
```

### Code Generation

```bash
make generate                 # Generate Terraform documentation (uses tfplugindocs)
make codegen-api-client       # Regenerate API client from OpenAPI spec
```

The `codegen-api-client` command requires the Archestra backend running locally at `http://localhost:9000` and uses `oapi-codegen` to generate `internal/client/archestra_client.go` from the OpenAPI spec.

### Local Development with Terraform

To use a locally-built provider, configure development overrides in `~/.terraformrc`:

```hcl
provider_installation {
  dev_overrides {
    "archestra-ai/archestra" = "/path/to/your/terraform-provider-archestra"
  }

  direct {}
}
```

Then run `make install` and use Terraform commands in the `examples/` directory.

## Architecture

### Resources and Data Sources

One file per resource (`internal/provider/resource_*.go`) and data source (`datasource_*.go`); names map mechanically (`resource_team.go` → `archestra_team`). `ls internal/provider/` for the canonical list. Shared helpers live in `agent_shared.go` (the three agent-type resources) and `retry.go` (async-read polling).

### Testing

Acceptance-test setup is covered in [Development Commands → Acceptance Tests](#acceptance-tests). Some tests opt in via extra environment variables:

- `ARCHESTRA_READONLY_VAULT_ENABLED=true` — required by BYOS-vault-dependent tests (`TestAccMcpRegistryCatalogItemResourceWithVaultRefs` and all `TestAccLLMProviderApiKeyResource*`). The helper `testAccRequireByosEnabled` calls `t.Fatal` loudly instead of silently skipping, so a DB-mode backend fails fast with an actionable message. The backend must also run with `ARCHESTRA_SECRETS_MANAGER=READONLY_VAULT` plus an enterprise license.
- `ARCHESTRA_TEST_IDP_ID=<uuid>` — run `TestAccMcpRegistryCatalogItemResourceWithEnterpriseManagedConfig` against an existing identity provider; skipped otherwise because the EE IdP API is license-gated.

## Important Notes

- The API client is generated code - always regenerate rather than editing manually
- Provider uses API key authentication via `Authorization` header (format: `arch_...`)
- Documentation is auto-generated from examples using tfplugindocs tool - run `make generate` after making changes to ensure docs are updated
- Backend models all four agent variants (`agent`, `llm_proxy`, `mcp_gateway`, plus the legacy `profile` type) on a single `agents` table with an `agentType` discriminator. The provider exposes them as three separate resources (`archestra_agent`, `archestra_llm_proxy`, `archestra_mcp_gateway`) for 1:1 parity with the UI; the legacy `profile` type is not surfaced.
- The `tool_id` attribute on `archestra_tool_invocation_policy` and `archestra_trusted_data_policy` is the **bare tool UUID** (matching the `tools` table — the backend stores `toolId` directly). It is NOT the agent-tool assignment composite ID. Preferred lookup: `archestra_mcp_server_installation.<n>.tool_id_by_name["<server>__<short>"]` — one line, no extra data source.
- Conditional vs bulk-default policy resources hit different backend endpoints — `archestra_tool_invocation_policy` (with non-empty `conditions`) maps to the UI's "Add Policy" button; `archestra_tool_invocation_policy_default` (with `tool_ids` + `action`) maps to the UI's `DEFAULT` row. Same split for `archestra_trusted_data_policy` ↔ `_default` and for `archestra_agent_tool` ↔ `archestra_agent_tool_batch`. Don't fake a default with a wildcard regex — use the `_default` resource.
- `MarkdownDescription` on every schema attribute is one short sentence stating what the field is (matches AWS/GCP/Datadog convention) — multi-paragraph rules, tutorials, or how-to guidance belong in the example file's comments, not the schema reference table.
- Every bug fix needs an acceptance test that would have caught it — coverage gap = fix gap, the regression test is part of the fix, not a follow-up.
- Tests gated on optional env vars must `t.Fatal` with an actionable message (helper `testAccRequireByosEnabled` is the reference pattern), never `t.Skip` — a silently-skipped test makes a misconfigured backend look like a green run.
- When a bug's root cause might be in the backend, read it. Many provider behaviors are constrained by platform semantics not visible in the OpenAPI spec (TOON scope must be `team` for the team flag to be honored; bulk-upsert never deletes orphans; slugified tool-name prefix is `catalog_item.name` for local installs and `install.name` for remote) — these are only discoverable in the platform repo source.
- Keep code comments tight. Explain WHY when non-obvious (hidden constraint, upstream-bug workaround), not WHAT — the code already shows that. Multi-paragraph rationale belongs in ARCHITECTURE.md or the PR description, not inline.
