package config_test

import (
	"context"
	"encoding/json"
	"testing"

	cluster "github.com/archestra-ai/provider-archestra/apis/cluster/organization/v1alpha1"
	namespaced "github.com/archestra-ai/provider-archestra/apis/namespaced/organization/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

type settingsParameters interface {
	GetParameters() (map[string]any, error)
	GetInitParameters() (map[string]any, error)
	GetMergedParameters(bool) (map[string]any, error)
	ResolveReferences(context.Context, client.Reader) error
}

func TestOrganizationDefaultCollectionsPreserveClearAndOmission(t *testing.T) {
	for _, tc := range []struct {
		name        string
		newResource func() settingsParameters
	}{
		{"cluster", func() settingsParameters { return &cluster.Settings{} }},
		{"namespaced", func() settingsParameters { return &namespaced.Settings{} }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			omitted := tc.newResource()
			parameters, err := omitted.GetParameters()
			if err != nil {
				t.Fatal(err)
			}
			for _, field := range []string{"default_user_limits", "default_permissions", "environment_defaults"} {
				if _, ok := parameters[field]; ok {
					t.Fatalf("unmanaged %s appeared in inputs", field)
				}
			}
			for _, source := range []string{"forProvider", "initProvider"} {
				value := tc.newResource()
				raw := []byte(`{"spec":{"` + source + `":{"defaultUserLimits":{"default":{"limitValue":10,"models":[],"cleanupInterval":"calendar_month"}},"defaultPermissions":{"agents":[]},"environmentDefaults":{}}}}`)
				if err := json.Unmarshal(raw, value); err != nil {
					t.Fatal(err)
				}
				getter := value.GetParameters
				if source == "initProvider" {
					getter = value.GetInitParameters
				}
				parameters, err := getter()
				if err != nil {
					t.Fatal(err)
				}
				assertSettingEmptyCollections(t, parameters)
				merged, err := value.GetMergedParameters(true)
				if err != nil {
					t.Fatal(err)
				}
				assertSettingEmptyCollections(t, merged)
				stored, err := json.Marshal(value)
				if err != nil {
					t.Fatal(err)
				}
				roundtrip := tc.newResource()
				if err := json.Unmarshal(stored, roundtrip); err != nil {
					t.Fatal(err)
				}
				restored, err := roundtrip.GetMergedParameters(true)
				if err != nil {
					t.Fatal(err)
				}
				assertSettingEmptyCollections(t, restored)
			}
		})
	}
}

func assertSettingEmptyCollections(t *testing.T, parameters map[string]any) {
	t.Helper()
	permissions, ok := parameters["default_permissions"].(map[string]any)
	if !ok {
		t.Fatalf("missing default permissions: %#v", parameters)
	}
	if values, ok := permissions["agents"].([]any); !ok || len(values) != 0 {
		t.Fatalf("explicit policy clear was lost: %#v", permissions)
	}
	defaults, ok := parameters["default_user_limits"].(map[string]any)
	if !ok {
		t.Fatalf("missing spending defaults: %#v", parameters)
	}
	member, ok := defaults["default"].(map[string]any)
	if !ok {
		t.Fatal("missing organization spending default")
	}
	if models, ok := member["models"].([]any); !ok || len(models) != 0 {
		t.Fatalf("all-models selection was lost: %#v", member)
	}
}

func TestDefaultEnvironmentReferenceResolutionPreservesUnmanagedSlots(t *testing.T) {
	for _, value := range []settingsParameters{&cluster.Settings{}, &namespaced.Settings{}} {
		if err := json.Unmarshal([]byte(`{"spec":{"forProvider":{"environmentDefaults":{}}}}`), value); err != nil {
			t.Fatal(err)
		}
		if err := value.ResolveReferences(context.Background(), fake.NewClientBuilder().Build()); err != nil {
			t.Fatal(err)
		}
		parameters, err := value.GetParameters()
		if err != nil {
			t.Fatal(err)
		}
		defaults, ok := parameters["environment_defaults"].(map[string]any)
		if !ok {
			t.Fatalf("missing defaults object: %#v", parameters)
		}
		for _, name := range []string{"agents", "apps", "mcp_gateways", "mcp_registry", "knowledge_connectors"} {
			if observed, ok := defaults[name]; ok && observed != nil {
				t.Fatalf("unmanaged %s became a configured reset: %#v", name, defaults)
			}
		}
	}
}

func TestDefaultEnvironmentReferenceResolutionPreservesExplicitResets(t *testing.T) {
	for _, value := range []settingsParameters{&cluster.Settings{}, &namespaced.Settings{}} {
		for _, source := range []string{"forProvider", "initProvider"} {
			raw := []byte(`{"spec":{"` + source + `":{"environmentDefaults":{"agents":"","apps":"","mcpGateways":"","mcpRegistry":"","knowledgeConnectors":""}}}}`)
			if err := json.Unmarshal(raw, value); err != nil {
				t.Fatal(err)
			}
			if err := value.ResolveReferences(t.Context(), fake.NewClientBuilder().Build()); err != nil {
				t.Fatal(err)
			}
			getter := value.GetParameters
			if source == "initProvider" {
				getter = value.GetInitParameters
			}
			parameters, err := getter()
			if err != nil {
				t.Fatal(err)
			}
			defaults := parameters["environment_defaults"].(map[string]any)
			for _, name := range []string{"agents", "apps", "mcp_gateways", "mcp_registry", "knowledge_connectors"} {
				if defaults[name] != "" {
					t.Fatalf("%s explicit %s reset became null or omission: %#v", source, name, defaults)
				}
			}
		}
	}
}
