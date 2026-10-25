package config_test

import (
	"encoding/json"
	"strings"
	"testing"

	clusterllm "github.com/archestra-ai/provider-archestra/apis/cluster/llm/v1alpha1"
	namespacedllm "github.com/archestra-ai/provider-archestra/apis/namespaced/llm/v1alpha1"
)

func TestModelProviderSecretsUseGeneratedSelectors(t *testing.T) {
	for _, tc := range []struct {
		name        string
		newResource func() any
		reference   string
	}{
		{"cluster", func() any { return &clusterllm.ModelProvider{} }, `{"name":"model-fixture","namespace":"crossplane-system","key":"api-key"}`},
		{"namespaced", func() any { return &namespacedllm.ModelProvider{} }, `{"name":"model-fixture","key":"api-key"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, source := range []string{"forProvider", "initProvider"} {
				input := `{"spec":{"` + source + `":{"apiKeySecretRef":` + tc.reference + `}}}`
				value := tc.newResource()
				decoder := json.NewDecoder(strings.NewReader(input))
				decoder.DisallowUnknownFields()
				if err := decoder.Decode(value); err != nil {
					t.Fatal("valid selector rejected", err)
				}
				encoded, err := json.Marshal(value)
				if err != nil {
					t.Fatal(err)
				}
				if !strings.Contains(string(encoded), `"apiKeySecretRef"`) {
					t.Fatal("selector was lost")
				}
			}
			decoder := json.NewDecoder(strings.NewReader(`{"spec":{"forProvider":{"apiKey":"synthetic-invalid-inline-input"}}}`))
			decoder.DisallowUnknownFields()
			if err := decoder.Decode(tc.newResource()); err == nil {
				t.Fatal("inline credential unexpectedly accepted")
			}
		})
	}
}
