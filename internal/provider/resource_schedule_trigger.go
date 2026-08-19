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
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

var _ resource.Resource = &ScheduleTriggerResource{}
var _ resource.ResourceWithImportState = &ScheduleTriggerResource{}

func NewScheduleTriggerResource() resource.Resource {
	return &ScheduleTriggerResource{}
}

type ScheduleTriggerResource struct {
	client *client.ClientWithResponses
}

type ScheduleTriggerResourceModel struct {
	ID              types.String `tfsdk:"id"`
	Name            types.String `tfsdk:"name"`
	AgentID         types.String `tfsdk:"agent_id"`
	MessageTemplate types.String `tfsdk:"message_template"`
	CronExpression  types.String `tfsdk:"cron_expression"`
	Timezone        types.String `tfsdk:"timezone"`
	Enabled         types.Bool   `tfsdk:"enabled"`
	ActorUserID     types.String `tfsdk:"actor_user_id"`
	LastExecutedAt  types.String `tfsdk:"last_executed_at"`
	CreatedAt       types.String `tfsdk:"created_at"`
}

// scheduleTriggerAPIResponse mirrors the JSON shape of a schedule
// trigger record across all three Get/Create/Update endpoints; the
// generated anonymous structs are identical so a single mirror keeps
// the projection helper readable and regen-stable.
type scheduleTriggerAPIResponse struct {
	ActorUserID     string     `json:"actorUserId"`
	AgentID         string     `json:"agentId"`
	CreatedAt       time.Time  `json:"createdAt"`
	CronExpression  string     `json:"cronExpression"`
	Enabled         bool       `json:"enabled"`
	ID              string     `json:"id"`
	LastExecutedAt  *time.Time `json:"lastExecutedAt"`
	MessageTemplate string     `json:"messageTemplate"`
	Name            string     `json:"name"`
	Timezone        string     `json:"timezone"`
}

func (r *ScheduleTriggerResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_schedule_trigger"
}

func (r *ScheduleTriggerResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Cron-scheduled trigger that runs an agent on a recurring schedule with a templated message.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Schedule trigger identifier.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Human-readable name of the trigger.",
				Validators:          []validator.String{stringvalidator.LengthAtLeast(1)},
			},
			"agent_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "ID of the internal agent (`agent_type = \"agent\"`) the trigger will message.",
			},
			"message_template": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Message body sent to the agent on each tick.",
				Validators:          []validator.String{stringvalidator.LengthAtLeast(1)},
			},
			"cron_expression": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Cron expression (e.g. `0 9 * * 1-5`). Validated by the backend; rejects invalid expressions at apply.",
				Validators:          []validator.String{stringvalidator.LengthAtLeast(1)},
			},
			"timezone": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "IANA timezone (e.g. `America/New_York`). Validated by the backend; rejects non-IANA values at apply.",
				Validators:          []validator.String{stringvalidator.LengthAtLeast(1)},
			},
			"enabled": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(true),
				MarkdownDescription: "Whether the trigger fires. Disable to pause without deleting.",
			},
			"actor_user_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "ID of the user the trigger runs as (assigned from the API key's caller at create time).",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"last_executed_at": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Last execution timestamp (RFC 3339), or null if never run.",
			},
			"created_at": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Creation timestamp (RFC 3339).",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
		},
	}
}

func (r *ScheduleTriggerResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *ScheduleTriggerResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data ScheduleTriggerResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	priorNull := tftypes.NewValue(req.Plan.Schema.Type().TerraformType(ctx), nil)
	patch := MergePatch(ctx, req.Plan.Raw, priorNull, scheduleTriggerAttrSpec, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	LogPatch(ctx, "archestra_schedule_trigger Create", patch, scheduleTriggerAttrSpec)

	bodyBytes, err := json.Marshal(patch)
	if err != nil {
		resp.Diagnostics.AddError("Marshal Error", fmt.Sprintf("Unable to marshal request body: %s", err))
		return
	}
	apiResp, err := r.client.CreateScheduleTriggerWithBodyWithResponse(ctx, "application/json", bytes.NewReader(bodyBytes))
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to create schedule trigger: %s", err))
		return
	}
	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Expected 200 OK, got status %d: %s", apiResp.StatusCode(), string(apiResp.Body)),
		)
		return
	}

	if !flattenScheduleTriggerBody(&data, apiResp.Body, &resp.Diagnostics) {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ScheduleTriggerResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data ScheduleTriggerResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id, err := uuid.Parse(data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid ID", err.Error())
		return
	}

	apiResp, err := r.client.GetScheduleTriggerWithResponse(ctx, id)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to read schedule trigger: %s", err))
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

	if !flattenScheduleTriggerBody(&data, apiResp.Body, &resp.Diagnostics) {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ScheduleTriggerResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data ScheduleTriggerResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id, err := uuid.Parse(data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid ID", err.Error())
		return
	}

	patch := MergePatch(ctx, req.Plan.Raw, req.State.Raw, scheduleTriggerAttrSpec, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	if len(patch) == 0 {
		// Refresh from API — `last_executed_at` is Computed without
		// UseStateForUnknown and would otherwise land as Unknown if
		// Update fires with no AttrSpec diff.
		apiResp, gerr := r.client.GetScheduleTriggerWithResponse(ctx, id)
		if gerr != nil {
			resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to refresh schedule trigger: %s", gerr))
			return
		}
		if IsNotFound(apiResp) {
			resp.Diagnostics.AddError(
				"Resource Deleted Outside Terraform",
				"The schedule trigger disappeared between refresh and apply. Re-run `terraform apply`.",
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
		if !flattenScheduleTriggerBody(&data, apiResp.Body, &resp.Diagnostics) {
			return
		}
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
		return
	}
	LogPatch(ctx, "archestra_schedule_trigger Update", patch, scheduleTriggerAttrSpec)

	bodyBytes, err := json.Marshal(patch)
	if err != nil {
		resp.Diagnostics.AddError("Marshal Error", err.Error())
		return
	}
	apiResp, err := r.client.UpdateScheduleTriggerWithBodyWithResponse(ctx, id, "application/json", bytes.NewReader(bodyBytes))
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to update schedule trigger: %s", err))
		return
	}
	if IsNotFound(apiResp) {
		resp.Diagnostics.AddError(
			"Resource Deleted Outside Terraform",
			"The schedule trigger was deleted on the backend between refresh and apply. Re-run `terraform apply`.",
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

	if !flattenScheduleTriggerBody(&data, apiResp.Body, &resp.Diagnostics) {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ScheduleTriggerResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data ScheduleTriggerResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id, err := uuid.Parse(data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid ID", err.Error())
		return
	}

	apiResp, err := r.client.DeleteScheduleTriggerWithResponse(ctx, id)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to delete schedule trigger: %s", err))
		return
	}
	if apiResp.JSON200 == nil && !IsNotFound(apiResp) {
		resp.Diagnostics.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Expected 200 OK or 404 Not Found, got status %d: %s", apiResp.StatusCode(), string(apiResp.Body)),
		)
	}
}

func (r *ScheduleTriggerResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// flattenScheduleTriggerBody decodes the raw API response body into
// state via a single-shape mirror struct. Returns false after appending
// diagnostics on decode failure.
func flattenScheduleTriggerBody(data *ScheduleTriggerResourceModel, body []byte, diags *diag.Diagnostics) bool {
	var src scheduleTriggerAPIResponse
	if err := json.Unmarshal(body, &src); err != nil {
		diags.AddError("API Response Decode Error", "Unable to decode schedule trigger response: "+err.Error())
		return false
	}

	data.ID = types.StringValue(src.ID)
	data.Name = types.StringValue(src.Name)
	data.AgentID = types.StringValue(src.AgentID)
	data.MessageTemplate = types.StringValue(src.MessageTemplate)
	data.CronExpression = types.StringValue(src.CronExpression)
	data.Timezone = types.StringValue(src.Timezone)
	data.Enabled = types.BoolValue(src.Enabled)
	data.ActorUserID = types.StringValue(src.ActorUserID)
	data.CreatedAt = types.StringValue(src.CreatedAt.Format(time.RFC3339))
	if src.LastExecutedAt != nil {
		data.LastExecutedAt = types.StringValue(src.LastExecutedAt.Format(time.RFC3339))
	} else if !data.LastExecutedAt.IsNull() {
		data.LastExecutedAt = types.StringNull()
	}
	return true
}
