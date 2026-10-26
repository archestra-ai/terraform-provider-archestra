package config_test

import (
	"encoding/json"
	"testing"

	cluster "github.com/archestra-ai/provider-archestra/apis/cluster/llm/v1alpha1"
	namespaced "github.com/archestra-ai/provider-archestra/apis/namespaced/llm/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestModelPoolReferencesReachTerraformInBothScopes(t *testing.T) {
	const providerID = "22222222-2222-4222-8222-222222222222"
	const virtualID = "33333333-3333-4333-8333-333333333333"
	for _, tc := range []struct {
		name              string
		provider, virtual client.Object
		pool              inlineParameters
		namespaced        bool
	}{
		{"cluster pool", &cluster.ModelProvider{}, &cluster.VirtualKey{}, &cluster.ModelPool{}, false},
		{"namespaced pool", &namespaced.ModelProvider{}, &namespaced.VirtualKey{}, &namespaced.ModelPool{}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			for _, add := range []func(*runtime.Scheme) error{cluster.AddToScheme, namespaced.AddToScheme} {
				if err := add(scheme); err != nil {
					t.Fatal(err)
				}
			}
			for _, object := range []struct {
				target   any
				document string
			}{
				{tc.provider, `{"metadata":{"name":"shared-provider","annotations":{"crossplane.io/external-name":"` + providerID + `"}}}`},
				{tc.virtual, `{"metadata":{"name":"caller","annotations":{"crossplane.io/external-name":"` + virtualID + `"}}}`},
				{tc.pool, `{"metadata":{"name":"balanced"},"spec":{"forProvider":{"name":"balanced-model","members":[{"providerApiKeyIdRef":{"name":"shared-provider"},"modelId":"deployment","priority":2,"weight":3,"enabled":false}],"virtualApiKeyIdsRefs":[{"name":"caller"}]}}}`},
			} {
				if err := json.Unmarshal([]byte(object.document), object.target); err != nil {
					t.Fatal(err)
				}
			}
			if tc.namespaced {
				tc.provider.SetNamespace("proof")
				tc.virtual.SetNamespace("proof")
				pool, ok := tc.pool.(client.Object)
				if !ok {
					t.Fatal("pool is not a managed object")
				}
				pool.SetNamespace("proof")
			}
			reader := fake.NewClientBuilder().WithScheme(scheme).WithObjects(tc.provider, tc.virtual).Build()
			if err := tc.pool.ResolveReferences(t.Context(), reader); err != nil {
				t.Fatal(err)
			}
			values, err := tc.pool.GetParameters()
			if err != nil {
				t.Fatal(err)
			}
			members, ok := values["members"].([]any)
			if !ok || len(members) != 1 {
				t.Fatalf("missing member parameters: %#v", values)
			}
			member, ok := members[0].(map[string]any)
			if !ok {
				t.Fatalf("invalid member parameters: %#v", members)
			}
			if member["provider_api_key_id"] != providerID || member["model_id"] != "deployment" || member["enabled"] != false {
				t.Fatalf("member reference/settings lost: %#v", values)
			}
			keys, ok := values["virtual_api_key_ids"].([]any)
			if !ok || len(keys) != 1 || keys[0] != virtualID {
				t.Fatalf("virtual key reference lost: %#v", values)
			}
		})
	}
}

func TestModelPoolExplicitEmptyGrantsSurviveSerialization(t *testing.T) {
	for _, tc := range []struct {
		name        string
		newResource func() credentialParameters
	}{
		{"cluster pool", func() credentialParameters { return &cluster.ModelPool{} }},
		{"namespaced pool", func() credentialParameters { return &namespaced.ModelPool{} }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pool := tc.newResource()
			if err := json.Unmarshal([]byte(`{"spec":{"forProvider":{"name":"balanced","members":[{"providerApiKeyId":"22222222-2222-4222-8222-222222222222","modelId":"deployment"}],"virtualApiKeyIds":[]}}}`), pool); err != nil {
				t.Fatal(err)
			}
			values, err := pool.GetMergedParameters(true)
			if err != nil {
				t.Fatal(err)
			}
			if keys, ok := values["virtual_api_key_ids"].([]any); !ok || len(keys) != 0 {
				t.Fatalf("explicit grant clear was lost: %#v", values)
			}
			serialized, err := json.Marshal(pool)
			if err != nil {
				t.Fatal(err)
			}
			var doc struct {
				Spec struct {
					ForProvider map[string]json.RawMessage `json:"forProvider"`
				} `json:"spec"`
			}
			if err := json.Unmarshal(serialized, &doc); err != nil {
				t.Fatal(err)
			}
			if string(doc.Spec.ForProvider["virtualApiKeyIds"]) != "[]" {
				t.Fatal("Kubernetes serialization dropped explicit grant clear")
			}
		})
	}
}
