package provider

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = &IncomingEmailResource{}
	_ resource.ResourceWithImportState = &IncomingEmailResource{}
)

func NewIncomingEmailResource() resource.Resource {
	return &IncomingEmailResource{}
}

type IncomingEmailResource struct {
	client *client.ClientWithResponses
}

// IncomingEmailResourceModel maps to the org-singleton incoming-email
// webhook subscription. The backend's setup endpoint deletes any
// existing subscription before creating a new one, so changing
// `webhook_url` is an in-place Update (re-setup), not a replace.
type IncomingEmailResourceModel struct {
	ID             types.String `tfsdk:"id"`
	WebhookURL     types.String `tfsdk:"webhook_url"`
	SubscriptionID types.String `tfsdk:"subscription_id"`
	EmailProvider  types.String `tfsdk:"email_provider"`
	ExpiresAt      types.String `tfsdk:"expires_at"`
	IsActive       types.Bool   `tfsdk:"is_active"`
}

func (r *IncomingEmailResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_incoming_email"
}

func (r *IncomingEmailResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Org-singleton incoming-email webhook subscription (Microsoft Graph for the Outlook provider). At most one subscription exists per organization; the backend deletes any existing subscription before creating a new one, so changing `webhook_url` is patched in place.\n\n" +
			"~> **Provider must be configured at the org level first.** When no email provider is set up, the backend returns 400 and this resource can't be created. Configure via `archestra_organization_settings` (or env, depending on platform deployment) before declaring this resource.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Subscription row UUID from the platform (not the provider-side subscription ID).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"webhook_url": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Publicly-reachable HTTPS URL the email provider POSTs notifications to. Must be HTTPS — Microsoft Graph rejects plain HTTP.",
				Validators: []validator.String{
					stringvalidator.RegexMatches(
						regexp.MustCompile(`^https://`),
						"webhook_url must use https://",
					),
				},
			},
			"subscription_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Provider-side subscription identifier (e.g. Microsoft Graph subscription GUID). Null when no provider is configured.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"email_provider": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Email provider name as reported by the backend (e.g. `outlook`). Renamed from the backend's `provider` field — `provider` is a reserved Terraform root attribute name.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"expires_at": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "RFC 3339 timestamp at which the subscription expires (set by the provider; renewable via re-applying or running `terraform taint`).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"is_active": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "True when the backend reports an active subscription.",
			},
		},
	}
}

func (r *IncomingEmailResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.ClientWithResponses)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *client.ClientWithResponses, got: %T", req.ProviderData))
		return
	}
	r.client = c
}

func (r *IncomingEmailResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan IncomingEmailResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.callSetup(ctx, plan.WebhookURL.ValueString(), &resp.Diagnostics); err != nil {
		return
	}

	// Refresh via status to populate computed fields with authoritative values.
	r.refreshFromStatus(ctx, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *IncomingEmailResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data IncomingEmailResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := r.client.GetIncomingEmailStatusWithResponse(ctx)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to read incoming email status: %s", err))
		return
	}
	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError("Unexpected API Response",
			fmt.Sprintf("Expected 200 OK, got status %d: %s", apiResp.StatusCode(), string(apiResp.Body)))
		return
	}
	// Backend dropped the subscription out-of-band → drop the resource.
	if apiResp.JSON200.Subscription == nil {
		resp.State.RemoveResource(ctx)
		return
	}
	sub := apiResp.JSON200.Subscription
	data.ID = types.StringValue(sub.Id)
	data.WebhookURL = types.StringValue(sub.WebhookUrl)
	data.SubscriptionID = types.StringValue(sub.SubscriptionId)
	data.EmailProvider = types.StringValue(sub.Provider)
	data.ExpiresAt = types.StringValue(sub.ExpiresAt.Format(time.RFC3339))
	data.IsActive = types.BoolValue(apiResp.JSON200.IsActive)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *IncomingEmailResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan IncomingEmailResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	// Update = re-setup; backend wipes the prior subscription and
	// creates a fresh one (documented as "Cleans up ALL existing
	// subscriptions" in the platform route).
	if err := r.callSetup(ctx, plan.WebhookURL.ValueString(), &resp.Diagnostics); err != nil {
		return
	}
	r.refreshFromStatus(ctx, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *IncomingEmailResource) Delete(ctx context.Context, _ resource.DeleteRequest, resp *resource.DeleteResponse) {
	apiResp, err := r.client.DeleteIncomingEmailSubscriptionWithResponse(ctx)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to delete incoming email subscription: %s", err))
		return
	}
	// 404 = already gone — fine.
	if apiResp.JSON200 == nil && apiResp.JSON404 == nil {
		resp.Diagnostics.AddError("Unexpected API Response",
			fmt.Sprintf("Expected 200 OK or 404 Not Found, got status %d: %s", apiResp.StatusCode(), string(apiResp.Body)))
	}
}

func (r *IncomingEmailResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Singleton: any non-empty ID brings the resource into state; Read
	// fills in the real values from /status.
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}

func (r *IncomingEmailResource) callSetup(ctx context.Context, webhookURL string, diags interface{ AddError(string, string) }) error {
	body := client.SetupIncomingEmailWebhookJSONRequestBody{WebhookUrl: webhookURL}
	apiResp, err := r.client.SetupIncomingEmailWebhookWithResponse(ctx, body)
	if err != nil {
		diags.AddError("API Error", fmt.Sprintf("Unable to setup incoming email webhook: %s", err))
		return err
	}
	if apiResp.JSON200 == nil {
		diags.AddError("Unexpected API Response",
			fmt.Sprintf("Setup returned status %d: %s", apiResp.StatusCode(), string(apiResp.Body)))
		return fmt.Errorf("non-200 setup response")
	}
	return nil
}

func (r *IncomingEmailResource) refreshFromStatus(ctx context.Context, data *IncomingEmailResourceModel, diags interface {
	AddError(string, string)
	HasError() bool
}) {
	apiResp, err := r.client.GetIncomingEmailStatusWithResponse(ctx)
	if err != nil {
		diags.AddError("API Error", fmt.Sprintf("Unable to read incoming email status after setup: %s", err))
		return
	}
	if apiResp.JSON200 == nil {
		diags.AddError("Unexpected API Response",
			fmt.Sprintf("Status fetch after setup returned %d: %s", apiResp.StatusCode(), string(apiResp.Body)))
		return
	}
	if apiResp.JSON200.Subscription == nil {
		// Non-Outlook providers don't create a Graph-style subscription;
		// fall back to webhook_url-derived synthetic state. The setup
		// call still succeeded (Backend returns success=true).
		data.ID = types.StringValue("singleton")
		data.SubscriptionID = types.StringNull()
		data.EmailProvider = types.StringNull()
		data.ExpiresAt = types.StringNull()
		data.IsActive = types.BoolValue(apiResp.JSON200.IsActive)
		return
	}
	sub := apiResp.JSON200.Subscription
	data.ID = types.StringValue(sub.Id)
	data.SubscriptionID = types.StringValue(sub.SubscriptionId)
	data.EmailProvider = types.StringValue(sub.Provider)
	data.ExpiresAt = types.StringValue(sub.ExpiresAt.Format(time.RFC3339))
	data.WebhookURL = types.StringValue(sub.WebhookUrl)
	data.IsActive = types.BoolValue(apiResp.JSON200.IsActive)
}
