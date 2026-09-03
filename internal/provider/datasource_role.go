package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &RoleDataSource{}

func NewRoleDataSource() datasource.DataSource {
	return &RoleDataSource{}
}

type RoleDataSource struct {
	client *client.ClientWithResponses
}

type RoleDataSourceModel struct {
	ID          types.String `tfsdk:"id"`
	Role        types.String `tfsdk:"role"`
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
	Predefined  types.Bool   `tfsdk:"predefined"`
	Permission  types.Map    `tfsdk:"permission"`
	CreatedAt   types.String `tfsdk:"created_at"`
	UpdatedAt   types.String `tfsdk:"updated_at"`
}

func (d *RoleDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_role"
}

func (d *RoleDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Looks up a role by ID. Accepts both base62 custom-role IDs and predefined role names (`admin`, `member`, `owner`, …).",
		Attributes: map[string]schema.Attribute{
			"id":          schema.StringAttribute{Required: true, MarkdownDescription: "Role identifier — base62 for custom roles or the literal name for predefined roles."},
			"role":        schema.StringAttribute{Computed: true, MarkdownDescription: "Immutable role slug."},
			"name":        schema.StringAttribute{Computed: true, MarkdownDescription: "Human-readable role name."},
			"description": schema.StringAttribute{Computed: true, MarkdownDescription: "Free-form description."},
			"predefined":  schema.BoolAttribute{Computed: true, MarkdownDescription: "True for backend-managed roles."},
			"permission": schema.MapAttribute{
				Computed:            true,
				ElementType:         types.ListType{ElemType: types.StringType},
				MarkdownDescription: "Permission grants keyed by resource name; values are lists of action verbs.",
			},
			"created_at": schema.StringAttribute{Computed: true, MarkdownDescription: "Creation timestamp (RFC 3339)."},
			"updated_at": schema.StringAttribute{Computed: true, MarkdownDescription: "Last-update timestamp (RFC 3339), or null if never updated."},
		},
	}
}

func (d *RoleDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.ClientWithResponses)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *client.ClientWithResponses, got: %T", req.ProviderData),
		)
		return
	}
	d.client = c
}

func (d *RoleDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data RoleDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := d.client.GetRoleWithResponse(ctx, data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to read role: %s", err))
		return
	}
	if IsNotFound(apiResp) {
		resp.Diagnostics.AddError("Not Found", fmt.Sprintf("Role %q not found.", data.ID.ValueString()))
		return
	}
	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError("Unexpected API Response", fmt.Sprintf("Expected 200 OK, got status %d: %s", apiResp.StatusCode(), string(apiResp.Body)))
		return
	}

	var src struct {
		CreatedAt   time.Time           `json:"createdAt"`
		Description *string             `json:"description"`
		ID          string              `json:"id"`
		Name        string              `json:"name"`
		Permission  map[string][]string `json:"permission"`
		Predefined  bool                `json:"predefined"`
		Role        string              `json:"role"`
		UpdatedAt   *time.Time          `json:"updatedAt"`
	}
	if err := json.Unmarshal(apiResp.Body, &src); err != nil {
		resp.Diagnostics.AddError("API Response Decode Error", err.Error())
		return
	}

	data.ID = types.StringValue(src.ID)
	data.Role = types.StringValue(src.Role)
	data.Name = types.StringValue(src.Name)
	if src.Description != nil {
		data.Description = types.StringValue(*src.Description)
	} else {
		data.Description = types.StringNull()
	}
	data.Predefined = types.BoolValue(src.Predefined)
	data.CreatedAt = types.StringValue(src.CreatedAt.Format(time.RFC3339))
	if src.UpdatedAt != nil {
		data.UpdatedAt = types.StringValue(src.UpdatedAt.Format(time.RFC3339))
	} else {
		data.UpdatedAt = types.StringNull()
	}

	listType := types.ListType{ElemType: types.StringType}
	if src.Permission == nil {
		data.Permission = types.MapNull(listType)
	} else {
		keys := make([]string, 0, len(src.Permission))
		for k := range src.Permission {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		out := make(map[string]attr.Value, len(keys))
		for _, k := range keys {
			list, lDiags := types.ListValueFrom(ctx, types.StringType, src.Permission[k])
			resp.Diagnostics.Append(lDiags...)
			out[k] = list
		}
		m, mDiags := types.MapValue(listType, out)
		resp.Diagnostics.Append(mDiags...)
		if resp.Diagnostics.HasError() {
			return
		}
		data.Permission = m
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
