package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

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

var _ resource.ResourceWithImportState = &GuardrailsPolicyResource{}

func NewGuardrailsPolicyResource() resource.Resource { return &GuardrailsPolicyResource{} }

// GuardrailsPolicyResource owns the text of the organization's OpenAPPA
// guardrails policy. The organization always has one policy, so creating the
// resource adopts it and destroying it only stops managing it.
type GuardrailsPolicyResource struct{ client *client.ClientWithResponses }

type GuardrailsPolicyResourceModel struct {
	ID          types.String `tfsdk:"id"`
	Content     types.String `tfsdk:"content"`
	Revision    types.Int64  `tfsdk:"revision"`
	ContentHash types.String `tfsdk:"content_hash"`
	UpdatedBy   types.String `tfsdk:"updated_by"`
	UpdatedAt   types.String `tfsdk:"updated_at"`
}

var guardrailsPolicyAttrSpec = []AttrSpec{
	{TFName: "content", JSONName: "content", Kind: Scalar},
}

func (r *GuardrailsPolicyResource) AttrSpecs() []AttrSpec { return guardrailsPolicyAttrSpec }
func (r *GuardrailsPolicyResource) APIShape() any         { return client.GetGuardrailsPolicyResponse{} }
func (r *GuardrailsPolicyResource) KnownIntentionallySkipped() []string {
	// The organization ID is the resource `id`.
	return []string{"organizationId"}
}

func (r *GuardrailsPolicyResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_guardrails_policy"
}

func (r *GuardrailsPolicyResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "The text of the organization's guardrails policy. The organization always has one policy: creating this resource replaces its text, and destroying the resource leaves the text as it is. This resource owns the whole text, so do not combine it with `archestra_openappa_battery_install`, which edits the same policy. Requires OpenAPPA guardrails to be enabled.",
		Attributes: map[string]schema.Attribute{
			"id":           schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, MarkdownDescription: "Organization ID."},
			"content":      schema.StringAttribute{Required: true, Validators: []validator.String{stringvalidator.LengthAtLeast(1)}, MarkdownDescription: "Policy text. It is validated before it is saved."},
			"revision":     schema.Int64Attribute{Computed: true, MarkdownDescription: "Revision of the saved policy."},
			"content_hash": schema.StringAttribute{Computed: true, MarkdownDescription: "Hash of the saved policy text."},
			"updated_by":   schema.StringAttribute{Computed: true, MarkdownDescription: "ID of the user who saved the revision."},
			"updated_at":   schema.StringAttribute{Computed: true, MarkdownDescription: "Time the revision was saved."},
		},
	}
}

func (r *GuardrailsPolicyResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = resourceClient(req, resp)
}

func (r *GuardrailsPolicyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data GuardrailsPolicyResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	current := r.read(ctx, &resp.Diagnostics)
	if current == nil {
		return
	}
	r.save(ctx, &data, current.Revision, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *GuardrailsPolicyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data GuardrailsPolicyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	// Nothing adopted yet: Create adopts the organization's policy.
	if data.ID.IsNull() || data.ID.IsUnknown() || data.ID.ValueString() == "" {
		resp.State.RemoveResource(ctx)
		return
	}
	current := r.read(ctx, &resp.Diagnostics)
	if current == nil {
		return
	}
	current.apply(&data)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *GuardrailsPolicyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data, prior GuardrailsPolicyResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &prior)...)
	if resp.Diagnostics.HasError() {
		return
	}
	// The refreshed revision: a save in between fails as a conflict instead of
	// overwriting it.
	r.save(ctx, &data, prior.Revision.ValueInt64(), &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *GuardrailsPolicyResource) Delete(ctx context.Context, _ resource.DeleteRequest, resp *resource.DeleteResponse) {
	resp.Diagnostics.AddWarning("Guardrails policy left in place", "The organization always has a guardrails policy. Terraform stopped managing it; its text is unchanged.")
}

func (r *GuardrailsPolicyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

type guardrailsPolicyWire struct {
	OrganizationID string     `json:"organizationId"`
	Revision       int64      `json:"revision"`
	Content        string     `json:"content"`
	ContentHash    string     `json:"contentHash"`
	UpdatedBy      *string    `json:"updatedBy"`
	UpdatedAt      *time.Time `json:"updatedAt"`
}

func (w *guardrailsPolicyWire) apply(data *GuardrailsPolicyResourceModel) {
	data.ID = types.StringValue(w.OrganizationID)
	data.Content = types.StringValue(w.Content)
	data.Revision = types.Int64Value(w.Revision)
	data.ContentHash = types.StringValue(w.ContentHash)
	data.UpdatedBy = types.StringPointerValue(w.UpdatedBy)
	data.UpdatedAt = canonicalAPITimestamp(w.UpdatedAt)
}

func (r *GuardrailsPolicyResource) read(ctx context.Context, diags *diag.Diagnostics) *guardrailsPolicyWire {
	result, err := r.client.GetGuardrailsPolicyWithResponse(ctx)
	if err != nil {
		diags.AddError("API Error", err.Error())
		return nil
	}
	if result.StatusCode() == 404 {
		diags.AddError("Guardrails Not Enabled", "OpenAPPA guardrails are not enabled on this Archestra deployment.")
		return nil
	}
	if result.StatusCode() != 200 {
		diags.AddError("Read Failed", fmt.Sprintf("API returned status %d: %s", result.StatusCode(), string(result.Body)))
		return nil
	}
	var wire guardrailsPolicyWire
	if err := json.Unmarshal(result.Body, &wire); err != nil {
		diags.AddError("Invalid API Response", err.Error())
		return nil
	}
	return &wire
}

func (r *GuardrailsPolicyResource) save(ctx context.Context, data *GuardrailsPolicyResourceModel, expectedRevision int64, diags *diag.Diagnostics) {
	result, err := r.client.UpdateGuardrailsPolicyWithResponse(ctx, client.UpdateGuardrailsPolicyJSONRequestBody{
		Content:          data.Content.ValueString(),
		ExpectedRevision: int(expectedRevision),
	})
	if err != nil {
		diags.AddError("API Error", err.Error())
		return
	}
	switch result.StatusCode() {
	case 200:
	case 409:
		diags.AddError("Guardrails Policy Conflict", "The policy changed since Terraform last read it, or Git sync manages it. Run terraform apply again to plan against the latest revision, or stop Git sync. "+string(result.Body))
		return
	default:
		diags.AddError("Save Failed", fmt.Sprintf("API returned status %d: %s", result.StatusCode(), string(result.Body)))
		return
	}
	var wire guardrailsPolicyWire
	if err := json.Unmarshal(result.Body, &wire); err != nil {
		diags.AddError("Invalid API Response", err.Error())
		return
	}
	wire.apply(data)
}
