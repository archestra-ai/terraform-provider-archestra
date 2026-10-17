package config

import (
	"encoding/json"
	"testing"

	ujconfig "github.com/crossplane/upjet/v2/pkg/config"
	conversion "github.com/crossplane/upjet/v2/pkg/types/conversion/tfjson"
	tfjson "github.com/hashicorp/terraform-json"
)

func TestSingleBlocksUseFrameworkObjectRepresentation(t *testing.T) {
	var document tfjson.ProviderSchemas
	if err := json.Unmarshal(normalizedProviderSchema(providerSchema), &document); err != nil {
		t.Fatal(err)
	}
	for _, p := range document.Schemas {
		identity := p.ResourceSchemas["archestra_identity_provider"].Block
		if _, ok := identity.NestedBlocks["oidc_config"]; ok {
			t.Fatal("single block still follows Upjet's list converter")
		}
		oidc := identity.Attributes["oidc_config"]
		if oidc.AttributeNestedType.NestingMode != tfjson.SchemaNestingModeSingle {
			t.Fatal("OIDC configuration lost object nesting")
		}
		secret := oidc.AttributeNestedType.Attributes["client_secret"]
		if !secret.Sensitive {
			t.Fatal("normalization dropped sensitive metadata")
		}
		role := identity.Attributes["role_mapping"].AttributeNestedType
		if role.Attributes["rules"].AttributeNestedType.NestingMode != tfjson.SchemaNestingModeList {
			t.Fatal("normalization changed a real list into an object")
		}
	}
	for name, p := range map[string]func() *ujconfig.Provider{"cluster": GetProvider, "namespaced": GetProviderNamespaced} {
		t.Run(name, func(t *testing.T) {
			schema := p().Resources["archestra_identity_provider"].TerraformResource.Schema["oidc_config"]
			if schema.Type != conversion.SchemaTypeObject {
				t.Fatal("configured generator does not use native Terraform object values")
			}
		})
	}
}
