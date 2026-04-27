data "archestra_agent_tool" "fetch_url" {
  agent_id  = "00000000-0000-0000-0000-000000000000"
  tool_name = "fetch_url"
}

resource "archestra_trusted_data_policy" "trust_company_api" {
  tool_id     = data.archestra_agent_tool.fetch_url.id
  description = "Mark data from company API as trusted"
  conditions = [
    { key = "url", operator = "contains", value = "api.company.com" },
  ]
  action = "mark_as_trusted"
}
