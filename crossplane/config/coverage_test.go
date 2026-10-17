package config

import (
	"encoding/json"
	"testing"
)

func TestBothScopesConfigureEveryTerraformResource(t *testing.T) {
	var schema struct {
		ProviderSchemas map[string]struct {
			ResourceSchemas map[string]json.RawMessage `json:"resource_schemas"`
		} `json:"provider_schemas"`
	}
	if err := json.Unmarshal([]byte(providerSchema), &schema); err != nil {
		t.Fatal(err)
	}
	actual := schema.ProviderSchemas["registry.terraform.io/archestra-ai/archestra"].ResourceSchemas
	if len(actual) == 0 {
		t.Fatal("no actual Terraform schemas were loaded")
	}
	cluster, namespaced := GetProvider(), GetProviderNamespaced()
	for name := range actual {
		if _, ok := ExternalNameConfigs[name]; !ok {
			t.Errorf("%s has no external-name mapping", name)
		}
		a, clusterOK := cluster.Resources[name]
		b, namespacedOK := namespaced.Resources[name]
		if !clusterOK || !namespacedOK {
			t.Errorf("%s is missing from a scope", name)
			continue
		}
		if a.Kind != b.Kind || a.ShortGroup != b.ShortGroup {
			t.Errorf("%s differs between scopes: %s/%s vs %s/%s", name, a.ShortGroup, a.Kind, b.ShortGroup, b.Kind)
		}
	}
	for name := range ExternalNameConfigs {
		if _, ok := actual[name]; !ok {
			t.Errorf("%s maps a retired or missing Terraform resource", name)
		}
	}
}
