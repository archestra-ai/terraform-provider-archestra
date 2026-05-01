package provider

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// 2m is a backstop, not a throttle: the retry helper tops out near 100s of
// cumulative wait, so this leaves headroom without papering over a hung backend.
const defaultHTTPTimeout = 2 * time.Minute

const envHTTPTimeout = "ARCHESTRA_HTTP_TIMEOUT"

// Ensure provider defined types fully satisfy framework interfaces.
var _ provider.Provider = &ArchestraProvider{}

// ArchestraProvider defines the provider implementation.
type ArchestraProvider struct {
	// version is set to the provider version on release, "dev" when the
	// provider is built and ran locally, and "test" when running acceptance
	// testing.
	version string
}

// ArchestraProviderModel describes the provider data model.
type ArchestraProviderModel struct {
	BaseURL types.String `tfsdk:"base_url"`
	APIKey  types.String `tfsdk:"api_key"`
}

func (p *ArchestraProvider) Metadata(ctx context.Context, req provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "archestra"
	resp.Version = p.version
}

func (p *ArchestraProvider) Schema(ctx context.Context, req provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "The Archestra provider lets you manage Archestra resources " +
			"(agents, MCP servers, identity providers, teams, LLM keys, security " +
			"policies, organization settings) as code. Both configuration values " +
			"can — and should — be supplied via environment variables so secrets " +
			"never enter HCL.",
		Attributes: map[string]schema.Attribute{
			"base_url": schema.StringAttribute{
				MarkdownDescription: "Base URL of the Archestra API (for example, `https://archestra.your-company.example`). " +
					"Defaults to `http://localhost:9000` if neither this attribute nor `ARCHESTRA_BASE_URL` is set. " +
					"Also reads from the `ARCHESTRA_BASE_URL` environment variable.",
				Optional: true,
			},
			"api_key": schema.StringAttribute{
				MarkdownDescription: "**Required for any operation that talks to the Archestra API.** Marked Optional in the schema only so the value can be supplied via the `ARCHESTRA_API_KEY` environment variable instead of inline HCL — prefer the env var to keep secrets out of source control. " +
					"Mint a key in the Archestra UI under Settings → API Keys (the value starts with `arch_`).",
				Optional:  true,
				Sensitive: true,
			},
		},
	}
}

func (p *ArchestraProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var config ArchestraProviderModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Configuration values are now available.
	baseURL := config.BaseURL.ValueString()
	apiKey := config.APIKey.ValueString()

	// If practitioner provided a configuration value for any of the
	// attributes, it must be a known value.

	if config.BaseURL.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			path.Root("base_url"),
			"Unknown Archestra API Base URL",
			"The provider cannot create the Archestra API client as there is an unknown configuration value for the Archestra API base URL. "+
				"Either target apply the source of the value first, set the value statically in the configuration, or use the ARCHESTRA_BASE_URL environment variable.",
		)
	}

	if config.APIKey.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			path.Root("api_key"),
			"Unknown Archestra API Key",
			"The provider cannot create the Archestra API client as there is an unknown configuration value for the Archestra API key. "+
				"Either target apply the source of the value first, set the value statically in the configuration, or use the ARCHESTRA_API_KEY environment variable.",
		)
	}

	if resp.Diagnostics.HasError() {
		return
	}

	// Default values to environment variables, but override
	// with Terraform configuration value if set.

	if baseURL == "" {
		if envBaseURL := os.Getenv("ARCHESTRA_BASE_URL"); envBaseURL != "" {
			baseURL = envBaseURL
		} else {
			baseURL = "http://localhost:9000"
		}
	}

	if apiKey == "" {
		if envAPIKey := os.Getenv("ARCHESTRA_API_KEY"); envAPIKey != "" {
			apiKey = envAPIKey
		} else {
			resp.Diagnostics.AddAttributeError(
				path.Root("api_key"),
				"Missing Archestra API Key",
				"The provider cannot create the Archestra API client as there is a missing or empty value for the Archestra API key. "+
					"Set the api_key value in the configuration or use the ARCHESTRA_API_KEY environment variable. "+
					"If either is already set, ensure the value is not empty.",
			)
		}
	}

	if resp.Diagnostics.HasError() {
		return
	}

	httpClient, err := buildHTTPClient()
	if err != nil {
		resp.Diagnostics.AddError("Invalid "+envHTTPTimeout, err.Error())
		return
	}

	// Create a new Archestra client using the configuration values
	apiClient, err := client.NewClientWithResponses(
		baseURL,
		client.WithHTTPClient(httpClient),
		client.WithRequestEditorFn(func(ctx context.Context, req *http.Request) error {
			req.Header.Set("Authorization", apiKey)
			return nil
		}),
	)

	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to Create Archestra API Client",
			"An unexpected error occurred when creating the Archestra API client. "+
				"If the error is not clear, please contact the provider developers.\n\n"+
				"Archestra Client Error: "+err.Error(),
		)
		return
	}

	// Make the Archestra client available during DataSource and Resource
	// type Configure methods.
	resp.DataSourceData = apiClient
	resp.ResourceData = apiClient
}

func (p *ArchestraProvider) Resources(ctx context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewAgentResource,
		NewLlmProxyResource,
		NewMcpGatewayResource,
		NewMCPServerResource,
		NewMCPServerRegistryResource,
		NewTrustedDataPolicyResource,
		NewToolInvocationPolicyResource,
		NewTeamResource,
		NewLlmModelResource,
		NewLimitResource,
		NewOptimizationRuleResource,
		NewOrganizationSettingsResource,
		NewTeamExternalGroupResource,
		NewLLMProviderApiKeyResource,
		NewAgentToolResource,
		NewAgentToolBatchResource,
		NewIdentityProviderResource,
		NewToolInvocationPolicyDefaultResource,
		NewTrustedDataPolicyDefaultResource,
		NewToolPolicyAutoConfigResource,
	}
}

func (p *ArchestraProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewTeamDataSource,
		NewToolDataSource,
		NewAgentToolDataSource,
		NewAgentToolsDataSource,
		NewMCPServerToolDataSource,
		NewMcpToolCallsDataSource,
		NewTeamExternalGroupsDataSource,
	}
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &ArchestraProvider{
			version: version,
		}
	}
}

func buildHTTPClient() (*http.Client, error) {
	timeout, err := resolveHTTPTimeout(os.Getenv(envHTTPTimeout))
	if err != nil {
		return nil, err
	}
	return &http.Client{
		Timeout:   timeout,
		Transport: newHTTPTransport(),
	}, nil
}

// http.Client treats Timeout == 0 as "no timeout" — the exact failure mode
// this setting exists to prevent — so reject zero and negatives.
func resolveHTTPTimeout(raw string) (time.Duration, error) {
	if raw == "" {
		return defaultHTTPTimeout, nil
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("%s=%q is not a valid Go duration (e.g. \"30s\", \"5m\"): %w", envHTTPTimeout, raw, err)
	}
	if d <= 0 {
		return 0, fmt.Errorf("%s=%q must be positive; zero disables the timeout", envHTTPTimeout, raw)
	}
	return d, nil
}

// Cloning DefaultTransport (vs. zeroing one) is load-bearing: it preserves
// Proxy, DialContext, and idle-connection settings. &http.Transport{} silently
// breaks HTTPS_PROXY.
func newHTTPTransport() *http.Transport {
	base, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		// http.DefaultTransport is always *http.Transport in stdlib;
		// branch exists for the forcetypeassert linter.
		base = &http.Transport{}
	}
	t := base.Clone()
	t.TLSHandshakeTimeout = 10 * time.Second
	return t
}
