/*
Copyright 2026 Archestra AI.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0
*/

// Package mcp configures upjet for the MCP-domain Archestra resources.
package mcp

import ujconfig "github.com/crossplane/upjet/v2/pkg/config"

// Configure registers per-resource overrides for the mcp.archestra.crossplane.io group.
func Configure(p *ujconfig.Provider) {
	p.AddResourceConfigurator("archestra_mcp_registry_catalog_item", func(r *ujconfig.Resource) {
		r.ShortGroup = "mcp"
		r.Kind = "RegistryCatalogItem"
	})

	p.AddResourceConfigurator("archestra_mcp_server_installation", func(r *ujconfig.Resource) {
		r.ShortGroup = "mcp"
		r.Kind = "ServerInstallation"
		// catalog_id references RegistryCatalogItem in the same group.
		r.References["catalog_id"] = ujconfig.Reference{
			TerraformName: "archestra_mcp_registry_catalog_item",
		}
	})
}
