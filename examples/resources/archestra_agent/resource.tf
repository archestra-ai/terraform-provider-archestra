resource "archestra_agent" "example" {
  name = "production-agent"

  labels = [
    {
      key   = "environment"
      value = "production"
    },
    {
      key   = "team"
      value = "backend"
    },
    {
      key   = "region"
      value = "us-west-2"
    }
  ]
}
