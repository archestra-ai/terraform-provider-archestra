/*
Copyright 2026 Archestra AI.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0
*/

// Package policy configures upjet for the policy-domain Archestra resources.
package policy

import ujconfig "github.com/crossplane/upjet/v2/pkg/config"

// Configure registers per-resource overrides for the policy.archestra.crossplane.io group.
func Configure(p *ujconfig.Provider) {
	p.AddResourceConfigurator("archestra_tool_invocation_policy_default", func(r *ujconfig.Resource) {
		r.ShortGroup = "policy"
		r.Kind = "ToolInvocationPolicyDefault"
		// tool_ids is a list of bare tool UUIDs — the same caveat as
		// agent_tool_batch.tool_ids: no Crossplane MR exists for individual
		// tools, so this stays a literal list rather than a reference.
	})
}
