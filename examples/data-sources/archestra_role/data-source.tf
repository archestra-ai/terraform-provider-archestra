# Give a service account a custom role created in the UI, by its name.
data "archestra_role" "release_managers" {
  name = "Release managers"
}

resource "archestra_service_account" "release_bot" {
  name = "release-bot"
  role = data.archestra_role.release_managers.role
}
