package config_test

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	cluster "github.com/archestra-ai/provider-archestra/apis/cluster"
	namespaced "github.com/archestra-ai/provider-archestra/apis/namespaced"
	"github.com/archestra-ai/provider-archestra/config"
	ujconfig "github.com/crossplane/upjet/v2/pkg/config"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestEveryConfiguredRelationshipResolvesToTerraform(t *testing.T) {
	scheme := runtime.NewScheme()
	for _, add := range []func(*runtime.Scheme) error{cluster.AddToScheme, namespaced.AddToScheme} {
		if err := add(scheme); err != nil {
			t.Fatal(err)
		}
	}
	for _, scope := range []struct {
		name, suffix, namespace string
		provider                *ujconfig.Provider
	}{
		{"cluster", "archestra.crossplane.io", "", config.GetProvider()},
		{"namespaced", "archestra.m.crossplane.io", "proof", config.GetProviderNamespaced()},
	} {
		gvk := func(resource *ujconfig.Resource) schema.GroupVersionKind {
			return schema.GroupVersionKind{Group: resource.ShortGroup + "." + scope.suffix, Version: "v1alpha1", Kind: resource.Kind}
		}
		for owner, r := range scope.provider.Resources {
			for path, ref := range r.References {
				for _, source := range []string{"ForProvider", "InitProvider"} {
					t.Run(scope.name+"/"+owner+"/"+source+"/"+path, func(t *testing.T) {
						target, err := scheme.New(gvk(scope.provider.Resources[ref.TerraformName]))
						if err != nil {
							t.Fatal(err)
						}
						raw := `{"metadata":{"name":"target","namespace":"` + scope.namespace + `","annotations":{"crossplane.io/external-name":"target-identity"}},"status":{"atProvider":{"role":"custom_reader","definitionId":"definition-identity"}}}`
						if err := json.Unmarshal([]byte(raw), target); err != nil {
							t.Fatal(err)
						}
						object, err := scheme.New(gvk(r))
						if err != nil {
							t.Fatal(err)
						}
						object.(client.Object).SetNamespace(scope.namespace)
						params := reflect.ValueOf(object).Elem().FieldByName("Spec").FieldByName(source)
						field := referenceParameter(t, params, strings.Split(path, "."))
						suffix := "Ref"
						if field.Type.Kind() == reflect.Slice {
							suffix = "Refs"
						}
						referenceField := field.Parent.FieldByName(field.Name + suffix)
						if !referenceField.IsValid() {
							t.Fatalf("missing generated reference for %s", path)
						}
						value := `{"name":"target"}`
						if suffix == "Refs" {
							value = "[" + value + "]"
						}
						if err := json.Unmarshal([]byte(value), referenceField.Addr().Interface()); err != nil {
							t.Fatal(err)
						}
						reader := fake.NewClientBuilder().WithScheme(scheme).WithObjects(target.(client.Object)).Build()
						resource := object.(inlineParameters)
						if err := resource.ResolveReferences(t.Context(), reader); err != nil {
							t.Fatal(err)
						}
						getter := resource.GetParameters
						if source == "InitProvider" {
							getter = resource.(interface {
								GetInitParameters() (map[string]any, error)
							}).GetInitParameters
						}
						values, err := getter()
						if err != nil {
							t.Fatal(err)
						}
						var actual any = values
						for _, segment := range strings.Split(path, ".") {
							if list, ok := actual.([]any); ok {
								actual = list[0]
							}
							actual = actual.(map[string]any)[segment]
						}
						want := "target-identity"
						if strings.Contains(ref.Extractor, `"role"`) {
							want = "custom_reader"
						}
						if strings.Contains(ref.Extractor, `"definition_id"`) {
							want = "definition-identity"
						}
						if suffix == "Refs" {
							if !reflect.DeepEqual(actual, []any{want}) {
								t.Fatalf("got %#v, expected [%s]", actual, want)
							}
						} else if actual != want {
							t.Fatalf("got %#v, expected %s", actual, want)
						}
					})
				}
			}
		}
	}
}

type parameterField struct {
	Parent reflect.Value
	Name   string
	Type   reflect.Type
}

func referenceParameter(t *testing.T, value reflect.Value, path []string) parameterField {
	t.Helper()
	for value.Kind() == reflect.Pointer || value.Kind() == reflect.Slice {
		if value.Kind() == reflect.Pointer {
			if value.IsNil() {
				value.Set(reflect.New(value.Type().Elem()))
			}
			value = value.Elem()
		} else {
			value.Set(reflect.MakeSlice(value.Type(), 1, 1))
			value = value.Index(0)
		}
	}
	for i := 0; i < value.NumField(); i++ {
		field := value.Type().Field(i)
		if strings.Split(field.Tag.Get("tf"), ",")[0] != path[0] {
			continue
		}
		if len(path) == 1 {
			return parameterField{value, field.Name, field.Type}
		}
		return referenceParameter(t, value.Field(i), path[1:])
	}
	t.Fatalf("no generated parameter %s in %s", path[0], value.Type())
	return parameterField{}
}
