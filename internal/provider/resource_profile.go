package provider

import (
	"context"
	"fmt"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &ProfileResource{}
var _ resource.ResourceWithImportState = &ProfileResource{}

func NewProfileResource() resource.Resource {
	return &ProfileResource{}
}

// ProfileResource defines the resource implementation.
type ProfileResource struct {
	client *client.ClientWithResponses
}

// ProfileLabelModel describes a label data model.
type ProfileLabelModel struct {
	Key   types.String `tfsdk:"key"`
	Value types.String `tfsdk:"value"`
}

// ProfileResourceModel describes the resource data model.
type ProfileResourceModel struct {
	ID     types.String        `tfsdk:"id"`
	Name   types.String        `tfsdk:"name"`
	Labels []ProfileLabelModel `tfsdk:"labels"`
}

func (r *ProfileResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_profile"
}

func (r *ProfileResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages an Archestra profile (formerly agent).",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Profile identifier",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "The name of the profile",
				Required:            true,
			},
			"labels": schema.ListNestedAttribute{
				MarkdownDescription: "Labels to organize and identify the profile",
				Optional:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"key": schema.StringAttribute{
							MarkdownDescription: "Label key",
							Required:            true,
						},
						"value": schema.StringAttribute{
							MarkdownDescription: "Label value",
							Required:            true,
						},
					},
				},
			},
		},
	}
}

func (r *ProfileResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *ProfileResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data ProfileResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Convert labels to API format
	var labels []struct {
		Key     string              `json:"key"`
		KeyId   *openapi_types.UUID `json:"keyId,omitempty"`
		Value   string              `json:"value"`
		ValueId *openapi_types.UUID `json:"valueId,omitempty"`
	}

	for _, label := range data.Labels {
		labels = append(labels, struct {
			Key     string              `json:"key"`
			KeyId   *openapi_types.UUID `json:"keyId,omitempty"`
			Value   string              `json:"value"`
			ValueId *openapi_types.UUID `json:"valueId,omitempty"`
		}{
			Key:   label.Key.ValueString(),
			Value: label.Value.ValueString(),
		})
	}

	// Create request body using generated type
	requestBody := client.CreateAgentJSONRequestBody{
		Name:   data.Name.ValueString(),
		Teams:  []string{}, // Empty teams array (required by API)
		Labels: &labels,
	}

	// Call API
	// Note: We are still using the "Agent" API endpoints as the underlying service likely hasn't renamed them yet.
	apiResp, err := r.client.CreateAgentWithResponse(ctx, requestBody)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to create profile, got error: %s", err))
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
	data.ID = types.StringValue(apiResp.JSON200.Id.String())
	data.Name = types.StringValue(apiResp.JSON200.Name)

	// Map labels from API response, preserving configuration order
	data.Labels = r.mapLabelsToConfigurationOrder(data.Labels, apiResp.JSON200.Labels)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ProfileResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data ProfileResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Parse UUID from state
	profileID, err := uuid.Parse(data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid ID", fmt.Sprintf("Unable to parse profile ID: %s", err))
		return
	}

	// Call API
	apiResp, err := r.client.GetAgentWithResponse(ctx, profileID)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to read profile, got error: %s", err))
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

	// Map labels from API response, preserving existing state order
	data.Labels = r.mapLabelsToConfigurationOrder(data.Labels, apiResp.JSON200.Labels)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ProfileResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data ProfileResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Parse UUID from state
	profileID, err := uuid.Parse(data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid ID", fmt.Sprintf("Unable to parse profile ID: %s", err))
		return
	}

	// Convert labels to API format
	var labels []struct {
		Key     string              `json:"key"`
		KeyId   *openapi_types.UUID `json:"keyId,omitempty"`
		Value   string              `json:"value"`
		ValueId *openapi_types.UUID `json:"valueId,omitempty"`
	}

	for _, label := range data.Labels {
		labels = append(labels, struct {
			Key     string              `json:"key"`
			KeyId   *openapi_types.UUID `json:"keyId,omitempty"`
			Value   string              `json:"value"`
			ValueId *openapi_types.UUID `json:"valueId,omitempty"`
		}{
			Key:   label.Key.ValueString(),
			Value: label.Value.ValueString(),
		})
	}

	// Create request body using generated type
	name := data.Name.ValueString()
	requestBody := client.UpdateAgentJSONRequestBody{
		Name:   &name,
		Labels: &labels,
	}

	// Call API
	apiResp, err := r.client.UpdateAgentWithResponse(ctx, profileID, requestBody)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to update profile, got error: %s", err))
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

	// Map labels from API response, preserving configuration order
	data.Labels = r.mapLabelsToConfigurationOrder(data.Labels, apiResp.JSON200.Labels)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ProfileResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data ProfileResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Parse UUID from state
	profileID, err := uuid.Parse(data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid ID", fmt.Sprintf("Unable to parse profile ID: %s", err))
		return
	}

	// Call API
	apiResp, err := r.client.DeleteAgentWithResponse(ctx, profileID)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to delete profile, got error: %s", err))
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

func (r *ProfileResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// mapLabelsToConfigurationOrder maps API response labels back to the configuration order
// to ensure Terraform doesn't detect false changes due to API reordering.
func (r *ProfileResource) mapLabelsToConfigurationOrder(configLabels []ProfileLabelModel, apiLabels []struct {
	Key     string              `json:"key"`
	KeyId   *openapi_types.UUID `json:"keyId,omitempty"`
	Value   string              `json:"value"`
	ValueId *openapi_types.UUID `json:"valueId,omitempty"`
}) []ProfileLabelModel {
	// Create a map of API labels for quick lookup
	apiLabelMap := make(map[string]string)
	for _, label := range apiLabels {
		apiLabelMap[label.Key] = label.Value
	}

	// Build result preserving configuration order
	result := make([]ProfileLabelModel, len(configLabels))
	for i, configLabel := range configLabels {
		key := configLabel.Key.ValueString()
		if apiValue, exists := apiLabelMap[key]; exists {
			result[i] = ProfileLabelModel{
				Key:   types.StringValue(key),
				Value: types.StringValue(apiValue),
			}
		} else {
			// Keep original if API doesn't have it (shouldn't happen normally)
			result[i] = configLabel
		}
	}

	return result
}
