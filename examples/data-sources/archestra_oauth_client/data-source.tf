data "archestra_oauth_client" "ci" {
  name = "CI pipeline"
}

output "ci_client_id" {
  value = data.archestra_oauth_client.ci.client_id
}
