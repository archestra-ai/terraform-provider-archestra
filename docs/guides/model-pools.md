---
page_title: "Model pools - archestra Provider"
subcategory: ""
description: |-
  Configure model pools through the existing Archestra API using Terraform or Crossplane.
---

# Model pools

Terraform configures the same model pools as the Archestra UI. Archestra stores
members and performs routing, weighting and failover; the provider manages their
configuration through the existing API. Crossplane is an alternative for
Kubernetes users, not a prerequisite for Terraform.

## Choose compatible releases

Use a published provider release whose resource reference includes
`archestra_model_pool`. Provider 3.0.4 does not contain this resource. Select that
release in `required_providers.archestra.version` according to your version policy
and commit `.terraform.lock.hcl` to pin the exact installed provider and checksums.
Provider and application version numbers are independent.

The current API contract first shipped in **Archestra 1.5.0-rc.12**. Enable
`ARCHESTRA_LLM_MODEL_POOLS_ENABLED=true` on that deployment, or enable
`ARCHESTRA_BETA` without explicitly disabling model pools. Official rc.11 uses an
older grant-replacing contract that the provider refuses; stable 1.4.8 has no
model-pool API. Check [API compatibility](api-compatibility.md) before choosing a
later release rather than assuming all newer versions are compatible.

## Configure and apply

Supply your management credential through `ARCHESTRA_API_KEY` and the existing
Archestra API endpoint through `ARCHESTRA_BASE_URL`. Use shared provider-key UUIDs
from that organization; these are references to existing credentials, not their
secret values. Personal provider keys cannot be pooled.

```terraform
terraform {
  required_providers {
    archestra = {
      source = "archestra-ai/archestra"
    }
  }
}

provider "archestra" {
  api_version_constraint = "= 1.5.0-rc.12"
}

variable "shared_provider_key_id" {
  type = string
}

resource "archestra_model_pool" "balanced" {
  name = "balanced-model"
  members = [
    {
      provider_api_key_id = var.shared_provider_key_id
      model_id            = "primary-deployment"
      priority            = 1
      weight              = 3
    },
    {
      provider_api_key_id = var.shared_provider_key_id
      model_id            = "fallback-deployment"
      priority            = 2
      weight              = 1
    },
  ]
}
```

Replace deployment names and provide the UUID, then run `terraform init`, inspect
`terraform plan`, and apply the reviewed configuration with `terraform apply`.
For an upgrade, change the provider version constraint, run `terraform init
-upgrade`, review the lockfile and plan, then apply. Keep Terraform state secure;
it records resource identities and may contain secrets from other resources.

The member set is authoritative: removing an entry removes it from the pool.
Members default to enabled; use `enabled = false` for disabled fixtures. Lower
priorities run first; weights split selections within a priority. See the
[resource example](../resources/model_pool.md) for multiple provider keys.

To adopt a pool already created in the UI, write a matching configuration and
import its UUID **before applying**, then inspect the plan:

```shell
terraform import archestra_model_pool.balanced 11111111-1111-4111-8111-111111111111
terraform plan
```

## Give callers access

Creating a pool does not grant every credential access. Pool
`virtual_api_key_ids` is read-only. Virtual keys with a member provider-key
mapping already have access; additional pool grants belong in credential
settings/API. The Terraform `archestra_virtual_key` resource does not currently
expose explicit `model_pool_ids`, while `archestra_oauth_client` does. Archestra
agents can reference the pool with `archestra_agent.model_pool_id`.

An authorized inference client requests the pool's **name** as its model. The
management API key used by Terraform is separate from the caller's inference
credential. Terraform does not send inference traffic or choose the upstream.

## Crossplane alternative

Install the corresponding released
[Crossplane package](https://marketplace.upbound.io/providers/archestra/provider-archestra)
and pin its version or digest. Configure its Secret/ProviderConfig with the same
management API endpoint and version constraint; the endpoint must be reachable
from the provider pod. Apply a `llm` / `ModelPool` in the desired cluster or
namespaced scope, using `providerApiKeyIdRef` for `ModelProvider` references.

See the [installation guide](https://github.com/archestra-ai/terraform-provider-archestra/tree/main/crossplane)
and [examples](https://github.com/archestra-ai/terraform-provider-archestra/tree/main/crossplane/examples).
Check `Ready` and `Synced`, and inspect the persisted pool in Archestra. To adopt
an existing pool, set `crossplane.io/external-name` to its UUID before applying a
matching managed resource. Deletion normally removes the Archestra pool.
