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
)

var _ resource.Resource = &UserRoleAssignmentResource{}
var _ resource.ResourceWithImportState = &UserRoleAssignmentResource{}

func NewUserRoleAssignmentResource() resource.Resource {
	return &UserRoleAssignmentResource{}
}

type UserRoleAssignmentResource struct {
	client *client.ClientWithResponses
}

type UserRoleAssignmentResourceModel struct {
	ID             types.String `tfsdk:"id"`
	UserID         types.String `tfsdk:"user_id"`
	RoleIdentifier types.String `tfsdk:"role_identifier"`
}

func (r *UserRoleAssignmentResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_user_role_assignment"
}

func (r *UserRoleAssignmentResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages the assignment of a role to an Archestra user.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Resource identifier (same as user_id)",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"user_id": schema.StringAttribute{
				MarkdownDescription: "The ID of the user to assign the role to",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"role_identifier": schema.StringAttribute{
				MarkdownDescription: "The ID of the role to assign",
				Required:            true,
			},
		},
	}
}

func (r *UserRoleAssignmentResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*client.ClientWithResponses)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *client.ClientWithResponses, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	r.client = client
}

func (r *UserRoleAssignmentResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data UserRoleAssignmentResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	userID := data.UserID.ValueString()
	roleNonUUIDIdentifier := data.RoleIdentifier.ValueString()

	apiResp, err := r.client.UpdateUserRoleWithResponse(ctx, userID, roleNonUUIDIdentifier)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to assign role to user, got error: %s", err))
		return
	}

	if apiResp.JSON200 == nil {
		status := apiResp.StatusCode()
		if status == 404 {
			resp.Diagnostics.AddError("Resource Not Found", fmt.Sprintf("User with ID %s not found", userID))
		} else {
			resp.Diagnostics.AddError("Unexpected API Response", fmt.Sprintf("Expected 200 OK, got status %d", status))
		}
		return
	}

	user := apiResp.JSON200
	if user.Role != roleNonUUIDIdentifier {
		resp.Diagnostics.AddWarning("Role Assignment Mismatch", "The API returned a user object but the role does not match the requested role. This might be due to eventual consistency or permissions.")
	}

	data.ID = types.StringValue(user.Id)
	data.UserID = types.StringValue(user.UserId)
	data.RoleIdentifier = types.StringValue(user.Role)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *UserRoleAssignmentResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data UserRoleAssignmentResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	userID := data.UserID.ValueString()

	apiResp, err := r.client.GetUserRoleWithResponse(ctx, userID)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to read user, got error: %s", err))
		return
	}

	if apiResp.JSON404 != nil {
		resp.State.RemoveResource(ctx)
		return
	}

	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError("Unexpected API Response", fmt.Sprintf("Expected 200 OK, got status %d", apiResp.StatusCode()))
		return
	}

	user := apiResp.JSON200
	data.ID = types.StringValue(user.Id)
	data.UserID = types.StringValue(user.UserId)
	data.RoleIdentifier = types.StringValue(user.Role)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *UserRoleAssignmentResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data UserRoleAssignmentResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	userID := data.UserID.ValueString()
	roleNonUUIDIdentifier := data.RoleIdentifier.ValueString()

	apiResp, err := r.client.UpdateUserRoleWithResponse(ctx, userID, roleNonUUIDIdentifier)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to assign role to user, got error: %s", err))
		return
	}

	if apiResp.JSON200 == nil {
		status := apiResp.StatusCode()
		if status == 404 {
			resp.Diagnostics.AddError("Resource Not Found", fmt.Sprintf("User with ID %s not found", userID))
		} else {
			resp.Diagnostics.AddError("Unexpected API Response", fmt.Sprintf("Expected 200 OK, got status %d", status))
		}
		return
	}

	member := apiResp.JSON200
	if member.Role != roleNonUUIDIdentifier {
		resp.Diagnostics.AddWarning("Role Assignment Mismatch", "The API returned a user object but the role does not match the requested role. This might be due to eventual consistency or permissions.")
	}

	data.ID = types.StringValue(member.Id)
	data.UserID = types.StringValue(member.UserId)
	data.RoleIdentifier = types.StringValue(member.Role)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *UserRoleAssignmentResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data UserRoleAssignmentResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	userID := data.UserID.ValueString()

	apiResp, err := r.client.DeleteRoleWithResponse(ctx, userID)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to remove role assignment, got error: %s", err))
		return
	}

	if apiResp.JSON200 == nil && apiResp.JSON404 == nil {
		resp.Diagnostics.AddError("Unexpected API Response", fmt.Sprintf("Expected 200 OK, got status %d", apiResp.StatusCode()))
		return
	}
}

func (r *UserRoleAssignmentResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
