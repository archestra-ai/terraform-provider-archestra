---
page_title: "Common Issues - archestra Provider"
subcategory: ""
description: |-
  Current resource lifecycle and migration guidance.
---

# Common issues

Supply credential values through sensitive inputs.
Imported resources cannot recover one-shot or redacted plaintext secrets.

Use each resource's documented UUID, credential key or composite import ID.
Tool assignment identifiers and underlying tool identifiers represent different objects.
An organization-scoped lookup can return 404 for inaccessible objects.

A permission policy revision conflict requires refresh and review of the current
grants. Do not overwrite concurrent changes with blind retries.

API keys support create/read/delete. Name or lifetime changes require replacement;
Crossplane refuses the replacement plan. Create a separate key and migrate its
consumers before deleting the original. Remove runtime credential bindings in one
apply before removing the credential definition in a subsequent apply.

Current Archestra rejects legacy sharing inputs and retired policy endpoints.
The [v3 migration guide](https://github.com/archestra-ai/terraform-provider-archestra/blob/main/MIGRATION-v3.md)
explains native state upgrades and safe orphan/adopt procedures.
