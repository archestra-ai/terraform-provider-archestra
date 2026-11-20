---
page_title: "Authentication - archestra Provider"
subcategory: ""
description: |-
  Credential precedence, API-key format, key-rotation procedure, and workload identity.
---

# Authentication

The provider needs two values to talk to your backend: a **base URL**
and a credential. The credential is an **API key**, or a **workload
identity**: an OIDC token from the workload's own identity provider that
the provider exchanges for a short-lived service account token. Every
value can come from the provider block, environment variables, or a
default.

## Precedence

For each value, the provider takes the first non-empty source in this
order:

| Value | 1st | 2nd | 3rd |
|---|---|---|---|
| `base_url` | `provider` block `base_url` | `ARCHESTRA_BASE_URL` env | `http://localhost:9000` |
| `api_key` | `provider` block `api_key` | `ARCHESTRA_API_KEY` env | error — apply fails, unless workload identity is set |
| `service_account_id` | `provider` block `service_account_id` | `ARCHESTRA_SERVICE_ACCOUNT_ID` env | unset — API key auth |
| `oidc_token` | `provider` block `oidc_token` | `ARCHESTRA_OIDC_TOKEN` env | unset |
| `oidc_token_file` | `provider` block `oidc_token_file` | `ARCHESTRA_OIDC_TOKEN_FILE` env | unset |

Use an API key or workload identity, not both: the provider refuses a
configuration that resolves both an API key and a service account ID.

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

## Workload identity

With workload identity, a CI job or Kubernetes workload holds no API key.
The provider exchanges the workload's OIDC token at
`/api/workload-identity/token` for a token that acts as an Archestra
service account, and exchanges again before that token expires. Requires
an Enterprise license.

1. Create a service account and give it the roles the configuration
   needs.
2. Add a federated credential to it whose issuer, audience and subject
   match the workload's token exactly. Use the
   `archestra_service_account_federated_credential` resource, the UI, or
   the Helm chart's bootstrap.
3. Point the provider at the account and the token:

```hcl
provider "archestra" {
  # export ARCHESTRA_BASE_URL="https://archestra.your-company.example"
  service_account_id = "00000000-0000-0000-0000-000000000000"
  oidc_token_file    = "/var/run/secrets/archestra/serviceaccount/token"
}
```

`oidc_token_file` is read again on every exchange, so a rotated projected
Kubernetes token is picked up during a long apply. On Kubernetes, project
a token whose audience matches the federated credential:

```yaml
volumes:
  - name: archestra-token
    projected:
      sources:
        - serviceAccountToken:
            path: token
            audience: https://archestra.your-company.example
            expirationSeconds: 3600
```

The token's subject is `system:serviceaccount:<namespace>:<service-account>`.
The cluster's issuer must be a public HTTPS URL that serves an OIDC
discovery document.

In GitHub Actions, request the token and pass it with `ARCHESTRA_OIDC_TOKEN`:

```yaml
permissions:
  id-token: write
steps:
  - run: |
      echo "ARCHESTRA_OIDC_TOKEN=$(curl -sS -H "Authorization: Bearer $ACTIONS_ID_TOKEN_REQUEST_TOKEN" \
        "$ACTIONS_ID_TOKEN_REQUEST_URL&audience=https://archestra.your-company.example" | jq -r .value)" >> "$GITHUB_ENV"
  - run: terraform apply -auto-approve
    env:
      ARCHESTRA_BASE_URL: https://archestra.your-company.example
      ARCHESTRA_SERVICE_ACCOUNT_ID: 00000000-0000-0000-0000-000000000000
```

A token passed with `oidc_token` cannot be refreshed: keep applies shorter
than its lifetime, or use `oidc_token_file`.

## Choosing the auth surface

| Use case | Recommended source |
|---|---|
| Local development | `ARCHESTRA_API_KEY` in your shell rc; `base_url` defaults to `http://localhost:9000` |
| CI / CD with an OIDC issuer (GitHub Actions, GitLab CI) | workload identity: `ARCHESTRA_SERVICE_ACCOUNT_ID` + `ARCHESTRA_OIDC_TOKEN` |
| Kubernetes job or controller | workload identity: `ARCHESTRA_SERVICE_ACCOUNT_ID` + `ARCHESTRA_OIDC_TOKEN_FILE` on a projected token |
| Other CI / CD | `ARCHESTRA_BASE_URL` + `ARCHESTRA_API_KEY` from secret store, injected as env vars |
| Multi-environment module | inline `provider` block with `var.api_key` per environment workspace |
| Terraform Cloud | environment variables marked Sensitive on the workspace |
