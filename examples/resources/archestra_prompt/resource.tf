resource "archestra_profile" "example" {
  name = "example-profile"
}

resource "archestra_prompt" "example" {
  profile_id    = archestra_profile.example.id
  name          = "example-prompt"
  system_prompt = "You are a helpful assistant."
  user_prompt   = "Hello, how can you help me today?"
}
