# Migration Guide: Agent to Profile Renaming

In version 1.0.0 (example version), the Archestra Terraform Provider introduces a renaming of `archestra_agent` resources to `archestra_profile`. This change aligns the provider with the terminology used in the Archestra product.

To ensure a smooth transition, the old resource names (`archestra_agent` and `archestra_agent_tool`) are preserved as **deprecated** aliases. Existing configurations will continue to work but will emit deprecation warnings.

We basically recommend migrating your configurations to use the new resource names. This guide outlines the steps to perform this migration without destroying and recreating your resources.

## Affected Resources

| Old Name | New Name |
| :--- | :--- |
| `archestra_agent` | `archestra_profile` |
| `archestra_agent_tool` | `archestra_profile_tool` |

## Migration Steps

The recommended way to migrate is using Terraform's `moved` blocks. This allows you to refactor your Terraform code and instruct Terraform to treat the old resource state as the new resource state.

### 1. Rename Resources in Configuration

Update your `.tf` files to use the new resource types.

**Before:**

```hcl
resource "archestra_agent" "example" {
  name = "example-agent"
  // ...
}

data "archestra_agent_tool" "example" {
  agent_id  = archestra_agent.example.id
  tool_name = "example-tool"
}

resource "archestra_trusted_data_policy" "example" {
  agent_tool_id = data.archestra_agent_tool.example.id
  // ...
}
```

**After:**

```hcl
resource "archestra_profile" "example" {
  name = "example-agent"
  // ...
}

data "archestra_profile_tool" "example" {
  agent_id  = archestra_profile.example.id // Note: agent_id fieldname is currently preserved for compatibility
  tool_name = "example-tool"
}

resource "archestra_trusted_data_policy" "example" {
  profile_tool_id = data.archestra_profile_tool.example.id // Use new profile_tool_id field
  // ...
}
```

### 2. Add `moved` Blocks

Add `moved` blocks to your configuration to map the old state to the new resources. You can place these in any `.tf` file, or a dedicated `versions.tf` or `migration.tf`.

```hcl
moved {
  from = archestra_agent.example
  to   = archestra_profile.example
}

moved {
  from = data.archestra_agent_tool.example
  to   = data.archestra_profile_tool.example
}
```

### 3. Run Terraform Plan and Apply

Run `terraform plan`. You should see that Terraform recognizes the move and plans **no changes** (or only metadata updates) to the actual infrastructure.

```bash
$ terraform plan
...
Plan: 0 to add, 0 to change, 0 to destroy.
```

If the plan looks correct, apply it:

```bash
$ terraform apply
```

Once applied, the state is migrated. You can remove the `moved` blocks in a future release if you wish, but keeping them is harmless and helps other team members migrate.

## Updating Dependent Resources

Dependent resources like `archestra_trusted_data_policy` and `archestra_tool_invocation_policy` now support a `profile_tool_id` attribute. The old `agent_tool_id` attribute is deprecated.

When migrating, simply change `agent_tool_id` to `profile_tool_id` in your configuration. The provider handles the underlying API mapping seamlessly.

```hcl
resource "archestra_trusted_data_policy" "example" {
- agent_tool_id = data.archestra_agent_tool.example.id
+ profile_tool_id = data.archestra_profile_tool.example.id
  // ...
}
```
