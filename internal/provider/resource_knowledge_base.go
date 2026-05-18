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
)

var _ resource.Resource = &KnowledgeBaseResource{}
var _ resource.ResourceWithImportState = &KnowledgeBaseResource{}

func NewKnowledgeBaseResource() resource.Resource {
	return &KnowledgeBaseResource{}
}

type KnowledgeBaseResource struct {
	client *client.ClientWithResponses
}

type KnowledgeBaseResourceModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
	Status      types.String `tfsdk:"status"`
	CreatedAt   types.String `tfsdk:"created_at"`
	UpdatedAt   types.String `tfsdk:"updated_at"`
}

func (r *KnowledgeBaseResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_knowledge_base"
}

func (r *KnowledgeBaseResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "A knowledge base groups documents and connectors that agents can search over.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Knowledge base identifier.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Human-readable name of the knowledge base.",
				Validators: []validator.String{
					stringvalidator.LengthBetween(1, 256),
				},
			},
			"description": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Free-form description.",
			},
			"status": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Lifecycle status managed by the backend.",
			},
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

func (r *KnowledgeBaseResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *KnowledgeBaseResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data KnowledgeBaseResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	priorNull := tftypes.NewValue(req.Plan.Schema.Type().TerraformType(ctx), nil)
	patch := MergePatch(ctx, req.Plan.Raw, priorNull, knowledgeBaseAttrSpec, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	LogPatch(ctx, "archestra_knowledge_base Create", patch, knowledgeBaseAttrSpec)

	bodyBytes, err := json.Marshal(patch)
	if err != nil {
		resp.Diagnostics.AddError("Marshal Error", fmt.Sprintf("Unable to marshal request body: %s", err))
		return
	}
	apiResp, err := r.client.CreateKnowledgeBaseWithBodyWithResponse(ctx, "application/json", bytes.NewReader(bodyBytes))
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to create knowledge base: %s", err))
		return
	}
	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Expected 200 OK, got status %d: %s", apiResp.StatusCode(), string(apiResp.Body)),
		)
		return
	}

	mapKnowledgeBaseCreate(&data, apiResp.JSON200)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *KnowledgeBaseResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data KnowledgeBaseResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id, err := uuid.Parse(data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid ID", err.Error())
		return
	}

	apiResp, err := r.client.GetKnowledgeBaseWithResponse(ctx, id.String())
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to read knowledge base: %s", err))
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

	mapKnowledgeBaseGet(&data, apiResp.JSON200)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *KnowledgeBaseResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data KnowledgeBaseResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id, err := uuid.Parse(data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid ID", err.Error())
		return
	}

	patch := MergePatch(ctx, req.Plan.Raw, req.State.Raw, knowledgeBaseAttrSpec, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	if len(patch) == 0 {
		// No wire-field diff. `updated_at` is Computed without
		// UseStateForUnknown (it tracks backend writes), so plan
		// carries Unknown for it; setting plan into state would surface
		// "Provider produced inconsistent result after apply". Re-Read
		// to refresh it from the API.
		apiResp, gerr := r.client.GetKnowledgeBaseWithResponse(ctx, id.String())
		if gerr != nil {
			resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to refresh knowledge base: %s", gerr))
			return
		}
		if IsNotFound(apiResp) {
			resp.Diagnostics.AddError(
				"Resource Deleted Outside Terraform",
				"The knowledge base disappeared between refresh and apply. "+
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
		mapKnowledgeBaseGet(&data, apiResp.JSON200)
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
		return
	}
	LogPatch(ctx, "archestra_knowledge_base Update", patch, knowledgeBaseAttrSpec)

	bodyBytes, err := json.Marshal(patch)
	if err != nil {
		resp.Diagnostics.AddError("Marshal Error", err.Error())
		return
	}
	apiResp, err := r.client.UpdateKnowledgeBaseWithBodyWithResponse(ctx, id.String(), "application/json", bytes.NewReader(bodyBytes))
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to update knowledge base: %s", err))
		return
	}
	if IsNotFound(apiResp) {
		resp.Diagnostics.AddError(
			"Resource Deleted Outside Terraform",
			"The knowledge base was deleted on the backend between refresh and apply. "+
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

	mapKnowledgeBaseUpdate(&data, apiResp.JSON200)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *KnowledgeBaseResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data KnowledgeBaseResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id, err := uuid.Parse(data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid ID", err.Error())
		return
	}

	apiResp, err := r.client.DeleteKnowledgeBaseWithResponse(ctx, id.String())
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to delete knowledge base: %s", err))
		return
	}
	if apiResp.JSON200 == nil && !IsNotFound(apiResp) {
		resp.Diagnostics.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Expected 200 OK or 404 Not Found, got status %d: %s", apiResp.StatusCode(), string(apiResp.Body)),
		)
	}
}

func (r *KnowledgeBaseResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// mapKnowledgeBaseGet projects the GetKnowledgeBase response into state.
// Type-specific aliases for Create/Update keep the call sites readable
// even though all three response shapes are structurally identical.
func mapKnowledgeBaseGet(data *KnowledgeBaseResourceModel, src *struct {
	CreatedAt      time.Time `json:"createdAt"`
	Description    *string   `json:"description"`
	Id             uuid.UUID `json:"id"`
	Name           string    `json:"name"`
	OrganizationId string    `json:"organizationId"`
	Status         string    `json:"status"`
	UpdatedAt      time.Time `json:"updatedAt"`
}) {
	data.ID = types.StringValue(src.Id.String())
	data.Name = types.StringValue(src.Name)
	optionalStringFromAPI(&data.Description, src.Description)
	data.Status = types.StringValue(src.Status)
	data.CreatedAt = types.StringValue(src.CreatedAt.Format(time.RFC3339))
	data.UpdatedAt = types.StringValue(src.UpdatedAt.Format(time.RFC3339))
}

func mapKnowledgeBaseCreate(data *KnowledgeBaseResourceModel, src *struct {
	CreatedAt      time.Time `json:"createdAt"`
	Description    *string   `json:"description"`
	Id             uuid.UUID `json:"id"`
	Name           string    `json:"name"`
	OrganizationId string    `json:"organizationId"`
	Status         string    `json:"status"`
	UpdatedAt      time.Time `json:"updatedAt"`
}) {
	mapKnowledgeBaseGet(data, src)
}

func mapKnowledgeBaseUpdate(data *KnowledgeBaseResourceModel, src *struct {
	CreatedAt      time.Time `json:"createdAt"`
	Description    *string   `json:"description"`
	Id             uuid.UUID `json:"id"`
	Name           string    `json:"name"`
	OrganizationId string    `json:"organizationId"`
	Status         string    `json:"status"`
	UpdatedAt      time.Time `json:"updatedAt"`
}) {
	mapKnowledgeBaseGet(data, src)
}
