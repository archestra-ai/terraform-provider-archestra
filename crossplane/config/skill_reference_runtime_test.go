package config_test

import (
	"context"
	"encoding/json"
	"testing"

	clustercredential "github.com/archestra-ai/provider-archestra/apis/cluster/credential/v1alpha1"
	clusterskill "github.com/archestra-ai/provider-archestra/apis/cluster/skill/v1alpha1"
	namespacedcredential "github.com/archestra-ai/provider-archestra/apis/namespaced/credential/v1alpha1"
	namespacedskill "github.com/archestra-ai/provider-archestra/apis/namespaced/skill/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestSkillGitLabReferenceUsesDefinitionIdentity(t *testing.T) {
	const definitionID = "11111111-1111-4111-8111-111111111111"
	for _, tc := range []struct {
		name       string
		credential client.Object
		skill      inlineParameters
		namespaced bool
	}{
		{"cluster", &clustercredential.Credential{}, &clusterskill.Skill{}, false},
		{"namespaced", &namespacedcredential.Credential{}, &namespacedskill.Skill{}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			for _, add := range []func(*runtime.Scheme) error{clustercredential.AddToScheme, clusterskill.AddToScheme, namespacedcredential.AddToScheme, namespacedskill.AddToScheme} {
				if err := add(scheme); err != nil {
					t.Fatal(err)
				}
			}
			raw := []byte(`{"metadata":{"name":"gitlab-fixture","annotations":{"crossplane.io/external-name":"logical-credential-key"}},"status":{"atProvider":{"definitionId":"` + definitionID + `"}}}`)
			if err := json.Unmarshal(raw, tc.credential); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal([]byte(`{"metadata":{"name":"skill-fixture"},"spec":{"forProvider":{"gitSource":{"gitlabTokenIdRef":{"name":"gitlab-fixture"}}}}}`), tc.skill); err != nil {
				t.Fatal(err)
			}
			if tc.namespaced {
				tc.credential.SetNamespace("proof")
				object, ok := tc.skill.(client.Object)
				if !ok {
					t.Fatal("skill is not a managed object")
				}
				object.SetNamespace("proof")
			}
			reader := fake.NewClientBuilder().WithScheme(scheme).WithObjects(tc.credential).Build()
			if err := tc.skill.ResolveReferences(context.Background(), reader); err != nil {
				t.Fatal(err)
			}
			parameters, err := tc.skill.GetParameters()
			if err != nil {
				t.Fatal(err)
			}
			source, ok := parameters["git_source"].(map[string]any)
			if !ok || source["gitlab_token_id"] != definitionID {
				t.Fatalf("GitLab reference used the credential key instead of its definition UUID: %#v", parameters)
			}
		})
	}
}
