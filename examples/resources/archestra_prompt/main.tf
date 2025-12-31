resource "archestra_prompt" "example" {
  profile_id    = "profile-123"
  name          = "Example Prompt"
  prompt        = "This is an example prompt."
  system_prompt = "You are an example system."
}