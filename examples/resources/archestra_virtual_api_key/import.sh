# Composite `<llm_provider_api_key_id>:<id>` — both halves are required
# because the virtual key is nested under its parent on the wire.
# The `value` (token) is unrecoverable after creation, so imported keys
# will have a null `value`; rotate by destroying + recreating if you
# need the token back.
terraform import archestra_virtual_api_key.example 00000000-0000-0000-0000-000000000000:11111111-1111-1111-1111-111111111111
