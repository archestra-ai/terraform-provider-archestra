# After import, `key` will be null (the backend doesn't re-return the
# token). To recover the token, destroy and re-create the resource.
terraform import archestra_api_key.example 00000000-0000-0000-0000-000000000000
