package provider

import (
	"context"
	"fmt"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = &RoleResource{}
var _ resource.ResourceWithImportState = &RoleResource{}

func NewRoleResource() resource.Resource {
	return &RoleResource{}
}

type RoleResource struct {
	client *client.ClientWithResponses
}

type RoleResourceModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Permissions types.Map    `tfsdk:"permissions"`
	Predefined  types.Bool   `tfsdk:"predefined"`
}

func (r *RoleResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_role"
}

func (r *RoleResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages an Archestra role with permissions.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Role identifier. If provided, manages an existing role instead of creating a new one.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "The name of the role. Will be used to create the roleNonUUIDIdentifier.",
				Required:            true,
			},
			"permissions": schema.MapAttribute{
				MarkdownDescription: "Permissions for the role, as a map of resource types to allowed actions. Valid actions: admin, cancel, create, delete, read, update",
				Required:            true,
				ElementType: types.ListType{
					ElemType: types.StringType,
				},
			},
			"predefined": schema.BoolAttribute{
				MarkdownDescription: "Whether this is a predefined (built-in) role",
				Computed:            true,
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *RoleResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	parsedPermissions := make(map[string][]client.CreateRoleJSONBodyPermission)

	elements := data.Permissions.Elements()
	for key, val := range elements {
		listVal, ok := val.(types.List)
		if !ok {
			resp.Diagnostics.AddError("Type Error", fmt.Sprintf("Expected list for permission key %s", key))
			continue
		}

		var actions []client.CreateRoleJSONBodyPermission
		for _, elem := range listVal.Elements() {
			strVal, ok := elem.(types.String)
			if !ok {
				resp.Diagnostics.AddError("Type Error", fmt.Sprintf("Expected string in permission list for key %s", key))
				continue
			}
			actions = append(actions, client.CreateRoleJSONBodyPermission(strVal.ValueString()))
		}
		parsedPermissions[key] = actions
	}

	if resp.Diagnostics.HasError() {
		return
	}

	requestBody := client.CreateRoleJSONRequestBody{
		Name:       data.Name.ValueString(),
		Permission: parsedPermissions,
	}

	apiResp, err := r.client.CreateRoleWithResponse(ctx, requestBody)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to create role, got error: %s", err))
		return
	}

	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Expected 200 OK, got status %d", apiResp.StatusCode()),
		)
		return
	}

	data.ID = types.StringValue(apiResp.JSON200.Id)
	data.Name = types.StringValue(apiResp.JSON200.Name)
	data.Predefined = types.BoolValue(apiResp.JSON200.Predefined)

	if apiResp.JSON200.Permission == nil {
		data.Permissions = types.MapNull(types.ListType{ElemType: types.StringType})
		return
	} else {
		permsMap := make(map[string]attr.Value)

		for key, actions := range apiResp.JSON200.Permission {
			actionElements := make([]attr.Value, len(actions))
			for i, action := range actions {
				actionElements[i] = types.StringValue(string(action))
			}

			listVal, diags := types.ListValue(types.StringType, actionElements)
			resp.Diagnostics.Append(diags...)
			if resp.Diagnostics.HasError() {
				return
			}

			permsMap[key] = listVal
		}

		permissionsMap, diags := types.MapValue(types.ListType{ElemType: types.StringType}, permsMap)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}

		data.Permissions = permissionsMap
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *RoleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data RoleResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := r.client.GetRoleWithResponse(ctx, data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to read role, got error: %s", err))
		return
	}

	if apiResp.JSON404 != nil {
		resp.State.RemoveResource(ctx)
		return
	}

	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Expected 200 OK, got status %d", apiResp.StatusCode()),
		)
		return
	} else {
		permsMap := make(map[string]attr.Value)

		for key, actions := range apiResp.JSON200.Permission {
			actionElements := make([]attr.Value, len(actions))
			for i, action := range actions {
				actionElements[i] = types.StringValue(string(action))
			}

			listVal, diags := types.ListValue(types.StringType, actionElements)
			resp.Diagnostics.Append(diags...)
			if resp.Diagnostics.HasError() {
				return
			}

			permsMap[key] = listVal
		}

		permissionsMap, diags := types.MapValue(types.ListType{ElemType: types.StringType}, permsMap)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}

		data.Permissions = permissionsMap
	}

	data.Name = types.StringValue(apiResp.JSON200.Name)
	data.Predefined = types.BoolValue(apiResp.JSON200.Predefined)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *RoleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data RoleResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	parsedPermissions := make(map[string][]client.UpdateRoleJSONBodyPermission)

	elements := data.Permissions.Elements()
	for key, val := range elements {
		listVal, ok := val.(types.List)
		if !ok {
			resp.Diagnostics.AddError("Type Error", fmt.Sprintf("Expected list for permission key %s", key))
			continue
		}

		var actions []client.UpdateRoleJSONBodyPermission
		for _, elem := range listVal.Elements() {
			strVal, ok := elem.(types.String)
			if !ok {
				resp.Diagnostics.AddError("Type Error", fmt.Sprintf("Expected string in permission list for key %s", key))
				continue
			}
			actions = append(actions, client.UpdateRoleJSONBodyPermission(strVal.ValueString()))
		}
		parsedPermissions[key] = actions
	}

	if resp.Diagnostics.HasError() {
		return
	}

	name := data.Name.ValueString()
	requestBody := client.UpdateRoleJSONRequestBody{
		Name:       &name,
		Permission: &parsedPermissions,
	}

	apiResp, err := r.client.UpdateRoleWithResponse(ctx, data.ID.ValueString(), requestBody)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to update role, got error: %s", err))
		return
	}

	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Expected 200 OK, got status %d", apiResp.StatusCode()),
		)
		return
	} else {
		permsMap := make(map[string]attr.Value)

		for key, actions := range apiResp.JSON200.Permission {
			actionElements := make([]attr.Value, len(actions))
			for i, action := range actions {
				actionElements[i] = types.StringValue(string(action))
			}

			listVal, diags := types.ListValue(types.StringType, actionElements)
			resp.Diagnostics.Append(diags...)
			if resp.Diagnostics.HasError() {
				return
			}

			permsMap[key] = listVal
		}

		permissionsMap, diags := types.MapValue(types.ListType{ElemType: types.StringType}, permsMap)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}

		data.Permissions = permissionsMap
	}

	data.Name = types.StringValue(apiResp.JSON200.Name)
	data.Predefined = types.BoolValue(apiResp.JSON200.Predefined)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *RoleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data RoleResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := r.client.DeleteRoleWithResponse(ctx, data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to delete role, got error: %s", err))
		return
	}

	if apiResp.JSON200 == nil && apiResp.JSON404 == nil {
		resp.Diagnostics.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Expected 200 OK or 404 Not Found, got status %d", apiResp.StatusCode()),
		)
		return
	}
}

func (r *RoleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
