# Composite ID: <uuid>:<name>. The backend stores the constructed
# `<baseName>-<ownerId|teamId>` for local installs (the suffix can't
# be recovered on import), so the composite carries the
# user-configured `<name>` through. Bare UUID is rejected — the
# error message points at this format.
terraform import archestra_mcp_server_installation.example 00000000-0000-0000-0000-000000000000:my-install-name
