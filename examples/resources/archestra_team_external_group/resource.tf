# Variables (declare in your variables.tf): engineering_idp_groups (list(string)) — IdP group names/IDs to map onto the team.

# Step 1: the team that the IdP group will sync into.
resource "archestra_team" "engineering" {
  name        = "Engineering"
  description = "Engineering team synced from the corporate IdP"
}

# Step 2: a single LDAP group → team mapping. The `external_group_id` is
# whatever the configured identity provider emits in its `groups` claim
# (LDAP DN, OIDC group name, SAML attribute value, etc.).
resource "archestra_team_external_group" "engineers_ldap" {
  team_id           = archestra_team.engineering.id
  external_group_id = "cn=engineers,ou=groups,dc=example,dc=com"
}

# A team can be the sync target for multiple groups — e.g. an OIDC group from
# Okta plus a SAML attribute from the legacy IdP.
resource "archestra_team_external_group" "engineers_oidc" {
  team_id           = archestra_team.engineering.id
  external_group_id = "engineering"
}

# Bulk fan-out from a list variable. Use `for_each` so adding/removing a group
# only churns its row instead of recreating the entire set.
variable "engineering_idp_groups" {
  type    = set(string)
  default = ["sre-oncall", "platform-eng", "data-eng"]
}

resource "archestra_team_external_group" "engineering_extra" {
  for_each          = var.engineering_idp_groups
  team_id           = archestra_team.engineering.id
  external_group_id = each.value
}
