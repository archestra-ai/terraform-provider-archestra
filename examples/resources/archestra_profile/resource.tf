resource "archestra_profile" "example" {
  name = "production-profile"

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
