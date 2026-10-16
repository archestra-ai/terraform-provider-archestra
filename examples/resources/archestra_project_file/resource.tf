resource "archestra_project" "example" {
  name = "Managed project"
}

# The initial upload must contain at least one byte. Later .md/.txt UTF-8
# content edits up to 1,000,000 bytes update in place, including clearing.
# Binary content, filename, MIME hint and project changes replace the file.
# Keep instructions.md in archestra_project.instructions instead.
resource "archestra_project_file" "example" {
  project_id     = archestra_project.example.id
  filename       = "notes.txt"
  content_base64 = base64encode("Synthetic project notes.\n")
}

# content_base64 is sensitive and is stored in Terraform state.
# Use filebase64("./asset.bin") for binary uploads up to 25 MiB.
# An existing filename causes an error; import its scoped identity to adopt it.
# When importing, omit upload_mime_type because that creation hint is not retained.
