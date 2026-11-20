package provider

import (
	"context"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
)

func NewRoleDataSource() datasource.DataSource {
	return &namedLookupDataSource{spec: namedLookupSpec{
		typeName:    "role",
		noun:        "Role",
		description: "Finds a predefined or custom role by its exact display name, for example one created in the UI. Use `role` wherever a role identifier is expected, such as the `role` of a service account. Granular roles are not listed: use their identifier, such as `fixed:agents:reader`, directly.",
		key:         namedLookupField{tf: "name", json: "name", desc: "Exact display name of the role."},
		fields: []namedLookupField{
			{tf: "role", json: "role", desc: "Role identifier."},
			{tf: "description", json: "description", desc: "What the role grants."},
			{tf: "predefined", json: "predefined", kind: lookupBool, desc: "Whether the role is predefined rather than custom."},
		},
		list: func(ctx context.Context, c *client.ClientWithResponses, name string) ([]map[string]any, error) {
			return listAllPages(func(limit, offset int) (int, []byte, error) {
				r, err := c.GetRolesWithResponse(ctx, &client.GetRolesParams{Name: &name, Limit: &limit, Offset: &offset})
				if err != nil {
					return 0, nil, err
				}
				return r.StatusCode(), r.Body, nil
			})
		},
	}}
}
