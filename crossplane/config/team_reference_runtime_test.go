package config_test

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	clusteridentity "github.com/archestra-ai/provider-archestra/apis/cluster/identity/v1alpha1"
	clusterteam "github.com/archestra-ai/provider-archestra/apis/cluster/team/v1alpha1"
	namespacedidentity "github.com/archestra-ai/provider-archestra/apis/namespaced/identity/v1alpha1"
	namespacedteam "github.com/archestra-ai/provider-archestra/apis/namespaced/team/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestTeamReferencesUseParentIdentityAndRoleSlugs(t *testing.T) {
	for _, scope := range []struct {
		name    string
		team    func() inlineParameters
		parent  func() client.Object
		role    func() client.Object
		account func() inlineParameters
	}{
		{"cluster", func() inlineParameters { return &clusterteam.Team{} }, func() client.Object { return &clusterteam.Team{} }, func() client.Object { return &clusteridentity.Role{} }, func() inlineParameters { return &clusteridentity.ServiceAccount{} }},
		{"namespaced", func() inlineParameters { return &namespacedteam.Team{} }, func() client.Object { return &namespacedteam.Team{} }, func() client.Object { return &namespacedidentity.Role{} }, func() inlineParameters { return &namespacedidentity.ServiceAccount{} }},
	} {
		t.Run(scope.name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			for _, add := range []func(*runtime.Scheme) error{clusteridentity.AddToScheme, clusterteam.AddToScheme, namespacedidentity.AddToScheme, namespacedteam.AddToScheme} {
				if err := add(scheme); err != nil {
					t.Fatal(err)
				}
			}
			parent, role := scope.parent(), scope.role()
			decode := func(raw string, target any) {
				t.Helper()
				if err := json.Unmarshal([]byte(raw), target); err != nil {
					t.Fatal(err)
				}
			}
			decode(`{"metadata":{"name":"parent","labels":{"fixture":"parent"},"annotations":{"crossplane.io/external-name":"parent-uuid"}}}`, parent)
			decode(`{"metadata":{"name":"reader","labels":{"fixture":"role"},"annotations":{"crossplane.io/external-name":"role-database-id"}},"status":{"atProvider":{"role":"custom_reader"}}}`, role)
			if scope.name == "namespaced" {
				parent.SetNamespace("proof")
				role.SetNamespace("proof")
			}
			reader := fake.NewClientBuilder().WithScheme(scheme).WithObjects(parent, role).Build()
			cases := []struct {
				name, input string
				want        map[string]any
			}{
				{"references", `{"parentIdRef":{"name":"parent"},"rolesRefs":[{"name":"reader"}]}`, map[string]any{"parent_id": "parent-uuid", "roles": []any{"custom_reader"}}},
				{"selectors", `{"parentIdSelector":{"matchLabels":{"fixture":"parent"}},"rolesSelector":{"matchLabels":{"fixture":"role"}}}`, map[string]any{"parent_id": "parent-uuid", "roles": []any{"custom_reader"}}},
				{"literal values", `{"parentId":"explicit-parent","roles":["member","literal_custom"]}`, map[string]any{"parent_id": "explicit-parent", "roles": []any{"member", "literal_custom"}}},
				{"cached literals", `{"parentId":"cached-parent","parentIdRef":{"name":"parent"},"roles":["member"],"rolesRefs":[{"name":"reader"}]}`, map[string]any{"parent_id": "cached-parent", "roles": []any{"member"}}},
				{"always resolve", `{"parentId":"cached-parent","parentIdRef":{"name":"parent","policy":{"resolve":"Always"}},"roles":["member"],"rolesRefs":[{"name":"reader","policy":{"resolve":"Always"}}]}`, map[string]any{"parent_id": "parent-uuid", "roles": []any{"custom_reader"}}},
				{"omitted", `{}`, map[string]any{}},
				{"clear roles", `{"roles":[]}`, map[string]any{"roles": []any{}}},
			}
			for _, source := range []string{"forProvider", "initProvider"} {
				for _, tc := range cases {
					t.Run(source+"/"+tc.name, func(t *testing.T) {
						value := scope.team()
						decode(`{"metadata":{"name":"child","namespace":"proof"},"spec":{"`+source+`":`+tc.input+`}}`, value)
						if scope.name == "cluster" {
							value.(client.Object).SetNamespace("")
						}
						if err := value.ResolveReferences(context.Background(), reader); err != nil {
							t.Fatal(err)
						}
						getter := value.GetParameters
						if source == "initProvider" {
							getter = value.(interface {
								GetInitParameters() (map[string]any, error)
							}).GetInitParameters
						}
						parameters, err := getter()
						if err != nil {
							t.Fatal(err)
						}
						if !reflect.DeepEqual(parameters, tc.want) {
							t.Fatalf("got %#v, want %#v", parameters, tc.want)
						}
					})
				}
			}
			account := scope.account()
			decode(`{"metadata":{"name":"bot","namespace":"proof"},"spec":{"forProvider":{"roleRef":{"name":"reader"}}}}`, account)
			if scope.name == "cluster" {
				account.(client.Object).SetNamespace("")
			}
			if err := account.ResolveReferences(context.Background(), reader); err != nil {
				t.Fatal(err)
			}
			params, err := account.GetParameters()
			if err != nil {
				t.Fatal(err)
			}
			if params["role"] != "custom_reader" {
				t.Fatalf("role identity mismatch: %#v", params)
			}
			unready := scope.role()
			decode(`{"metadata":{"name":"unready","namespace":"proof","annotations":{"crossplane.io/external-name":"not-a-role-slug"}}}`, unready)
			if scope.name == "cluster" {
				unready.SetNamespace("")
			}
			unresolved := scope.team()
			decode(`{"metadata":{"namespace":"proof"},"spec":{"forProvider":{"rolesRefs":[{"name":"unready"}]}}}`, unresolved)
			if scope.name == "cluster" {
				unresolved.(client.Object).SetNamespace("")
			}
			unreadyReader := fake.NewClientBuilder().WithScheme(scheme).WithObjects(unready).Build()
			if err := unresolved.ResolveReferences(t.Context(), unreadyReader); err == nil {
				t.Fatal("role without observed slug succeeded")
			}
			if scope.name == "namespaced" {
				for _, input := range []string{`{"rolesRefs":[{"name":"reader"}]}`, `{"rolesSelector":{"matchLabels":{"fixture":"role"}}}`} {
					other := scope.team()
					decode(`{"metadata":{"namespace":"other"},"spec":{"forProvider":`+input+`}}`, other)
					if err := other.ResolveReferences(t.Context(), reader); err == nil {
						t.Fatal("cross-namespace role reference/selector succeeded")
					}
				}
			}
			missing := scope.team()
			decode(`{"spec":{"forProvider":{"parentIdRef":{"name":"missing"}}}}`, missing)
			if err := missing.ResolveReferences(context.Background(), reader); err == nil {
				t.Fatal("unresolved required parent reference succeeded")
			}
		})
	}
}
