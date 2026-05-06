---
page_title: "Working with Vault-backed Secrets (BYOS) - archestra Provider"
subcategory: ""
description: |-
  Configure the Archestra provider against a backend running in BYOS / READONLY_VAULT mode.
---

# Working with Vault-backed Secrets (BYOS mode)

The Archestra backend runs in one of two **secrets-storage modes**, and
the mode determines which forms of `archestra_llm_provider_api_key` (and
`archestra_mcp_server_installation` with `is_byos_vault = true`) are
accepted on the wire:

| Mode | `ARCHESTRA_SECRETS_MANAGER` | `api_key` accepted? | `vault_secret_path` accepted? |
| --- | --- | --- | --- |
| **DB** | unset (default in the public quickstart) or `DB` | yes — encrypted server-side | not used; backend has no Vault configured |
| **BYOS / READONLY_VAULT** | `READONLY_VAULT` | no — backend rejects with 400 | required |

If you submit the wrong form for the active mode, the API rejects with:

```text
Either apiKey or both vaultSecretPath and vaultSecretKey must be provided
```

— even for providers like Ollama that don't actually use the key. The
backend validates the **shape** before looking at the provider.

## Detecting which mode your backend is in

If you have shell access to the platform deployment:

```bash
# Kubernetes:
kubectl exec deploy/archestra-platform -- env | grep ARCHESTRA_SECRETS_MANAGER

# Docker:
docker exec archestra-platform env | grep ARCHESTRA_SECRETS_MANAGER
```

No output → DB mode. `ARCHESTRA_SECRETS_MANAGER=READONLY_VAULT` → BYOS.

If you don't have shell access (e.g., a managed Archestra instance run
by a platform team), ask the platform admin which mode is enabled and
which Vault path / key shape they expect — the rest of this guide
assumes you have those answers.

## Activating BYOS mode on your backend

The platform expects three env vars when BYOS is enabled:

| Variable | Purpose |
| --- | --- |
| `ARCHESTRA_SECRETS_MANAGER=READONLY_VAULT` | Switch to BYOS mode |
| `ARCHESTRA_HASHICORP_VAULT_ADDR` | Vault endpoint, e.g. `https://vault.your-company.example:8200` |
| `ARCHESTRA_HASHICORP_VAULT_TOKEN` | Static token the backend uses to read secrets |

The backend authenticates to Vault via **static token** in the bundled
local/CI scripts. Whether other auth methods (AppRole, Kubernetes auth,
etc.) are supported by the platform image isn't documented in this
provider repo — check your platform release notes. Whichever method you
use, the backend only needs **read** on the secret paths you reference;
it never writes to Vault.

## Seeding a secret

Vault's KV v2 engine is assumed (the path you give Terraform must
include the `data/` segment, even though `vault kv put` doesn't).

Operator side — seed the secret with the keys your Terraform references:

```bash
vault kv put secret/your-org/llm \
  openai_api_key=sk-... \
  anthropic_api_key=sk-ant-...
```

…and the path the resource references in HCL is
`secret/data/your-org/llm` (with the `data/` segment that KV v2 inserts
into the raw API path).

Each Vault entry can hold multiple keys; `vault_secret_key` on the
provider resource selects which one. The shape inside the secret is
whatever you want — `{api_key: "..."}`, `{anthropic_api_key: "..."}`,
etc. — the resource's `vault_secret_key` must match.

## Concrete BYOS example

```hcl
resource "archestra_llm_provider_api_key" "anthropic" {
  name              = "Production Anthropic"
  llm_provider      = "anthropic"
  vault_secret_path = "secret/data/your-org/llm"
  vault_secret_key  = "anthropic_api_key"
  scope             = "org"
}
```

The plaintext key never reaches Terraform state — the backend reads it
from Vault on demand at LLM-call time.

## Local development with Vault

If you're testing the BYOS path locally, the simplest setup is the
[`scripts/bootstrap-local-stack.sh`](https://github.com/archestra-ai/terraform-provider-archestra/blob/main/scripts/bootstrap-local-stack.sh)
helper bundled with this repo (intended for contributors but usable by
anyone running the provider against a Kind cluster). It deploys a
dev-mode Vault, seeds a test secret, and configures the backend
container with the env vars above. See the
[contributing guide](https://github.com/archestra-ai/terraform-provider-archestra/blob/main/CONTRIBUTING.md#acceptance-tests)
for the workflow. For your own infrastructure, mirror the env-var block from
[`scripts/k8s/vault-dev.yaml`](https://github.com/archestra-ai/terraform-provider-archestra/blob/main/scripts/k8s/vault-dev.yaml) and
[`.github/values-ci.yaml`](https://github.com/archestra-ai/terraform-provider-archestra/blob/main/.github/values-ci.yaml) onto your platform
deployment.
