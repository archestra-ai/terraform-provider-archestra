package provider

import (
	"bytes"
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
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

var _ resource.ResourceWithImportState = &SiteNotificationResource{}

func NewSiteNotificationResource() resource.Resource { return &SiteNotificationResource{} }

// SiteNotificationResource manages the organization's single site
// notification: creating one replaces whatever notification was current.
type SiteNotificationResource struct{ client *client.ClientWithResponses }

type SiteNotificationResourceModel struct {
	ID          types.String `tfsdk:"id"`
	Content     types.String `tfsdk:"content"`
	ExpiresAt   types.String `tfsdk:"expires_at"`
	IsActive    types.Bool   `tfsdk:"is_active"`
	Dismissible types.Bool   `tfsdk:"dismissible"`
	CreatedAt   types.String `tfsdk:"created_at"`
}

var siteNotificationAttrSpec = []AttrSpec{
	{TFName: "content", JSONName: "content", Kind: Scalar},
	{TFName: "expires_at", JSONName: "expiresAt", Kind: Scalar},
	{TFName: "is_active", JSONName: "isActive", Kind: Scalar},
	{TFName: "dismissible", JSONName: "dismissible", Kind: Scalar},
}

func (r *SiteNotificationResource) AttrSpecs() []AttrSpec { return siteNotificationAttrSpec }
func (r *SiteNotificationResource) APIShape() any {
	return client.GetSiteNotificationSettingsResponse{}
}
func (r *SiteNotificationResource) KnownIntentionallySkipped() []string { return nil }

func (r *SiteNotificationResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_site_notification"
}

func (r *SiteNotificationResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "The organization's site notification: a banner every member sees. An organization has one notification, so declare this resource once; creating it replaces the current notification, and destroying it removes the banner.",
		Attributes: map[string]schema.Attribute{
			"id":          schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, MarkdownDescription: "Notification ID."},
			"content":     schema.StringAttribute{Required: true, Validators: []validator.String{stringvalidator.LengthAtLeast(1)}, MarkdownDescription: "Text of the banner. Markdown is supported."},
			"expires_at":  schema.StringAttribute{Optional: true, MarkdownDescription: "RFC 3339 time after which the banner is hidden."},
			"is_active":   schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(true), MarkdownDescription: "Whether the banner is shown."},
			"dismissible": schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(true), MarkdownDescription: "Whether members can close the banner."},
			"created_at":  schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, MarkdownDescription: "Creation timestamp."},
		},
	}
}

func (r *SiteNotificationResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = resourceClient(req, resp)
}

func (r *SiteNotificationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data SiteNotificationResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	patch := MergePatch(ctx, req.Plan.Raw, tftypes.NewValue(req.Plan.Raw.Type(), nil), siteNotificationAttrSpec, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	// Create takes every value; a default the plan holds still has to be sent.
	patch["isActive"] = data.IsActive.ValueBool()
	patch["dismissible"] = data.Dismissible.ValueBool()
	body, err := json.Marshal(patch)
	if err != nil {
		resp.Diagnostics.AddError("Request Encoding Failed", err.Error())
		return
	}
	result, err := r.client.CreateSiteNotificationWithBodyWithResponse(ctx, "application/json", bytes.NewReader(body))
	if err != nil {
		resp.Diagnostics.AddError("API Error", err.Error())
		return
	}
	if result.JSON200 == nil {
		resp.Diagnostics.AddError("Creation Failed", fmt.Sprintf("API returned status %d: %s", result.StatusCode(), string(result.Body)))
		return
	}
	flattenSiteNotification(&data, result.Body, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *SiteNotificationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data SiteNotificationResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if data.ID.IsNull() || data.ID.IsUnknown() || data.ID.ValueString() == "" {
		resp.State.RemoveResource(ctx)
		return
	}
	result, err := r.client.GetSiteNotificationSettingsWithResponse(ctx)
	if err != nil {
		resp.Diagnostics.AddError("API Error", err.Error())
		return
	}
	if result.StatusCode() != 200 {
		resp.Diagnostics.AddError("Read Failed", fmt.Sprintf("API returned status %d", result.StatusCode()))
		return
	}
	// The organization keeps one notification: once another replaced this one,
	// it is gone.
	if result.JSON200 == nil || result.JSON200.Id != data.ID.ValueString() {
		resp.State.RemoveResource(ctx)
		return
	}
	flattenSiteNotification(&data, result.Body, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *SiteNotificationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data, prior SiteNotificationResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &prior)...)
	patch := MergePatch(ctx, req.Plan.Raw, req.State.Raw, siteNotificationAttrSpec, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	body, err := json.Marshal(patch)
	if err != nil {
		resp.Diagnostics.AddError("Request Encoding Failed", err.Error())
		return
	}
	result, err := r.client.UpdateSiteNotificationWithBodyWithResponse(ctx, prior.ID.ValueString(), "application/json", bytes.NewReader(body))
	if err != nil {
		resp.Diagnostics.AddError("API Error", err.Error())
		return
	}
	if result.JSON200 == nil {
		resp.Diagnostics.AddError("Update Failed", fmt.Sprintf("API returned status %d: %s", result.StatusCode(), string(result.Body)))
		return
	}
	flattenSiteNotification(&data, result.Body, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *SiteNotificationResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data SiteNotificationResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	result, err := r.client.DeleteSiteNotificationWithResponse(ctx, data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("API Error", err.Error())
		return
	}
	if result.StatusCode() != 200 && !IsNotFound(result) {
		resp.Diagnostics.AddError("Delete Failed", fmt.Sprintf("API returned status %d", result.StatusCode()))
	}
}

func (r *SiteNotificationResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func flattenSiteNotification(data *SiteNotificationResourceModel, body []byte, diags *diag.Diagnostics) {
	var value struct {
		ID          string     `json:"id"`
		Content     string     `json:"content"`
		ExpiresAt   *time.Time `json:"expiresAt"`
		CreatedAt   *time.Time `json:"createdAt"`
		IsActive    bool       `json:"isActive"`
		Dismissible *bool      `json:"dismissible"`
	}
	if err := json.Unmarshal(body, &value); err != nil {
		diags.AddError("Invalid API Response", err.Error())
		return
	}
	data.ID = types.StringValue(value.ID)
	data.Content = types.StringValue(value.Content)
	data.ExpiresAt = credentialConfiguredTimestamp(data.ExpiresAt, value.ExpiresAt)
	data.CreatedAt = canonicalAPITimestamp(value.CreatedAt)
	data.IsActive = types.BoolValue(value.IsActive)
	// Archestra releases before dismissible banners omit the field; keep the
	// planned value rather than report a change the server never made.
	if value.Dismissible != nil {
		data.Dismissible = types.BoolValue(*value.Dismissible)
	} else if data.Dismissible.IsNull() || data.Dismissible.IsUnknown() {
		data.Dismissible = types.BoolValue(true)
	}
}
