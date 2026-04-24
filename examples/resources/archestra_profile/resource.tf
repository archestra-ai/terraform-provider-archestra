resource "archestra_profile" "example" {
  name  = "production-profile"
  scope = "org" # one of "personal", "team", "org"

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
