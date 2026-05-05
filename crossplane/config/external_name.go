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
// Archestra resources all use server-generated UUIDs, so IdentifierFromProvider
// is the right choice for every resource here. This map also functions as the
// upjet IncludeList — only resources listed here become Crossplane MRs.
var ExternalNameConfigs = map[string]ujconfig.ExternalName{
	"archestra_mcp_registry_catalog_item":      ujconfig.IdentifierFromProvider,
	"archestra_mcp_server_installation":        ujconfig.IdentifierFromProvider,
	"archestra_agent":                          ujconfig.IdentifierFromProvider,
	"archestra_agent_tool_batch":               ujconfig.IdentifierFromProvider,
	"archestra_tool_invocation_policy_default": ujconfig.IdentifierFromProvider,
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
