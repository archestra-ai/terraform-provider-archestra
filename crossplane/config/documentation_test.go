package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sigs.k8s.io/yaml"
)

func TestGeneratedDocumentation(t *testing.T) {
	for _, scope := range []string{"cluster", "namespaced"} {
		p := GetProvider()
		suffix := "archestra.crossplane.io"
		if scope == "namespaced" {
			p = GetProviderNamespaced()
			suffix = "archestra.m.crossplane.io"
		}
		for name, r := range p.Resources {
			t.Run(scope+"/"+name, func(t *testing.T) {
				if r.MetaResource == nil || strings.TrimSpace(r.MetaResource.Description) == "" || len(r.MetaResource.Examples) == 0 {
					t.Fatal("resource needs a Terraform documentation summary and example")
				}
				// The schema already supplies field descriptions. Scraped table formatting
				// must not override them with Terraform types and nested-schema links.
				if len(r.MetaResource.ArgumentDocs) != 0 {
					t.Fatal("field descriptions must come from the Terraform schema")
				}
				path := filepath.Join("..", "examples-generated", scope, r.ShortGroup, r.Version, strings.ToLower(r.Kind)+".yaml")
				raw, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				var example struct {
					APIVersion string `json:"apiVersion"`
					Kind       string `json:"kind"`
				}
				if err := yaml.Unmarshal(raw, &example); err != nil {
					t.Fatal(err)
				}
				if example.APIVersion != r.ShortGroup+"."+suffix+"/"+r.Version || example.Kind != r.Kind {
					t.Fatalf("example GVK does not match %s/%s", r.ShortGroup, r.Kind)
				}
			})
		}
	}
}

func TestPackageReadme(t *testing.T) {
	readme, err := os.ReadFile("../README.md")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile("../package/crossplane.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var pkg struct {
		Metadata struct {
			Annotations map[string]string `json:"annotations"`
		} `json:"metadata"`
	}
	if err := yaml.Unmarshal(raw, &pkg); err != nil {
		t.Fatal(err)
	}
	want := strings.TrimSpace(string(readme))
	if strings.TrimSpace(pkg.Metadata.Annotations["meta.crossplane.io/readme"]) != want {
		t.Fatal("package README is stale; run make generate")
	}
}
