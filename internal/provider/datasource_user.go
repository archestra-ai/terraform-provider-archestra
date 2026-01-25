package provider

import (
	"context"
	"fmt"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

var _ datasource.DataSource = &UserDataSource{}

func NewUserDataSource() datasource.DataSource {
	return &UserDataSource{}
}

type UserDataSource struct {
	client *client.ClientWithResponses
}

type UserDataSourceModel struct {
	ID       types.String `tfsdk:"id"`
	Name     types.String `tfsdk:"name"`
	Email    types.String `tfsdk:"email"`
	Image    types.String `tfsdk:"image"`
	Role     types.String `tfsdk:"role"`
	MemberID types.String `tfsdk:"member_id"`
}

func (d *UserDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_user"
}

func (d *UserDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Fetches an Archestra user by ID or email. Use this to look up existing users for role assignments.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "User identifier. Either id or email must be provided.",
				Optional:            true,
				Computed:            true,
			},
			"email": schema.StringAttribute{
				MarkdownDescription: "The email address of the user. Either id or email must be provided.",
				Optional:            true,
				Computed:            true,
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "The name of the user",
				Computed:            true,
			},
			"image": schema.StringAttribute{
				MarkdownDescription: "Profile image URL",
				Computed:            true,
			},
			"role": schema.StringAttribute{
				MarkdownDescription: "Current role assigned to the user",
				Computed:            true,
			},
			"member_id": schema.StringAttribute{
				MarkdownDescription: "Organization member ID (used for role assignments)",
				Computed:            true,
			},
		},
	}
}

func (d *UserDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*client.ClientWithResponses)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *client.ClientWithResponses, got: %T", req.ProviderData),
		)
		return
	}

	d.client = client
}

func (d *UserDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data UserDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Validate that at least one identifier is provided
	if data.ID.IsNull() && data.Email.IsNull() {
		resp.Diagnostics.AddError(
			"Missing Required Attribute",
			"Either 'id' or 'email' must be provided to look up a user.",
		)
		return
	}

	// Look up by email if provided, otherwise by ID
	if !data.Email.IsNull() {
		email := openapi_types.Email(data.Email.ValueString())
		userResp, err := d.client.GetUserByEmailWithResponse(ctx, email)
		if err != nil {
			resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to read user, got error: %s", err))
			return
		}

		if userResp.JSON404 != nil {
			resp.Diagnostics.AddError("Not Found", fmt.Sprintf("User with email '%s' not found", data.Email.ValueString()))
			return
		}

		if userResp.JSON200 == nil {
			resp.Diagnostics.AddError("Unexpected API Response", fmt.Sprintf("Expected 200 OK, got status %d", userResp.StatusCode()))
			return
		}

		user := userResp.JSON200
		data.ID = types.StringValue(user.Id)
		data.Name = types.StringValue(user.Name)
		data.Email = types.StringValue(user.Email)
		if user.Image != nil {
			data.Image = types.StringValue(*user.Image)
		} else {
			data.Image = types.StringNull()
		}
		if user.Member != nil {
			data.Role = types.StringValue(user.Member.Role)
			data.MemberID = types.StringValue(user.Member.Id)
		} else {
			data.Role = types.StringNull()
			data.MemberID = types.StringNull()
		}
	} else {
		userResp, err := d.client.GetUserByIdWithResponse(ctx, data.ID.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to read user, got error: %s", err))
			return
		}

		if userResp.JSON404 != nil {
			resp.Diagnostics.AddError("Not Found", fmt.Sprintf("User with ID '%s' not found", data.ID.ValueString()))
			return
		}

		if userResp.JSON200 == nil {
			resp.Diagnostics.AddError("Unexpected API Response", fmt.Sprintf("Expected 200 OK, got status %d", userResp.StatusCode()))
			return
		}

		user := userResp.JSON200
		data.ID = types.StringValue(user.Id)
		data.Name = types.StringValue(user.Name)
		data.Email = types.StringValue(user.Email)
		if user.Image != nil {
			data.Image = types.StringValue(*user.Image)
		} else {
			data.Image = types.StringNull()
		}
		if user.Member != nil {
			data.Role = types.StringValue(user.Member.Role)
			data.MemberID = types.StringValue(user.Member.Id)
		} else {
			data.Role = types.StringNull()
			data.MemberID = types.StringNull()
		}
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
