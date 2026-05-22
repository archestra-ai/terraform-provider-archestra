package config

import (
	// Note(turkenh): we are importing this to embed provider schema document
	_ "embed"

	ujconfig "github.com/crossplane/upjet/v2/pkg/config"

	agentCluster "github.com/archestra-ai/provider-archestra/config/cluster/agent"
	mcpCluster "github.com/archestra-ai/provider-archestra/config/cluster/mcp"
	policyCluster "github.com/archestra-ai/provider-archestra/config/cluster/policy"
	agentNamespaced "github.com/archestra-ai/provider-archestra/config/namespaced/agent"
	mcpNamespaced "github.com/archestra-ai/provider-archestra/config/namespaced/mcp"
	policyNamespaced "github.com/archestra-ai/provider-archestra/config/namespaced/policy"
)

const (
	resourcePrefix = "archestra"
	modulePath     = "github.com/archestra-ai/provider-archestra"
)

//go:embed schema.json
var providerSchema string

//go:embed provider-metadata.yaml
var providerMetadata string

// GetProvider returns provider configuration
func GetProvider() *ujconfig.Provider {
	pc := ujconfig.NewProvider([]byte(providerSchema), resourcePrefix, modulePath, []byte(providerMetadata),
		ujconfig.WithRootGroup("archestra.crossplane.io"),
		ujconfig.WithIncludeList(ExternalNameConfigured()),
		ujconfig.WithFeaturesPackage("internal/features"),
		ujconfig.WithDefaultResourceOptions(
			ExternalNameConfigurations(),
		))

	for _, configure := range []func(provider *ujconfig.Provider){
		mcpCluster.Configure,
		agentCluster.Configure,
		policyCluster.Configure,
	} {
		configure(pc)
	}

	pc.ConfigureResources()
	return pc
}

// GetProviderNamespaced returns the namespaced provider configuration
func GetProviderNamespaced() *ujconfig.Provider {
	pc := ujconfig.NewProvider([]byte(providerSchema), resourcePrefix, modulePath, []byte(providerMetadata),
		ujconfig.WithRootGroup("archestra.m.crossplane.io"),
		ujconfig.WithIncludeList(ExternalNameConfigured()),
		ujconfig.WithFeaturesPackage("internal/features"),
		ujconfig.WithDefaultResourceOptions(
			ExternalNameConfigurations(),
		),
		ujconfig.WithExampleManifestConfiguration(ujconfig.ExampleManifestConfiguration{
			ManagedResourceNamespace: "crossplane-system",
		}))

	for _, configure := range []func(provider *ujconfig.Provider){
		mcpNamespaced.Configure,
		agentNamespaced.Configure,
		policyNamespaced.Configure,
	} {
		configure(pc)
	}

	pc.ConfigureResources()
	return pc
}
