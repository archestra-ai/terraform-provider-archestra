package provider

import (
	"github.com/hashicorp/terraform-plugin-framework/types"
	"testing"
)

func TestIdentityProviderPreservesAllConfiguredRedactedSecrets(t *testing.T) {
	prior := IdentityProviderResourceModel{
		OidcConfig: &OidcConfigModel{EnterpriseManagedCredentials: &EnterpriseManagedCredentialsModel{}},
		SamlConfig: &SamlConfigModel{IdpMetadata: &SamlIdpMetadata{}, SpMetadata: &SamlSpMetadata{}},
	}
	paths := make([]string, 0, len(identitySecretFields))
	for path, fields := range identitySecretFields {
		field := identitySecretField(&prior, fields)
		if field == nil {
			t.Fatalf("invalid field mapping: %s", path)
		}
		*field = types.StringValue("synthetic-secret")
		paths = append(paths, path)
	}
	body := map[string]any{"configuredSecretPaths": paths, "oidcConfig": map[string]any{"enterpriseManagedCredentials": map[string]any{}}, "samlConfig": map[string]any{"idpMetadata": map[string]any{}, "spMetadata": map[string]any{}}}
	if err := mapIdentityProviderResponse(body, &prior, false, false); err != nil {
		t.Fatal(err)
	}
	for path, fields := range identitySecretFields {
		field := identitySecretField(&prior, fields)
		if field == nil || field.ValueString() != "synthetic-secret" {
			t.Errorf("redaction lost managed secret at %s", path)
		}
	}
}

func TestIdentityProviderImportNeverInventsSecretValues(t *testing.T) {
	var imported IdentityProviderResourceModel
	if err := mapIdentityProviderResponse(map[string]any{"configuredSecretPaths": []string{"oidcConfig.clientSecret"}, "oidcConfig": map[string]any{}}, &imported, false, false); err != nil {
		t.Fatal(err)
	}
	if imported.OidcConfig == nil || !imported.OidcConfig.ClientSecret.IsNull() {
		t.Fatal("import must leave the unrecoverable secret null")
	}
	if len(imported.ConfiguredSecretPaths.Elements()) != 1 {
		t.Fatal("import must retain secret presence metadata")
	}
}

func TestIdentityProviderDoesNotResurrectRemovedSecretParents(t *testing.T) {
	prior := IdentityProviderResourceModel{OidcConfig: &OidcConfigModel{ClientSecret: types.StringValue("synthetic-secret")}}
	if err := mapIdentityProviderResponse(map[string]any{"configuredSecretPaths": []string{}}, &prior, false, false); err != nil {
		t.Fatal(err)
	}
	if prior.OidcConfig != nil {
		t.Fatal("removed parent was resurrected")
	}
}
