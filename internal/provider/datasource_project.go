package provider

import (
	"context"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
)

func NewProjectDataSource() datasource.DataSource {
	return &namedLookupDataSource{spec: namedLookupSpec{
		typeName:    "project",
		noun:        "Project",
		description: "Finds a project by its exact name among the projects these credentials can see.",
		key:         namedLookupField{tf: "name", json: "name", desc: "Exact name of the project."},
		fields: []namedLookupField{
			{tf: "description", json: "description", desc: "Project description."},
			{tf: "visibility", json: "visibility", desc: "Who the project is shared with."},
		},
		list: func(ctx context.Context, c *client.ClientWithResponses, name string) ([]map[string]any, error) {
			return listAllPages(func(limit, offset int) (int, []byte, error) {
				r, err := c.GetProjectsWithResponse(ctx, &client.GetProjectsParams{Search: &name, Limit: &limit, Offset: &offset})
				if err != nil {
					return 0, nil, err
				}
				return r.StatusCode(), r.Body, nil
			})
		},
	}}
}
