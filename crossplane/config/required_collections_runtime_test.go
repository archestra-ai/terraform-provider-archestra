package config_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/archestra-ai/provider-archestra/config"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	ujresource "github.com/crossplane/upjet/v2/pkg/resource"
	apiextensions "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"sigs.k8s.io/yaml"

	agent "github.com/archestra-ai/provider-archestra/apis/cluster/agent/v1alpha1"
	mcp "github.com/archestra-ai/provider-archestra/apis/cluster/gateway/v1alpha1"
	agentnamespaced "github.com/archestra-ai/provider-archestra/apis/namespaced/agent/v1alpha1"
	mcpnamespaced "github.com/archestra-ai/provider-archestra/apis/namespaced/gateway/v1alpha1"
)

type inlineParameters interface {
	GetParameters() (map[string]any, error)
	GetMergedParameters(bool) (map[string]any, error)
	ResolveReferences(context.Context, client.Reader) error
}

func TestGeneratedInlineCollectionsReachTerraformAndKubernetes(t *testing.T) {
	cases := []struct {
		name           string
		empty, omitted inlineParameters
		hooks          bool
	}{
		{"cluster agent", &agent.Agent{Spec: agent.AgentSpec{ForProvider: agent.AgentParameters{SubagentIds: []*string{}, RemoteAgentIds: []*string{}, ExcludedSubagentIds: []*string{}, ExcludedConnectorIds: []*string{}, Tools: []agent.ToolsParameters{}, ExcludedToolIds: []*string{}, Hooks: []agent.HooksParameters{{Requirements: []*string{}}}, Skills: &agent.SkillsParameters{Allowed: []agent.AllowedParameters{}, Excluded: []agent.ExcludedParameters{}}}}}, &agent.Agent{}, true},
		{"namespaced agent", &agentnamespaced.Agent{Spec: agentnamespaced.AgentSpec{ForProvider: agentnamespaced.AgentParameters{SubagentIds: []*string{}, RemoteAgentIds: []*string{}, ExcludedSubagentIds: []*string{}, ExcludedConnectorIds: []*string{}, Tools: []agentnamespaced.ToolsParameters{}, ExcludedToolIds: []*string{}, Hooks: []agentnamespaced.HooksParameters{{Requirements: []*string{}}}, Skills: &agentnamespaced.SkillsParameters{Allowed: []agentnamespaced.AllowedParameters{}, Excluded: []agentnamespaced.ExcludedParameters{}}}}}, &agentnamespaced.Agent{}, true},
		{"cluster gateway", &mcp.Gateway{Spec: mcp.GatewaySpec{ForProvider: mcp.GatewayParameters{SubagentIds: []*string{}, RemoteAgentIds: []*string{}, ExcludedSubagentIds: []*string{}, ExcludedConnectorIds: []*string{}, Tools: []mcp.ToolsParameters{}, ExcludedToolIds: []*string{}, Skills: &mcp.SkillsParameters{Ids: []*string{}, ExcludedIds: []*string{}}}}}, &mcp.Gateway{}, false},
		{"namespaced gateway", &mcpnamespaced.Gateway{Spec: mcpnamespaced.GatewaySpec{ForProvider: mcpnamespaced.GatewayParameters{SubagentIds: []*string{}, RemoteAgentIds: []*string{}, ExcludedSubagentIds: []*string{}, ExcludedConnectorIds: []*string{}, Tools: []mcpnamespaced.ToolsParameters{}, ExcludedToolIds: []*string{}, Skills: &mcpnamespaced.SkillsParameters{Ids: []*string{}, ExcludedIds: []*string{}}}}}, &mcpnamespaced.Gateway{}, false},
	}
	for _, item := range cases {
		t.Run(item.name, func(t *testing.T) {
			parameters, err := item.empty.GetParameters()
			if err != nil {
				t.Fatal(err)
			}
			encoded, err := json.Marshal(item.empty)
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
			for tfName, jsonName := range map[string]string{"subagent_ids": "subagentIds", "remote_agent_ids": "remoteAgentIds", "excluded_subagent_ids": "excludedSubagentIds", "excluded_connector_ids": "excludedConnectorIds", "tools": "tools", "excluded_tool_ids": "excludedToolIds"} {
				assertEmptyCollection(t, parameters, tfName)
				assertEmptyCollection(t, wire.Spec.ForProvider, jsonName)
			}
			skillKeys := []string{"ids", "excluded_ids"}
			jsonSkillKeys := []string{"ids", "excludedIds"}
			if item.hooks {
				skillKeys = []string{"allowed", "excluded"}
				jsonSkillKeys = skillKeys
				hooks := parameters["hooks"].([]any)
				assertEmptyCollection(t, hooks[0].(map[string]any), "requirements")
				wireHooks := wire.Spec.ForProvider["hooks"].([]any)
				assertEmptyCollection(t, wireHooks[0].(map[string]any), "requirements")
			}
			for _, name := range skillKeys {
				assertEmptyCollection(t, parameters["skills"].(map[string]any), name)
			}
			for _, name := range jsonSkillKeys {
				assertEmptyCollection(t, wire.Spec.ForProvider["skills"].(map[string]any), name)
			}
			if err := item.omitted.ResolveReferences(t.Context(), fake.NewClientBuilder().Build()); err != nil {
				t.Fatal(err)
			}
			omitted, err := item.omitted.GetMergedParameters(true)
			if err != nil {
				t.Fatal(err)
			}
			for _, name := range []string{"subagent_ids", "remote_agent_ids", "excluded_subagent_ids", "excluded_connector_ids", "tools", "excluded_tool_ids", "hooks"} {
				if _, ok := omitted[name]; ok {
					t.Errorf("unmanaged collection %s was emitted: %#v", name, omitted[name])
				}
			}
		})
	}
}
func assertEmptyCollection(t *testing.T, parameters map[string]any, name string) {
	t.Helper()
	items, ok := parameters[name].([]any)
	if !ok || len(items) != 0 {
		t.Fatalf("explicit empty collection %s was dropped or changed: %#v", name, parameters[name])
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
	expected := 0
	for _, r := range config.GetProvider().Resources {
		if _, ok := r.TerraformResource.Schema["initial_grants"]; ok {
			expected += 2
		}
	}
	if checked != expected || expected == 0 {
		t.Fatalf("expected %d both-scope creation-grant schemas, checked %d", expected, checked)
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
