package clients

import (
	"encoding/json"
	"testing"

	clusterenvironment "github.com/archestra-ai/provider-archestra/apis/cluster/environment/v1alpha1"
	clusterconfig "github.com/archestra-ai/provider-archestra/apis/cluster/v1beta1"
	namespacedenvironment "github.com/archestra-ai/provider-archestra/apis/namespaced/environment/v1alpha1"
	namespacedconfig "github.com/archestra-ai/provider-archestra/apis/namespaced/v1beta1"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestAPIConstraintReachesTerraformInBothScopes(t *testing.T) {
	for _, kind := range []string{"legacy", "namespaced", "cluster-config"} {
		t.Run(kind, func(t *testing.T) {
			scheme := runtime.NewScheme()
			for _, add := range []func(*runtime.Scheme) error{corev1.AddToScheme, clusterconfig.SchemeBuilder.AddToScheme, namespacedconfig.SchemeBuilder.AddToScheme, clusterenvironment.AddToScheme, namespacedenvironment.AddToScheme} {
				if err := add(scheme); err != nil {
					t.Fatal(err)
				}
			}
			const namespace = "fixture"
			secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "credentials", Namespace: namespace}, Data: map[string][]byte{"credentials": []byte(`{"api_key":"synthetic-fixture","base_url":"http://127.0.0.1:19000","api_version_constraint":"= 1.4.0-rc.48","unrelated":"must-not-forward"}`)}}
			var pc client.Object
			var mg resource.Managed
			pcMeta := map[string]any{"name": "fixture"}
			ref := map[string]string{"name": "fixture"}
			switch kind {
			case "legacy":
				pc = &clusterconfig.ProviderConfig{}
				mg = &clusterenvironment.Environment{}
			case "namespaced":
				pc = &namespacedconfig.ProviderConfig{}
				mg = &namespacedenvironment.Environment{}
				pcMeta["namespace"] = namespace
				ref["kind"] = "ProviderConfig"
			case "cluster-config":
				pc = &namespacedconfig.ClusterProviderConfig{}
				mg = &namespacedenvironment.Environment{}
				ref["kind"] = "ClusterProviderConfig"
			}
			pcJSON, err := json.Marshal(map[string]any{"metadata": pcMeta, "spec": map[string]any{"credentials": map[string]any{"source": "Secret", "secretRef": map[string]string{"name": "credentials", "namespace": namespace, "key": "credentials"}}}})
			if err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(pcJSON, pc); err != nil {
				t.Fatal(err)
			}
			managedJSON, err := json.Marshal(map[string]any{"metadata": map[string]string{"name": "managed-fixture", "namespace": namespace, "uid": "fixture-uid"}, "spec": map[string]any{"providerConfigRef": ref}})
			if err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(managedJSON, mg); err != nil {
				t.Fatal(err)
			}
			if kind == "legacy" {
				mg.SetNamespace("")
			}
			mg.SetUID(types.UID("fixture-uid"))
			fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(secret, pc, mg).Build()
			result, err := TerraformSetupBuilder("1.5.7", "archestra-ai/archestra", "3.0.0")(t.Context(), fakeClient, mg)
			if err != nil {
				t.Fatal(err)
			}
			if result.Configuration["api_version_constraint"] != "= 1.4.0-rc.48" {
				t.Fatal("the deployment API constraint was dropped before native provider configuration")
			}
			if result.Configuration["api_key"] != "synthetic-fixture" || result.Configuration["base_url"] != "http://127.0.0.1:19000" || len(result.Configuration) != 3 {
				t.Fatal("provider credential allowlist changed")
			}
		})
	}
}
