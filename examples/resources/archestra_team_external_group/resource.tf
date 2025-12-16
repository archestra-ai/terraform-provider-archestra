# First, create or reference a team
resource "archestra_team" "engineering" {
  name        = "Engineering Team"
  description = "Engineering team synced with external IdP"
}

# Map an external identity provider group to the team
# This enables automatic team membership sync from LDAP, OIDC, or SAML
resource "archestra_team_external_group" "example" {
  team_id           = archestra_team.engineering.id
  external_group_id = "cn=engineers,ou=groups,dc=example,dc=com"
}
