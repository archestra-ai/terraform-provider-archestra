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
	for _, tc := range []struct {
		name       string
		provider   client.Object
		pool       inlineParameters
		namespaced bool
	}{
		{"cluster pool", &cluster.ModelProvider{}, &cluster.ModelPool{}, false},
		{"namespaced pool", &namespaced.ModelProvider{}, &namespaced.ModelPool{}, true},
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
				{tc.pool, `{"metadata":{"name":"balanced"},"spec":{"forProvider":{"name":"balanced-model","members":[{"providerApiKeyIdRef":{"name":"shared-provider"},"modelId":"deployment","priority":2,"weight":3,"enabled":false}]}},"status":{"atProvider":{"virtualApiKeyIds":["33333333-3333-4333-8333-333333333333"]}}}`},
			} {
				if err := json.Unmarshal([]byte(object.document), object.target); err != nil {
					t.Fatal(err)
				}
			}
			if tc.namespaced {
				tc.provider.SetNamespace("proof")
				pool, ok := tc.pool.(client.Object)
				if !ok {
					t.Fatal("pool is not a managed object")
				}
				pool.SetNamespace("proof")
			}
			reader := fake.NewClientBuilder().WithScheme(scheme).WithObjects(tc.provider).Build()
			if err := tc.pool.ResolveReferences(t.Context(), reader); err != nil {
				t.Fatal(err)
			}
			values, err := tc.pool.GetParameters()
			if err != nil {
				t.Fatal(err)
			}
			members, ok := values["members"].([]any)
			if !ok || len(members) != 1 {
				t.Fatalf("missing members: %#v", values)
			}
			member, ok := members[0].(map[string]any)
			if !ok {
				t.Fatalf("invalid member: %#v", members)
			}
			if member["provider_api_key_id"] != providerID || member["model_id"] != "deployment" || member["enabled"] != false {
				t.Fatalf("member reference/settings lost: %#v", values)
			}
			if _, present := values["virtual_api_key_ids"]; present {
				t.Fatal("read-only credential grants were sent to Terraform")
			}
		})
	}
}
