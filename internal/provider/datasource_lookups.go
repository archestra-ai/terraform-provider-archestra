package provider

import (
	"context"
	"fmt"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Lookups of objects that usually exist before Terraform does, by the name a
// person knows them by. Each returns the ID that grants (`subject_id`) and
// references (`environment_id`, `service_account_id`) expect.

var (
	_ datasource.DataSource = &UserDataSource{}
	_ datasource.DataSource = &ServiceAccountDataSource{}
	_ datasource.DataSource = &EnvironmentDataSource{}
)

func NewUserDataSource() datasource.DataSource           { return &UserDataSource{} }
func NewServiceAccountDataSource() datasource.DataSource { return &ServiceAccountDataSource{} }
func NewEnvironmentDataSource() datasource.DataSource    { return &EnvironmentDataSource{} }

// dataSourceClient reads the API client the provider configured.
func dataSourceClient(req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) *client.ClientWithResponses {
	if req.ProviderData == nil {
		return nil
	}
	apiClient, ok := req.ProviderData.(*client.ClientWithResponses)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Data Source Configure Type", fmt.Sprintf("Expected *client.ClientWithResponses, got: %T", req.ProviderData))
		return nil
	}
	return apiClient
}

// ---- archestra_user ----

type UserDataSource struct {
	client *client.ClientWithResponses
}

type UserDataSourceModel struct {
	ID    types.String `tfsdk:"id"`
	Email types.String `tfsdk:"email"`
	Name  types.String `tfsdk:"name"`
	Role  types.String `tfsdk:"role"`
}

func (d *UserDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_user"
}

func (d *UserDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Finds a member of the organization by email address. Use `id` as the `subject_id` of a `user` grant.",
		Attributes: map[string]schema.Attribute{
			"id":    schema.StringAttribute{Computed: true, MarkdownDescription: "User ID."},
			"email": schema.StringAttribute{Required: true, MarkdownDescription: "Email address of the member."},
			"name":  schema.StringAttribute{Computed: true, MarkdownDescription: "Display name of the member."},
			"role":  schema.StringAttribute{Computed: true, MarkdownDescription: "Organization role of the member."},
		},
	}
}

func (d *UserDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = dataSourceClient(req, resp)
}

func (d *UserDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data UserDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	member, err := d.client.GetOrganizationMemberWithResponse(ctx, data.Email.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to read the member: %s", err))
		return
	}
	if member.StatusCode() == 404 {
		resp.Diagnostics.AddError("User Not Found", fmt.Sprintf("No member of the organization has the email address %q.", data.Email.ValueString()))
		return
	}
	if member.JSON200 == nil {
		resp.Diagnostics.AddError("Unexpected API Response", fmt.Sprintf("Expected 200 OK, got status %d", member.StatusCode()))
		return
	}
	data.ID = types.StringValue(member.JSON200.Id)
	data.Name = types.StringValue(member.JSON200.Name)
	data.Role = types.StringValue(member.JSON200.Role)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// ---- archestra_service_account ----

type ServiceAccountDataSource struct {
	client *client.ClientWithResponses
}

type ServiceAccountDataSourceModel struct {
	ID       types.String `tfsdk:"id"`
	Name     types.String `tfsdk:"name"`
	Role     types.String `tfsdk:"role"`
	TeamID   types.String `tfsdk:"team_id"`
	Disabled types.Bool   `tfsdk:"disabled"`
}

func (d *ServiceAccountDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_service_account"
}

func (d *ServiceAccountDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Finds a service account by its exact name. Use `id` as the `subject_id` of a `serviceAccount` grant, or wherever a service account ID is expected.",
		Attributes: map[string]schema.Attribute{
			"id":       schema.StringAttribute{Computed: true, MarkdownDescription: "Service account ID."},
			"name":     schema.StringAttribute{Required: true, MarkdownDescription: "Exact name of the service account."},
			"role":     schema.StringAttribute{Computed: true, MarkdownDescription: "Role of the service account."},
			"team_id":  schema.StringAttribute{Computed: true, MarkdownDescription: "Team the service account belongs to, when it has one."},
			"disabled": schema.BoolAttribute{Computed: true, MarkdownDescription: "Whether the service account is disabled."},
		},
	}
}

func (d *ServiceAccountDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = dataSourceClient(req, resp)
}

func (d *ServiceAccountDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data ServiceAccountDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	accounts, err := d.client.GetServiceAccountsWithResponse(ctx, nil)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to list service accounts: %s", err))
		return
	}
	if accounts.JSON200 == nil {
		resp.Diagnostics.AddError("Unexpected API Response", fmt.Sprintf("Expected 200 OK, got status %d", accounts.StatusCode()))
		return
	}
	found := -1
	for index, account := range *accounts.JSON200 {
		if account.Name != data.Name.ValueString() {
			continue
		}
		if found >= 0 {
			resp.Diagnostics.AddError("Ambiguous Service Account", fmt.Sprintf("Several service accounts are named %q.", data.Name.ValueString()))
			return
		}
		found = index
	}
	if found < 0 {
		resp.Diagnostics.AddError("Service Account Not Found", fmt.Sprintf("No service account visible to this API key is named %q.", data.Name.ValueString()))
		return
	}
	account := (*accounts.JSON200)[found]
	data.ID = types.StringValue(account.Id.String())
	data.Role = types.StringValue(account.Role)
	data.TeamID = types.StringPointerValue(account.TeamId)
	data.Disabled = types.BoolValue(account.Disabled)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// ---- archestra_environment ----

type EnvironmentDataSource struct {
	client *client.ClientWithResponses
}

type EnvironmentDataSourceModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
	Namespace   types.String `tfsdk:"namespace"`
}

func (d *EnvironmentDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_environment"
}

func (d *EnvironmentDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Finds an environment by its exact name. Use `id` wherever an `environment_id` is expected.",
		Attributes: map[string]schema.Attribute{
			"id":          schema.StringAttribute{Computed: true, MarkdownDescription: "Environment ID."},
			"name":        schema.StringAttribute{Required: true, MarkdownDescription: "Exact name of the environment."},
			"description": schema.StringAttribute{Computed: true, MarkdownDescription: "Description of the environment."},
			"namespace":   schema.StringAttribute{Computed: true, MarkdownDescription: "Kubernetes namespace of the environment, when it has its own."},
		},
	}
}

func (d *EnvironmentDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = dataSourceClient(req, resp)
}

func (d *EnvironmentDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data EnvironmentDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	environments, err := d.client.ListEnvironmentsWithResponse(ctx, nil)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to list environments: %s", err))
		return
	}
	if environments.JSON200 == nil {
		resp.Diagnostics.AddError("Unexpected API Response", fmt.Sprintf("Expected 200 OK, got status %d", environments.StatusCode()))
		return
	}
	found := -1
	for index, environment := range environments.JSON200.Environments {
		if environment.Name != data.Name.ValueString() {
			continue
		}
		if found >= 0 {
			resp.Diagnostics.AddError("Ambiguous Environment", fmt.Sprintf("Several environments are named %q.", data.Name.ValueString()))
			return
		}
		found = index
	}
	if found < 0 {
		resp.Diagnostics.AddError("Environment Not Found", fmt.Sprintf("No environment visible to this API key is named %q.", data.Name.ValueString()))
		return
	}
	environment := environments.JSON200.Environments[found]
	data.ID = types.StringValue(environment.Id.String())
	data.Description = types.StringPointerValue(environment.Description)
	data.Namespace = types.StringPointerValue(environment.Namespace)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
