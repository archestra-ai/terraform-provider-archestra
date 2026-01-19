// TODO: User API endpoints are not exposed by the backend, in a way that they can be easily codegen'd
// (right now they are exposed as a single "catch-all" route, /api/auth/*, which makes codegen impossible)
// so this datasource is not yet available.
package provider

import (
	"context"
	"fmt"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

var _ resource.Resource = &UserResource{}
var _ resource.ResourceWithImportState = &UserResource{}

func NewUserResource() resource.Resource {
	return &UserResource{}
}

type UserResource struct {
	client *client.ClientWithResponses
}

type UserResourceModel struct {
	Id       types.String `tfsdk:"id"`
	Name     types.String `tfsdk:"name"`
	Email    types.String `tfsdk:"email"`
	Image    types.String `tfsdk:"image"`
	Password types.String `tfsdk:"password"`
}

func (r *UserResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_user"
}

func (r *UserResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages an Archestra user.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "User identifier",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "The name of the user",
				Required:            true,
			},
			"email": schema.StringAttribute{
				MarkdownDescription: "The email address of the user",
				Required:            true,
			},
			"password": schema.StringAttribute{
				MarkdownDescription: "The password of the user",
				Required:            true,
			},
			"image": schema.StringAttribute{
				MarkdownDescription: "Profile image URL",
				Optional:            true,
			},
		},
	}
}

func (r *UserResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*client.ClientWithResponses)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *client.ClientWithResponses, got: %T", req.ProviderData),
		)
		return
	}

	r.client = client
}

func (r *UserResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data UserResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	user := client.CreateUserJSONRequestBody{
		Name:     data.Name.ValueString(),
		Email:    data.Email.ValueString(),
		Password: data.Password.ValueString(),
	}

	if !data.Id.IsNull() {
		id := data.Id.ValueString()
		user.Id = &id
	}

	if !data.Image.IsNull() {
		img := data.Image.ValueString()
		user.Image = &img
	}

	apiResponse, err := r.client.CreateUserWithResponse(ctx, user)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create user, got error: %s", err))
		return
	}

	if apiResponse.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Expected 200 OK, got status %d", apiResponse.StatusCode()),
		)
		return
	}
	created := apiResponse.JSON200

	data.Id = types.StringValue(created.Id)
	data.Name = types.StringValue(created.Name)
	data.Email = types.StringValue(created.Email)

	if created.Image != nil {
		data.Image = types.StringValue(*created.Image)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *UserResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data UserResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	params := &client.GetUserParams{
		Id: data.Id.ValueString(),
	}
	apiResponse, err := r.client.GetUserWithResponse(ctx, params)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read user, got error: %s", err))
		return
	}

	if apiResponse.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Expected 200 OK, got status %d", apiResponse.StatusCode()),
		)
		return
	}
	user := apiResponse.JSON200

	data.Name = types.StringValue(user.Name)
	data.Email = types.StringValue(user.Email)
	if user.Image != nil {
		data.Image = types.StringValue(*user.Image)
	} else {
		data.Image = types.StringNull()
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *UserResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data UserResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	name := data.Name.ValueString()
	email := openapi_types.Email(data.Email.ValueString())

	body := client.UpdateUserJSONRequestBody{
		Name:  &name,
		Email: &email,
	}

	if !data.Image.IsNull() {
		img := data.Image.ValueString()
		body.Image = &img
	}

	apiResponse, err := r.client.UpdateUserWithResponse(ctx, data.Id.ValueString(), body)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update user, got error: %s", err))
		return
	}

	if apiResponse.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Expected 200 OK, got status %d", apiResponse.StatusCode()),
		)
		return
	}
	updated := apiResponse.JSON200

	data.Name = types.StringValue(updated.Name)
	data.Email = types.StringValue(updated.Email)
	if updated.Image != nil {
		data.Image = types.StringValue(*updated.Image)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *UserResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data UserResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResponse, err := r.client.DeleteUserWithResponse(ctx, data.Id.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete user, got error: %s", err))
		return
	}

	if apiResponse.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Expected 200 OK, got status %d", apiResponse.StatusCode()),
		)
		return
	}
}

func (r *UserResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
