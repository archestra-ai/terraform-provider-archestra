data "archestra_profile_tool" "fetch_url" {
  agent_id  = "agent-id-here"
  tool_name = "fetch_url"
}

resource "archestra_trusted_data_policy" "trust_company_api" {
  profile_tool_id = data.archestra_profile_tool.fetch_url.id
  description     = "Mark data from company API as trusted"
  attribute_path  = "url"
  operator        = "contains"
  value           = "api.company.com"
  action          = "mark_as_trusted"
}
