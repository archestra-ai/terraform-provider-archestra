Review the following pull request.

$ARGUMENTS

## Focus areas

1. **Code Quality** — Clean code, error handling, edge cases, readability. Check that all Terraform Plugin Framework patterns are followed (CRUD methods, Configure, Schema, Model structs with `tfsdk` tags).
2. **API Compatibility** — Verify field names match the OpenAPI spec at `localhost:9000/openapi.json`. Check that request body fields, response mappings, and enum values are correct. Ensure nullable vs non-nullable fields are handled properly (nil-check for nullable, direct assignment for NOT NULL).
3. **Security** — Sensitive fields marked `Sensitive: true` (API keys, secrets, private keys). No secrets logged in error messages. Auth header handling correct.
4. **Performance** — Duplicate API calls in CRUD paths, pagination for list endpoints (use `Pagination.HasNext` loop, not hardcoded limit), retry logic using `RetryUntilFound` from `retry.go` (not hand-rolled polling).
5. **Testing** — Acceptance tests for all resources/data sources. Tests use real backend at `localhost:9000` with `TF_ACC=1`. Tests should use `data.archestra_tool` for built-in tool lookups (not `archestra__whoami` auto-assignment). Check that `Computed` fields with backend defaults don't cause plan inconsistency.
6. **Project conventions** — Check CLAUDE.md for coding standards. Generated client (`internal/client/archestra_client.go`) must NOT be manually edited. Disabled resources use `//go:build ignore` tags. The `profile_tool_id` attribute in policy resources maps to the tool ID (not the assignment ID).
7. **Documentation** — Docs are auto-generated via `make generate` from examples in `examples/`. Verify `make generate` was run after schema changes. Check that `docs/resources/` and `docs/data-sources/` reflect current schemas. For disabled resources, verify TODO comments explain what replaced the API (not just "removed from spec").

## Provider-specific checks

- **OpenAPI patching** — If `oapi-config.yaml` or `scripts/patch_openapi.py` changed, verify `make codegen-api-client` still works.
- **Org settings defaults** — Verify defaults match backend DB schema (`font` → `lato`, `theme` → `cosmic-night`, `convert_tool_results_to_toon` → `true`, `compression_scope` → `organization`).
- **SSO provider** — OIDC bool fields (`override_user_info`, `skip_discovery`, `enable_rp_initiated_logout`) must initialize to `types.BoolValue(false)` when API returns nil (not Go zero value).
- **Profile fields** — NOT NULL fields with defaults (`incoming_email_enabled`, `incoming_email_security_mode`, `consider_context_untrusted`, `is_default`, `agent_type`) must be `Computed: true` to avoid plan inconsistency.

## Guidelines

- Be concise. No filler praise.
- Use inline review comments for specific issues.
- If the PR is clean, approve with a one-line summary.
- Flag blocking issues as REQUEST_CHANGES. Use COMMENT for suggestions.
- Missing documentation for user-facing changes is a blocking issue (REQUEST_CHANGES).
- Group related issues.
- Exclude `internal/client/archestra_client.go` (auto-generated) and `go.sum` from review.
