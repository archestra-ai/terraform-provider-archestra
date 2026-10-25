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

func TestSecretInputsMatchSupportedStorageInBothScopes(t *testing.T) {
	for name, constructor := range map[string]func() *ujconfig.Provider{"cluster": GetProvider, "namespaced": GetProviderNamespaced} {
		t.Run(name, func(t *testing.T) {
			p := constructor()
			removed := map[string][]string{
				"archestra_team":               {"vault_folder"},
				"archestra_model_provider":     {"vault_secret_path", "vault_secret_key"},
				"archestra_mcp_registry_entry": {"local_config_vault_path", "local_config_vault_key", "oauth_client_secret_vault_path", "oauth_client_secret_vault_key"},
				"archestra_mcp_server":         {"is_byos_vault"},
			}
			for owner, attributes := range removed {
				for _, attribute := range attributes {
					if _, exists := p.Resources[owner].TerraformResource.Schema[attribute]; exists {
						t.Errorf("%s still exposes %s", owner, attribute)
					}
				}
			}
			for owner, attributes := range map[string][]string{
				"archestra_team":               {"external_group_ids", "roles"},
				"archestra_model_provider":     {"api_key", "aws_secret_access_key"},
				"archestra_mcp_registry_entry": {"client_secret_id", "local_config_secret_id"},
				"archestra_mcp_server":         {"access_token", "environment_values", "secret_id"},
			} {
				for _, attribute := range attributes {
					field := p.Resources[owner].TerraformResource.Schema[attribute]
					if field == nil {
						t.Errorf("%s lost %s", owner, attribute)
						continue
					}
					if (attribute == "api_key" || attribute == "aws_secret_access_key" || attribute == "access_token") && !field.Sensitive {
						t.Errorf("%s.%s lost secret marking", owner, attribute)
					}
				}
			}
		})
	}
}
