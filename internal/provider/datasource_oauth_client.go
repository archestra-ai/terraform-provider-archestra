package provider

import (
	"context"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
)

func NewOauthClientDataSource() datasource.DataSource {
	return &namedLookupDataSource{spec: namedLookupSpec{
		typeName:    "oauth_client",
		noun:        "OAuth Client",
		description: "Finds an OAuth client by its exact name. Its secret is never returned.",
		key:         namedLookupField{tf: "name", json: "name", desc: "Exact name of the OAuth client."},
		fields: []namedLookupField{
			{tf: "client_id", json: "clientId", desc: "Public OAuth client ID."},
			{tf: "service_account_id", json: "serviceAccountId", desc: "Service account the client signs in as, when it has one."},
			{tf: "grant_type", json: "grantType", desc: "OAuth grant the client uses."},
			{tf: "disabled", json: "disabled", kind: lookupBool, desc: "Whether the client is disabled."},
		},
		list: func(ctx context.Context, c *client.ClientWithResponses, name string) ([]map[string]any, error) {
			return listAllPages(func(limit, offset int) (int, []byte, error) {
				r, err := c.GetOauthClientsWithResponse(ctx, &client.GetOauthClientsParams{Search: &name, Limit: &limit, Offset: &offset})
				if err != nil {
					return 0, nil, err
				}
				return r.StatusCode(), r.Body, nil
			})
		},
	}}
}
