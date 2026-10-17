package config_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	knowledge "github.com/archestra-ai/provider-archestra/apis/cluster/knowledge/v1alpha1"
	project "github.com/archestra-ai/provider-archestra/apis/cluster/project/v1alpha1"
	knowledgeNamespaced "github.com/archestra-ai/provider-archestra/apis/namespaced/knowledge/v1alpha1"
	projectNamespaced "github.com/archestra-ai/provider-archestra/apis/namespaced/project/v1alpha1"
	v1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	ujresource "github.com/crossplane/upjet/v2/pkg/resource"
)

type fileSecretReader struct{ names []string }

func (*fileSecretReader) GetSecretData(context.Context, *v1.SecretReference) (map[string][]byte, error) {
	return nil, fmt.Errorf("unexpected whole-Secret read")
}
func (r *fileSecretReader) GetSecretValue(_ context.Context, sel v1.SecretKeySelector) ([]byte, error) {
	if sel.Name == "" || sel.Namespace != "proof" || sel.Key != "content" {
		return nil, fmt.Errorf("invalid selector: %#v", sel)
	}
	r.names = append(r.names, sel.Name)
	return []byte(sel.Name), nil
}

func TestRequiredFileSecretsReachTerraformFromEitherSource(t *testing.T) {
	for _, tc := range []struct {
		name        string
		newResource func() ujresource.Terraformed
		namespaced  bool
	}{
		{"cluster knowledge", func() ujresource.Terraformed { return &knowledge.File{} }, false},
		{"namespaced knowledge", func() ujresource.Terraformed { return &knowledgeNamespaced.File{} }, true},
		{"cluster project", func() ujresource.Terraformed { return &project.File{} }, false},
		{"namespaced project", func() ujresource.Terraformed { return &projectNamespaced.File{} }, true},
	} {
		for _, mode := range []string{"omitted", "init", "for", "both"} {
			t.Run(tc.name+"/"+mode, func(t *testing.T) {
				selector := func(name string) map[string]any {
					ref := map[string]any{"name": name, "key": "content"}
					if !tc.namespaced {
						ref["namespace"] = "proof"
					}
					return ref
				}
				forProvider, initProvider := map[string]any{}, map[string]any{}
				if mode == "init" || mode == "both" {
					initProvider["contentBase64SecretRef"] = selector("initial")
				}
				if mode == "for" || mode == "both" {
					forProvider["contentBase64SecretRef"] = selector("desired")
				}
				tr := tc.newResource()
				raw, err := json.Marshal(map[string]any{"metadata": map[string]string{"namespace": "proof"}, "spec": map[string]any{"forProvider": forProvider, "initProvider": initProvider}})
				if err != nil {
					t.Fatal(err)
				}
				if err := json.Unmarshal(raw, tr); err != nil {
					t.Fatal(err)
				}
				reader := &fileSecretReader{}
				params := map[string]any{}
				if err := ujresource.GetSensitiveParameters(t.Context(), reader, tr, params, tr.GetConnectionDetailsMapping()); err != nil {
					t.Fatal(err)
				}
				want := "initial"
				if mode == "for" || mode == "both" {
					want = "desired"
				}
				if mode == "omitted" {
					if len(reader.names) != 0 || len(params) != 0 {
						t.Fatalf("omitted source attempted Secret lookup: %#v %#v", reader.names, params)
					}
					return
				}
				if params["content_base64"] != want {
					t.Fatalf("Secret source precedence incorrect: %#v", params)
				}
				wire, err := json.Marshal(tr)
				if err != nil {
					t.Fatal(err)
				}
				var object struct {
					Spec struct {
						ForProvider map[string]any `json:"forProvider"`
					} `json:"spec"`
				}
				if err := json.Unmarshal(wire, &object); err != nil {
					t.Fatal(err)
				}
				if mode == "init" {
					if _, ok := object.Spec.ForProvider["contentBase64SecretRef"]; ok {
						t.Fatal("omitted forProvider selector serialized and would override initProvider")
					}
				}
			})
		}
	}
}
