// NOTE: User role assignment API endpoints are not yet implemented in the backend.
// This resource is disabled until the following endpoints are available:
// - POST /users/{userId}/roles (assign role)
// - DELETE /users/{userId}/roles/{roleId} (unassign role)
// - GET /users/{userId}/roles (list user roles)
//
// When these endpoints become available in the OpenAPI spec and are generated
//  in the client, this file can be uncommented and the resource enabled in provider.go

package provider

// import (
// 	"context"
// 	"fmt"
// 	"strings"

// 	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
// 	"github.com/hashicorp/terraform-plugin-framework/path"
// 	"github.com/hashicorp/terraform-plugin-framework/resource"
// 	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
// 	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
// 	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
// 	"github.com/hashicorp/terraform-plugin-framework/types"
// )

// var _ resource.Resource = &UserRoleAssignmentResource{}
// var _ resource.ResourceWithImportState = &UserRoleAssignmentResource{}

// func NewUserRoleAssignmentResource() resource.Resource {
// 	return &UserRoleAssignmentResource{}
// }

// type UserRoleAssignmentResource struct {
// 	client *client.ClientWithResponses
// }

// type UserRoleAssignmentResourceModel struct {
// 	ID     types.String `tfsdk:"id"`
// 	UserID types.String `tfsdk:"user_id"`
// 	RoleID types.String `tfsdk:"role_id"`
// }

// func (r *UserRoleAssignmentResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
// 	resp.TypeName = req.ProviderTypeName + "_user_role_assignment"
// }

// func (r *UserRoleAssignmentResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
// 	resp.Schema = schema.Schema{
// 		MarkdownDescription: "Assigns a role to a user in Archestra.",

// 		Attributes: map[string]schema.Attribute{
// 			"id": schema.StringAttribute{
// 				Computed:            true,
// 				MarkdownDescription: "Assignment identifier (format: user_id:role_id)",
// 				PlanModifiers: []planmodifier.String{
// 					stringplanmodifier.UseStateForUnknown(),
// 				},
// 			},
// 			"user_id": schema.StringAttribute{
// 				MarkdownDescription: "The ID of the user",
// 				Required:            true,
// 				PlanModifiers: []planmodifier.String{
// 					stringplanmodifier.RequiresReplace(),
// 				},
// 			},
// 			"role_id": schema.StringAttribute{
// 				MarkdownDescription: "The ID of the role to assign",
// 				Required:            true,
// 				PlanModifiers: []planmodifier.String{
// 					stringplanmodifier.RequiresReplace(),
// 				},
// 			},
// 		},
// 	}
// }

// func (r *UserRoleAssignmentResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
// 	if req.ProviderData == nil {
// 		return
// 	}

// 	client, ok := req.ProviderData.(*client.ClientWithResponses)
// 	if !ok {
// 		resp.Diagnostics.AddError(
// 			"Unexpected Resource Configure Type",
// 			fmt.Sprintf("Expected *client.ClientWithResponses, got: %T. Please report this issue to the provider developers.", req.ProviderData),
// 		)
// 		return
// 	}

// 	r.client = client
// }

// func (r *UserRoleAssignmentResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
// 	var data UserRoleAssignmentResourceModel
// 	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
// 	if resp.Diagnostics.HasError() {
// 		return
// 	}

// 	// Create assignment ID
// 	data.ID = types.StringValue(fmt.Sprintf("%s:%s", data.UserID.ValueString(), data.RoleID.ValueString()))

// 	// TODO: Call API to assign role to user when endpoint is available
// 	// Expected: POST /users/{userId}/roles with body containing roleId
// 	//
// 	// requestBody := client.AssignUserRoleJSONRequestBody{
// 	// 	RoleId: data.RoleID.ValueString(),
// 	// }
// 	// apiResp, err := r.client.AssignUserRoleWithResponse(ctx, data.UserID.ValueString(), requestBody)

// 	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
// }

// func (r *UserRoleAssignmentResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
// 	var data UserRoleAssignmentResourceModel
// 	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
// 	if resp.Diagnostics.HasError() {
// 		return
// 	}

// 	// TODO: Get user's roles and verify this assignment still exists when endpoint is available
// 	// Expected: GET /users/{userId}/roles
//  //
// 	// rolesResp, err := r.client.GetUserRolesWithResponse(ctx, data.UserID.ValueString())

// 	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
// }

// func (r *UserRoleAssignmentResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
// 	// Since both user_id and role_id require replacement, update should not be called
// 	resp.Diagnostics.AddError(
// 		"Update Not Supported",
// 		"User role assignments cannot be updated. Both user_id and role_id require replacement.",
// 	)
// }

// func (r *UserRoleAssignmentResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
// 	var data UserRoleAssignmentResourceModel
// 	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
// 	if resp.Diagnostics.HasError() {
// 		return
// 	}

// 	// TODO: Call API to unassign role from user when endpoint is available
// 	// Expected: DELETE /users/{userId}/roles/{roleId}
// 	//
// 	// apiResp, err := r.client.UnassignUserRoleWithResponse(ctx, data.UserID.ValueString(), data.RoleID.ValueString())
// }

// func (r *UserRoleAssignmentResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
// 	// Import format: user_id:role_id
// 	parts := strings.Split(req.ID, ":")
// 	if len(parts) != 2 {
// 		resp.Diagnostics.AddError(
// 			"Invalid Import ID",
// 			fmt.Sprintf("Expected import ID in format 'user_id:role_id', got: %s", req.ID),
// 		)
// 		return
// 	}

// 	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
// 	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("user_id"), parts[0])...)
// 	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("role_id"), parts[1])...)
// }
