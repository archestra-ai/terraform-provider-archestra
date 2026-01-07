package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
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
		MarkdownDescription: "Manages custom RBAC roles.",

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
			"description": schema.StringAttribute{
				MarkdownDescription: "Description of the role",
				Optional:            true,
			},
			"permissions": schema.ListAttribute{
				MarkdownDescription: "List of permissions granted by this role",
				Required:            true,
				ElementType:         types.StringType,
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

	permissionMap := buildCreatePermissionMap(data.Permissions, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	createBody := client.CreateRoleJSONRequestBody{
		Name:       data.Name.ValueString(),
		Permission: permissionMap,
	}
	if !data.Description.IsNull() {
		desc := data.Description.ValueString()
		createBody.Description = &desc
	}

	apiResp, err := r.client.CreateRoleWithResponse(ctx, createBody)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to create role: %s", err))
		return
	}

	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Unexpected status %d while creating role", apiResp.StatusCode()),
		)
		return
	}

	data.ID = types.StringValue(apiResp.JSON200.Id)
	data.Name = types.StringValue(apiResp.JSON200.Name)
	if apiResp.JSON200.Description != nil {
		data.Description = types.StringValue(*apiResp.JSON200.Description)
	} else if !data.Description.IsNull() {
		// Preserve planned description if API omits it to avoid state drift
		data.Description = data.Description
	} else {
		data.Description = types.StringNull()
	}
	data.Permissions = convertCreateRolePermissionMapToStringValues(apiResp.JSON200.Permission)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *RoleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data RoleResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Get by role ID using the Terraform provider's HTTP transport
	// Since the generated client has issues with anonymous struct parameters,
	// we fetch the role by making a direct HTTP request
	apiResp, err := getRoleByID(r.client, ctx, data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to read role: %s", err))
		return
	}
	defer apiResp.Body.Close()

	if apiResp.StatusCode == 404 {
		resp.State.RemoveResource(ctx)
		return
	}

	if apiResp.StatusCode != 200 {
		bodyBytes, _ := io.ReadAll(apiResp.Body)
		resp.Diagnostics.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Unexpected status %d while reading role %s: %s", apiResp.StatusCode, data.ID.ValueString(), string(bodyBytes)),
		)
		return
	}

	var role struct {
		Id          string                                   `json:"id"`
		Name        string                                   `json:"name"`
		Description *string                                  `json:"description,omitempty"`
		Permission  map[string][]client.GetRole200Permission `json:"permission"`
	}
	if err := json.NewDecoder(apiResp.Body).Decode(&role); err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to parse role response: %s", err))
		return
	}

	data.Name = types.StringValue(role.Name)
	if role.Description != nil {
		data.Description = types.StringValue(*role.Description)
	} else if !data.Description.IsNull() {
		// Preserve existing state description if API omits it
		data.Description = data.Description
	} else {
		data.Description = types.StringNull()
	}
	data.Permissions = convertPermissionMapToStringValues(role.Permission)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *RoleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data RoleResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// ID is computed; pull from prior state to issue update request.
	var state RoleResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	data.ID = state.ID

	permissionMap := buildUpdatePermissionMap(data.Permissions, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	updateBody := client.UpdateRoleJSONRequestBody{
		Name: func(s string) *string { return &s }(data.Name.ValueString()),
		Permission: func(m map[string][]client.UpdateRoleJSONBodyPermission) *map[string][]client.UpdateRoleJSONBodyPermission {
			return &m
		}(permissionMap),
	}
	if !data.Description.IsNull() {
		desc := data.Description.ValueString()
		updateBody.Description = &desc
	}

	// Use helper function to work around anonymous struct type issues
	bodyBytes, err := json.Marshal(updateBody)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to marshal update body: %s", err))
		return
	}

	httpResp, err := updateRoleByID(r.client, ctx, data.ID.ValueString(), string(bodyBytes))
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to update role: %s", err))
		return
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != 200 {
		resp.Diagnostics.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Unexpected status %d while updating role", httpResp.StatusCode),
		)
		return
	}

	var updatedRole struct {
		Id          string                                      `json:"id"`
		Name        string                                      `json:"name"`
		Description *string                                     `json:"description,omitempty"`
		Permission  map[string][]client.UpdateRole200Permission `json:"permission"`
	}
	if err := json.NewDecoder(httpResp.Body).Decode(&updatedRole); err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to parse update response: %s", err))
		return
	}

	data.Name = types.StringValue(updatedRole.Name)
	if updatedRole.Description != nil {
		data.Description = types.StringValue(*updatedRole.Description)
	} else {
		data.Description = types.StringNull()
	}
	data.Permissions = convertUpdateRolePermissionMapToStringValues(updatedRole.Permission)

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
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to delete role: %s", err))
		return
	}

	switch apiResp.StatusCode() {
	case 200, 204, 404:
		// Treat missing as already gone.
		resp.State.RemoveResource(ctx)
	default:
		resp.Diagnostics.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Unexpected status %d while deleting role", apiResp.StatusCode()),
		)
	}
}

func (r *RoleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func convertStringSlice(values []types.String) []string {
	result := make([]string, 0, len(values))
	for _, v := range values {
		if !v.IsNull() {
			result = append(result, v.ValueString())
		}
	}
	return result
}

func convertStringValues(values []string) []types.String {
	result := make([]types.String, 0, len(values))
	for _, v := range values {
		result = append(result, types.StringValue(v))
	}
	return result
}

func convertPermissionMapToStringValues(permissions map[string][]client.GetRole200Permission) []types.String {
	result := make([]types.String, 0)
	for resource, actions := range permissions {
		for _, action := range actions {
			result = append(result, types.StringValue(fmt.Sprintf("%s:%s", resource, string(action))))
		}
	}
	return result
}

func convertCreateRolePermissionMapToStringValues(permissions map[string][]client.CreateRole200Permission) []types.String {
	result := make([]types.String, 0)
	for resource, actions := range permissions {
		for _, action := range actions {
			result = append(result, types.StringValue(fmt.Sprintf("%s:%s", resource, string(action))))
		}
	}
	return result
}

func convertUpdateRolePermissionMapToStringValues(permissions map[string][]client.UpdateRole200Permission) []types.String {
	result := make([]types.String, 0)
	for resource, actions := range permissions {
		for _, action := range actions {
			result = append(result, types.StringValue(fmt.Sprintf("%s:%s", resource, string(action))))
		}
	}
	return result
}

var allowedRoleActions = map[string]struct{}{
	"create": {},
	"read":   {},
	"update": {},
	"delete": {},
	"admin":  {},
	"cancel": {},
}

func buildCreatePermissionMap(values []types.String, diags *diag.Diagnostics) map[string][]client.CreateRoleJSONBodyPermission {
	result := make(map[string][]client.CreateRoleJSONBodyPermission)

	for _, v := range values {
		if v.IsNull() {
			continue
		}

		s := v.ValueString()
		parts := strings.SplitN(s, ":", 2)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			diags.AddError("Invalid permission format", fmt.Sprintf("Permission %q must be in the form resource:action", s))
			continue
		}

		resource := parts[0]
		action := strings.ToLower(parts[1])
		if _, ok := allowedRoleActions[action]; !ok {
			diags.AddError("Invalid permission action", fmt.Sprintf("Action %q is not allowed; valid actions are create, read, update, delete, admin, cancel", action))
			continue
		}

		result[resource] = append(result[resource], client.CreateRoleJSONBodyPermission(action))
	}

	if len(result) == 0 {
		diags.AddError("No valid permissions", "At least one valid resource:action permission is required")
	}

	return result
}

func buildUpdatePermissionMap(values []types.String, diags *diag.Diagnostics) map[string][]client.UpdateRoleJSONBodyPermission {
	result := make(map[string][]client.UpdateRoleJSONBodyPermission)

	for _, v := range values {
		if v.IsNull() {
			continue
		}

		s := v.ValueString()
		parts := strings.SplitN(s, ":", 2)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			diags.AddError("Invalid permission format", fmt.Sprintf("Permission %q must be in the form resource:action", s))
			continue
		}

		resource := parts[0]
		action := strings.ToLower(parts[1])
		if _, ok := allowedRoleActions[action]; !ok {
			diags.AddError("Invalid permission action", fmt.Sprintf("Action %q is not allowed; valid actions are create, read, update, delete, admin, cancel", action))
			continue
		}

		result[resource] = append(result[resource], client.UpdateRoleJSONBodyPermission(action))
	}

	if len(result) == 0 {
		diags.AddError("No valid permissions", "At least one valid resource:action permission is required")
	}

	return result
}

// getRoleByID fetches a role by ID using the client's HTTP transport
func getRoleByID(c *client.ClientWithResponses, ctx context.Context, roleID string) (*http.Response, error) {
	// Get the server URL and request editors from the embedded Client
	serverURL := ""
	var requestEditors []client.RequestEditorFn
	if cl, ok := c.ClientInterface.(*client.Client); ok {
		serverURL = cl.Server
		requestEditors = cl.RequestEditors
	} else {
		return nil, fmt.Errorf("failed to get client configuration: ClientInterface is not *client.Client")
	}

	if serverURL == "" {
		return nil, fmt.Errorf("server URL is empty")
	}

	url := serverURL + "/api/roles/" + roleID
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	// Apply all request editors (including auth)
	for _, editor := range requestEditors {
		if err := editor(ctx, req); err != nil {
			return nil, fmt.Errorf("failed to apply request editor: %w", err)
		}
	}

	httpClient := &http.Client{}
	return httpClient.Do(req)
}

// updateRoleByID updates a role by ID using the client's HTTP transport
func updateRoleByID(c *client.ClientWithResponses, ctx context.Context, roleID string, body string) (*http.Response, error) {
	// Get the server URL and request editors from the embedded Client
	serverURL := ""
	var requestEditors []client.RequestEditorFn
	if cl, ok := c.ClientInterface.(*client.Client); ok {
		serverURL = cl.Server
		requestEditors = cl.RequestEditors
	} else {
		return nil, fmt.Errorf("failed to get client configuration: ClientInterface is not *client.Client")
	}

	if serverURL == "" {
		return nil, fmt.Errorf("server URL is empty")
	}

	url := serverURL + "/api/roles/" + roleID
	req, err := http.NewRequestWithContext(ctx, "PUT", url, strings.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	// Apply all request editors (including auth)
	for _, editor := range requestEditors {
		if err := editor(ctx, req); err != nil {
			return nil, fmt.Errorf("failed to apply request editor: %w", err)
		}
	}

	httpClient := &http.Client{}
	return httpClient.Do(req)
}
