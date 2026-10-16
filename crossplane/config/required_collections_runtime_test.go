package config_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	ujresource "github.com/crossplane/upjet/v2/pkg/resource"
	apiextensions "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"sigs.k8s.io/yaml"

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

func TestGeneratedInitialGrantsPreserveExplicitEmptyAndUnmanagedValues(t *testing.T) {
	cluster := &agent.Agent{Spec: agent.AgentSpec{ForProvider: agent.AgentParameters{InitialGrants: []agent.InitialGrantsParameters{}}}}
	namespaced := &agentnamespaced.Agent{Spec: agentnamespaced.AgentSpec{ForProvider: agentnamespaced.AgentParameters{InitialGrants: []agentnamespaced.InitialGrantsParameters{}}}}
	for _, value := range []interface {
		GetParameters() (map[string]any, error)
	}{cluster, namespaced} {
		parameters, err := value.GetParameters()
		if err != nil {
			t.Fatal(err)
		}
		values, ok := parameters["initial_grants"].([]any)
		if !ok || len(values) != 0 {
			t.Fatalf("explicit empty initial grants must reach Terraform: %#v", parameters["initial_grants"])
		}
		encoded, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		var wire struct {
			Spec struct {
				ForProvider map[string]any `json:"forProvider"`
			} `json:"spec"`
		}
		if err = json.Unmarshal(encoded, &wire); err != nil {
			t.Fatal(err)
		}
		if values, ok := wire.Spec.ForProvider["initialGrants"].([]any); !ok || len(values) != 0 {
			t.Fatalf("explicit empty initial grants must survive Kubernetes JSON: %s", encoded)
		}
	}
	for _, value := range []interface {
		GetMergedParameters(bool) (map[string]any, error)
	}{
		&agent.Agent{},
		&agentnamespaced.Agent{},
	} {
		parameters, err := value.GetMergedParameters(true)
		if err != nil {
			t.Fatal(err)
		}
		if _, present := parameters["initial_grants"]; present {
			t.Fatalf("omitted creation grants must be absent for Upjet ignore_changes: %#v", parameters["initial_grants"])
		}
		ignored := ujresource.GetTerraformIgnoreChanges(parameters, map[string]any{"initial_grants": []any{}})
		if len(ignored) != 1 || ignored[0] != "initial_grants" {
			t.Fatalf("initProvider creation grants must be ignored after create: %#v", ignored)
		}
	}
	for _, value := range []interface {
		GetMergedParameters(bool) (map[string]any, error)
	}{
		&agent.Agent{Spec: agent.AgentSpec{InitProvider: agent.AgentInitParameters{InitialGrants: []agent.InitialGrantsInitParameters{}}}},
		&agentnamespaced.Agent{Spec: agentnamespaced.AgentSpec{InitProvider: agentnamespaced.AgentInitParameters{InitialGrants: []agentnamespaced.InitialGrantsInitParameters{}}}},
	} {
		parameters, err := value.GetMergedParameters(true)
		if err != nil {
			t.Fatal(err)
		}
		if values, ok := parameters["initial_grants"].([]any); !ok || len(values) != 0 {
			t.Fatalf("explicit empty initProvider grants were dropped: %#v", parameters["initial_grants"])
		}
	}
}

func TestGeneratedInitialGrantCRDsKeepUnmanagedInputsOptional(t *testing.T) {
	files, err := filepath.Glob("../package/crds/*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	checked := 0
	for _, file := range files {
		body, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		var crd apiextensions.CustomResourceDefinition
		if err = yaml.Unmarshal(body, &crd); err != nil {
			t.Fatal(err)
		}
		for _, version := range crd.Spec.Versions {
			fields := version.Schema.OpenAPIV3Schema.Properties["spec"].Properties
			if _, ok := fields["forProvider"].Properties["initialGrants"]; !ok {
				continue
			}
			for _, source := range []string{"forProvider", "initProvider"} {
				for _, required := range fields[source].Required {
					if required == "initialGrants" {
						t.Fatalf("%s %s rejects an unmanaged creation policy", crd.Name, source)
					}
				}
			}
			checked++
		}
	}
	if checked != 16 {
		t.Fatalf("expected both-scope creation-grant schemas for eight families, checked %d", checked)
	}
}

func TestGeneratedCreationGrantsAreNotLateInitialized(t *testing.T) {
	for _, value := range []interface {
		LateInitialize([]byte) (bool, error)
		GetParameters() (map[string]any, error)
	}{&agent.Agent{}, &agentnamespaced.Agent{}} {
		if _, err := value.LateInitialize([]byte(`{"initial_grants": []}`)); err != nil {
			t.Fatal(err)
		}
		parameters, err := value.GetParameters()
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := parameters["initial_grants"]; ok {
			t.Fatal("creation grants moved from initProvider to managed forProvider inputs")
		}
	}
}
