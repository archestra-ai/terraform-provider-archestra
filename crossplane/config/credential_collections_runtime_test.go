package config_test

import (
	"encoding/json"
	clustercredential "github.com/archestra-ai/provider-archestra/apis/cluster/credential/v1alpha1"
	clusterllm "github.com/archestra-ai/provider-archestra/apis/cluster/llm/v1alpha1"
	namespacedcredential "github.com/archestra-ai/provider-archestra/apis/namespaced/credential/v1alpha1"
	namespacedllm "github.com/archestra-ai/provider-archestra/apis/namespaced/llm/v1alpha1"
	"testing"
)

type credentialParameters interface {
	GetParameters() (map[string]any, error)
	GetInitParameters() (map[string]any, error)
	GetMergedParameters(bool) (map[string]any, error)
	LateInitialize([]byte) (bool, error)
}

func TestCredentialOptionalCollectionsKeepOmissionAndEmptyDistinct(t *testing.T) {
	cases := []struct {
		name        string
		newResource func() credentialParameters
		fields      map[string]string
	}{
		{"cluster virtual", func() credentialParameters { return &clusterllm.VirtualKey{} }, map[string]string{"providerApiKeys": "provider_api_keys", "labels": "labels"}},
		{"namespaced virtual", func() credentialParameters { return &namespacedllm.VirtualKey{} }, map[string]string{"providerApiKeys": "provider_api_keys", "labels": "labels"}},
		{"cluster OAuth", func() credentialParameters { return &clustercredential.OAuthClient{} }, map[string]string{"providerApiKeys": "provider_api_keys", "labels": "labels", "allowedGatewayIds": "allowed_gateway_ids", "redirectUris": "redirect_uris"}},
		{"namespaced OAuth", func() credentialParameters { return &namespacedcredential.OAuthClient{} }, map[string]string{"providerApiKeys": "provider_api_keys", "labels": "labels", "allowedGatewayIds": "allowed_gateway_ids", "redirectUris": "redirect_uris"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, source := range []string{"forProvider", "initProvider"} {
				value := tc.newResource()
				get := value.GetParameters
				if source == "initProvider" {
					get = value.GetInitParameters
				}
				omitted, err := get()
				if err != nil {
					t.Fatal(err)
				}
				for _, field := range tc.fields {
					if _, present := omitted[field]; present {
						t.Fatalf("unmanaged collection %s is present", field)
					}
				}
				fields := map[string]any{}
				for field := range tc.fields {
					fields[field] = []any{}
				}
				encoded, err := json.Marshal(map[string]any{"spec": map[string]any{source: fields}})
				if err != nil {
					t.Fatal(err)
				}
				if err := json.Unmarshal(encoded, value); err != nil {
					t.Fatal(err)
				}
				parameters, err := get()
				if err != nil {
					t.Fatal(err)
				}
				merged, err := value.GetMergedParameters(true)
				if err != nil {
					t.Fatal(err)
				}
				for _, field := range tc.fields {
					for _, p := range []map[string]any{parameters, merged} {
						if list, ok := p[field].([]any); !ok || len(list) != 0 {
							t.Fatalf("explicit empty %s was lost: %#v", field, p[field])
						}
					}
				}
				stored, err := json.Marshal(value)
				if err != nil {
					t.Fatal(err)
				}
				var document struct {
					Spec map[string]map[string]json.RawMessage `json:"spec"`
				}
				if err := json.Unmarshal(stored, &document); err != nil {
					t.Fatal(err)
				}
				for field := range tc.fields {
					if string(document.Spec[source][field]) != "[]" {
						t.Fatalf("Kubernetes serialization dropped %s", field)
					}
				}
			}
			value := tc.newResource()
			changed, err := value.LateInitialize([]byte(`{"provider_api_keys":[{"provider":"ollama","provider_api_key_id":"fixture"}],"labels":[{"key":"purpose","value":"old"}],"billing_team_id":"fixture","service_account_id":"fixture","spend_cap":{"limit_value":100,"cleanup_interval":"24h"},"expires_at":"2040-01-01T00:00:00Z","allowed_gateway_ids":["fixture"],"redirect_uris":["https://example.test"]}`))
			if err != nil {
				t.Fatal(err)
			}
			if changed {
				t.Fatal("late initialization consumed an explicit clear or unmanaged routing input")
			}
		})
	}
}
