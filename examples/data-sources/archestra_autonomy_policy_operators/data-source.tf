# Operators supported by the current backend version for tool-invocation
# and trusted-data policy conditions. Use to gate HCL on whether a
# specific operator (e.g. `regex`) is available before referencing it.
data "archestra_autonomy_policy_operators" "all" {}

output "supported_operators" {
  value = [for op in data.archestra_autonomy_policy_operators.all.operators : op.value]
}

# Fail the plan if the operator the policy depends on is not supported.
locals {
  operator_values = toset([for op in data.archestra_autonomy_policy_operators.all.operators : op.value])
}

check "regex_supported" {
  assert {
    condition     = contains(local.operator_values, "regex")
    error_message = "Backend does not advertise `regex` operator — upgrade the platform or drop the regex-based policy."
  }
}
