package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

var _ resource.Resource = &MCPServerRegistryResource{}
var _ resource.ResourceWithImportState = &MCPServerRegistryResource{}

func NewMCPServerRegistryResource() resource.Resource {
	return &MCPServerRegistryResource{}
}

type MCPServerRegistryResource struct {
	client *client.ClientWithResponses
}

type MCPServerRegistryResourceModel struct {
	ID                  types.String `tfsdk:"id"`
	Name                types.String `tfsdk:"name"`
	Description         types.String `tfsdk:"description"`
	DocsURL             types.String `tfsdk:"docs_url"`
	InstallationCommand types.String `tfsdk:"installation_command"`
	AuthDescription     types.String `tfsdk:"auth_description"`
	LocalConfig         types.Object `tfsdk:"local_config"`
	RemoteConfig        types.Object `tfsdk:"remote_config"`
	AuthFields          types.List   `tfsdk:"auth_fields"`
	Version             types.String `tfsdk:"version"`
	Repository          types.String `tfsdk:"repository"`
	Instructions        types.String `tfsdk:"instructions"`
	Icon                types.String `tfsdk:"icon"`
	RequiresAuth        types.Bool   `tfsdk:"requires_auth"`
	DeploymentSpecYaml  types.String `tfsdk:"deployment_spec_yaml"`
	Scope               types.String `tfsdk:"scope"`
	Teams               types.List   `tfsdk:"teams"`
	Labels              types.List   `tfsdk:"labels"`

	ClientSecretId             types.String `tfsdk:"client_secret_id"`
	LocalConfigSecretId        types.String `tfsdk:"local_config_secret_id"`
	LocalConfigVaultKey        types.String `tfsdk:"local_config_vault_key"`
	LocalConfigVaultPath       types.String `tfsdk:"local_config_vault_path"`
	OauthClientSecretVaultKey  types.String `tfsdk:"oauth_client_secret_vault_key"`
	OauthClientSecretVaultPath types.String `tfsdk:"oauth_client_secret_vault_path"`

	EnterpriseManagedConfig *EnterpriseManagedConfigModel `tfsdk:"enterprise_managed_config"`

	UserConfig types.Map `tfsdk:"user_config"`
}

// UserConfigFieldModel mirrors a single entry in the `userConfig` map on an MCP catalog item.
// Defaults are polymorphic on the backend (string | number | bool | []string); we expose the
// default as a JSON-encoded string so all variants round-trip losslessly. Users can write
// plain strings directly or use `jsonencode(...)` for non-string defaults.
type UserConfigFieldModel struct {
	Title                types.String  `tfsdk:"title"`
	Description          types.String  `tfsdk:"description"`
	Type                 types.String  `tfsdk:"type"`
	Default              types.String  `tfsdk:"default"`
	Required             types.Bool    `tfsdk:"required"`
	Sensitive            types.Bool    `tfsdk:"sensitive"`
	Multiple             types.Bool    `tfsdk:"multiple"`
	Min                  types.Float64 `tfsdk:"min"`
	Max                  types.Float64 `tfsdk:"max"`
	HeaderName           types.String  `tfsdk:"header_name"`
	PromptOnInstallation types.Bool    `tfsdk:"prompt_on_installation"`
}

var userConfigAttrTypes = map[string]attr.Type{
	"title":                  types.StringType,
	"description":            types.StringType,
	"type":                   types.StringType,
	"default":                types.StringType,
	"required":               types.BoolType,
	"sensitive":              types.BoolType,
	"multiple":               types.BoolType,
	"min":                    types.Float64Type,
	"max":                    types.Float64Type,
	"header_name":            types.StringType,
	"prompt_on_installation": types.BoolType,
}

// EnterpriseManagedConfigModel mirrors the enterpriseManagedConfig object for catalog items
// with identity-provider-managed credentials.
type EnterpriseManagedConfigModel struct {
	IdentityProviderId      types.String `tfsdk:"identity_provider_id"`
	ResourceType            types.String `tfsdk:"resource_type"`
	ResourceIdentifier      types.String `tfsdk:"resource_identifier"`
	RequestedIssuer         types.String `tfsdk:"requested_issuer"`
	RequestedCredentialType types.String `tfsdk:"requested_credential_type"`
	Scopes                  types.List   `tfsdk:"scopes"`
	Audience                types.String `tfsdk:"audience"`
	ClientIdOverride        types.String `tfsdk:"client_id_override"`
	TokenInjectionMode      types.String `tfsdk:"token_injection_mode"`
	HeaderName              types.String `tfsdk:"header_name"`
	EnvVarName              types.String `tfsdk:"env_var_name"`
	BodyFieldName           types.String `tfsdk:"body_field_name"`
	ResponseFieldPath       types.String `tfsdk:"response_field_path"`
	FallbackMode            types.String `tfsdk:"fallback_mode"`
	CacheTtlSeconds         types.Int64  `tfsdk:"cache_ttl_seconds"`
	AssertionMode           types.String `tfsdk:"assertion_mode"`
}

type LabelModel struct {
	Key   types.String `tfsdk:"key"`
	Value types.String `tfsdk:"value"`
}

type LocalConfigModel struct {
	Command          types.String `tfsdk:"command"`
	Arguments        types.List   `tfsdk:"arguments"`
	Environment      types.Set    `tfsdk:"environment"`
	EnvFrom          types.List   `tfsdk:"env_from"`
	DockerImage      types.String `tfsdk:"docker_image"`
	TransportType    types.String `tfsdk:"transport_type"`
	HTTPPort         types.Int64  `tfsdk:"http_port"`
	HTTPPath         types.String `tfsdk:"http_path"`
	ServiceAccount   types.String `tfsdk:"service_account"`
	NodePort         types.Int64  `tfsdk:"node_port"`
	ImagePullSecrets types.List   `tfsdk:"image_pull_secrets"`
}

// EnvironmentVariableModel mirrors the wire shape one-to-one.
type EnvironmentVariableModel struct {
	Key                  types.String `tfsdk:"key"`
	Type                 types.String `tfsdk:"type"`
	Value                types.String `tfsdk:"value"`
	PromptOnInstallation types.Bool   `tfsdk:"prompt_on_installation"`
	Required             types.Bool   `tfsdk:"required"`
	Description          types.String `tfsdk:"description"`
	Default              types.String `tfsdk:"default"`
	Mounted              types.Bool   `tfsdk:"mounted"`
}

type ImagePullSecretModel struct {
	Source   types.String `tfsdk:"source"`
	Name     types.String `tfsdk:"name"`
	Server   types.String `tfsdk:"server"`
	Username types.String `tfsdk:"username"`
	Password types.String `tfsdk:"password"`
	Email    types.String `tfsdk:"email"`
}

type EnvFromModel struct {
	Type   types.String `tfsdk:"type"`
	Name   types.String `tfsdk:"name"`
	Prefix types.String `tfsdk:"prefix"`
}

type RemoteConfigModel struct {
	URL         types.String `tfsdk:"url"`
	OAuthConfig types.Object `tfsdk:"oauth_config"`
}

type OAuthConfigModel struct {
	ClientID                 types.String  `tfsdk:"client_id"`
	ClientSecret             types.String  `tfsdk:"client_secret"`
	RedirectURIs             types.List    `tfsdk:"redirect_uris"`
	Scopes                   types.List    `tfsdk:"scopes"`
	DefaultScopes            types.List    `tfsdk:"default_scopes"`
	SupportsResourceMetadata types.Bool    `tfsdk:"supports_resource_metadata"`
	AuthorizationEndpoint    types.String  `tfsdk:"authorization_endpoint"`
	TokenEndpoint            types.String  `tfsdk:"token_endpoint"`
	AuthServerURL            types.String  `tfsdk:"auth_server_url"`
	ResourceMetadataURL      types.String  `tfsdk:"resource_metadata_url"`
	WellKnownURL             types.String  `tfsdk:"well_known_url"`
	GrantType                types.String  `tfsdk:"grant_type"`
	Audience                 types.String  `tfsdk:"audience"`
	AccessTokenEnvVar        types.String  `tfsdk:"access_token_env_var"`
	BrowserAuth              types.Bool    `tfsdk:"browser_auth"`
	GenericOauth             types.Bool    `tfsdk:"generic_oauth"`
	RequiresProxy            types.Bool    `tfsdk:"requires_proxy"`
	ProviderName             types.String  `tfsdk:"provider_name"`
	StreamableHTTPURL        types.String  `tfsdk:"streamable_http_url"`
	StreamableHTTPPort       types.Int64   `tfsdk:"streamable_http_port"`
}


type AuthFieldModel struct {
	Name        types.String `tfsdk:"name"`
	Label       types.String `tfsdk:"label"`
	Type        types.String `tfsdk:"type"`
	Required    types.Bool   `tfsdk:"required"`
	Description types.String `tfsdk:"description"`
}

func (r *MCPServerRegistryResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_mcp_registry_catalog_item"
}

func (r *MCPServerRegistryResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages an MCP server in the Private MCP Registry. This allows you to register local MCP servers that can then be installed by profiles.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "MCP server catalog identifier",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "The name of the MCP server",
				Required:            true,
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "Description of the MCP server",
				Optional:            true,
			},
			"docs_url": schema.StringAttribute{
				MarkdownDescription: "URL to the MCP server documentation",
				Optional:            true,
			},
			"installation_command": schema.StringAttribute{
				MarkdownDescription: "Installation command for the MCP server (e.g., npm install -g @example/mcp-server)",
				Optional:            true,
			},
			"auth_description": schema.StringAttribute{
				MarkdownDescription: "Description of the authentication requirements",
				Optional:            true,
			},
			"local_config": schema.SingleNestedAttribute{
				MarkdownDescription: "Configuration for MCP servers run in the Archestra orchestrator MCP runtime",
				Optional:            true,
				Attributes: map[string]schema.Attribute{
					"command": schema.StringAttribute{
						MarkdownDescription: "The executable command to run (e.g., 'node', 'python', 'npx'). Optional if Docker Image is set (will use image's default CMD).",
						Optional:            true,
					},
					"arguments": schema.ListAttribute{
						MarkdownDescription: "Arguments to pass to the command",
						Optional:            true,
						ElementType:         types.StringType,
					},
					"environment": schema.SetNestedAttribute{
						MarkdownDescription: "Environment variables declared on the MCP server. Each entry mirrors the backend's wire shape one-to-one: `key`, `type`, optional `value`, `default`, `description`, plus `prompt_on_installation`, `required`, and `mounted` flags.",
						Optional:            true,
						NestedObject: schema.NestedAttributeObject{
							Attributes: map[string]schema.Attribute{
								"key": schema.StringAttribute{
									MarkdownDescription: "Environment variable name.",
									Required:            true,
								},
								"type": schema.StringAttribute{
									MarkdownDescription: "Variable type. One of `plain_text`, `secret`, `boolean`, `number`.",
									Required:            true,
									Validators: []validator.String{
										stringvalidator.OneOf("plain_text", "secret", "boolean", "number"),
									},
								},
								"value": schema.StringAttribute{
									MarkdownDescription: "Value for `plain_text` / `secret` variables.",
									Optional:            true,
								},
								"prompt_on_installation": schema.BoolAttribute{
									MarkdownDescription: "Whether the installer must supply this value at install time. Required field on the wire — defaults to `false`.",
									Optional:            true,
									Computed:            true,
									Default:             booldefault.StaticBool(false),
								},
								"required": schema.BoolAttribute{
									MarkdownDescription: "Whether the value must be set.",
									Optional:            true,
								},
								"description": schema.StringAttribute{
									MarkdownDescription: "Human-readable description of the variable.",
									Optional:            true,
								},
								"default": schema.StringAttribute{
									MarkdownDescription: "Default value. Use `jsonencode(...)` to encode non-string defaults (number, bool). Plain strings may be provided as-is.",
									Optional:            true,
								},
								"mounted": schema.BoolAttribute{
									MarkdownDescription: "When true, the value is mounted as a file at `/secrets/<key>` rather than injected as an env var.",
									Optional:            true,
								},
							},
						},
					},
					"env_from": schema.ListNestedAttribute{
						MarkdownDescription: "List of sources to populate environment variables from (Kubernetes secrets or configMaps)",
						Optional:            true,
						NestedObject: schema.NestedAttributeObject{
							Attributes: map[string]schema.Attribute{
								"type": schema.StringAttribute{
									MarkdownDescription: "Source type: 'secret' or 'configMap'",
									Required:            true,
									Validators: []validator.String{
										stringvalidator.OneOf("secret", "configMap"),
									},
								},
								"name": schema.StringAttribute{
									MarkdownDescription: "Name of the secret or configMap",
									Required:            true,
								},
								"prefix": schema.StringAttribute{
									MarkdownDescription: "Optional prefix for environment variable names",
									Optional:            true,
								},
							},
						},
					},
					"docker_image": schema.StringAttribute{
						MarkdownDescription: "Custom Docker image URL. If not specified, Archestra's default base image will be used.",
						Optional:            true,
					},
					"transport_type": schema.StringAttribute{
						MarkdownDescription: "Transport type: 'stdio' or 'streamable-http'. Defaults to 'stdio'",
						Optional:            true,
						Validators: []validator.String{
							stringvalidator.OneOf("stdio", "streamable-http"),
						},
					},
					"http_port": schema.Int64Attribute{
						MarkdownDescription: "HTTP port for streamable-http transport. Range 0..65535.",
						Optional:            true,
						Validators: []validator.Int64{
							int64validator.Between(0, 65535),
						},
					},
					"http_path": schema.StringAttribute{
						MarkdownDescription: "HTTP path for streamable-http transport (e.g., '/sse')",
						Optional:            true,
					},
					"service_account": schema.StringAttribute{
						MarkdownDescription: "Kubernetes service account for the MCP server pod",
						Optional:            true,
					},
					"node_port": schema.Int64Attribute{
						MarkdownDescription: "Node port for the MCP server service. Kubernetes NodePort range 30000..32767.",
						Optional:            true,
						Validators: []validator.Int64{
							int64validator.Between(30000, 32767),
						},
					},
					"image_pull_secrets": schema.ListNestedAttribute{
						MarkdownDescription: "Kubernetes image pull secrets for the MCP server pod. Supports two variants: `source = \"existing\"` references a pre-existing secret by `name`; `source = \"credentials\"` creates a new secret from explicit registry credentials (`server`, `username`, `password`, optional `email`).",
						Optional:            true,
						NestedObject: schema.NestedAttributeObject{
							Attributes: map[string]schema.Attribute{
								"source": schema.StringAttribute{
									MarkdownDescription: "Source of the pull secret. One of `existing`, `credentials`. Defaults to `existing` for backward compatibility when only `name` is set.",
									Optional:            true,
									Computed:            true,
									Default:             stringdefault.StaticString("existing"),
									Validators: []validator.String{
										stringvalidator.OneOf("existing", "credentials"),
									},
								},
								"name": schema.StringAttribute{
									MarkdownDescription: "Name of the existing Kubernetes secret (required for `source = existing`).",
									Optional:            true,
								},
								"server": schema.StringAttribute{
									MarkdownDescription: "Docker registry server URL (required for `source = credentials`).",
									Optional:            true,
								},
								"username": schema.StringAttribute{
									MarkdownDescription: "Registry username (required for `source = credentials`).",
									Optional:            true,
								},
								"password": schema.StringAttribute{
									MarkdownDescription: "Registry password (required for `source = credentials`). Write-only: the backend never echoes it back. To stay consistent across refreshes the provider preserves it keyed by `(server, username)`; rotating either of those values drops the password from state and forces re-entry.",
									Optional:            true,
									Sensitive:           true,
								},
								"email": schema.StringAttribute{
									MarkdownDescription: "Registry email (optional for `source = credentials`).",
									Optional:            true,
								},
							},
						},
					},
				},
			},
			"remote_config": schema.SingleNestedAttribute{
				MarkdownDescription: "Configuration for remote/hosted MCP servers accessed via HTTP",
				Optional:            true,
				Attributes: map[string]schema.Attribute{
					"url": schema.StringAttribute{
						MarkdownDescription: "The URL of the remote MCP server (e.g., https://api.githubcopilot.com/mcp/)",
						Required:            true,
					},
					"oauth_config": schema.SingleNestedAttribute{
						MarkdownDescription: "OAuth configuration for the remote MCP server. If not specified, users can authenticate with a Personal Access Token (PAT) via auth_fields.",
						Optional:            true,
						Attributes: map[string]schema.Attribute{
							"client_id": schema.StringAttribute{
								MarkdownDescription: "OAuth Client ID. Leave empty if the server supports dynamic client registration.",
								Optional:            true,
							},
							"client_secret": schema.StringAttribute{
								MarkdownDescription: "OAuth Client Secret (optional)",
								Optional:            true,
								Sensitive:           true,
							},
							"redirect_uris": schema.ListAttribute{
								MarkdownDescription: "Comma-separated list of redirect URIs",
								Required:            true,
								ElementType:         types.StringType,
							},
							"scopes": schema.ListAttribute{
								MarkdownDescription: "List of OAuth scopes to request (e.g., ['read', 'write'])",
								Optional:            true,
								ElementType:         types.StringType,
							},
							"default_scopes": schema.ListAttribute{
								MarkdownDescription: "Scopes requested by default when the server doesn't advertise its own.",
								Optional:            true,
								ElementType:         types.StringType,
							},
							"supports_resource_metadata": schema.BoolAttribute{
								MarkdownDescription: "Enable if the server publishes OAuth metadata at /.well-known/oauth-authorization-server for automatic endpoint discovery. Defaults to `false` (matching the backend default).",
								Optional:            true,
								Computed:            true,
								Default:             booldefault.StaticBool(false),
							},
							"authorization_endpoint": schema.StringAttribute{
								MarkdownDescription: "Custom OAuth authorization endpoint URL",
								Optional:            true,
							},
							"token_endpoint": schema.StringAttribute{
								MarkdownDescription: "Custom OAuth token endpoint URL.",
								Optional:            true,
							},
							"auth_server_url": schema.StringAttribute{
								MarkdownDescription: "Override for the OAuth authorization server root URL.",
								Optional:            true,
							},
							"resource_metadata_url": schema.StringAttribute{
								MarkdownDescription: "URL of the protected-resource metadata document.",
								Optional:            true,
							},
							"well_known_url": schema.StringAttribute{
								MarkdownDescription: "Override for the `.well-known` discovery document URL.",
								Optional:            true,
							},
							"grant_type": schema.StringAttribute{
								MarkdownDescription: "OAuth grant type. One of `authorization_code`, `client_credentials`.",
								Optional:            true,
								Validators: []validator.String{
									stringvalidator.OneOf("authorization_code", "client_credentials"),
								},
							},
							"audience": schema.StringAttribute{
								MarkdownDescription: "`aud` claim to request when performing token exchange.",
								Optional:            true,
							},
							"access_token_env_var": schema.StringAttribute{
								MarkdownDescription: "Environment variable name to inject the acquired access token into.",
								Optional:            true,
							},
							"browser_auth": schema.BoolAttribute{
								MarkdownDescription: "Prompt the installer through an interactive browser auth flow.",
								Optional:            true,
							},
							"generic_oauth": schema.BoolAttribute{
								MarkdownDescription: "Treat the server as a generic OAuth provider (skip vendor-specific probes).",
								Optional:            true,
							},
							"requires_proxy": schema.BoolAttribute{
								MarkdownDescription: "Route OAuth redirects through the Archestra proxy.",
								Optional:            true,
							},
							"provider_name": schema.StringAttribute{
								MarkdownDescription: "Human-readable name of the OAuth provider for display.",
								Optional:            true,
							},
							"streamable_http_url": schema.StringAttribute{
								MarkdownDescription: "Streamable-HTTP MCP server URL override.",
								Optional:            true,
							},
							"streamable_http_port": schema.Int64Attribute{
								MarkdownDescription: "Streamable-HTTP MCP server port override. Range 0..65535.",
								Optional:            true,
								Validators: []validator.Int64{
									int64validator.Between(0, 65535),
								},
							},
						},
					},
				},
			},
			"auth_fields": schema.ListNestedAttribute{
				MarkdownDescription: "Custom authentication fields required by the MCP server",
				Optional:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"name": schema.StringAttribute{
							MarkdownDescription: "Field name (used as environment variable)",
							Required:            true,
						},
						"label": schema.StringAttribute{
							MarkdownDescription: "Display label for the field",
							Required:            true,
						},
						"type": schema.StringAttribute{
							MarkdownDescription: "Field type: 'text', 'password', 'select', etc.",
							Required:            true,
						},
						"required": schema.BoolAttribute{
							MarkdownDescription: "Whether this field is required",
							Required:            true,
						},
						"description": schema.StringAttribute{
							MarkdownDescription: "Description of the field",
							Optional:            true,
						},
					},
				},
			},
			"version": schema.StringAttribute{
				MarkdownDescription: "Version string for the MCP server",
				Optional:            true,
			},
			"repository": schema.StringAttribute{
				MarkdownDescription: "Repository URL for the MCP server",
				Optional:            true,
			},
			"instructions": schema.StringAttribute{
				MarkdownDescription: "Installation instructions text for the MCP server",
				Optional:            true,
			},
			"icon": schema.StringAttribute{
				MarkdownDescription: "Icon string for the MCP server",
				Optional:            true,
			},
			"requires_auth": schema.BoolAttribute{
				MarkdownDescription: "Whether the MCP server requires authentication",
				Optional:            true,
				Computed:            true,
			},
			"deployment_spec_yaml": schema.StringAttribute{
				MarkdownDescription: "Custom Kubernetes deployment YAML for the MCP server",
				Optional:            true,
			},
			"scope": schema.StringAttribute{
				MarkdownDescription: "Visibility scope for the MCP server catalog item (e.g., 'personal', 'team', 'org')",
				Optional:            true,
				Computed:            true,
			},
			"teams": schema.ListAttribute{
				MarkdownDescription: "Team IDs that have access to this MCP server",
				Optional:            true,
				ElementType:         types.StringType,
			},
			"labels": schema.ListNestedAttribute{
				MarkdownDescription: "Labels for the MCP server catalog item",
				Optional:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"key": schema.StringAttribute{
							MarkdownDescription: "Label key",
							Required:            true,
						},
						"value": schema.StringAttribute{
							MarkdownDescription: "Label value",
							Required:            true,
						},
					},
				},
			},
			"client_secret_id": schema.StringAttribute{
				MarkdownDescription: "UUID of a stored secret holding the OAuth client secret. Mutually exclusive with inline `oauth_config.client_secret`. Computed when the backend auto-creates a BYOS vault reference.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"local_config_secret_id": schema.StringAttribute{
				MarkdownDescription: "UUID of a stored secret holding local_config environment values. Computed when the backend auto-creates a BYOS vault reference.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"local_config_vault_key": schema.StringAttribute{
				MarkdownDescription: "BYOS vault key for local_config secrets.",
				Optional:            true,
			},
			"local_config_vault_path": schema.StringAttribute{
				MarkdownDescription: "BYOS vault path for local_config secrets.",
				Optional:            true,
			},
			"oauth_client_secret_vault_key": schema.StringAttribute{
				MarkdownDescription: "BYOS vault key for the OAuth client secret.",
				Optional:            true,
			},
			"oauth_client_secret_vault_path": schema.StringAttribute{
				MarkdownDescription: "BYOS vault path for the OAuth client secret.",
				Optional:            true,
			},
			"enterprise_managed_config": schema.SingleNestedAttribute{
				MarkdownDescription: "Enterprise-managed credential configuration. Binds this catalog item to an identity provider that issues credentials at runtime rather than using static secrets.",
				Optional:            true,
				Attributes: map[string]schema.Attribute{
					"identity_provider_id": schema.StringAttribute{Optional: true, MarkdownDescription: "Identity provider UUID issuing credentials."},
					"resource_type": schema.StringAttribute{
						Optional:            true,
						MarkdownDescription: "Resource type. One of `mcp`, `oauth_protected_resource`, `secret`, `service_account`, `custom_http`.",
						Validators: []validator.String{
							stringvalidator.OneOf("mcp", "oauth_protected_resource", "secret", "service_account", "custom_http"),
						},
					},
					"resource_identifier": schema.StringAttribute{Optional: true},
					"requested_issuer":    schema.StringAttribute{Optional: true},
					"requested_credential_type": schema.StringAttribute{
						Optional:            true,
						MarkdownDescription: "Credential type requested. One of `id_jag`, `bearer_token`, `secret`, `service_account`, `opaque_json`.",
						Validators: []validator.String{
							stringvalidator.OneOf("id_jag", "bearer_token", "secret", "service_account", "opaque_json"),
						},
					},
					"scopes":             schema.ListAttribute{Optional: true, ElementType: types.StringType},
					"audience":           schema.StringAttribute{Optional: true},
					"client_id_override": schema.StringAttribute{Optional: true},
					"token_injection_mode": schema.StringAttribute{
						Optional:            true,
						MarkdownDescription: "How the token is injected into the downstream request. One of `authorization_bearer`, `raw_authorization`, `header`, `env`, `body_field`.",
						Validators: []validator.String{
							stringvalidator.OneOf("authorization_bearer", "raw_authorization", "header", "env", "body_field"),
						},
					},
					"header_name":         schema.StringAttribute{Optional: true},
					"env_var_name":        schema.StringAttribute{Optional: true},
					"body_field_name":     schema.StringAttribute{Optional: true},
					"response_field_path": schema.StringAttribute{Optional: true},
					"fallback_mode": schema.StringAttribute{
						Optional:            true,
						MarkdownDescription: "Behavior when credential exchange fails. One of `fail_closed`, `fallback_to_dynamic`, `fallback_to_static`.",
						Validators: []validator.String{
							stringvalidator.OneOf("fail_closed", "fallback_to_dynamic", "fallback_to_static"),
						},
					},
					"cache_ttl_seconds": schema.Int64Attribute{
						Optional:            true,
						MarkdownDescription: "Cache TTL in seconds. Non-negative.",
						Validators: []validator.Int64{
							int64validator.AtLeast(0),
						},
					},
					"assertion_mode": schema.StringAttribute{
						Optional:            true,
						MarkdownDescription: "Assertion exchange mode. One of `exchange`, `passthrough`.",
						Validators: []validator.String{
							stringvalidator.OneOf("exchange", "passthrough"),
						},
					},
				},
			},
			"user_config": schema.MapNestedAttribute{
				MarkdownDescription: "User-configurable fields collected from the installer at install time. The map key is the field name the installer will see.",
				Optional:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"title":       schema.StringAttribute{Required: true, MarkdownDescription: "Human-readable field title shown to the installer."},
						"description": schema.StringAttribute{Required: true, MarkdownDescription: "Description of the field shown to the installer."},
						"type": schema.StringAttribute{
							Required:            true,
							MarkdownDescription: "Field type. One of `string`, `number`, `boolean`, `file`, `directory`.",
							Validators: []validator.String{
								stringvalidator.OneOf("string", "number", "boolean", "file", "directory"),
							},
						},
						"default": schema.StringAttribute{
							Optional:            true,
							MarkdownDescription: "Default value. Use `jsonencode(...)` to encode non-string defaults (number, bool, or []string). Plain strings may be provided as-is.",
						},
						"required":               schema.BoolAttribute{Optional: true, MarkdownDescription: "Whether the installer must supply this field."},
						"sensitive":              schema.BoolAttribute{Optional: true, MarkdownDescription: "If true, the value is redacted in logs and UI."},
						"multiple":               schema.BoolAttribute{Optional: true, MarkdownDescription: "Whether multiple values may be supplied."},
						"min":                    schema.Float64Attribute{Optional: true, MarkdownDescription: "Minimum value (for numeric fields)."},
						"max":                    schema.Float64Attribute{Optional: true, MarkdownDescription: "Maximum value (for numeric fields)."},
						"header_name":            schema.StringAttribute{Optional: true, MarkdownDescription: "HTTP header name to bind this value to when installing a remote server."},
						"prompt_on_installation": schema.BoolAttribute{Optional: true, MarkdownDescription: "Whether to prompt the user for this value during installation."},
					},
				},
			},
		},
	}
}

func (r *MCPServerRegistryResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*client.ClientWithResponses)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *client.ClientWithResponses, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	r.client = client
}

// AttrSpecs implements resourceWithAttrSpec for the merge-patch drift check.
func (r *MCPServerRegistryResource) AttrSpecs() []AttrSpec {
	return catalogItemAttrSpec
}

func (r *MCPServerRegistryResource) APIShape() any {
	return client.GetInternalMcpCatalogItemResponse{}
}

// KnownIntentionallySkipped — wire fields not modeled on this resource:
//   - createdAt/updatedAt/publishedAt: audit timestamps.
//   - authorId/authorName: publishing metadata.
//   - verified/reviews/installations/installCount: curated-catalog flags
//     and frontend metrics.
//   - authDescription/authFields/approval*/submitted*: catalog-browse-page
//     hints + curation workflow; not part of the manage-this-server surface.
//   - oauthConfig/serverUrl: wire-side top-level fields that the schema
//     nests inside the ergonomic `remote_config` block (oauth_config /
//     remote_config.url respectively). The Synthetic remote_config
//     decomposition in finalizeCatalogItemPatch handles the wire shape.
//   - serverType: wire discriminator the provider derives from which of
//     local_config/remote_config the user populated.
//   - organizationId: ownership metadata, never user-managed.
func (r *MCPServerRegistryResource) KnownIntentionallySkipped() []string {
	return []string{
		"createdAt", "updatedAt", "publishedAt", "authorId", "authorName",
		"verified", "reviews", "installations", "installCount",
		"authDescription", "authFields", "approvalStatus", "approvedAt",
		"approvedBy", "rejectionReason", "submittedAt", "submittedBy",
		"oauthConfig", "serverType", "serverUrl", "organizationId",
	}
}

func (r *MCPServerRegistryResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data MCPServerRegistryResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if !data.LocalConfig.IsNull() && !data.RemoteConfig.IsNull() {
		resp.Diagnostics.AddError(
			"Invalid Configuration",
			"Only one of 'local_config' or 'remote_config' can be specified, not both.",
		)
		return
	}
	if data.LocalConfig.IsNull() && data.RemoteConfig.IsNull() {
		resp.Diagnostics.AddError(
			"Invalid Configuration",
			"One of 'local_config' or 'remote_config' must be specified.",
		)
		return
	}
	if !data.LocalConfig.IsNull() {
		var lc LocalConfigModel
		resp.Diagnostics.Append(data.LocalConfig.As(ctx, &lc, basetypes.ObjectAsOptions{})...)
		if resp.Diagnostics.HasError() {
			return
		}
		if lc.Command.IsNull() && lc.DockerImage.IsNull() {
			resp.Diagnostics.AddError(
				"Invalid Configuration",
				"Either 'command' or 'docker_image' must be specified in 'local_config'.",
			)
			return
		}
	}

	plan := req.Plan.Raw
	prior := tftypes.NewValue(plan.Type(), nil)
	patch := MergePatch(ctx, plan, prior, catalogItemAttrSpec, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	serverName := data.Name.ValueString()
	finalizeCatalogItemPatch(ctx, patch, plan, prior, serverName, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	if !data.RemoteConfig.IsNull() {
		patch["serverType"] = "remote"
	} else {
		patch["serverType"] = "local"
	}

	if data.RequiresAuth.IsNull() && !data.RemoteConfig.IsNull() {
		if !data.AuthFields.IsNull() || patch["oauthConfig"] != nil {
			patch["requiresAuth"] = true
		}
	}

	LogPatch(ctx, "create catalog item", patch, catalogItemAttrSpec)
	bodyBytes, err := json.Marshal(patch)
	if err != nil {
		resp.Diagnostics.AddError("Marshal Error", fmt.Sprintf("Unable to marshal request body: %s", err))
		return
	}
	apiResp, err := r.client.CreateInternalMcpCatalogItemWithBodyWithResponse(ctx, "application/json", bytes.NewReader(bodyBytes))
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to create MCP server, got error: %s", err))
		return
	}
	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Expected 200 OK, got status %d: %s", apiResp.StatusCode(), string(apiResp.Body)),
		)
		return
	}

	data.ID = types.StringValue(apiResp.JSON200.Id.String())

	readResp, err := r.client.GetInternalMcpCatalogItemWithResponse(ctx, apiResp.JSON200.Id)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to read catalog item after creation: %s", err))
		return
	}
	if readResp.JSON200 == nil {
		resp.Diagnostics.AddError("Unexpected API Response", fmt.Sprintf("Expected 200 OK on read after create, got status %d", readResp.StatusCode()))
		return
	}

	r.mapGetResponseToState(ctx, &data, readResp, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *MCPServerRegistryResource) mapGetResponseToState(ctx context.Context, data *MCPServerRegistryResourceModel, apiResp *client.GetInternalMcpCatalogItemResponse, diags *diag.Diagnostics) {
	// Map response to Terraform state
	data.Name = types.StringValue(apiResp.JSON200.Name)

	if apiResp.JSON200.Description != nil {
		data.Description = types.StringValue(*apiResp.JSON200.Description)
	} else {
		data.Description = types.StringNull()
	}

	if apiResp.JSON200.DocsUrl != nil {
		data.DocsURL = types.StringValue(*apiResp.JSON200.DocsUrl)
	} else {
		data.DocsURL = types.StringNull()
	}

	if apiResp.JSON200.InstallationCommand != nil {
		data.InstallationCommand = types.StringValue(*apiResp.JSON200.InstallationCommand)
	} else {
		data.InstallationCommand = types.StringNull()
	}

	if apiResp.JSON200.AuthDescription != nil {
		data.AuthDescription = types.StringValue(*apiResp.JSON200.AuthDescription)
	} else {
		data.AuthDescription = types.StringNull()
	}

	if apiResp.JSON200.Version != nil {
		data.Version = types.StringValue(*apiResp.JSON200.Version)
	} else {
		data.Version = types.StringNull()
	}

	if apiResp.JSON200.Repository != nil {
		data.Repository = types.StringValue(*apiResp.JSON200.Repository)
	} else {
		data.Repository = types.StringNull()
	}

	if apiResp.JSON200.Instructions != nil {
		data.Instructions = types.StringValue(*apiResp.JSON200.Instructions)
	} else {
		data.Instructions = types.StringNull()
	}

	if apiResp.JSON200.Icon != nil {
		data.Icon = types.StringValue(*apiResp.JSON200.Icon)
	} else {
		data.Icon = types.StringNull()
	}

	data.RequiresAuth = types.BoolValue(apiResp.JSON200.RequiresAuth)

	if apiResp.JSON200.DeploymentSpecYaml != nil {
		data.DeploymentSpecYaml = types.StringValue(*apiResp.JSON200.DeploymentSpecYaml)
	} else {
		data.DeploymentSpecYaml = types.StringNull()
	}

	data.Scope = types.StringValue(string(apiResp.JSON200.Scope))

	if apiResp.JSON200.ClientSecretId != nil {
		data.ClientSecretId = types.StringValue(apiResp.JSON200.ClientSecretId.String())
	} else {
		data.ClientSecretId = types.StringNull()
	}
	if apiResp.JSON200.LocalConfigSecretId != nil {
		data.LocalConfigSecretId = types.StringValue(apiResp.JSON200.LocalConfigSecretId.String())
	} else {
		data.LocalConfigSecretId = types.StringNull()
	}
	// Vault key/path are write-only on the backend; preserve whatever is already in state.

	if apiResp.JSON200.EnterpriseManagedConfig != nil {
		emc := apiResp.JSON200.EnterpriseManagedConfig
		model := &EnterpriseManagedConfigModel{
			IdentityProviderId: stringValueOrNull(emc.IdentityProviderId),
			ResourceIdentifier: stringValueOrNull(emc.ResourceIdentifier),
			RequestedIssuer:    stringValueOrNull(emc.RequestedIssuer),
			Audience:           stringValueOrNull(emc.Audience),
			ClientIdOverride:   stringValueOrNull(emc.ClientIdOverride),
			HeaderName:         stringValueOrNull(emc.HeaderName),
			EnvVarName:         stringValueOrNull(emc.EnvVarName),
			BodyFieldName:      stringValueOrNull(emc.BodyFieldName),
			ResponseFieldPath:  stringValueOrNull(emc.ResponseFieldPath),
		}
		if emc.ResourceType != nil {
			model.ResourceType = types.StringValue(string(*emc.ResourceType))
		} else {
			model.ResourceType = types.StringNull()
		}
		if emc.RequestedCredentialType != nil {
			model.RequestedCredentialType = types.StringValue(string(*emc.RequestedCredentialType))
		} else {
			model.RequestedCredentialType = types.StringNull()
		}
		if emc.TokenInjectionMode != nil {
			model.TokenInjectionMode = types.StringValue(string(*emc.TokenInjectionMode))
		} else {
			model.TokenInjectionMode = types.StringNull()
		}
		if emc.AssertionMode != nil {
			model.AssertionMode = types.StringValue(string(*emc.AssertionMode))
		} else {
			model.AssertionMode = types.StringNull()
		}
		if emc.FallbackMode != nil {
			model.FallbackMode = types.StringValue(string(*emc.FallbackMode))
		} else {
			model.FallbackMode = types.StringNull()
		}
		if emc.CacheTtlSeconds != nil {
			model.CacheTtlSeconds = types.Int64Value(int64(*emc.CacheTtlSeconds))
		} else {
			model.CacheTtlSeconds = types.Int64Null()
		}
		if emc.Scopes != nil {
			list, _ := types.ListValueFrom(context.Background(), types.StringType, *emc.Scopes)
			model.Scopes = list
		} else {
			model.Scopes = types.ListNull(types.StringType)
		}
		data.EnterpriseManagedConfig = model
	} else {
		data.EnterpriseManagedConfig = nil
	}

	// Map UserConfig. All callers pass a non-nil diags, so marshal/unmarshal
	// errors always surface on the response.
	if apiResp.JSON200.UserConfig != nil {
		uc, ucDiags := flattenUserConfig(apiResp.JSON200.UserConfig)
		diags.Append(ucDiags...)
		data.UserConfig = uc
	} else {
		data.UserConfig = types.MapNull(types.ObjectType{AttrTypes: userConfigAttrTypes})
	}

	// Map Teams from API response
	if len(apiResp.JSON200.Teams) > 0 {
		teamValues := make([]attr.Value, len(apiResp.JSON200.Teams))
		for i, team := range apiResp.JSON200.Teams {
			teamValues[i] = types.StringValue(team.Id)
		}
		data.Teams, _ = types.ListValue(types.StringType, teamValues)
	} else {
		data.Teams = types.ListNull(types.StringType)
	}

	// Map Labels from API response
	labelAttrTypes := map[string]attr.Type{
		"key":   types.StringType,
		"value": types.StringType,
	}
	labelObjectType := types.ObjectType{AttrTypes: labelAttrTypes}
	if len(apiResp.JSON200.Labels) > 0 {
		labelValues := make([]attr.Value, len(apiResp.JSON200.Labels))
		for i, label := range apiResp.JSON200.Labels {
			labelValues[i], _ = types.ObjectValue(labelAttrTypes, map[string]attr.Value{
				"key":   types.StringValue(label.Key),
				"value": types.StringValue(label.Value),
			})
		}
		data.Labels, _ = types.ListValue(labelObjectType, labelValues)
	} else {
		data.Labels = types.ListNull(labelObjectType)
	}

	// Map LocalConfig from API response if present
	envFromAttrTypes := map[string]attr.Type{
		"type":   types.StringType,
		"name":   types.StringType,
		"prefix": types.StringType,
	}
	envFromObjectType := types.ObjectType{AttrTypes: envFromAttrTypes}

	ipSecretAttrTypes := map[string]attr.Type{
		"source":   types.StringType,
		"name":     types.StringType,
		"server":   types.StringType,
		"username": types.StringType,
		"password": types.StringType,
		"email":    types.StringType,
	}
	ipSecretObjectType := types.ObjectType{AttrTypes: ipSecretAttrTypes}

	envVariableAttrTypes := map[string]attr.Type{
		"key":                    types.StringType,
		"type":                   types.StringType,
		"value":                  types.StringType,
		"prompt_on_installation": types.BoolType,
		"required":               types.BoolType,
		"description":            types.StringType,
		"default":                types.StringType,
		"mounted":                types.BoolType,
	}
	envVariableObjectType := types.ObjectType{AttrTypes: envVariableAttrTypes}

	if apiResp.JSON200.LocalConfig != nil {
		localConfigObj := map[string]attr.Value{
			"command":            types.StringNull(),
			"arguments":          types.ListNull(types.StringType),
			"environment":        types.SetNull(envVariableObjectType),
			"env_from":           types.ListNull(envFromObjectType),
			"docker_image":       types.StringNull(),
			"transport_type":     types.StringNull(),
			"http_port":          types.Int64Null(),
			"http_path":          types.StringNull(),
			"service_account":    types.StringNull(),
			"node_port":          types.Int64Null(),
			"image_pull_secrets": types.ListNull(ipSecretObjectType),
		}

		// Command
		if apiResp.JSON200.LocalConfig.Command != nil {
			localConfigObj["command"] = types.StringValue(*apiResp.JSON200.LocalConfig.Command)
		}

		// Arguments
		if apiResp.JSON200.LocalConfig.Arguments != nil && len(*apiResp.JSON200.LocalConfig.Arguments) > 0 {
			argValues := make([]attr.Value, len(*apiResp.JSON200.LocalConfig.Arguments))
			for i, arg := range *apiResp.JSON200.LocalConfig.Arguments {
				argValues[i] = types.StringValue(arg)
			}
			localConfigObj["arguments"], _ = types.ListValue(types.StringType, argValues)
		}

		if apiResp.JSON200.LocalConfig.Environment != nil && len(*apiResp.JSON200.LocalConfig.Environment) > 0 {
			envValues := make([]attr.Value, 0, len(*apiResp.JSON200.LocalConfig.Environment))
			for _, envVar := range *apiResp.JSON200.LocalConfig.Environment {
				fields := map[string]attr.Value{
					"key":                    types.StringValue(envVar.Key),
					"type":                   types.StringValue(string(envVar.Type)),
					"value":                  types.StringNull(),
					"prompt_on_installation": types.BoolValue(envVar.PromptOnInstallation),
					"required":               types.BoolNull(),
					"description":            types.StringNull(),
					"default":                types.StringNull(),
					"mounted":                types.BoolNull(),
				}
				if envVar.Value != nil {
					fields["value"] = types.StringValue(*envVar.Value)
				}
				if envVar.Required != nil {
					fields["required"] = types.BoolValue(*envVar.Required)
				}
				if envVar.Description != nil {
					fields["description"] = types.StringValue(*envVar.Description)
				}
				if envVar.Default != nil {
					if encoded, err := json.Marshal(envVar.Default); err == nil {
						// Strings round-trip with surrounding quotes; collapse to bare value.
						if s, isStr := stringFromJSONScalar(encoded); isStr {
							fields["default"] = types.StringValue(s)
						} else {
							fields["default"] = types.StringValue(string(encoded))
						}
					}
				}
				if envVar.Mounted != nil {
					fields["mounted"] = types.BoolValue(*envVar.Mounted)
				}
				obj, _ := types.ObjectValue(envVariableAttrTypes, fields)
				envValues = append(envValues, obj)
			}
			localConfigObj["environment"], _ = types.SetValue(envVariableObjectType, envValues)
		}

		// Optional fields
		if apiResp.JSON200.LocalConfig.DockerImage != nil {
			localConfigObj["docker_image"] = types.StringValue(*apiResp.JSON200.LocalConfig.DockerImage)
		}
		if apiResp.JSON200.LocalConfig.HttpPath != nil {
			localConfigObj["http_path"] = types.StringValue(*apiResp.JSON200.LocalConfig.HttpPath)
		}
		if apiResp.JSON200.LocalConfig.HttpPort != nil {
			localConfigObj["http_port"] = types.Int64Value(int64(*apiResp.JSON200.LocalConfig.HttpPort))
		}
		if apiResp.JSON200.LocalConfig.TransportType != nil {
			localConfigObj["transport_type"] = types.StringValue(string(*apiResp.JSON200.LocalConfig.TransportType))
		}
		if apiResp.JSON200.LocalConfig.ServiceAccount != nil {
			localConfigObj["service_account"] = types.StringValue(*apiResp.JSON200.LocalConfig.ServiceAccount)
		}
		if apiResp.JSON200.LocalConfig.NodePort != nil {
			localConfigObj["node_port"] = types.Int64Value(int64(*apiResp.JSON200.LocalConfig.NodePort))
		}

		// ImagePullSecrets — parse from raw response body; the generated union type has unexported fields.
		{
			var rawResp struct {
				LocalConfig *struct {
					ImagePullSecrets *[]struct {
						Source   string `json:"source"`
						Name     string `json:"name,omitempty"`
						Server   string `json:"server,omitempty"`
						Username string `json:"username,omitempty"`
						Password string `json:"password,omitempty"`
						Email    string `json:"email,omitempty"`
					} `json:"imagePullSecrets,omitempty"`
				} `json:"localConfig"`
			}
			if parseErr := json.Unmarshal(apiResp.Body, &rawResp); parseErr == nil &&
				rawResp.LocalConfig != nil && rawResp.LocalConfig.ImagePullSecrets != nil &&
				len(*rawResp.LocalConfig.ImagePullSecrets) > 0 {
				// The backend never echoes passwords back; preserve from prior state
				// keyed by server|username to keep sensitive-value consistency on apply.
				priorPasswords := make(map[string]types.String)
				if !data.LocalConfig.IsNull() && !data.LocalConfig.IsUnknown() {
					var priorLC LocalConfigModel
					if d := data.LocalConfig.As(ctx, &priorLC, basetypes.ObjectAsOptions{}); !d.HasError() && !priorLC.ImagePullSecrets.IsNull() && !priorLC.ImagePullSecrets.IsUnknown() {
						var priorIPS []ImagePullSecretModel
						if d := priorLC.ImagePullSecrets.ElementsAs(ctx, &priorIPS, false); !d.HasError() {
							for _, p := range priorIPS {
								key := p.Server.ValueString() + "|" + p.Username.ValueString()
								priorPasswords[key] = p.Password
							}
						}
					}
				}

				ipsValues := make([]attr.Value, 0, len(*rawResp.LocalConfig.ImagePullSecrets))
				for _, ips := range *rawResp.LocalConfig.ImagePullSecrets {
					password := types.StringNull()
					if prev, ok := priorPasswords[ips.Server+"|"+ips.Username]; ok && !prev.IsNull() {
						password = prev
					}
					fields := map[string]attr.Value{
						"source":   types.StringValue(ips.Source),
						"name":     strOrNull(ips.Name),
						"server":   strOrNull(ips.Server),
						"username": strOrNull(ips.Username),
						"password": password,
						"email":    strOrNull(ips.Email),
					}
					obj, _ := types.ObjectValue(ipSecretAttrTypes, fields)
					ipsValues = append(ipsValues, obj)
				}
				localConfigObj["image_pull_secrets"], _ = types.ListValue(ipSecretObjectType, ipsValues)
			}
		}

		// EnvFrom
		if apiResp.JSON200.LocalConfig.EnvFrom != nil && len(*apiResp.JSON200.LocalConfig.EnvFrom) > 0 {
			envFromValues := make([]attr.Value, len(*apiResp.JSON200.LocalConfig.EnvFrom))
			for i, ef := range *apiResp.JSON200.LocalConfig.EnvFrom {
				efMap := map[string]attr.Value{
					"type":   types.StringValue(string(ef.Type)),
					"name":   types.StringValue(ef.Name),
					"prefix": types.StringNull(),
				}
				if ef.Prefix != nil {
					efMap["prefix"] = types.StringValue(*ef.Prefix)
				}
				envFromValues[i], _ = types.ObjectValue(envFromAttrTypes, efMap)
			}
			localConfigObj["env_from"], _ = types.ListValue(envFromObjectType, envFromValues)
		}

		localConfigAttrTypes := map[string]attr.Type{
			"command":            types.StringType,
			"arguments":          types.ListType{ElemType: types.StringType},
			"environment":        types.SetType{ElemType: envVariableObjectType},
			"env_from":           types.ListType{ElemType: envFromObjectType},
			"docker_image":       types.StringType,
			"transport_type":     types.StringType,
			"http_port":          types.Int64Type,
			"http_path":          types.StringType,
			"service_account":    types.StringType,
			"node_port":          types.Int64Type,
			"image_pull_secrets": types.ListType{ElemType: ipSecretObjectType},
		}

		data.LocalConfig, _ = types.ObjectValue(localConfigAttrTypes, localConfigObj)
	} else {
		data.LocalConfig = types.ObjectNull(map[string]attr.Type{
			"command":            types.StringType,
			"arguments":          types.ListType{ElemType: types.StringType},
			"environment":        types.SetType{ElemType: envVariableObjectType},
			"env_from":           types.ListType{ElemType: envFromObjectType},
			"docker_image":       types.StringType,
			"transport_type":     types.StringType,
			"http_port":          types.Int64Type,
			"http_path":          types.StringType,
			"service_account":    types.StringType,
			"node_port":          types.Int64Type,
			"image_pull_secrets": types.ListType{ElemType: ipSecretObjectType},
		})
	}

	// Map RemoteConfig from API response if server type is remote
	oauthConfigAttrTypes := map[string]attr.Type{
		"client_id":                  types.StringType,
		"client_secret":              types.StringType,
		"redirect_uris":              types.ListType{ElemType: types.StringType},
		"scopes":                     types.ListType{ElemType: types.StringType},
		"default_scopes":             types.ListType{ElemType: types.StringType},
		"supports_resource_metadata": types.BoolType,
		"authorization_endpoint":     types.StringType,
		"token_endpoint":             types.StringType,
		"auth_server_url":            types.StringType,
		"resource_metadata_url":      types.StringType,
		"well_known_url":             types.StringType,
		"grant_type":                 types.StringType,
		"audience":                   types.StringType,
		"access_token_env_var":       types.StringType,
		"browser_auth":               types.BoolType,
		"generic_oauth":              types.BoolType,
		"requires_proxy":             types.BoolType,
		"provider_name":              types.StringType,
		"streamable_http_url":        types.StringType,
		"streamable_http_port":       types.Int64Type,
	}

	remoteConfigAttrTypes := map[string]attr.Type{
		"url":          types.StringType,
		"oauth_config": types.ObjectType{AttrTypes: oauthConfigAttrTypes},
	}

	if string(apiResp.JSON200.ServerType) == "remote" && apiResp.JSON200.ServerUrl != nil {
		remoteConfigObj := map[string]attr.Value{
			"url": types.StringValue(*apiResp.JSON200.ServerUrl),
		}

		// client_secret round-trips via the secret manager: on write it's
		// extracted to `clientSecretId`, on read the backend rehydrates it
		// into oauthConfig.client_secret. So GET is a faithful source.
		if apiResp.JSON200.OauthConfig != nil {
			oc := apiResp.JSON200.OauthConfig
			oauthConfigObj := map[string]attr.Value{
				"client_id":                  types.StringValue(oc.ClientId),
				"client_secret":              stringValueOrNull(oc.ClientSecret),
				"redirect_uris":              types.ListNull(types.StringType),
				"scopes":                     types.ListNull(types.StringType),
				"default_scopes":             types.ListNull(types.StringType),
				"supports_resource_metadata": types.BoolValue(oc.SupportsResourceMetadata),
				"authorization_endpoint":     stringValueOrNull(oc.AuthorizationEndpoint),
				"token_endpoint":             stringValueOrNull(oc.TokenEndpoint),
				"auth_server_url":            stringValueOrNull(oc.AuthServerUrl),
				"resource_metadata_url":      stringValueOrNull(oc.ResourceMetadataUrl),
				"well_known_url":             stringValueOrNull(oc.WellKnownUrl),
				"audience":                   stringValueOrNull(oc.Audience),
				"access_token_env_var":       stringValueOrNull(oc.AccessTokenEnvVar),
				"browser_auth":               boolValueOrNull(oc.BrowserAuth),
				"generic_oauth":              boolValueOrNull(oc.GenericOauth),
				"requires_proxy":             boolValueOrNull(oc.RequiresProxy),
				"provider_name":              stringValueOrNull(oc.ProviderName),
				"streamable_http_url":        stringValueOrNull(oc.StreamableHttpUrl),
				"streamable_http_port":       types.Int64Null(),
				"grant_type":                 types.StringNull(),
			}
			if oc.GrantType != nil {
				oauthConfigObj["grant_type"] = types.StringValue(string(*oc.GrantType))
			}
			if oc.StreamableHttpPort != nil {
				oauthConfigObj["streamable_http_port"] = types.Int64Value(int64(*oc.StreamableHttpPort))
			}

			// Redirect URIs
			if len(oc.RedirectUris) > 0 {
				redirectValues := make([]attr.Value, len(oc.RedirectUris))
				for i, uri := range oc.RedirectUris {
					redirectValues[i] = types.StringValue(uri)
				}
				oauthConfigObj["redirect_uris"], _ = types.ListValue(types.StringType, redirectValues)
			}

			// Scopes
			if len(oc.Scopes) > 0 {
				scopeValues := make([]attr.Value, len(oc.Scopes))
				for i, scope := range oc.Scopes {
					scopeValues[i] = types.StringValue(scope)
				}
				oauthConfigObj["scopes"], _ = types.ListValue(types.StringType, scopeValues)
			}

			// Default scopes
			if len(oc.DefaultScopes) > 0 {
				dsValues := make([]attr.Value, len(oc.DefaultScopes))
				for i, s := range oc.DefaultScopes {
					dsValues[i] = types.StringValue(s)
				}
				oauthConfigObj["default_scopes"], _ = types.ListValue(types.StringType, dsValues)
			}

			remoteConfigObj["oauth_config"], _ = types.ObjectValue(oauthConfigAttrTypes, oauthConfigObj)
		} else {
			remoteConfigObj["oauth_config"] = types.ObjectNull(oauthConfigAttrTypes)
		}

		data.RemoteConfig, _ = types.ObjectValue(remoteConfigAttrTypes, remoteConfigObj)
	} else {
		data.RemoteConfig = types.ObjectNull(remoteConfigAttrTypes)
	}

	// Map AuthFields from API response if present
	if apiResp.JSON200.AuthFields != nil && len(*apiResp.JSON200.AuthFields) > 0 {
		authFieldValues := make([]attr.Value, len(*apiResp.JSON200.AuthFields))
		authFieldAttrTypes := map[string]attr.Type{
			"name":        types.StringType,
			"label":       types.StringType,
			"type":        types.StringType,
			"required":    types.BoolType,
			"description": types.StringType,
		}

		for i, af := range *apiResp.JSON200.AuthFields {
			authFieldMap := map[string]attr.Value{
				"name":        types.StringValue(af.Name),
				"label":       types.StringValue(af.Label),
				"type":        types.StringValue(af.Type),
				"required":    types.BoolValue(af.Required),
				"description": types.StringNull(),
			}
			if af.Description != nil {
				authFieldMap["description"] = types.StringValue(*af.Description)
			}
			authFieldValues[i], _ = types.ObjectValue(authFieldAttrTypes, authFieldMap)
		}
		data.AuthFields, _ = types.ListValue(types.ObjectType{AttrTypes: authFieldAttrTypes}, authFieldValues)
	} else {
		data.AuthFields = types.ListNull(types.ObjectType{AttrTypes: map[string]attr.Type{
			"name":        types.StringType,
			"label":       types.StringType,
			"type":        types.StringType,
			"required":    types.BoolType,
			"description": types.StringType,
		}})
	}
}

func (r *MCPServerRegistryResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data MCPServerRegistryResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Parse UUID from state
	serverID, err := uuid.Parse(data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid ID", fmt.Sprintf("Unable to parse MCP server ID: %s", err))
		return
	}

	// Call API
	apiResp, err := r.client.GetInternalMcpCatalogItemWithResponse(ctx, serverID)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to read MCP server, got error: %s", err))
		return
	}

	// Handle not found
	if apiResp.JSON404 != nil {
		resp.State.RemoveResource(ctx)
		return
	}

	// Check response
	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Expected 200 OK, got status %d", apiResp.StatusCode()),
		)
		return
	}

	r.mapGetResponseToState(ctx, &data, apiResp, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *MCPServerRegistryResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data MCPServerRegistryResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if !data.LocalConfig.IsNull() && !data.RemoteConfig.IsNull() {
		resp.Diagnostics.AddError(
			"Invalid Configuration",
			"Only one of 'local_config' or 'remote_config' can be specified, not both.",
		)
		return
	}
	if data.LocalConfig.IsNull() && data.RemoteConfig.IsNull() {
		resp.Diagnostics.AddError(
			"Invalid Configuration",
			"One of 'local_config' or 'remote_config' must be specified.",
		)
		return
	}
	if !data.LocalConfig.IsNull() {
		var lc LocalConfigModel
		resp.Diagnostics.Append(data.LocalConfig.As(ctx, &lc, basetypes.ObjectAsOptions{})...)
		if resp.Diagnostics.HasError() {
			return
		}
		if lc.Command.IsNull() && lc.DockerImage.IsNull() {
			resp.Diagnostics.AddError(
				"Invalid Configuration",
				"Either 'command' or 'docker_image' must be specified in 'local_config'.",
			)
			return
		}
	}

	serverID, err := uuid.Parse(data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid ID", fmt.Sprintf("Unable to parse MCP server ID: %s", err))
		return
	}

	plan := req.Plan.Raw
	prior := req.State.Raw
	patch := MergePatch(ctx, plan, prior, catalogItemAttrSpec, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	serverName := data.Name.ValueString()
	finalizeCatalogItemPatch(ctx, patch, plan, prior, serverName, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	// serverType is mode-derived and the backend treats it as required on
	// Update; always send so the backend's Zod schema accepts the body.
	if !data.RemoteConfig.IsNull() {
		patch["serverType"] = "remote"
	} else {
		patch["serverType"] = "local"
	}

	if data.RequiresAuth.IsNull() && !data.RemoteConfig.IsNull() {
		if !data.AuthFields.IsNull() || patch["oauthConfig"] != nil {
			patch["requiresAuth"] = true
		}
	}

	LogPatch(ctx, "update catalog item", patch, catalogItemAttrSpec)
	bodyBytes, err := json.Marshal(patch)
	if err != nil {
		resp.Diagnostics.AddError("Marshal Error", fmt.Sprintf("Unable to marshal request body: %s", err))
		return
	}
	apiResp, err := r.client.UpdateInternalMcpCatalogItemWithBodyWithResponse(ctx, serverID, "application/json", bytes.NewReader(bodyBytes))
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to update MCP server, got error: %s", err))
		return
	}
	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Expected 200 OK, got status %d: %s", apiResp.StatusCode(), string(apiResp.Body)),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
	readReq := resource.ReadRequest{State: resp.State}
	readResp := resource.ReadResponse{State: resp.State}
	r.Read(ctx, readReq, &readResp)
	resp.Diagnostics.Append(readResp.Diagnostics...)
	resp.State = readResp.State
}

func (r *MCPServerRegistryResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data MCPServerRegistryResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Parse UUID from state
	serverID, err := uuid.Parse(data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid ID", fmt.Sprintf("Unable to parse MCP server ID: %s", err))
		return
	}

	// Call API
	apiResp, err := r.client.DeleteInternalMcpCatalogItemWithResponse(ctx, serverID)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to delete MCP server, got error: %s", err))
		return
	}

	// Check response (200 or 404 are both acceptable for delete)
	if apiResp.JSON200 == nil && apiResp.JSON404 == nil {
		resp.Diagnostics.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Expected 200 OK or 404 Not Found, got status %d", apiResp.StatusCode()),
		)
		return
	}
}

func (r *MCPServerRegistryResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// strOrNull returns a null Terraform string for empty go strings, otherwise a string value.
func strOrNull(s string) types.String {
	if s == "" {
		return types.StringNull()
	}
	return types.StringValue(s)
}

// flattenUserConfig reads the backend's generic userConfig payload back into a Terraform map.
// Uses a JSON round-trip so callers can pass the generated anonymous-struct pointer directly.
func flattenUserConfig(raw interface{}) (types.Map, diag.Diagnostics) {
	var diags diag.Diagnostics
	objType := types.ObjectType{AttrTypes: userConfigAttrTypes}
	if raw == nil {
		return types.MapNull(objType), diags
	}

	b, err := json.Marshal(raw)
	if err != nil {
		diags.AddError("Failed to marshal user_config", err.Error())
		return types.MapNull(objType), diags
	}
	var entries map[string]map[string]interface{}
	if err := json.Unmarshal(b, &entries); err != nil {
		diags.AddError("Failed to unmarshal user_config", err.Error())
		return types.MapNull(objType), diags
	}
	if len(entries) == 0 {
		return types.MapNull(objType), diags
	}

	values := make(map[string]attr.Value, len(entries))
	for key, v := range entries {
		fieldVals := map[string]attr.Value{
			"title":                  types.StringNull(),
			"description":            types.StringNull(),
			"type":                   types.StringNull(),
			"default":                types.StringNull(),
			"required":               types.BoolNull(),
			"sensitive":              types.BoolNull(),
			"multiple":               types.BoolNull(),
			"min":                    types.Float64Null(),
			"max":                    types.Float64Null(),
			"header_name":            types.StringNull(),
			"prompt_on_installation": types.BoolNull(),
		}
		if s, ok := v["title"].(string); ok {
			fieldVals["title"] = types.StringValue(s)
		}
		if s, ok := v["description"].(string); ok {
			fieldVals["description"] = types.StringValue(s)
		}
		if s, ok := v["type"].(string); ok {
			fieldVals["type"] = types.StringValue(s)
		}
		if d, ok := v["default"]; ok && d != nil {
			if s, isStr := d.(string); isStr {
				fieldVals["default"] = types.StringValue(s)
			} else {
				encoded, _ := json.Marshal(d)
				fieldVals["default"] = types.StringValue(string(encoded))
			}
		}
		if b, ok := v["required"].(bool); ok {
			fieldVals["required"] = types.BoolValue(b)
		}
		if b, ok := v["sensitive"].(bool); ok {
			fieldVals["sensitive"] = types.BoolValue(b)
		}
		if b, ok := v["multiple"].(bool); ok {
			fieldVals["multiple"] = types.BoolValue(b)
		}
		if n, ok := v["min"].(float64); ok {
			fieldVals["min"] = types.Float64Value(n)
		}
		if n, ok := v["max"].(float64); ok {
			fieldVals["max"] = types.Float64Value(n)
		}
		if s, ok := v["headerName"].(string); ok {
			fieldVals["header_name"] = types.StringValue(s)
		}
		if b, ok := v["promptOnInstallation"].(bool); ok {
			fieldVals["prompt_on_installation"] = types.BoolValue(b)
		}
		obj, d := types.ObjectValue(userConfigAttrTypes, fieldVals)
		diags.Append(d...)
		values[key] = obj
	}
	result, d := types.MapValue(objType, values)
	diags.Append(d...)
	return result, diags
}
