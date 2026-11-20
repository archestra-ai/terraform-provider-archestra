package provider

import (
	"context"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
)

func NewKnowledgeBaseDataSource() datasource.DataSource {
	return &namedLookupDataSource{spec: namedLookupSpec{
		typeName:    "knowledge_base",
		noun:        "Knowledge Base",
		description: "Finds a knowledge base by its exact name among those these credentials can see, for example to assign one created in the UI to an agent.",
		key:         namedLookupField{tf: "name", json: "name", desc: "Exact name of the knowledge base."},
		fields: []namedLookupField{
			{tf: "description", json: "description", desc: "Knowledge base description."},
			{tf: "visibility", json: "visibility", desc: "Who the knowledge base is shared with."},
			{tf: "team_ids", json: "teamIds", kind: lookupStringList, desc: "Teams the knowledge base is shared with."},
		},
		list: func(ctx context.Context, c *client.ClientWithResponses, name string) ([]map[string]any, error) {
			return listAllPages(func(limit, offset int) (int, []byte, error) {
				r, err := c.GetKnowledgeBasesWithResponse(ctx, &client.GetKnowledgeBasesParams{Search: &name, Limit: &limit, Offset: &offset})
				if err != nil {
					return 0, nil, err
				}
				return r.StatusCode(), r.Body, nil
			})
		},
	}}
}
