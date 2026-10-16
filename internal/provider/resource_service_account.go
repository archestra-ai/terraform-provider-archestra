package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

var _ resource.Resource = &ServiceAccountResource{}
var _ resource.ResourceWithImportState = &ServiceAccountResource{}

func NewServiceAccountResource() resource.Resource {
	return &ServiceAccountResource{}
}

type ServiceAccountResource struct {
	client *client.ClientWithResponses
}

type ServiceAccountResourceModel struct {
	TeamID           types.String      `tfsdk:"team_id"`
	InitialGrants    types.Set         `tfsdk:"initial_grants"`
	Labels           []AgentLabelModel `tfsdk:"labels"`
	CreatedBy        types.String      `tfsdk:"created_by"`
	ID               types.String      `tfsdk:"id"`
	Name             types.String      `tfsdk:"name"`
	Icon             types.String      `tfsdk:"icon"`
	Role             types.String      `tfsdk:"role"`
	Disabled         types.Bool        `tfsdk:"disabled"`
	TokenCount       types.Int64       `tfsdk:"token_count"`
	ActiveTokenCount types.Int64       `tfsdk:"active_token_count"`
	LastUsedAt       types.String      `tfsdk:"last_used_at"`
	SoonestExpiryAt  types.String      `tfsdk:"soonest_expiry_at"`
	CreatedAt        types.String      `tfsdk:"created_at"`
	UpdatedAt        types.String      `tfsdk:"updated_at"`
}

func (r *ServiceAccountResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_service_account"
}

func (r *ServiceAccountResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Non-human organization identity with a role and explicit permission grants; credentials are managed separately with archestra_service_account_token.",
		Attributes: map[string]schema.Attribute{
			"initial_grants": initialGrantsAttribute(),
			"team_id":        schema.StringAttribute{Optional: true, MarkdownDescription: "Owning team; remove this field to return to organization ownership.", Validators: []validator.String{stringvalidator.LengthAtLeast(1)}},
			"labels":         schema.SetNestedAttribute{Optional: true, MarkdownDescription: "Service-account labels.", NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{"key": schema.StringAttribute{Required: true}, "value": schema.StringAttribute{Required: true}}}},
			"created_by":     schema.StringAttribute{Computed: true, MarkdownDescription: "Creator identity metadata as JSON."},
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Service account identifier.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Human-readable name of the service account.",
				Validators: []validator.String{
					// Mirrors backend `z.string().min(1)`; no upper bound.
					stringvalidator.LengthBetween(1, 256),
				},
			},
			"icon": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Emoji or image icon.",
			},
			"role":               schema.StringAttribute{Required: true, MarkdownDescription: "Organization role assigned to the service account.", Validators: []validator.String{stringvalidator.LengthBetween(1, 256)}},
			"disabled":           schema.BoolAttribute{Optional: true, Computed: true, MarkdownDescription: "Whether the identity is disabled."},
			"token_count":        schema.Int64Attribute{Computed: true, MarkdownDescription: "Total issued tokens."},
			"active_token_count": schema.Int64Attribute{Computed: true, MarkdownDescription: "Tokens currently able to authenticate."},
			"last_used_at":       schema.StringAttribute{Computed: true, MarkdownDescription: "Most recent use across the account's tokens."},
			"soonest_expiry_at":  schema.StringAttribute{Computed: true, MarkdownDescription: "Earliest expiry among active tokens."},
			"created_at": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Creation timestamp (RFC 3339).",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"updated_at": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Last-update timestamp (RFC 3339).",
			},
		},
	}
}

func (r *ServiceAccountResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.ClientWithResponses)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *client.ClientWithResponses, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}
	r.client = c
}

func (r *ServiceAccountResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data ServiceAccountResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	priorNull := tftypes.NewValue(req.Plan.Schema.Type().TerraformType(ctx), nil)
	patch := MergePatch(ctx, req.Plan.Raw, priorNull, serviceAccountAttrSpec, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	delete(patch, "disabled")
	LogPatch(ctx, "archestra_service_account Create", patch, serviceAccountAttrSpec)

	bodyBytes, err := json.Marshal(patch)
	if err != nil {
		resp.Diagnostics.AddError("Marshal Error", fmt.Sprintf("Unable to marshal request body: %s", err))
		return
	}
	apiResp, err := r.client.CreateServiceAccountWithBodyWithResponse(ctx, "application/json", bytes.NewReader(bodyBytes))
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to create service account: %s", err))
		return
	}
	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Expected 200 OK, got status %d: %s", apiResp.StatusCode(), string(apiResp.Body)),
		)
		return
	}

	plannedDisabled := data.Disabled
	if err := mapServiceAccountResponse(&data, apiResp.Body); err != nil {
		resp.Diagnostics.AddError("API Response Decode Error", err.Error())
		return
	}
	resolveInitialGrants(&data.InitialGrants)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
	if !plannedDisabled.IsNull() && !plannedDisabled.IsUnknown() && plannedDisabled.ValueBool() != data.Disabled.ValueBool() {
		body := fmt.Sprintf(`{"disabled":%t}`, plannedDisabled.ValueBool())
		result, err := r.client.UpdateServiceAccountWithBodyWithResponse(ctx, apiResp.JSON200.Id, "application/json", bytes.NewBufferString(body))
		if err != nil {
			resp.Diagnostics.AddError("Account Disable Failed", err.Error())
			return
		}
		if result.JSON200 == nil {
			resp.Diagnostics.AddError("Account Disable Failed", fmt.Sprintf("API returned status %d", result.StatusCode()))
			return
		}
		if err := mapServiceAccountResponse(&data, result.Body); err != nil {
			resp.Diagnostics.AddError("Invalid API Response", err.Error())
			return
		}
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
	}

}

func (r *ServiceAccountResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data ServiceAccountResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if data.ID.IsNull() || data.ID.ValueString() == "" {
		resp.State.RemoveResource(ctx)
		return
	}

	id, err := uuid.Parse(data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid ID", err.Error())
		return
	}

	apiResp, err := r.client.GetServiceAccountWithResponse(ctx, id)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to read service account: %s", err))
		return
	}
	if IsNotFound(apiResp) {
		resp.State.RemoveResource(ctx)
		return
	}
	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Expected 200 OK, got status %d: %s", apiResp.StatusCode(), string(apiResp.Body)),
		)
		return
	}

	if err := mapServiceAccountResponse(&data, apiResp.Body); err != nil {
		resp.Diagnostics.AddError("API Response Decode Error", err.Error())
		return
	}
	resolveInitialGrants(&data.InitialGrants)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ServiceAccountResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data ServiceAccountResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id, err := uuid.Parse(data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid ID", err.Error())
		return
	}

	patch := MergePatch(ctx, req.Plan.Raw, req.State.Raw, serviceAccountAttrSpec, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	if len(patch) == 0 {
		// No wire-field diff. `updated_at` is Computed without
		// UseStateForUnknown (it tracks backend writes), so plan
		// carries Unknown for it; setting plan into state would surface
		// "Provider produced inconsistent result after apply". Re-Read
		// to refresh it from the API.
		apiResp, gerr := r.client.GetServiceAccountWithResponse(ctx, id)
		if gerr != nil {
			resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to refresh service account: %s", gerr))
			return
		}
		if IsNotFound(apiResp) {
			resp.Diagnostics.AddError(
				"Resource Deleted Outside Terraform",
				"The service account disappeared between refresh and apply. "+
					"Re-run `terraform apply` — the next refresh drops it from state and the plan recreates it.",
			)
			return
		}
		if apiResp.JSON200 == nil {
			resp.Diagnostics.AddError(
				"Unexpected API Response",
				fmt.Sprintf("Expected 200 OK, got status %d: %s", apiResp.StatusCode(), string(apiResp.Body)),
			)
			return
		}
		if err := mapServiceAccountResponse(&data, apiResp.Body); err != nil {
			resp.Diagnostics.AddError("API Response Decode Error", err.Error())
			return
		}
		resolveInitialGrants(&data.InitialGrants)
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
		return
	}
	LogPatch(ctx, "archestra_service_account Update", patch, serviceAccountAttrSpec)

	bodyBytes, err := json.Marshal(patch)
	if err != nil {
		resp.Diagnostics.AddError("Marshal Error", err.Error())
		return
	}
	apiResp, err := r.client.UpdateServiceAccountWithBodyWithResponse(ctx, id, "application/json", bytes.NewReader(bodyBytes))
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to update service account: %s", err))
		return
	}
	if IsNotFound(apiResp) {
		resp.Diagnostics.AddError(
			"Resource Deleted Outside Terraform",
			"The service account was deleted on the backend between refresh and apply. "+
				"Re-run `terraform apply` — the next refresh drops it from state and the plan recreates it.",
		)
		return
	}
	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Expected 200 OK, got status %d: %s", apiResp.StatusCode(), string(apiResp.Body)),
		)
		return
	}

	if err := mapServiceAccountResponse(&data, apiResp.Body); err != nil {
		resp.Diagnostics.AddError("API Response Decode Error", err.Error())
		return
	}
	resolveInitialGrants(&data.InitialGrants)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ServiceAccountResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data ServiceAccountResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id, err := uuid.Parse(data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid ID", err.Error())
		return
	}

	apiResp, err := r.client.DeleteServiceAccountWithResponse(ctx, id)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to delete service account: %s", err))
		return
	}
	if apiResp.JSON200 == nil && !IsNotFound(apiResp) {
		resp.Diagnostics.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Expected 200 OK or 404 Not Found, got status %d: %s", apiResp.StatusCode(), string(apiResp.Body)),
		)
	}
}

func (r *ServiceAccountResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// Decode the stable wire projection independently of generated anonymous types.
func mapServiceAccountResponse(data *ServiceAccountResourceModel, body []byte) error {
	var src struct {
		Labels []struct {
			Key     string              `json:"key"`
			KeyId   *openapi_types.UUID `json:"keyId,omitempty"`
			Value   string              `json:"value"`
			ValueId *openapi_types.UUID `json:"valueId,omitempty"`
		} `json:"labels"`
		TeamID           *string        `json:"teamId"`
		CreatedBy        map[string]any `json:"createdBy"`
		CreatedAt        time.Time      `json:"createdAt"`
		Icon             *string        `json:"icon"`
		Role             string         `json:"role"`
		Id               uuid.UUID      `json:"id"`
		Name             string         `json:"name"`
		Disabled         bool           `json:"disabled"`
		TokenCount       int64          `json:"tokenCount"`
		ActiveTokenCount int64          `json:"activeTokenCount"`
		LastUsedAt       *string        `json:"lastUsedAt"`
		SoonestExpiryAt  *string        `json:"soonestExpiryAt"`
		UpdatedAt        time.Time      `json:"updatedAt"`
	}
	if err := json.Unmarshal(body, &src); err != nil {
		return err
	}
	data.Labels = flattenAgentLabels(data.Labels, src.Labels)
	data.CreatedBy = types.StringNull()
	if src.CreatedBy != nil {
		encoded, err := json.Marshal(src.CreatedBy)
		if err != nil {
			return err
		}
		data.CreatedBy = types.StringValue(string(encoded))
	}
	data.ID = types.StringValue(src.Id.String())
	data.Name = types.StringValue(src.Name)
	data.Icon = types.StringPointerValue(src.Icon)
	data.Role = types.StringValue(src.Role)
	data.TeamID = types.StringPointerValue(src.TeamID)
	data.Disabled = types.BoolValue(src.Disabled)
	data.TokenCount = types.Int64Value(src.TokenCount)
	data.ActiveTokenCount = types.Int64Value(src.ActiveTokenCount)
	data.LastUsedAt = types.StringPointerValue(src.LastUsedAt)
	data.SoonestExpiryAt = types.StringPointerValue(src.SoonestExpiryAt)
	data.CreatedAt = types.StringValue(src.CreatedAt.Format(time.RFC3339))
	data.UpdatedAt = types.StringValue(src.UpdatedAt.Format(time.RFC3339))
	return nil
}
