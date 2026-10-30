package config_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	clusterteam "github.com/archestra-ai/provider-archestra/apis/cluster/team/v1alpha1"
	namespacedteam "github.com/archestra-ai/provider-archestra/apis/namespaced/team/v1alpha1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestTeamRoleReferencesThroughKubernetesClient(t *testing.T) {
	for _, scope := range []struct {
		name, group, namespace string
		newTeam                func() inlineParameters
		scope                  meta.RESTScope
	}{
		{"cluster", "identity.archestra.crossplane.io", "", func() inlineParameters { return &clusterteam.Team{} }, meta.RESTScopeRoot},
		{"namespaced", "identity.archestra.m.crossplane.io", "proof", func() inlineParameters { return &namespacedteam.Team{} }, meta.RESTScopeNamespace},
	} {
		t.Run(scope.name, func(t *testing.T) {
			version := schema.GroupVersion{Group: scope.group, Version: "v1alpha1"}
			path := "/apis/" + version.String()
			if scope.namespace != "" {
				path += "/namespaces/" + scope.namespace
			}
			path += "/roles"
			role := map[string]any{"apiVersion": version.String(), "kind": "Role", "metadata": map[string]any{"name": "reader", "namespace": scope.namespace, "labels": map[string]string{"fixture": "role"}}, "status": map[string]any{"atProvider": map[string]any{"role": "observed_reader", "id": "database-id"}}}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				var value any = role
				switch r.URL.Path {
				case path:
					value = map[string]any{"apiVersion": version.String(), "kind": "RoleList", "items": []any{role}}
				case path + "/reader":
				default:
					t.Errorf("unexpected Kubernetes request: %s", r.URL)
					http.NotFound(w, r)
					return
				}
				if err := json.NewEncoder(w).Encode(value); err != nil {
					t.Error(err)
				}
			}))
			defer server.Close()
			mapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{version})
			mapper.Add(version.WithKind("Role"), scope.scope)
			kube, err := client.New(&rest.Config{Host: server.URL}, client.Options{Scheme: runtime.NewScheme(), Mapper: mapper})
			if err != nil {
				t.Fatal(err)
			}
			for _, input := range []string{`{"rolesRefs":[{"name":"reader"}]}`, `{"rolesSelector":{"matchLabels":{"fixture":"role"}}}`} {
				team := scope.newTeam()
				if err := json.Unmarshal([]byte(`{"metadata":{"namespace":"`+scope.namespace+`"},"spec":{"forProvider":`+input+`}}`), team); err != nil {
					t.Fatal(err)
				}
				if err := team.ResolveReferences(t.Context(), kube); err != nil {
					t.Fatal(err)
				}
				parameters, err := team.GetParameters()
				if err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(parameters["roles"], []any{"observed_reader"}) {
					t.Fatalf("real client lost observed slug: %#v", parameters)
				}
			}
		})
	}
}
