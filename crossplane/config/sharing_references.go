/*
Copyright 2026 Archestra AI.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0
*/

package config

import ujconfig "github.com/crossplane/upjet/v2/pkg/config"

// configureSharingReferences lets a sharing grant name a team or service
// account managed in the same sync (teamRef, serviceAccountRef), and a limit
// name its team, virtual key, agent, environment or OAuth client the same way.
// Every resource whose Terraform schema has a `permissions` set gets the grant
// references, so new shareable resources are covered without listing them.
func configureSharingReferences(p *ujconfig.Provider) {
	for name, resource := range p.Resources {
		if resource.TerraformResource == nil || resource.TerraformResource.Schema["permissions"] == nil {
			continue
		}
		p.AddResourceConfigurator(name, func(r *ujconfig.Resource) { addGrantReferences(r, "permissions") })
	}

	p.AddResourceConfigurator("archestra_limit", func(r *ujconfig.Resource) {
		for field, target := range limitTargetReferences {
			r.References[field] = ujconfig.Reference{TerraformName: target}
		}
		// The entity is planned from whichever form is set. Late-initializing
		// the observed entity_type, entity_id or another target would set two.
		r.LateInitializer.IgnoredFields = append(r.LateInitializer.IgnoredFields, "entity_type", "entity_id")
		for field := range limitTargetReferences {
			r.LateInitializer.IgnoredFields = append(r.LateInitializer.IgnoredFields, field)
		}
	})
}

var limitTargetReferences = map[string]string{
	"team_id":             "archestra_team",
	"virtual_key_id":      "archestra_virtual_key",
	"agent_id":            "archestra_agent",
	"environment_id":      "archestra_environment",
	"llm_oauth_client_id": "archestra_oauth_client",
}

func addGrantReferences(r *ujconfig.Resource, field string) {
	r.References[field+".team_id"] = ujconfig.Reference{TerraformName: "archestra_team"}
	r.References[field+".service_account_id"] = ujconfig.Reference{TerraformName: "archestra_service_account"}
}
