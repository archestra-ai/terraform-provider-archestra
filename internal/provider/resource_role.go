package provider

import (
	"context"
	"fmt"
	"net/http"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &RoleResource{}
var _ resource.ResourceWithConfigure = &RoleResource{}
var _ resource.ResourceWithImportState = &RoleResource{}

func NewRoleResource() resource.Resource {
	return &RoleResource{}
}

// RoleResource defines the resource implementation.
type RoleResource struct {
	client *client.ClientWithResponses
}

// RoleResourceModel describes the resource data model.
type RoleResourceModel struct {
	ID          types.String   `tfsdk:"id"`
	Name        types.String   `tfsdk:"name"`
	Description types.String   `tfsdk:"description"`
	Permissions []types.String `tfsdk:"permissions"`
}

func (r *RoleResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_role"
}

func (r *RoleResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		// This description is used by the documentation generator and the language server.
		MarkdownDescription: "Archestra Role",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Role identifier",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Detailed name of the role",
			},
			"description": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Description of the role",
			},
			"permissions": schema.ListAttribute{
				ElementType:         types.StringType,
				Required:            true,
				MarkdownDescription: "List of permissions assigned to the role",
			},
		},
	}
}

func (r *RoleResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
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

func (r *RoleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data RoleResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	permissions := make([]string, len(data.Permissions))
	for i, p := range data.Permissions {
		permissions[i] = p.ValueString()
	}

	body := client.CreateCustomRoleJSONBody{
		Name:        data.Name.ValueString(),
		Permissions: permissions,
	}

	if !data.Description.IsNull() {
		desc := data.Description.ValueString()
		body.Description = &desc
	}

	// Create new role
	res, err := r.client.CreateCustomRoleWithResponse(ctx, body)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error creating role",
			"Could not create role, unexpected error: "+err.Error(),
		)
		return
	}

	if res.JSON201 == nil {
		if res.HTTPResponse.StatusCode == http.StatusForbidden {
			resp.Diagnostics.AddError(
				"Forbidden",
				"You do not have permission to create this role.",
			)
			return
		}

		resp.Diagnostics.AddError(
			"Error creating role",
			fmt.Sprintf("Could not create role, unexpected status: %s", res.HTTPResponse.Status),
		)
		return
	}

	// Write logs using the tflog package
	// tflog.Trace(ctx, "created a role")

	// Save data into Terraform state
	data.ID = types.StringValue(res.JSON201.Id.String())
	data.Name = types.StringValue(res.JSON201.Name)
	if res.JSON201.Description != nil {
		data.Description = types.StringValue(*res.JSON201.Description)
	} else {
		data.Description = types.StringNull()
	}

	respPermissions := make([]types.String, len(res.JSON201.Permissions))
	for i, p := range res.JSON201.Permissions {
		respPermissions[i] = types.StringValue(p)
	}
	data.Permissions = respPermissions

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *RoleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data RoleResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	roleUUID, err := uuid.Parse(data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Invalid Role ID",
			fmt.Sprintf("Could not parse role ID: %s", err.Error()),
		)
		return
	}

	res, err := r.client.GetCustomRoleWithResponse(ctx, roleUUID)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error reading role",
			"Could not read role, unexpected error: "+err.Error(),
		)
		return
	}

	if res.JSON200 == nil {
		if res.HTTPResponse.StatusCode == http.StatusNotFound {
			resp.State.RemoveResource(ctx)
			return
		}

		resp.Diagnostics.AddError(
			"Error reading role",
			fmt.Sprintf("Could not read role, unexpected status: %s", res.HTTPResponse.Status),
		)
		return
	}

	data.Name = types.StringValue(res.JSON200.Name)
	if res.JSON200.Description != nil {
		data.Description = types.StringValue(*res.JSON200.Description)
	} else {
		data.Description = types.StringNull()
	}

	respPermissions := make([]types.String, len(res.JSON200.Permissions))
	for i, p := range res.JSON200.Permissions {
		respPermissions[i] = types.StringValue(p)
	}
	data.Permissions = respPermissions

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *RoleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data RoleResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	permissions := make([]string, len(data.Permissions))
	for i, p := range data.Permissions {
		permissions[i] = p.ValueString()
	}

	name := data.Name.ValueString()
	// Update role
	body := client.UpdateCustomRoleJSONBody{
		Name:        &name,
		Permissions: &permissions,
	}

	if !data.Description.IsNull() {
		desc := data.Description.ValueString()
		body.Description = &desc
	}

	roleUUID, err := uuid.Parse(data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Invalid Role ID",
			fmt.Sprintf("Could not parse role ID: %s", err.Error()),
		)
		return
	}

	res, err := r.client.UpdateCustomRoleWithResponse(ctx, roleUUID, body)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error updating role",
			"Could not update role, unexpected error: "+err.Error(),
		)
		return
	}

	if res.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Error updating role",
			fmt.Sprintf("Could not update role, unexpected status: %s", res.HTTPResponse.Status),
		)
		return
	}

	// Update the model with the response
	data.Name = types.StringValue(res.JSON200.Name)
	if res.JSON200.Description != nil {
		data.Description = types.StringValue(*res.JSON200.Description)
	} else {
		data.Description = types.StringNull()
	}

	respPermissions := make([]types.String, len(res.JSON200.Permissions))
	for i, p := range res.JSON200.Permissions {
		respPermissions[i] = types.StringValue(p)
	}
	data.Permissions = respPermissions

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *RoleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data RoleResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	roleUUID, err := uuid.Parse(data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Invalid Role ID",
			fmt.Sprintf("Could not parse role ID: %s", err.Error()),
		)
		return
	}

	res, err := r.client.DeleteCustomRoleWithResponse(ctx, roleUUID)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error deleting role",
			"Could not delete role, unexpected error: "+err.Error(),
		)
		return
	}

	if res.HTTPResponse.StatusCode != http.StatusOK && res.HTTPResponse.StatusCode != http.StatusNoContent {
		resp.Diagnostics.AddError(
			"Error deleting role",
			fmt.Sprintf("Could not delete role, unexpected status: %s", res.HTTPResponse.Status),
		)
		return
	}
}

func (r *RoleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
