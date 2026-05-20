# After import, `credentials.api_token` will be null (the backend
# never echoes the token). Re-apply with the token set in HCL to
# write it back; the apply emits one diff rewriting credentials.
terraform import archestra_knowledge_connector.example 00000000-0000-0000-0000-000000000000
