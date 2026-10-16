package config_test

import (
	"encoding/json"
	"testing"

	agent "github.com/archestra-ai/provider-archestra/apis/cluster/agent/v1alpha1"
	permission "github.com/archestra-ai/provider-archestra/apis/cluster/permission/v1alpha1"
	agentnamespaced "github.com/archestra-ai/provider-archestra/apis/namespaced/agent/v1alpha1"
)

func TestGeneratedEmptyRequiredCollectionsReachTerraform(t *testing.T) {
	resources := []struct {
		name   string
		get    func() (map[string]any, error)
		fields []string
	}{
		{"cluster skill policy", (&agent.SkillPolicy{Spec: agent.SkillPolicySpec{ForProvider: agent.SkillPolicyParameters{AllowedReferences: []agent.AllowedReferencesParameters{}, ExcludedReferences: []agent.ExcludedReferencesParameters{}}}}).GetParameters, []string{"allowed_references", "excluded_references"}},
		{"namespaced skill policy", (&agentnamespaced.SkillPolicy{Spec: agentnamespaced.SkillPolicySpec{ForProvider: agentnamespaced.SkillPolicyParameters{AllowedReferences: []agentnamespaced.AllowedReferencesParameters{}, ExcludedReferences: []agentnamespaced.ExcludedReferencesParameters{}}}}).GetParameters, []string{"allowed_references", "excluded_references"}},
		{"cluster subagent exclusions", (&agent.SubagentExclusions{Spec: agent.SubagentExclusionsSpec{ForProvider: agent.SubagentExclusionsParameters{ExcludedSubagentIds: []*string{}}}}).GetParameters, []string{"excluded_subagent_ids"}},
		{"cluster grants", (&permission.Policy{Spec: permission.PolicySpec{ForProvider: permission.PolicyParameters{Grants: []permission.GrantsParameters{}}}}).GetParameters, []string{"grants"}},
	}
	for _, r := range resources {
		t.Run(r.name, func(t *testing.T) {
			parameters, err := r.get()
			if err != nil {
				t.Fatal(err)
			}
			for _, name := range r.fields {
				value, ok := parameters[name]
				if !ok {
					t.Fatalf("required empty set %q omitted", name)
				}
				items, ok := value.([]any)
				if !ok || len(items) != 0 {
					t.Fatalf("required empty set %q serialized as %#v", name, value)
				}
			}
		})
	}
}

func TestGeneratedEmptyRequiredCollectionsSurviveKubernetesSerialization(t *testing.T) {
	for _, value := range []any{
		&agent.SkillPolicy{Spec: agent.SkillPolicySpec{ForProvider: agent.SkillPolicyParameters{AllowedReferences: []agent.AllowedReferencesParameters{}, ExcludedReferences: []agent.ExcludedReferencesParameters{}}}},
		&agentnamespaced.SkillPolicy{Spec: agentnamespaced.SkillPolicySpec{ForProvider: agentnamespaced.SkillPolicyParameters{AllowedReferences: []agentnamespaced.AllowedReferencesParameters{}, ExcludedReferences: []agentnamespaced.ExcludedReferencesParameters{}}}},
	} {
		encoded, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		var decoded struct {
			Spec struct {
				ForProvider map[string]any `json:"forProvider"`
			} `json:"spec"`
		}
		if err := json.Unmarshal(encoded, &decoded); err != nil {
			t.Fatal(err)
		}
		for _, name := range []string{"allowedReferences", "excludedReferences"} {
			v, ok := decoded.Spec.ForProvider[name]
			if !ok {
				t.Fatalf("Kubernetes JSON omitted required empty set %q", name)
			}
			items, ok := v.([]any)
			if !ok || len(items) != 0 {
				t.Fatalf("Kubernetes JSON set %q: %#v", name, v)
			}
		}
	}
}
