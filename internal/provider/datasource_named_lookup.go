package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// namedLookupDataSource finds exactly one object by an exact name among those
// a list endpoint returns, and exposes a few of its fields. The archestra_role,
// archestra_project, archestra_knowledge_base, archestra_oauth_client,
// archestra_identity_provider and archestra_mcp_registry_entry data sources are
// declared with it.
type namedLookupDataSource struct {
	client *client.ClientWithResponses
	spec   namedLookupSpec
}

type namedLookupSpec struct {
	typeName    string
	noun        string
	description string
	key         namedLookupField
	fields      []namedLookupField
	// list returns every object visible to the caller whose key might match.
	list func(ctx context.Context, c *client.ClientWithResponses, key string) ([]map[string]any, error)
}

type namedLookupField struct {
	tf, json, desc string
	kind           namedLookupKind
}

type namedLookupKind int

const (
	lookupString namedLookupKind = iota
	lookupBool
	lookupStringList
)

func (d *namedLookupDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_" + d.spec.typeName
}

func (d *namedLookupDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	attrs := map[string]schema.Attribute{
		"id":          schema.StringAttribute{Computed: true, MarkdownDescription: d.spec.noun + " ID."},
		d.spec.key.tf: schema.StringAttribute{Required: true, MarkdownDescription: d.spec.key.desc},
	}
	for _, f := range d.spec.fields {
		switch f.kind {
		case lookupBool:
			attrs[f.tf] = schema.BoolAttribute{Computed: true, MarkdownDescription: f.desc}
		case lookupStringList:
			attrs[f.tf] = schema.ListAttribute{Computed: true, ElementType: types.StringType, MarkdownDescription: f.desc}
		default:
			attrs[f.tf] = schema.StringAttribute{Computed: true, MarkdownDescription: f.desc}
		}
	}
	resp.Schema = schema.Schema{MarkdownDescription: d.spec.description, Attributes: attrs}
}

func (d *namedLookupDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = dataSourceClient(req, resp)
}

func (d *namedLookupDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var key types.String
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root(d.spec.key.tf), &key)...)
	if resp.Diagnostics.HasError() {
		return
	}
	items, err := d.spec.list(ctx, d.client, key.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to list %ss: %s", d.spec.noun, err))
		return
	}
	var match map[string]any
	for _, item := range items {
		if value, _ := item[d.spec.key.json].(string); value != key.ValueString() {
			continue
		}
		if match != nil {
			resp.Diagnostics.AddError(fmt.Sprintf("Ambiguous %s", d.spec.noun), fmt.Sprintf("More than one %s has the %s %q.", d.spec.noun, d.spec.key.tf, key.ValueString()))
			return
		}
		match = item
	}
	if match == nil {
		resp.Diagnostics.AddError(fmt.Sprintf("%s Not Found", d.spec.noun), fmt.Sprintf("No %s visible to these credentials has the %s %q.", d.spec.noun, d.spec.key.tf, key.ValueString()))
		return
	}

	set := func(name string, value any) {
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root(name), value)...)
	}
	set("id", lookupString.value(match["id"]))
	set(d.spec.key.tf, key)
	for _, f := range d.spec.fields {
		set(f.tf, f.kind.value(match[f.json]))
	}
}

func (k namedLookupKind) value(raw any) any {
	switch k {
	case lookupBool:
		if b, ok := raw.(bool); ok {
			return types.BoolValue(b)
		}
		return types.BoolNull()
	case lookupStringList:
		list, _ := raw.([]any)
		values := make([]string, 0, len(list))
		for _, item := range list {
			if s, ok := item.(string); ok {
				values = append(values, s)
			}
		}
		return values
	default:
		switch v := raw.(type) {
		case string:
			return types.StringValue(v)
		case float64, bool:
			return types.StringValue(fmt.Sprint(v))
		}
		return types.StringNull()
	}
}

// listItems decodes a list response: a plain array, or one page of a
// `{data, pagination}` envelope with whether another page follows.
func listItems(status int, body []byte) ([]map[string]any, bool, error) {
	if status != http.StatusOK {
		return nil, false, fmt.Errorf("API returned status %d: %s", status, string(body))
	}
	var array []map[string]any
	if err := json.Unmarshal(body, &array); err == nil {
		return array, false, nil
	}
	var page struct {
		Data       []map[string]any `json:"data"`
		Pagination struct {
			HasNext bool `json:"hasNext"`
		} `json:"pagination"`
	}
	if err := json.Unmarshal(body, &page); err != nil {
		return nil, false, err
	}
	return page.Data, page.Pagination.HasNext, nil
}

// listAllPages pages through a `{data, pagination}` endpoint, 100 at a time.
func listAllPages(fetch func(limit, offset int) (int, []byte, error)) ([]map[string]any, error) {
	const limit = 100
	var all []map[string]any
	for offset := 0; ; offset += limit {
		status, body, err := fetch(limit, offset)
		if err != nil {
			return nil, err
		}
		items, hasNext, err := listItems(status, body)
		if err != nil {
			return nil, err
		}
		all = append(all, items...)
		if !hasNext || len(items) == 0 {
			return all, nil
		}
	}
}
