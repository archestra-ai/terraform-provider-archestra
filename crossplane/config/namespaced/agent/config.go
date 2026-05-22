/*
Copyright 2026 Archestra AI.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0
*/

// Package agent configures upjet for the agent-domain Archestra resources.
package agent

import ujconfig "github.com/crossplane/upjet/v2/pkg/config"

// Configure registers per-resource overrides for the agent.archestra.crossplane.io group.
func Configure(p *ujconfig.Provider) {
	p.AddResourceConfigurator("archestra_agent", func(r *ujconfig.Resource) {
		r.ShortGroup = "agent"
		r.Kind = "Agent"
	})

	p.AddResourceConfigurator("archestra_agent_tool_batch", func(r *ujconfig.Resource) {
		r.ShortGroup = "agent"
		r.Kind = "ToolBatch"
		r.References["agent_id"] = ujconfig.Reference{
			TerraformName: "archestra_agent",
		}
		r.References["mcp_server_id"] = ujconfig.Reference{
			TerraformName: "archestra_mcp_server_installation",
		}
		// tool_ids is a list of bare tool UUIDs from
		// archestra_mcp_server_installation.tool_id_by_name — no Crossplane
		// MR exists for individual tools, so this stays a literal list.
	})
}
