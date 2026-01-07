package provider

import (
	"context"
	"fmt"
	"sort"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
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
	ID             types.String `tfsdk:"id"`
	Name           types.String `tfsdk:"name"`
	Permissions    types.Map    `tfsdk:"permissions"`
	Predefined     types.Bool   `tfsdk:"predefined"`
	OrganizationID types.String `tfsdk:"organization_id"`
}

func (r *RoleResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_role"
}

func (r *RoleResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a custom RBAC role in Archestra.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Role identifier",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "The name of the role",
				Required:            true,
			},
			"permissions": schema.MapAttribute{
				MarkdownDescription: "Map of resource names to list of allowed actions (e.g., {\"agents\": [\"read\", \"create\"]}). Valid actions: admin, cancel, create, delete, read, update",
				Required:            true,
				ElementType:         types.ListType{ElemType: types.StringType},
			},
			"predefined": schema.BoolAttribute{
				MarkdownDescription: "Whether this is a system-defined role (read-only)",
				Computed:            true,
			},
			"organization_id": schema.StringAttribute{
				MarkdownDescription: "The organization ID this role belongs to",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *RoleResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	apiClient, ok := req.ProviderData.(*client.ClientWithResponses)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *client.ClientWithResponses, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	r.client = apiClient
}

func (r *RoleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data RoleResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Convert permissions map to API format
	permissionsMap, diagsConv := r.permissionsToAPIFormat(ctx, data.Permissions)
	resp.Diagnostics.Append(diagsConv...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Create request body
	requestBody := client.CreateRoleJSONRequestBody{
		Name:       data.Name.ValueString(),
		Permission: permissionsMap,
	}

	// Call API
	apiResp, err := r.client.CreateRoleWithResponse(ctx, requestBody)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to create role, got error: %s", err))
		return
	}

	// Check response
	if apiResp.JSON200 == nil {
		errMsg := fmt.Sprintf("Expected 200 OK, got status %d", apiResp.StatusCode())
		if apiResp.JSON400 != nil {
			errMsg = fmt.Sprintf("Bad request: %s", apiResp.JSON400.Error.Message)
		} else if apiResp.JSON409 != nil {
			errMsg = fmt.Sprintf("Conflict: %s", apiResp.JSON409.Error.Message)
		}
		resp.Diagnostics.AddError("Unexpected API Response", errMsg)
		return
	}

	// Map response to Terraform state
	permissions := make(map[string][]string)
	for k, v := range apiResp.JSON200.Permission {
		strSlice := make([]string, len(v))
		for i, perm := range v {
			strSlice[i] = string(perm)
		}
		permissions[k] = strSlice
	}

	r.updateStateFromResponse(&data, apiResp.JSON200.Id, apiResp.JSON200.Name,
		permissions, apiResp.JSON200.Predefined, apiResp.JSON200.OrganizationId, &resp.Diagnostics)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *RoleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data RoleResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Get role by listing all and finding by ID
	// This is a workaround for the union type issue in the generated client
	rolesResp, err := r.client.GetRolesWithResponse(ctx)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to list roles, got error: %s", err))
		return
	}

	if rolesResp.JSON200 == nil {
		resp.Diagnostics.AddError("Unexpected API Response", fmt.Sprintf("Expected 200 OK, got status %d", rolesResp.StatusCode()))
		return
	}

	// Find role by ID
	var found bool
	for _, role := range *rolesResp.JSON200 {
		if role.Id == data.ID.ValueString() {
			permissions := make(map[string][]string)
			for k, v := range role.Permission {
				strSlice := make([]string, len(v))
				for i, perm := range v {
					strSlice[i] = string(perm)
				}
				permissions[k] = strSlice
			}

			r.updateStateFromResponse(&data, role.Id, role.Name,
				permissions, role.Predefined, role.OrganizationId, &resp.Diagnostics)
			found = true
			break
		}
	}

	if !found {
		resp.State.RemoveResource(ctx)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *RoleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data RoleResourceModel
	var state RoleResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Due to the union type issue in the generated client, we implement update
	// via delete + create. This maintains the same ID if the API supports it,
	// but may result in a new ID.

	// First, delete the old role
	deleteResp, err := r.client.DeleteRoleWithResponse(ctx, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to delete old role during update, got error: %s", err))
		return
	}

	if deleteResp.JSON200 == nil && deleteResp.JSON404 == nil {
		resp.Diagnostics.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Unable to delete old role during update, got status %d", deleteResp.StatusCode()),
		)
		return
	}

	// Now create the new role
	permissionsMap, diagsConv := r.permissionsToAPIFormat(ctx, data.Permissions)
	resp.Diagnostics.Append(diagsConv...)
	if resp.Diagnostics.HasError() {
		return
	}

	requestBody := client.CreateRoleJSONRequestBody{
		Name:       data.Name.ValueString(),
		Permission: permissionsMap,
	}

	apiResp, err := r.client.CreateRoleWithResponse(ctx, requestBody)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to create role during update, got error: %s", err))
		return
	}

	if apiResp.JSON200 == nil {
		errMsg := fmt.Sprintf("Expected 200 OK during create, got status %d", apiResp.StatusCode())
		if apiResp.JSON400 != nil {
			errMsg = fmt.Sprintf("Bad request: %s", apiResp.JSON400.Error.Message)
		} else if apiResp.JSON409 != nil {
			errMsg = fmt.Sprintf("Conflict: %s", apiResp.JSON409.Error.Message)
		}
		resp.Diagnostics.AddError("Unexpected API Response", errMsg)
		return
	}

	// Map response to Terraform state
	permissions := make(map[string][]string)
	for k, v := range apiResp.JSON200.Permission {
		strSlice := make([]string, len(v))
		for i, perm := range v {
			strSlice[i] = string(perm)
		}
		permissions[k] = strSlice
	}

	r.updateStateFromResponse(&data, apiResp.JSON200.Id, apiResp.JSON200.Name,
		permissions, apiResp.JSON200.Predefined, apiResp.JSON200.OrganizationId, &resp.Diagnostics)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *RoleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data RoleResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Call API
	apiResp, err := r.client.DeleteRoleWithResponse(ctx, data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to delete role, got error: %s", err))
		return
	}

	// Check response (200 or 404 are both acceptable for delete)
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

// Helper function to convert Terraform permissions map to API format for Create
func (r *RoleResource) permissionsToAPIFormat(ctx context.Context, permissions types.Map) (map[string][]client.CreateRoleJSONBodyPermission, diag.Diagnostics) {
	result := make(map[string][]client.CreateRoleJSONBodyPermission)
	var diagsResult diag.Diagnostics

	if permissions.IsNull() || permissions.IsUnknown() {
		return result, diagsResult
	}

	elements := permissions.Elements()
	for key, value := range elements {
		listVal, ok := value.(types.List)
		if !ok {
			diagsResult.AddError("Invalid Permission Format", fmt.Sprintf("Expected list for key %s", key))
			continue
		}

		var actions []string
		diagsResult.Append(listVal.ElementsAs(ctx, &actions, false)...)

		apiActions := make([]client.CreateRoleJSONBodyPermission, len(actions))
		for i, action := range actions {
			apiActions[i] = client.CreateRoleJSONBodyPermission(action)
		}
		result[key] = apiActions
	}

	return result, diagsResult
}

// Helper to update state from API response
func (r *RoleResource) updateStateFromResponse(data *RoleResourceModel, id, name string, permissions map[string][]string, predefined bool, orgId *string, diagsResult *diag.Diagnostics) {
	data.ID = types.StringValue(id)
	data.Name = types.StringValue(name)
	data.Predefined = types.BoolValue(predefined)

	if orgId != nil {
		data.OrganizationID = types.StringValue(*orgId)
	} else {
		data.OrganizationID = types.StringNull()
	}

	// Build the map value
	tfPermMap := make(map[string]attr.Value)
	for k, v := range permissions {
		// Sort to ensure consistent ordering
		sorted := make([]string, len(v))
		copy(sorted, v)
		sort.Strings(sorted)

		listVals := make([]attr.Value, len(sorted))
		for i, s := range sorted {
			listVals[i] = types.StringValue(s)
		}
		listVal, listDiags := types.ListValue(types.StringType, listVals)
		diagsResult.Append(listDiags...)
		tfPermMap[k] = listVal
	}

	mapVal, mapDiags := types.MapValue(types.ListType{ElemType: types.StringType}, tfPermMap)
	diagsResult.Append(mapDiags...)
	data.Permissions = mapVal
}
