package config_test

import (
	"encoding/json"
	"slices"
	"testing"

	"github.com/crossplane/upjet/v2/pkg/resource"

	clusterapp "github.com/archestra-ai/provider-archestra/apis/cluster/app/v1alpha1"
	namespacedapp "github.com/archestra-ai/provider-archestra/apis/namespaced/app/v1alpha1"
)

func TestInitOnlySecretInputIsIgnoredAfterCreation(t *testing.T) {
	cases := []struct {
		name        string
		newResource func() credentialParameters
		ref         map[string]any
	}{
		{"cluster", func() credentialParameters { return &clusterapp.App{} }, map[string]any{"name": "html", "namespace": "crossplane-system", "key": "initial"}},
		{"namespaced", func() credentialParameters { return &namespacedapp.App{} }, map[string]any{"name": "html", "key": "initial"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, sources := range []struct {
				name    string
				spec    map[string]any
				ignored bool
			}{
				{"init only", map[string]any{"forProvider": map[string]any{}, "initProvider": map[string]any{"htmlSecretRef": tc.ref}}, true},
				{"both", map[string]any{"forProvider": map[string]any{"htmlSecretRef": tc.ref}, "initProvider": map[string]any{"htmlSecretRef": tc.ref}}, false},
				{"forProvider only", map[string]any{"forProvider": map[string]any{"htmlSecretRef": tc.ref}}, false},
			} {
				value := tc.newResource()
				encoded, err := json.Marshal(map[string]any{"spec": sources.spec})
				if err != nil {
					t.Fatal(err)
				}
				if err := json.Unmarshal(encoded, value); err != nil {
					t.Fatal(err)
				}
				params, err := value.GetParameters()
				if err != nil {
					t.Fatal(err)
				}
				init, err := value.GetInitParameters()
				if err != nil {
					t.Fatal(err)
				}
				if got := slices.Contains(resource.GetTerraformIgnoreChanges(params, init), "html"); got != sources.ignored {
					t.Fatalf("%s: html ignored=%v, want %v", sources.name, got, sources.ignored)
				}
			}
		})
	}
}
