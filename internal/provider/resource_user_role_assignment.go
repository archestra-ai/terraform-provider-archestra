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
	RoleID         types.String `tfsdk:"role_id"`
	OrganizationID types.String `tfsdk:"organization_id"`
}

func (r *UserRoleAssignmentResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_user_role_assignment"
}

func (r *UserRoleAssignmentResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Assigns an RBAC role to a user in Archestra. The role can be a predefined system role (admin, editor, member) or a custom role ID.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The member ID (organization membership identifier)",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"user_id": schema.StringAttribute{
				MarkdownDescription: "The user ID to assign the role to",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"role_id": schema.StringAttribute{
				MarkdownDescription: "The role to assign. Can be a predefined role name ('admin', 'editor', 'member') or a custom role ID",
				Required:            true,
			},
			"organization_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The organization ID this assignment belongs to",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
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

	// Build typed request body and call API
	requestBody := client.UpdateMemberRoleJSONRequestBody{
		Role: data.RoleID.ValueString(),
	}

	apiResp, err := r.client.UpdateMemberRoleWithResponse(ctx, data.UserID.ValueString(), requestBody)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to assign role to user, got error: %s", err))
		return
	}

	if apiResp.JSON200 == nil {
		errMsg := fmt.Sprintf("Expected 200 OK, got status %d", apiResp.StatusCode())
		if apiResp.JSON400 != nil {
			errMsg = apiResp.JSON400.Error.Message
		} else if apiResp.JSON404 != nil {
			errMsg = apiResp.JSON404.Error.Message
		}
		resp.Diagnostics.AddError("API Error", errMsg)
		return
	}

	member := apiResp.JSON200
	data.ID = types.StringValue(member.Id)
	data.OrganizationID = types.StringValue(member.OrganizationId)
	// Update role_id with what the API returned (in case of normalization)
	data.RoleID = types.StringValue(member.Role)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *UserRoleAssignmentResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data UserRoleAssignmentResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Get the organization member to read the current role
	apiResp, err := r.client.GetOrganizationMemberWithResponse(ctx, data.UserID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to read user role assignment, got error: %s", err))
		return
	}

	if apiResp.JSON404 != nil {
		resp.State.RemoveResource(ctx)
		return
	}

	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Expected 200 OK, got status %d", apiResp.StatusCode()))
		return
	}

	member := apiResp.JSON200
	data.ID = types.StringValue(member.Id)
	data.UserID = types.StringValue(member.UserId)
	data.RoleID = types.StringValue(member.Role)
	data.OrganizationID = types.StringValue(member.OrganizationId)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *UserRoleAssignmentResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data UserRoleAssignmentResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Build typed request body and call API
	requestBody := client.UpdateMemberRoleJSONRequestBody{
		Role: data.RoleID.ValueString(),
	}

	apiResp, err := r.client.UpdateMemberRoleWithResponse(ctx, data.UserID.ValueString(), requestBody)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to update user role assignment, got error: %s", err))
		return
	}

	if apiResp.JSON200 == nil {
		errMsg := fmt.Sprintf("Expected 200 OK, got status %d", apiResp.StatusCode())
		if apiResp.JSON400 != nil {
			errMsg = apiResp.JSON400.Error.Message
		} else if apiResp.JSON404 != nil {
			errMsg = apiResp.JSON404.Error.Message
		}
		resp.Diagnostics.AddError("API Error", errMsg)
		return
	}

	member := apiResp.JSON200
	data.ID = types.StringValue(member.Id)
	data.OrganizationID = types.StringValue(member.OrganizationId)
	data.RoleID = types.StringValue(member.Role)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *UserRoleAssignmentResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data UserRoleAssignmentResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// On delete, reset the user's role to "member" (the default role)
	requestBody := client.UpdateMemberRoleJSONRequestBody{
		Role: "member",
	}

	_, err := r.client.UpdateMemberRoleWithResponse(ctx, data.UserID.ValueString(), requestBody)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to reset user role, got error: %s", err))
		return
	}
	// We don't check the response here - if the user was deleted, that's fine
}

func (r *UserRoleAssignmentResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Import by user_id
	resource.ImportStatePassthroughID(ctx, path.Root("user_id"), req, resp)
}
