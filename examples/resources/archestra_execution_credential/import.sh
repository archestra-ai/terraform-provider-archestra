# Import by credential key. `organization_value` is write-only and cannot be
# recovered — if configured in HCL, the first apply after import re-sends it.
terraform import archestra_execution_credential.github github
