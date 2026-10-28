package config

import (
	"slices"
	"testing"
)

func TestModelPoolDerivedAnchorsRemainUnmanagedInBothScopes(t *testing.T) {
	for scope, p := range map[string]bool{"cluster": false, "namespaced": true} {
		t.Run(scope, func(t *testing.T) {
			provider := GetProvider()
			if p {
				provider = GetProviderNamespaced()
			}
			for owner, fields := range map[string][]string{"archestra_agent": {"model_pool_id", "model_id", "llm_api_key_id"}, "archestra_oauth_client": {"model_pool_ids"}} {
				ignored := provider.Resources[owner].LateInitializer.IgnoredFields
				for _, field := range fields {
					if !slices.Contains(ignored, field) {
						t.Errorf("%s late-initializes %s, conflicting with model-pool selection or omitted grants", owner, field)
					}
				}
			}
		})
	}
}
