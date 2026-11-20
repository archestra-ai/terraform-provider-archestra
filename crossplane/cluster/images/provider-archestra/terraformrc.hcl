provider_installation {
  filesystem_mirror {
    path    = "/terraform/provider-mirror"
    include = ["registry.terraform.io/*/*"]
  }
  direct {
    exclude = ["registry.terraform.io/*/*"]
  }
}
