---
page_title: "Authentication - archestra Provider"
subcategory: ""
description: |-
  Credential precedence, API-key format, and key-rotation procedure.
---

# Authentication

The provider needs two values to talk to your backend: a **base URL**
and an **API key**. Both can come from the provider block, environment
variables, or a default.

## Precedence

For each value, the provider takes the first non-empty source in this
order:

| Value | 1st | 2nd | 3rd |
|---|---|---|---|
| `base_url` | `provider` block `base_url` | `ARCHESTRA_BASE_URL` env | `http://localhost:9000` |
| `api_key` | `provider` block `api_key` | `ARCHESTRA_API_KEY` env | error — apply fails |

Inline HCL always wins; the env var is only consulted when the inline
value is empty or unset. If both are set, the env var is silently
ignored — useful when a parent module pins one and a deployer wants to
inspect plans against a different backend without editing HCL.

## API key format

API keys are minted in the Archestra UI under **Settings → API Keys**.
The token always starts with `arch_`. Treat it like a password:

- Don't commit it to source. The HCL field is marked `Sensitive`, so
  Terraform redacts it from plan/apply output, but it still lands in
  state and `.terraform.tfstate.backup`.
- Prefer the `ARCHESTRA_API_KEY` env var so secrets never enter HCL.
- For CI: mint a dedicated CI-only key with a finite expiry; rotate via
  the procedure below.

## Key rotation

Provider auth doesn't have a rolling-key feature — there's exactly one
key in use at apply time. Rotation is therefore a small dance to avoid
gaps:

1. **Mint a new key** in the UI (Settings → API Keys → New Key). Note
   the value.
2. **Stage the new key** alongside the old one — set
   `ARCHESTRA_API_KEY` (or the inline value) to the new key in your
   shell / CI runner. Don't revoke the old key yet.
3. **Run `terraform plan`** to confirm the provider authenticates with
   the new key. If the plan errors at provider configure, you've got
   the wrong key — fix before revoking.
4. **Apply once** with the new key so any in-flight changes commit
   under the new credential.
5. **Revoke the old key** in the UI. Subsequent applies use the new
   key only.

If the old key was leaked, skip step 1's "stage" sequence — revoke
immediately, then mint and configure the new key. A short outage on
in-flight applies is preferable to extended exposure.

## Choosing the auth surface

| Use case | Recommended source |
|---|---|
| Local development | `ARCHESTRA_API_KEY` in your shell rc; `base_url` defaults to `http://localhost:9000` |
| CI / CD | `ARCHESTRA_BASE_URL` + `ARCHESTRA_API_KEY` from secret store, injected as env vars |
| Multi-environment module | inline `provider` block with `var.api_key` per environment workspace |
| Terraform Cloud | environment variables marked Sensitive on the workspace |
