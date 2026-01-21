package provider

import (
	"context"
	"fmt"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &UserDataSource{}

func NewUserDataSource() datasource.DataSource {
	return &UserDataSource{}
}

type UserDataSource struct {
	client *client.ClientWithResponses
}

type UserDataSourceModel struct {
	ID            types.String `tfsdk:"id"`
	Name          types.String `tfsdk:"name"`
	Email         types.String `tfsdk:"email"`
	EmailVerified types.Bool   `tfsdk:"email_verified"`
	Image         types.String `tfsdk:"image"`
	Role          types.String `tfsdk:"role"`
	Banned        types.Bool   `tfsdk:"banned"`
	BanReason     types.String `tfsdk:"ban_reason"`
}

func (d *UserDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_user"
}

func (d *UserDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Fetches an Archestra user by ID or email.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "User identifier",
				Optional:            true,
				Computed:            true,
				Validators: []validator.String{
					stringvalidator.ExactlyOneOf(path.MatchRoot("email")),
				},
			},
			"email": schema.StringAttribute{
				MarkdownDescription: "The email address of the user",
				Optional:            true,
				Computed:            true,
				Validators: []validator.String{
					stringvalidator.ExactlyOneOf(path.MatchRoot("id")),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "The name of the user",
				Computed:            true,
			},
			"email_verified": schema.BoolAttribute{
				MarkdownDescription: "Whether the user's email is verified",
				Computed:            true,
			},
			"image": schema.StringAttribute{
				MarkdownDescription: "The URL of the user's profile image",
				Computed:            true,
			},
			"role": schema.StringAttribute{
				MarkdownDescription: "The role of the user",
				Computed:            true,
			},
			"banned": schema.BoolAttribute{
				MarkdownDescription: "Whether the user is banned",
				Computed:            true,
			},
			"ban_reason": schema.StringAttribute{
				MarkdownDescription: "The reason for the user's ban",
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

	var user *UserSharedModel
	var err error

	if !data.ID.IsNull() {
		userId := data.ID.ValueString()
		user, err = getUser(ctx, d.client, userId, "")
	} else if !data.Email.IsNull() {
		email := data.Email.ValueString()
		user, err = getUser(ctx, d.client, "", email)
	} else {
		resp.Diagnostics.AddError("Missing Configuration", "One of 'id' or 'email' must be configured.")
		return
	}

	if err != nil {
		resp.Diagnostics.AddError("API Error", err.Error())
		return
	}

	if user == nil {
		resp.Diagnostics.AddError("Not Found", "User not found")
		return
	}

	safeString := func(s *string) types.String {
		if s != nil {
			return types.StringValue(*s)
		}
		return types.StringNull()
	}

	data.ID = types.StringValue(user.Id)
	data.Name = types.StringValue(user.Name)
	data.Email = types.StringValue(user.Email)

	data.EmailVerified = types.BoolValue(user.EmailVerified)

	if user.Banned != nil {
		data.Banned = types.BoolValue(*user.Banned)
	} else {
		data.Banned = types.BoolNull()
	}

	data.Image = safeString(user.Image)
	data.Role = safeString(user.Role)
	data.BanReason = safeString(user.BanReason)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
