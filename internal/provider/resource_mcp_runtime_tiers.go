package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/hashicorp/terraform-plugin-framework-validators/float64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.ResourceWithImportState = &McpRuntimeTiersResource{}

func NewMcpRuntimeTiersResource() resource.Resource { return &McpRuntimeTiersResource{} }

// McpRuntimeTiersResource owns the organization's MCP CPU/RAM tiers. The
// organization always has tiers (built-in defaults until changed), so creating
// the resource replaces them and destroying it restores the defaults.
type McpRuntimeTiersResource struct{ client *client.ClientWithResponses }
type McpRuntimeTiersResourceModel struct {
	ID    types.String `tfsdk:"id"`
	Tiers types.List   `tfsdk:"tiers"`
}
type mcpRuntimeTierModel struct {
	Name                      types.String  `tfsdk:"name"`
	CPURequest                types.Float64 `tfsdk:"cpu_request"`
	CPULimit                  types.Float64 `tfsdk:"cpu_limit"`
	MemoryRequestMiB          types.Int64   `tfsdk:"memory_request_mib"`
	MemoryLimitMiB            types.Int64   `tfsdk:"memory_limit_mib"`
	EffectiveMemoryRequestMiB types.Int64   `tfsdk:"effective_memory_request_mib"`
	EffectiveMemoryLimitMiB   types.Int64   `tfsdk:"effective_memory_limit_mib"`
}

var mcpRuntimeTierType = types.ObjectType{AttrTypes: map[string]attr.Type{
	"name": types.StringType, "cpu_request": types.Float64Type, "cpu_limit": types.Float64Type,
	"memory_request_mib": types.Int64Type, "memory_limit_mib": types.Int64Type,
	"effective_memory_request_mib": types.Int64Type, "effective_memory_limit_mib": types.Int64Type,
}}

var mcpRuntimeTiersAttrSpec = []AttrSpec{
	{TFName: "tiers", JSONName: "tiers", Kind: List, Children: []AttrSpec{
		{TFName: "name", JSONName: "name", Kind: Scalar},
		{TFName: "cpu_request", JSONName: "cpuRequest", Kind: Scalar},
		{TFName: "cpu_limit", JSONName: "cpuLimit", Kind: Scalar},
		{TFName: "memory_request_mib", JSONName: "memoryRequestMiB", Kind: Scalar},
		{TFName: "memory_limit_mib", JSONName: "memoryLimitMiB", Kind: Scalar},
	}},
}

func (r *McpRuntimeTiersResource) AttrSpecs() []AttrSpec { return mcpRuntimeTiersAttrSpec }
func (r *McpRuntimeTiersResource) APIShape() any         { return client.GetMcpRuntimeTiersResponse{} }
func (r *McpRuntimeTiersResource) KnownIntentionallySkipped() []string {
	return nil
}
func (r *McpRuntimeTiersResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_mcp_runtime_tiers"
}
func (r *McpRuntimeTiersResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "The organization's CPU and memory tiers for locally hosted MCP servers, which `archestra_mcp_registry_entry` selects with `local_config.runtime_tier`. The organization always has tiers: creating this resource replaces them, and destroying it restores the defaults (`S`, `M` and `L`). A tier that MCP servers still use cannot be removed or renamed. Requires an administrator.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, MarkdownDescription: "Organization ID."},
			"tiers": schema.ListNestedAttribute{
				Required:            true,
				Validators:          []validator.List{listvalidator.SizeBetween(1, 12)},
				MarkdownDescription: "Tiers offered to MCP servers, with unique names.",
				NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
					"name":                         schema.StringAttribute{Required: true, Validators: []validator.String{stringvalidator.LengthBetween(1, 32)}, MarkdownDescription: "Tier name, matched without regard to case."},
					"cpu_request":                  schema.Float64Attribute{Required: true, Validators: []validator.Float64{float64validator.Between(0.1, 256)}, MarkdownDescription: "CPU cores requested for the server."},
					"cpu_limit":                    schema.Float64Attribute{Optional: true, Validators: []validator.Float64{float64validator.Between(0.1, 256)}, MarkdownDescription: "CPU core limit; unset leaves CPU unlimited."},
					"memory_request_mib":           schema.Int64Attribute{Optional: true, Validators: []validator.Int64{int64validator.AtLeast(1)}, MarkdownDescription: "Memory request in MiB; unset derives it from the CPU request."},
					"memory_limit_mib":             schema.Int64Attribute{Optional: true, Validators: []validator.Int64{int64validator.AtLeast(1)}, MarkdownDescription: "Memory limit in MiB; unset derives it from the CPU and memory requests."},
					"effective_memory_request_mib": schema.Int64Attribute{Computed: true, MarkdownDescription: "Memory request in MiB in effect."},
					"effective_memory_limit_mib":   schema.Int64Attribute{Computed: true, MarkdownDescription: "Memory limit in MiB in effect."},
				}},
			},
		},
	}
}
func (r *McpRuntimeTiersResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = resourceClient(req, resp)
}
func (r *McpRuntimeTiersResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data McpRuntimeTiersResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.save(ctx, &data, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	r.readID(ctx, &data, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
func (r *McpRuntimeTiersResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data McpRuntimeTiersResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if data.ID.IsNull() || data.ID.IsUnknown() || data.ID.ValueString() == "" {
		resp.State.RemoveResource(ctx)
		return
	}
	result, err := r.client.GetMcpRuntimeTiersWithResponse(ctx)
	if err != nil {
		resp.Diagnostics.AddError("API Error", err.Error())
		return
	}
	if result.StatusCode() != 200 {
		mcpRuntimeTiersStatusError(result.StatusCode(), result.Body, &resp.Diagnostics)
		return
	}
	r.flatten(ctx, &data, result.Body, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
func (r *McpRuntimeTiersResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data McpRuntimeTiersResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.save(ctx, &data, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
func (r *McpRuntimeTiersResource) Delete(ctx context.Context, _ resource.DeleteRequest, resp *resource.DeleteResponse) {
	result, err := r.client.UpdateMcpRuntimeTiersWithBodyWithResponse(ctx, "application/json", strings.NewReader("null"))
	if err != nil {
		resp.Diagnostics.AddError("API Error", err.Error())
		return
	}
	if result.StatusCode() != 200 {
		mcpRuntimeTiersStatusError(result.StatusCode(), result.Body, &resp.Diagnostics)
	}
}
func (r *McpRuntimeTiersResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// save sends every field: a missing memory value keeps the old override, and
// cpuLimit is required (null means unlimited).
func (r *McpRuntimeTiersResource) save(ctx context.Context, data *McpRuntimeTiersResourceModel, diags *diag.Diagnostics) {
	var tiers []mcpRuntimeTierModel
	diags.Append(data.Tiers.ElementsAs(ctx, &tiers, false)...)
	if diags.HasError() {
		return
	}
	body := struct {
		Tiers []map[string]any `json:"tiers"`
	}{Tiers: make([]map[string]any, 0, len(tiers))}
	for _, tier := range tiers {
		body.Tiers = append(body.Tiers, map[string]any{
			"name":             tier.Name.ValueString(),
			"cpuRequest":       tier.CPURequest.ValueFloat64(),
			"cpuLimit":         tier.CPULimit.ValueFloat64Pointer(),
			"memoryRequestMiB": tier.MemoryRequestMiB.ValueInt64Pointer(),
			"memoryLimitMiB":   tier.MemoryLimitMiB.ValueInt64Pointer(),
		})
	}
	encoded, err := json.Marshal(body)
	if err != nil {
		diags.AddError("Request Encoding Failed", err.Error())
		return
	}
	result, err := r.client.UpdateMcpRuntimeTiersWithBodyWithResponse(ctx, "application/json", bytes.NewReader(encoded))
	if err != nil {
		diags.AddError("API Error", err.Error())
		return
	}
	if result.StatusCode() != 200 {
		mcpRuntimeTiersStatusError(result.StatusCode(), result.Body, diags)
		return
	}
	r.flatten(ctx, data, result.Body, diags)
}

func (r *McpRuntimeTiersResource) readID(ctx context.Context, data *McpRuntimeTiersResourceModel, diags *diag.Diagnostics) {
	result, err := r.client.GetOrganizationWithResponse(ctx)
	if err != nil {
		diags.AddError("API Error", err.Error())
		return
	}
	if result.JSON200 == nil {
		diags.AddError("Read Failed", fmt.Sprintf("API returned status %d", result.StatusCode()))
		return
	}
	data.ID = types.StringValue(result.JSON200.Id)
}

func mcpRuntimeTiersStatusError(status int, body []byte, diags *diag.Diagnostics) {
	if status == 404 {
		diags.AddError("MCP Runtime Tiers Unavailable", "This Archestra release has no MCP runtime tiers; they need 1.5.0-rc.41 or later. "+string(body))
		return
	}
	diags.AddError("MCP Runtime Tiers Request Failed", fmt.Sprintf("API returned status %d: %s", status, body))
}

// flatten keeps the configured tier order: the API returns tiers smallest first.
func (r *McpRuntimeTiersResource) flatten(ctx context.Context, data *McpRuntimeTiersResourceModel, body []byte, diags *diag.Diagnostics) {
	var wire struct {
		Tiers []struct {
			Name                     string   `json:"name"`
			CPURequest               float64  `json:"cpuRequest"`
			CPULimit                 *float64 `json:"cpuLimit"`
			MemoryRequestMiB         int64    `json:"memoryRequestMiB"`
			MemoryLimitMiB           int64    `json:"memoryLimitMiB"`
			MemoryRequestMiBOverride *int64   `json:"memoryRequestMiBOverride"`
			MemoryLimitMiBOverride   *int64   `json:"memoryLimitMiBOverride"`
		} `json:"tiers"`
	}
	if err := json.Unmarshal(body, &wire); err != nil {
		diags.AddError("Invalid API Response", err.Error())
		return
	}
	order := map[string]int{}
	if !data.Tiers.IsNull() && !data.Tiers.IsUnknown() {
		var prior []mcpRuntimeTierModel
		diags.Append(data.Tiers.ElementsAs(ctx, &prior, false)...)
		for i, tier := range prior {
			order[strings.ToLower(tier.Name.ValueString())] = i
		}
	}
	type positioned struct {
		pos   int
		value attr.Value
	}
	tiers := make([]positioned, len(wire.Tiers))
	for i, tier := range wire.Tiers {
		pos, ok := order[strings.ToLower(tier.Name)]
		if !ok {
			pos = len(order) + i
		}
		tiers[i] = positioned{pos, types.ObjectValueMust(mcpRuntimeTierType.AttrTypes, map[string]attr.Value{
			"name":                         types.StringValue(tier.Name),
			"cpu_request":                  types.Float64Value(tier.CPURequest),
			"cpu_limit":                    types.Float64PointerValue(tier.CPULimit),
			"memory_request_mib":           types.Int64PointerValue(tier.MemoryRequestMiBOverride),
			"memory_limit_mib":             types.Int64PointerValue(tier.MemoryLimitMiBOverride),
			"effective_memory_request_mib": types.Int64Value(tier.MemoryRequestMiB),
			"effective_memory_limit_mib":   types.Int64Value(tier.MemoryLimitMiB),
		})}
	}
	sort.SliceStable(tiers, func(i, j int) bool { return tiers[i].pos < tiers[j].pos })
	values := make([]attr.Value, len(tiers))
	for i, tier := range tiers {
		values[i] = tier.value
	}
	list, d := types.ListValue(mcpRuntimeTierType, values)
	diags.Append(d...)
	data.Tiers = list
}
