#!/bin/bash

# Import a profile tool assignment using the format: profile_id:tool_id
# Both IDs must be UUIDs

# Example with placeholder UUIDs (replace with your actual IDs)
terraform import archestra_profile_tool.github_create_issue "123e4567-e89b-12d3-a456-426614174000:789e4567-e89b-12d3-a456-426614174111"

# To find the profile_id:
# terraform console
# > data.archestra_profile.default.id

# To find the tool_id:
# terraform console
# > data.archestra_mcp_server_tool.github_create_issue.id
