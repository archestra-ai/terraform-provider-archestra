resource "archestra_llm_proxy_profile" "example" {
  name                = "External application gateway"
  passthrough_headers = ["x-request-id"]
}
