package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.ResourceWithImportState = &EnvironmentDefaultResource{}

func NewEnvironmentDefaultResource() resource.Resource { return &EnvironmentDefaultResource{} }

type EnvironmentDefaultResource struct{ client *client.ClientWithResponses }
type EnvironmentDefaultResourceModel struct {
	ID            types.String `tfsdk:"id"`
	ResourceType  types.String `tfsdk:"resource_type"`
	EnvironmentID types.String `tfsdk:"environment_id"`
}

var environmentDefaultTypes = []string{"mcpRegistry", "app", "agent", "mcpGateway", "knowledgeSource"}

func (r *EnvironmentDefaultResource) AttrSpecs() []AttrSpec {
	return []AttrSpec{{TFName: "resource_type", Kind: Synthetic}, {TFName: "environment_id", Kind: Synthetic}}
}
func (r *EnvironmentDefaultResource) APIShape() any {
	return client.UpdateEnvironmentResourceDefaultsResponse{}
}

// Each wire key is selected by resource_type and represented by environment_id.
func (r *EnvironmentDefaultResource) KnownIntentionallySkipped() []string {
	return environmentDefaultTypes
}
func (r *EnvironmentDefaultResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_environment_default"
}
func (r *EnvironmentDefaultResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{MarkdownDescription: "The default deployment environment for one resource kind. Declare at most one per resource type. Omitted environment_id selects the organization's implicit Default environment; destroy resets only this resource type and preserves other defaults. Import by resource_type.", Attributes: map[string]schema.Attribute{
		"id":             schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, MarkdownDescription: "Resource type identifying the default slot."},
		"resource_type":  schema.StringAttribute{Required: true, Validators: []validator.String{stringvalidator.OneOf(environmentDefaultTypes...)}, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}, MarkdownDescription: "Resource kind: mcpRegistry, app, agent, mcpGateway, or knowledgeSource."},
		"environment_id": schema.StringAttribute{Optional: true, MarkdownDescription: "Default environment ID, or omitted for the implicit Default environment."},
	}}
}
func (r *EnvironmentDefaultResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.ClientWithResponses)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Resource Configure Type", fmt.Sprintf("Expected API client, got %T", req.ProviderData))
		return
	}
	r.client = c
}
func (r *EnvironmentDefaultResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data EnvironmentDefaultResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !r.set(ctx, &data, false, &resp.Diagnostics) {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
func (r *EnvironmentDefaultResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data EnvironmentDefaultResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if data.ID.IsNull() || data.ID.ValueString() == "" {
		resp.State.RemoveResource(ctx)
		return
	}
	result, err := r.client.ListEnvironmentsWithResponse(ctx, nil)
	if err != nil {
		resp.Diagnostics.AddError("API Error", err.Error())
		return
	}
	if result.JSON200 == nil {
		resp.Diagnostics.AddError("Read Failed", fmt.Sprintf("API returned status %d", result.StatusCode()))
		return
	}
	var body struct {
		ResourceDefaults map[string]*string `json:"resourceDefaults"`
	}
	if err := json.Unmarshal(result.Body, &body); err != nil {
		resp.Diagnostics.AddError("Invalid API Response", err.Error())
		return
	}
	r.flatten(&data, body.ResourceDefaults, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
func (r *EnvironmentDefaultResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data EnvironmentDefaultResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !r.set(ctx, &data, false, &resp.Diagnostics) {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
func (r *EnvironmentDefaultResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data EnvironmentDefaultResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.set(ctx, &data, true, &resp.Diagnostics)
}
func (r *EnvironmentDefaultResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	valid := false
	for _, kind := range environmentDefaultTypes {
		if req.ID == kind {
			valid = true
			break
		}
	}
	if !valid {
		resp.Diagnostics.AddError("Invalid Import ID", "Import by one of mcpRegistry, app, agent, mcpGateway, knowledgeSource.")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("resource_type"), req.ID)...)
}
func (r *EnvironmentDefaultResource) set(ctx context.Context, data *EnvironmentDefaultResourceModel, reset bool, diags *diag.Diagnostics) bool {
	var environment any
	if !reset && !data.EnvironmentID.IsNull() {
		environment = data.EnvironmentID.ValueString()
	}
	body, err := json.Marshal(map[string]any{data.ResourceType.ValueString(): environment})
	if err != nil {
		diags.AddError("Request Encoding Failed", err.Error())
		return false
	}
	result, err := r.client.UpdateEnvironmentResourceDefaultsWithBodyWithResponse(ctx, "application/json", bytes.NewReader(body))
	if err != nil {
		diags.AddError("API Error", err.Error())
		return false
	}
	if result.JSON200 == nil {
		diags.AddError("Default Update Failed", fmt.Sprintf("API returned status %d: %s", result.StatusCode(), result.Body))
		return false
	}
	var defaults map[string]*string
	if err := json.Unmarshal(result.Body, &defaults); err != nil {
		diags.AddError("Invalid API Response", err.Error())
		return false
	}
	r.flatten(data, defaults, diags)
	return !diags.HasError()
}
func (r *EnvironmentDefaultResource) flatten(data *EnvironmentDefaultResourceModel, defaults map[string]*string, diags *diag.Diagnostics) {
	value, exists := defaults[data.ResourceType.ValueString()]
	if !exists {
		diags.AddError("Invalid API Response", "The API response did not include the requested default slot.")
		return
	}
	data.ID = data.ResourceType
	data.EnvironmentID = types.StringPointerValue(value)
}
