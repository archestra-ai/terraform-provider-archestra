package provider

import (
	"context"
	"encoding/json"
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
	ID             types.String `tfsdk:"id"`
	Name           types.String `tfsdk:"name"`
	Role           types.String `tfsdk:"role"`
	OrganizationID types.String `tfsdk:"organization_id"`
	Predefined     types.Bool   `tfsdk:"predefined"`
	Permission     types.Map    `tfsdk:"permission"`
}

func (r *RoleResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_role"
}

func (r *RoleResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages an Archestra custom role with permissions.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Role identifier (base62)",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "The display name of the role (1-50 characters)",
				Required:            true,
			},
			"role": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The immutable role identifier (auto-generated from name)",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"organization_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The organization ID this role belongs to",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"predefined": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "Whether the role is a predefined (immutable) role",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"permission": schema.MapAttribute{
				MarkdownDescription: "Map of resource names to allowed actions. Resources: profile, tool, policy, interaction, dualLlmConfig, dualLlmResult, organization, ssoProvider, member, invitation, internalMcpCatalog, mcpServer, mcpServerInstallationRequest, mcpToolCall, team, conversation, limit, tokenPrice, chatSettings, prompt, ac. Actions: create, read, update, delete, admin, cancel.",
				Required:            true,
				ElementType: types.ListType{
					ElemType: types.StringType,
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

// makeRoleIdParam creates the union type struct needed for GetRole/UpdateRole API calls
func makeRoleIdParam(id string) struct{ union json.RawMessage } {
	rawBytes, _ := json.Marshal(id)
	return struct{ union json.RawMessage }{union: rawBytes}
}

// convertPermissionToAPI converts Terraform map to API permission format
func convertPermissionToAPI(ctx context.Context, permission types.Map) (map[string][]client.CreateRoleJSONBodyPermission, error) {
	result := make(map[string][]client.CreateRoleJSONBodyPermission)

	if permission.IsNull() || permission.IsUnknown() {
		return result, nil
	}

	elements := permission.Elements()
	for resource, actionsVal := range elements {
		actionsList, ok := actionsVal.(types.List)
		if !ok {
			return nil, fmt.Errorf("expected list for resource %s", resource)
		}

		var actions []client.CreateRoleJSONBodyPermission
		for _, actionVal := range actionsList.Elements() {
			actionStr, ok := actionVal.(types.String)
			if !ok {
				return nil, fmt.Errorf("expected string action for resource %s", resource)
			}
			actions = append(actions, client.CreateRoleJSONBodyPermission(actionStr.ValueString()))
		}
		result[resource] = actions
	}

	return result, nil
}

// convertPermissionToUpdateAPI converts Terraform map to API update permission format
func convertPermissionToUpdateAPI(ctx context.Context, permission types.Map) (map[string][]client.UpdateRoleJSONBodyPermission, error) {
	result := make(map[string][]client.UpdateRoleJSONBodyPermission)

	if permission.IsNull() || permission.IsUnknown() {
		return result, nil
	}

	elements := permission.Elements()
	for resource, actionsVal := range elements {
		actionsList, ok := actionsVal.(types.List)
		if !ok {
			return nil, fmt.Errorf("expected list for resource %s", resource)
		}

		var actions []client.UpdateRoleJSONBodyPermission
		for _, actionVal := range actionsList.Elements() {
			actionStr, ok := actionVal.(types.String)
			if !ok {
				return nil, fmt.Errorf("expected string action for resource %s", resource)
			}
			actions = append(actions, client.UpdateRoleJSONBodyPermission(actionStr.ValueString()))
		}
		result[resource] = actions
	}

	return result, nil
}

// convertAPIPermissionToTerraform converts API permission response to Terraform map
func convertAPIPermissionToTerraform[T ~string](permission map[string][]T) types.Map {
	elements := make(map[string]attr.Value)
	for resource, actions := range permission {
		actionValues := make([]attr.Value, len(actions))
		for i, action := range actions {
			actionValues[i] = types.StringValue(string(action))
		}
		elements[resource] = types.ListValueMust(types.StringType, actionValues)
	}

	return types.MapValueMust(types.ListType{ElemType: types.StringType}, elements)
}

func (r *RoleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data RoleResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Convert permission map
	permission, err := convertPermissionToAPI(ctx, data.Permission)
	if err != nil {
		resp.Diagnostics.AddError("Invalid Permission", fmt.Sprintf("Unable to parse permission: %s", err))
		return
	}

	// Create request body
	requestBody := client.CreateRoleJSONRequestBody{
		Name:       data.Name.ValueString(),
		Permission: permission,
	}

	// Call API
	apiResp, err := r.client.CreateRoleWithResponse(ctx, requestBody)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to create role, got error: %s", err))
		return
	}

	// Check response
	if apiResp.JSON200 == nil {
		errMsg := "Unknown error"
		if apiResp.JSON400 != nil {
			errMsg = apiResp.JSON400.Error.Message
		} else if apiResp.JSON403 != nil {
			errMsg = apiResp.JSON403.Error.Message
		}
		resp.Diagnostics.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Expected 200 OK, got status %d: %s", apiResp.StatusCode(), errMsg),
		)
		return
	}

	// Map response to Terraform state
	data.ID = types.StringValue(apiResp.JSON200.Id)
	data.Name = types.StringValue(apiResp.JSON200.Name)
	data.Role = types.StringValue(apiResp.JSON200.Role)
	data.Predefined = types.BoolValue(apiResp.JSON200.Predefined)
	if apiResp.JSON200.OrganizationId != nil {
		data.OrganizationID = types.StringValue(*apiResp.JSON200.OrganizationId)
	} else {
		data.OrganizationID = types.StringNull()
	}
	data.Permission = convertAPIPermissionToTerraform(apiResp.JSON200.Permission)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *RoleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data RoleResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Call API with union type parameter
	apiResp, err := r.client.GetRoleWithResponse(ctx, makeRoleIdParam(data.ID.ValueString()))
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to read role, got error: %s", err))
		return
	}

	// Handle not found
	if apiResp.JSON404 != nil {
		resp.State.RemoveResource(ctx)
		return
	}

	// Check response
	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Expected 200 OK, got status %d", apiResp.StatusCode()),
		)
		return
	}

	// Map response to Terraform state
	data.Name = types.StringValue(apiResp.JSON200.Name)
	data.Role = types.StringValue(apiResp.JSON200.Role)
	data.Predefined = types.BoolValue(apiResp.JSON200.Predefined)
	if apiResp.JSON200.OrganizationId != nil {
		data.OrganizationID = types.StringValue(*apiResp.JSON200.OrganizationId)
	} else {
		data.OrganizationID = types.StringNull()
	}
	data.Permission = convertAPIPermissionToTerraform(apiResp.JSON200.Permission)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *RoleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data RoleResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Convert permission map
	permission, err := convertPermissionToUpdateAPI(ctx, data.Permission)
	if err != nil {
		resp.Diagnostics.AddError("Invalid Permission", fmt.Sprintf("Unable to parse permission: %s", err))
		return
	}

	// Create request body
	name := data.Name.ValueString()
	requestBody := client.UpdateRoleJSONRequestBody{
		Name:       &name,
		Permission: &permission,
	}

	// Call API with union type parameter
	apiResp, err := r.client.UpdateRoleWithResponse(ctx, makeRoleIdParam(data.ID.ValueString()), requestBody)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to update role, got error: %s", err))
		return
	}

	// Check response
	if apiResp.JSON200 == nil {
		errMsg := "Unknown error"
		if apiResp.JSON400 != nil {
			errMsg = apiResp.JSON400.Error.Message
		} else if apiResp.JSON403 != nil {
			errMsg = apiResp.JSON403.Error.Message
		} else if apiResp.JSON404 != nil {
			errMsg = apiResp.JSON404.Error.Message
		}
		resp.Diagnostics.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Expected 200 OK, got status %d: %s", apiResp.StatusCode(), errMsg),
		)
		return
	}

	// Map response to Terraform state
	data.Name = types.StringValue(apiResp.JSON200.Name)
	data.Role = types.StringValue(apiResp.JSON200.Role)
	data.Predefined = types.BoolValue(apiResp.JSON200.Predefined)
	if apiResp.JSON200.OrganizationId != nil {
		data.OrganizationID = types.StringValue(*apiResp.JSON200.OrganizationId)
	} else {
		data.OrganizationID = types.StringNull()
	}
	data.Permission = convertAPIPermissionToTerraform(apiResp.JSON200.Permission)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *RoleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data RoleResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Call API (DeleteRole uses simple string, not union type)
	apiResp, err := r.client.DeleteRoleWithResponse(ctx, data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to delete role, got error: %s", err))
		return
	}

	// Check response (200 or 404 are both acceptable for delete)
	if apiResp.JSON200 == nil && apiResp.JSON404 == nil {
		errMsg := "Unknown error"
		if apiResp.JSON400 != nil {
			errMsg = apiResp.JSON400.Error.Message
		} else if apiResp.JSON403 != nil {
			errMsg = apiResp.JSON403.Error.Message
		}
		resp.Diagnostics.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Expected 200 OK or 404 Not Found, got status %d: %s", apiResp.StatusCode(), errMsg),
		)
		return
	}
}

func (r *RoleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
