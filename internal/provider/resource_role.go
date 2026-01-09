package provider

import (
	"context"
	"fmt"
	"net/http"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/attr"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &RoleResource{}
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
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Permission  types.Set    `tfsdk:"permission"` // Set of PermissionModel
	Predefined  types.Bool   `tfsdk:"predefined"`
	CreatedAt   types.String `tfsdk:"created_at"`
	UpdatedAt   types.String `tfsdk:"updated_at"`
}

type PermissionModel struct {
	Resource types.String `tfsdk:"resource"`
	Actions  types.Set    `tfsdk:"actions"`
}

func (r *RoleResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_role"
}

func (r *RoleResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages an Archestra role.",
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
			"permission": schema.SetNestedAttribute{
				MarkdownDescription: "The permissions associated with the role",
				Required:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"resource": schema.StringAttribute{
							MarkdownDescription: "The resource to grant permissions on",
							Required:            true,
						},
						"actions": schema.SetAttribute{
							MarkdownDescription: "The actions allowed on the resource",
							Required:            true,
							ElementType:         types.StringType,
						},
					},
				},
			},
			"predefined": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "Whether the role is predefined (immutable)",
			},
			"created_at": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Role creation timestamp",
			},
			"updated_at": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Role update timestamp",
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

	permissions, diags := mapPermissionsFromState(ctx, data.Permission)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	requestBodyPermissions := make(map[string][]client.CreateRoleJSONBodyPermission)
    for k, v := range permissions {
        var actions []client.CreateRoleJSONBodyPermission
        for _, a := range v {
            actions = append(actions, client.CreateRoleJSONBodyPermission(a))
        }
        requestBodyPermissions[k] = actions
    }

	requestBody := client.CreateRoleJSONRequestBody{
		Name:       data.Name.ValueString(),
		Permission: requestBodyPermissions,
	}

	apiResp, err := r.client.CreateRoleWithResponse(ctx, requestBody)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error creating role",
			"Could not create role, unexpected error: "+err.Error(),
		)
		return
	}

	if apiResp.StatusCode() != http.StatusOK {
		resp.Diagnostics.AddError(
			"Error creating role",
			fmt.Sprintf("Could not create role, API returned status: %d, body: %s", apiResp.StatusCode(), string(apiResp.Body)),
		)
		return
	}

	// Map response to state
	result := apiResp.JSON200
	if result == nil {
		resp.Diagnostics.AddError("Error creating role", "API returned empty response body")
		return
	}

	data.ID = types.StringValue(result.Id)
	data.Name = types.StringValue(result.Name)
	data.Predefined = types.BoolValue(result.Predefined)
	if !result.CreatedAt.IsZero() {
		data.CreatedAt = types.StringValue(result.CreatedAt.String())
	}
	if !result.UpdatedAt.IsZero() {
		data.UpdatedAt = types.StringValue(result.UpdatedAt.String())
	}
    
	// Convert API permissions to map[string][]string
    statePermissions := make(map[string][]string)
    for k, v := range result.Permission {
        var actions []string
        for _, a := range v {
            actions = append(actions, string(a))
        }
        statePermissions[k] = actions
    }
    
    permissionSet, diags := mapPermissionsToState(ctx, statePermissions)
    resp.Diagnostics.Append(diags...)
    if resp.Diagnostics.HasError() {
        return
    }
    data.Permission = permissionSet

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
		resp.Diagnostics.AddError(
			"Error reading role",
			"Could not read role, unexpected error: "+err.Error(),
		)
		return
	}

	if apiResp.StatusCode() == http.StatusNotFound {
		resp.State.RemoveResource(ctx)
		return
	}

	if apiResp.StatusCode() != http.StatusOK {
		resp.Diagnostics.AddError(
			"Error reading role",
			fmt.Sprintf("Could not read role, API returned status: %d", apiResp.StatusCode()),
		)
		return
	}
    
	result := apiResp.JSON200
    data.Name = types.StringValue(result.Name)
	data.Predefined = types.BoolValue(result.Predefined)
	if !result.CreatedAt.IsZero() {
		data.CreatedAt = types.StringValue(result.CreatedAt.String())
	}
	if !result.UpdatedAt.IsZero() {
		data.UpdatedAt = types.StringValue(result.UpdatedAt.String())
	}
    
    // Convert API permissions to map[string][]string
    statePermissions := make(map[string][]string)
    for k, v := range result.Permission {
        var actions []string
        for _, a := range v {
            actions = append(actions, string(a))
        }
        statePermissions[k] = actions
    }
    
    permissionSet, diags := mapPermissionsToState(ctx, statePermissions)
    resp.Diagnostics.Append(diags...)
    if resp.Diagnostics.HasError() {
        return
    }
    data.Permission = permissionSet

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *RoleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data RoleResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

    permissions, diags := mapPermissionsFromState(ctx, data.Permission)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	requestBodyPermissions := make(map[string][]client.UpdateRoleJSONBodyPermission)
    for k, v := range permissions {
        var actions []client.UpdateRoleJSONBodyPermission
        for _, a := range v {
            actions = append(actions, client.UpdateRoleJSONBodyPermission(a))
        }
        requestBodyPermissions[k] = actions
    }

    name := data.Name.ValueString()
	requestBody := client.UpdateRoleJSONRequestBody{
		Name:       &name,
		Permission: &requestBodyPermissions,
	}

	apiResp, err := r.client.UpdateRoleWithResponse(ctx, data.ID.ValueString(), requestBody)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error updating role",
			"Could not update role, unexpected error: "+err.Error(),
		)
		return
	}

	if apiResp.StatusCode() != http.StatusOK {
		resp.Diagnostics.AddError(
			"Error updating role",
			fmt.Sprintf("Could not update role, API returned status: %d", apiResp.StatusCode()),
		)
		return
	}
    
    result := apiResp.JSON200
    data.Name = types.StringValue(result.Name)
	data.Predefined = types.BoolValue(result.Predefined)
    // Use result.Permission directly but cast actions to string
    apiStatePermissions := make(map[string][]string)
    for k, v := range result.Permission {
        apiStatePermissions[k] = make([]string, len(v))
        for i, p := range v {
            apiStatePermissions[k][i] = string(p)
        }
    }

    permissionSet, diags := mapPermissionsToState(ctx, apiStatePermissions)
    resp.Diagnostics.Append(diags...)
    if resp.Diagnostics.HasError() {
        return
    }
    data.Permission = permissionSet

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *RoleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data RoleResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// DeleteRole takes a string roleId
	apiResp, err := r.client.DeleteRoleWithResponse(ctx, data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error deleting role",
			"Could not delete role, unexpected error: "+err.Error(),
		)
		return
	}

	if apiResp.StatusCode() != http.StatusOK {
		resp.Diagnostics.AddError(
			"Error deleting role",
			fmt.Sprintf("Could not delete role, API returned status: %d", apiResp.StatusCode()),
		)
		return
	}
}

func (r *RoleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// Helpers

func mapPermissionsFromState(ctx context.Context, set types.Set) (map[string][]string, diag.Diagnostics) {
    if set.IsNull() || set.IsUnknown() {
        return nil, nil
    }
    
    var models []PermissionModel
    diags := set.ElementsAs(ctx, &models, false)
    if diags.HasError() {
        return nil, diags
    }
    
    result := make(map[string][]string)
    for _, m := range models {
        var actions []string
        d := m.Actions.ElementsAs(ctx, &actions, false)
        if d.HasError() {
            diags.Append(d...)
            continue
        }
        result[m.Resource.ValueString()] = actions
    }
    return result, diags
}

func mapPermissionsToState(ctx context.Context, permissions map[string][]string) (types.Set, diag.Diagnostics) {
    // Actually the generated client uses different types for Create/Update/Read responses,
    // but the underlying type is usually string.
    // The previous implementation used map[string][]string which matched the old client.
    // Now Read returns map[string][]GetRole200Permission
    // And GetRole200Permission is string.
    // So we need to update the parameter type to map[string][]client.GetRole200Permission (or implicit cast if possible).
    // EXCEPT Go doesn't implicit cast nested maps.
    // So caller (Read/Create) needs to convert first, OR we change this function to take explicit type.
    // Let's keep it take map[string][]string and have caller convert.
    // Checked previous update: In Create, I call mapPermissionsToState(ctx, result.Permission).
    // result.Permission is map[string][]CreateRole200Permission (or similar).
    // So simple call fails.
    // I should rewrite this function to take interface{} and reflection? No.
    // Just rewrite it to accept the specific type returned by Read/Create.
    // Wait, Create returns CreateRole200Permission, Read returns GetRole200Permission.
    // They are distinct types.
    // So I need a generic mapper or convert at call site.
    // I will convert at call site to map[string][]string.
    
    var diags diag.Diagnostics
    if permissions == nil {
        return types.SetNull(types.ObjectType{
			AttrTypes: map[string]attr.Type{
				"resource": types.StringType,
				"actions":  types.SetType{ElemType: types.StringType},
			},
		}), nil
    }
    
    var models []PermissionModel
    for res, acts := range permissions {
        actionsSet, d := types.SetValueFrom(ctx, types.StringType, acts)
        if d.HasError() {
            diags.Append(d...)
            continue
        }
        models = append(models, PermissionModel{
            Resource: types.StringValue(res),
            Actions:  actionsSet,
        })
    }
    
    set, d := types.SetValueFrom(ctx, types.ObjectType{
		AttrTypes: map[string]attr.Type{
			"resource": types.StringType,
			"actions":  types.SetType{ElemType: types.StringType},
		},
	}, models)
    diags.Append(d...)
    
    return set, diags
}

