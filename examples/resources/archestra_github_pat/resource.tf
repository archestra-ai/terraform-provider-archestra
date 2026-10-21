variable "github_pat" {
  type      = string
  sensitive = true
}

resource "archestra_github_pat" "example" {
  name  = "existing-source-token"
  token = var.github_pat
}
