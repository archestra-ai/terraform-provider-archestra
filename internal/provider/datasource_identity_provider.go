package provider

import (
	"context"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
)

func NewIdentityProviderDataSource() datasource.DataSource {
	return &namedLookupDataSource{spec: namedLookupSpec{
		typeName:    "identity_provider",
		noun:        "Identity Provider",
		description: "Finds an identity provider by its provider ID. Its secrets are never returned.",
		key:         namedLookupField{tf: "provider_id", json: "providerId", desc: "Provider ID of the identity provider, as used in its sign-in URL."},
		fields: []namedLookupField{
			{tf: "issuer", json: "issuer", desc: "Issuer URL."},
			{tf: "domain", json: "domain", desc: "Email domain the provider signs in."},
			{tf: "sso_login_enabled", json: "ssoLoginEnabled", kind: lookupBool, desc: "Whether people can sign in with it."},
		},
		list: func(ctx context.Context, c *client.ClientWithResponses, _ string) ([]map[string]any, error) {
			r, err := c.GetIdentityProvidersWithResponse(ctx)
			if err != nil {
				return nil, err
			}
			items, _, err := listItems(r.StatusCode(), r.Body)
			return items, err
		},
	}}
}
