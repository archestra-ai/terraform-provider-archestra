/*
Copyright 2026 Archestra AI.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0
*/

package config

import ujconfig "github.com/crossplane/upjet/v2/pkg/config"

// ExternalNameConfigs maps Terraform resource → ExternalName scheme.
// IDs include server-generated UUIDs, credential keys, and composite IDs.
// IdentifierFromProvider preserves the exact ID returned by each Terraform resource. This map also functions as the
// upjet IncludeList — only resources listed here become Crossplane MRs.
var ExternalNameConfigs = map[string]ujconfig.ExternalName{
	"archestra_knowledge_file":          ujconfig.IdentifierFromProvider,
	"archestra_project_file":            ujconfig.IdentifierFromProvider,
	"archestra_share_link":              ujconfig.IdentifierFromProvider,
	"archestra_plugin":                  ujconfig.IdentifierFromProvider,
	"archestra_github_pat":              ujconfig.IdentifierFromProvider,
	"archestra_github_app":              ujconfig.IdentifierFromProvider,
	"archestra_virtual_key":             ujconfig.IdentifierFromProvider,
	"archestra_oauth_client":            ujconfig.IdentifierFromProvider,
	"archestra_remote_agent":            ujconfig.IdentifierFromProvider,
	"archestra_service_account_api_key": ujconfig.IdentifierFromProvider,
	"archestra_service_account":         ujconfig.IdentifierFromProvider,
	"archestra_knowledge_directory":     ujconfig.IdentifierFromProvider,
	"archestra_project":                 ujconfig.IdentifierFromProvider,
	"archestra_knowledge_base":          ujconfig.IdentifierFromProvider,
	"archestra_role":                    ujconfig.IdentifierFromProvider,
	"archestra_api_key":                 ujconfig.IdentifierFromProvider,
	"archestra_schedule":                ujconfig.IdentifierFromProvider,
	"archestra_environment":             ujconfig.IdentifierFromProvider,
	"archestra_agent":                   ujconfig.IdentifierFromProvider,
	"archestra_skill":                   ujconfig.IdentifierFromProvider,
	"archestra_identity_provider":       ujconfig.IdentifierFromProvider,
	"archestra_limit":                   ujconfig.IdentifierFromProvider,
	"archestra_model":                   ujconfig.IdentifierFromProvider,
	"archestra_model_provider":          ujconfig.IdentifierFromProvider,
	"archestra_mcp_gateway":             ujconfig.IdentifierFromProvider,
	"archestra_mcp_registry_entry":      ujconfig.IdentifierFromProvider,
	"archestra_mcp_server":              ujconfig.IdentifierFromProvider,
	"archestra_organization_settings":   ujconfig.IdentifierFromProvider,
	"archestra_credential":              ujconfig.IdentifierFromProvider,
	"archestra_team":                    ujconfig.IdentifierFromProvider,
}

// ExternalNameConfigurations applies external-name configs as a default
// ResourceOption in NewProvider.
func ExternalNameConfigurations() ujconfig.ResourceOption {
	return func(r *ujconfig.Resource) {
		if e, ok := ExternalNameConfigs[r.Name]; ok {
			r.ExternalName = e
		}
	}
}

// ExternalNameConfigured returns the resource names that have an explicit
// external-name config, used as the upjet IncludeList.
func ExternalNameConfigured() []string {
	out := make([]string, 0, len(ExternalNameConfigs))
	for k := range ExternalNameConfigs {
		// upjet treats the include list as regex; anchor for exact match.
		out = append(out, k+"$")
	}
	return out
}
