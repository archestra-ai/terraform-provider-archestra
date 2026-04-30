variable "app_name" {
  type        = string
  description = "Branding name shown in the chat UI."
  default     = "Demo Copilot"
}

variable "footer_text" {
  type        = string
  description = "Footer string shown under the chat surface."
  default     = "© 2026 Demo Inc."
}

variable "oidc_issuer" {
  type        = string
  description = "OIDC issuer URL (e.g. https://your-keycloak.example.com/realms/main). Public providers like accounts.google.com work for a smoke test."
}

variable "oidc_discovery_endpoint" {
  type        = string
  description = "Well-known discovery endpoint for the OIDC provider, typically <issuer>/.well-known/openid-configuration."
}

variable "oidc_client_id" {
  type        = string
  description = "OAuth client_id registered with the IdP."
}

variable "oidc_client_secret" {
  type        = string
  description = "OAuth client_secret for the registered IdP application. Pass via TF_VAR_oidc_client_secret, never inline."
  sensitive   = true
}

variable "oidc_domain" {
  type        = string
  description = "Domain to associate with this IdP — users with email at this domain are routed through it."
  default     = "demo.example.com"
}

variable "saml_cert" {
  type        = string
  description = "PEM-encoded signing certificate for the SAML IdP. The smoke-test default is a syntactically-valid placeholder; replace before any real auth flow."
  default     = "-----BEGIN CERTIFICATE-----\nMIICiTCCAg+gAwIBAgIJAJ8l4HnPq7F8MAOGA1UEBhMCVVMxCzAJBgNVBAgTAkNB\nMRYwFAYDVQQHEw1TYW4gRnJhbmNpc2NvMRowGAYDVQQKExFPcGVuQU1QIFNhbXBs\nZSBDb21wYW55IENBMRYwFAYDVQQDEx1vcGVuYW1wLmV4YW1wbGUuY29tMB4XDTE0\nMDgxOTE2MjQyNVoXDTIyMDgxODE2MjQyNVowdTELMAkGA1UEBhMCVVMxCzAJBgNV\nBAgTAkNBMRYwFAYDVQQHEw1TYW4gRnJhbmNpc2NvMRowGAYDVQQKExFPcGVuQU1Q\nIFNhbXBsZSBDb21wYW55IENBMRYwFAYDVQQDEx1vcGVuYW1wLmV4YW1wbGUuY29t\nMFwwDQYJKoZIhvcNAQEBBQADSwAwSAJBANgOqCbLsKv5CF+vGmJ9Vq5PJKKuiU8+\nLpqtHKHC9q3mRWxHF8dlE8j9D6Kz+N+CK+qGzFjWNBT3UVFzU5GJUYCAwEAAaNQ\nME4wHQYDVR0OBBYEFG7CJM9GjHn7Lqt8kJc8W5proUwWMB8GA1UdIwQYMBaAFG7C\nJM9GjHn7Lqt8kJc8W5proUwWMAwGA1UdEwQFMAMBAf8wDQYJKoZIhvcNAQEFBQAD\nggEBABYIUUUeWDJ+wZF0lZ+mJnRnGZpXL2fKe3+KGjNM8xJfPf2YvqU4mgdMxgJn\n-----END CERTIFICATE-----"
}
