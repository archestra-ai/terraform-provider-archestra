package provider

import (
	"context"
	"fmt"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

var _ resource.Resource = &MCPServerRegistryResource{}
var _ resource.ResourceWithImportState = &MCPServerRegistryResource{}

func NewMCPServerRegistryResource() resource.Resource {
	return &MCPServerRegistryResource{}
}

type MCPServerRegistryResource struct {
	client *client.ClientWithResponses
}

type MCPServerRegistryResourceModel struct {
	ID                  types.String `tfsdk:"id"`
	Name                types.String `tfsdk:"name"`
	Description         types.String `tfsdk:"description"`
	DocsURL             types.String `tfsdk:"docs_url"`
	InstallationCommand types.String `tfsdk:"installation_command"`
	AuthDescription     types.String `tfsdk:"auth_description"`
	LocalConfig         types.Object `tfsdk:"local_config"`
	AuthFields          types.List   `tfsdk:"auth_fields"`
}

type LocalConfigModel struct {
	Command       types.String `tfsdk:"command"`
	Arguments     types.List   `tfsdk:"arguments"`
	Environment   types.Map    `tfsdk:"environment"`
	DockerImage   types.String `tfsdk:"docker_image"`
	TransportType types.String `tfsdk:"transport_type"`
	HTTPPort      types.Int64  `tfsdk:"http_port"`
	HTTPPath      types.String `tfsdk:"http_path"`
}

type AuthFieldModel struct {
	Name        types.String `tfsdk:"name"`
	Label       types.String `tfsdk:"label"`
	Type        types.String `tfsdk:"type"`
	Required    types.Bool   `tfsdk:"required"`
	Description types.String `tfsdk:"description"`
}

func (r *MCPServerRegistryResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_mcp_server"
}

func (r *MCPServerRegistryResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages an MCP server in the Private MCP Registry. This allows you to register local MCP servers that can then be installed by agents.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "MCP server catalog identifier",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "The name of the MCP server",
				Required:            true,
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "Description of the MCP server",
				Optional:            true,
			},
			"docs_url": schema.StringAttribute{
				MarkdownDescription: "URL to the MCP server documentation",
				Optional:            true,
			},
			"installation_command": schema.StringAttribute{
				MarkdownDescription: "Installation command for the MCP server (e.g., npm install -g @example/mcp-server)",
				Optional:            true,
			},
			"auth_description": schema.StringAttribute{
				MarkdownDescription: "Description of the authentication requirements",
				Optional:            true,
			},
			"local_config": schema.SingleNestedAttribute{
				MarkdownDescription: "Configuration for MCP servers run in the Archestra orchestrator MCP runtime",
				Optional:            true,
				Attributes: map[string]schema.Attribute{
					"command": schema.StringAttribute{
						MarkdownDescription: "The executable command to run (e.g., 'node', 'python', 'npx'). Optional if Docker Image is set (will use image's default CMD).",
						Required:            true,
					},
					"arguments": schema.ListAttribute{
						MarkdownDescription: "Arguments to pass to the command",
						Optional:            true,
						ElementType:         types.StringType,
					},
					"environment": schema.MapAttribute{
						MarkdownDescription: "Environment variables for the MCP server (KEY=value format)",
						Optional:            true,
						ElementType:         types.StringType,
					},
					"docker_image": schema.StringAttribute{
						MarkdownDescription: "Custom Docker image URL. If not specified, Archestra's default base image will be used.",
						Optional:            true,
					},
					"transport_type": schema.StringAttribute{
						MarkdownDescription: "Transport type: 'stdio' or 'streamable-http'. Defaults to 'stdio'",
						Optional:            true,
						Validators: []validator.String{
							stringvalidator.OneOf("stdio", "streamable-http"),
						},
					},
					"http_port": schema.Int64Attribute{
						MarkdownDescription: "HTTP port for streamable-http transport",
						Optional:            true,
					},
					"http_path": schema.StringAttribute{
						MarkdownDescription: "HTTP path for streamable-http transport (e.g., '/sse')",
						Optional:            true,
					},
				},
			},
			"auth_fields": schema.ListNestedAttribute{
				MarkdownDescription: "Custom authentication fields required by the MCP server",
				Optional:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"name": schema.StringAttribute{
							MarkdownDescription: "Field name (used as environment variable)",
							Required:            true,
						},
						"label": schema.StringAttribute{
							MarkdownDescription: "Display label for the field",
							Required:            true,
						},
						"type": schema.StringAttribute{
							MarkdownDescription: "Field type: 'text', 'password', 'select', etc.",
							Required:            true,
						},
						"required": schema.BoolAttribute{
							MarkdownDescription: "Whether this field is required",
							Required:            true,
						},
						"description": schema.StringAttribute{
							MarkdownDescription: "Description of the field",
							Optional:            true,
						},
					},
				},
			},
		},
	}
}

func (r *MCPServerRegistryResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *MCPServerRegistryResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data MCPServerRegistryResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Build the request body
	requestBody := client.CreateInternalMcpCatalogItemJSONRequestBody{
		Name:       data.Name.ValueString(),
		ServerType: "local", // For now, we only support local servers
	}

	// Set optional string fields
	if !data.Description.IsNull() {
		desc := data.Description.ValueString()
		requestBody.Description = &desc
	}
	if !data.DocsURL.IsNull() {
		url := data.DocsURL.ValueString()
		requestBody.DocsUrl = &url
	}
	if !data.InstallationCommand.IsNull() {
		cmd := data.InstallationCommand.ValueString()
		requestBody.InstallationCommand = &cmd
	}
	if !data.AuthDescription.IsNull() {
		desc := data.AuthDescription.ValueString()
		requestBody.AuthDescription = &desc
	}

	// Handle LocalConfig
	if !data.LocalConfig.IsNull() {
		var localConfig LocalConfigModel
		resp.Diagnostics.Append(data.LocalConfig.As(ctx, &localConfig, basetypes.ObjectAsOptions{})...)
		if resp.Diagnostics.HasError() {
			return
		}

		lcStruct := struct {
			Arguments   *[]string `json:"arguments,omitempty"`
			Command     *string   `json:"command,omitempty"`
			DockerImage *string   `json:"dockerImage,omitempty"`
			Environment *[]struct {
				Description          *string                                                               `json:"description,omitempty"`
				Key                  string                                                                `json:"key"`
				PromptOnInstallation bool                                                                  `json:"promptOnInstallation"`
				Required             *bool                                                                 `json:"required,omitempty"`
				Type                 client.CreateInternalMcpCatalogItemJSONBodyLocalConfigEnvironmentType `json:"type"`
				Value                *string                                                               `json:"value,omitempty"`
			} `json:"environment,omitempty"`
			HttpPath      *string                                                              `json:"httpPath,omitempty"`
			HttpPort      *float32                                                             `json:"httpPort,omitempty"`
			TransportType *client.CreateInternalMcpCatalogItemJSONBodyLocalConfigTransportType `json:"transportType,omitempty"`
		}{}

		// Command
		if !localConfig.Command.IsNull() {
			cmd := localConfig.Command.ValueString()
			lcStruct.Command = &cmd
		}

		// Arguments
		if !localConfig.Arguments.IsNull() {
			var args []string
			resp.Diagnostics.Append(localConfig.Arguments.ElementsAs(ctx, &args, false)...)
			if resp.Diagnostics.HasError() {
				return
			}
			lcStruct.Arguments = &args
		}

		// Environment - convert map[string]string to new struct format
		if !localConfig.Environment.IsNull() {
			var env map[string]string
			resp.Diagnostics.Append(localConfig.Environment.ElementsAs(ctx, &env, false)...)
			if resp.Diagnostics.HasError() {
				return
			}
			envSlice := make([]struct {
				Description          *string                                                               `json:"description,omitempty"`
				Key                  string                                                                `json:"key"`
				PromptOnInstallation bool                                                                  `json:"promptOnInstallation"`
				Required             *bool                                                                 `json:"required,omitempty"`
				Type                 client.CreateInternalMcpCatalogItemJSONBodyLocalConfigEnvironmentType `json:"type"`
				Value                *string                                                               `json:"value,omitempty"`
			}, 0, len(env))
			for k, v := range env {
				val := v
				envSlice = append(envSlice, struct {
					Description          *string                                                               `json:"description,omitempty"`
					Key                  string                                                                `json:"key"`
					PromptOnInstallation bool                                                                  `json:"promptOnInstallation"`
					Required             *bool                                                                 `json:"required,omitempty"`
					Type                 client.CreateInternalMcpCatalogItemJSONBodyLocalConfigEnvironmentType `json:"type"`
					Value                *string                                                               `json:"value,omitempty"`
				}{
					Key:   k,
					Value: &val,
					Type:  "string",
				})
			}
			lcStruct.Environment = &envSlice
		}

		// Optional fields
		if !localConfig.DockerImage.IsNull() {
			img := localConfig.DockerImage.ValueString()
			lcStruct.DockerImage = &img
		}
		if !localConfig.HTTPPath.IsNull() {
			path := localConfig.HTTPPath.ValueString()
			lcStruct.HttpPath = &path
		}
		if !localConfig.HTTPPort.IsNull() {
			port := float32(localConfig.HTTPPort.ValueInt64())
			lcStruct.HttpPort = &port
		}
		if !localConfig.TransportType.IsNull() {
			tt := client.CreateInternalMcpCatalogItemJSONBodyLocalConfigTransportType(localConfig.TransportType.ValueString())
			lcStruct.TransportType = &tt
		}

		requestBody.LocalConfig = &lcStruct
	}

	// Handle AuthFields
	if !data.AuthFields.IsNull() {
		var authFields []AuthFieldModel
		resp.Diagnostics.Append(data.AuthFields.ElementsAs(ctx, &authFields, false)...)
		if resp.Diagnostics.HasError() {
			return
		}

		afSlice := make([]struct {
			Description *string `json:"description,omitempty"`
			Label       string  `json:"label"`
			Name        string  `json:"name"`
			Required    bool    `json:"required"`
			Type        string  `json:"type"`
		}, len(authFields))

		for i, af := range authFields {
			afSlice[i].Name = af.Name.ValueString()
			afSlice[i].Label = af.Label.ValueString()
			afSlice[i].Type = af.Type.ValueString()
			afSlice[i].Required = af.Required.ValueBool()
			if !af.Description.IsNull() {
				desc := af.Description.ValueString()
				afSlice[i].Description = &desc
			}
		}

		requestBody.AuthFields = &afSlice
	}

	// Call API
	apiResp, err := r.client.CreateInternalMcpCatalogItemWithResponse(ctx, requestBody)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to create MCP server, got error: %s", err))
		return
	}

	// Check response
	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Expected 200 OK, got status %d: %s", apiResp.StatusCode(), string(apiResp.Body)),
		)
		return
	}

	// Map response to Terraform state
	data.ID = types.StringValue(apiResp.JSON200.Id.String())
	data.Name = types.StringValue(apiResp.JSON200.Name)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *MCPServerRegistryResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data MCPServerRegistryResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Parse UUID from state
	serverID, err := uuid.Parse(data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid ID", fmt.Sprintf("Unable to parse MCP server ID: %s", err))
		return
	}

	// Call API
	apiResp, err := r.client.GetInternalMcpCatalogItemWithResponse(ctx, serverID)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to read MCP server, got error: %s", err))
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

	if apiResp.JSON200.Description != nil {
		data.Description = types.StringValue(*apiResp.JSON200.Description)
	} else {
		data.Description = types.StringNull()
	}

	if apiResp.JSON200.DocsUrl != nil {
		data.DocsURL = types.StringValue(*apiResp.JSON200.DocsUrl)
	} else {
		data.DocsURL = types.StringNull()
	}

	if apiResp.JSON200.InstallationCommand != nil {
		data.InstallationCommand = types.StringValue(*apiResp.JSON200.InstallationCommand)
	} else {
		data.InstallationCommand = types.StringNull()
	}

	if apiResp.JSON200.AuthDescription != nil {
		data.AuthDescription = types.StringValue(*apiResp.JSON200.AuthDescription)
	} else {
		data.AuthDescription = types.StringNull()
	}

	// Map LocalConfig from API response if present
	if apiResp.JSON200.LocalConfig != nil {
		localConfigObj := map[string]attr.Value{
			"command":        types.StringNull(),
			"arguments":      types.ListNull(types.StringType),
			"environment":    types.MapNull(types.StringType),
			"docker_image":   types.StringNull(),
			"transport_type": types.StringNull(),
			"http_port":      types.Int64Null(),
			"http_path":      types.StringNull(),
		}

		// Command
		if apiResp.JSON200.LocalConfig.Command != nil {
			localConfigObj["command"] = types.StringValue(*apiResp.JSON200.LocalConfig.Command)
		}

		// Arguments
		if apiResp.JSON200.LocalConfig.Arguments != nil && len(*apiResp.JSON200.LocalConfig.Arguments) > 0 {
			argValues := make([]attr.Value, len(*apiResp.JSON200.LocalConfig.Arguments))
			for i, arg := range *apiResp.JSON200.LocalConfig.Arguments {
				argValues[i] = types.StringValue(arg)
			}
			localConfigObj["arguments"], _ = types.ListValue(types.StringType, argValues)
		}

		// Environment
		if apiResp.JSON200.LocalConfig.Environment != nil && len(*apiResp.JSON200.LocalConfig.Environment) > 0 {
			envMap := make(map[string]attr.Value)
			for _, envVar := range *apiResp.JSON200.LocalConfig.Environment {
				if envVar.Value != nil {
					envMap[envVar.Key] = types.StringValue(*envVar.Value)
				} else {
					envMap[envVar.Key] = types.StringValue("")
				}
			}
			localConfigObj["environment"], _ = types.MapValue(types.StringType, envMap)
		}

		// Optional fields
		if apiResp.JSON200.LocalConfig.DockerImage != nil {
			localConfigObj["docker_image"] = types.StringValue(*apiResp.JSON200.LocalConfig.DockerImage)
		}
		if apiResp.JSON200.LocalConfig.HttpPath != nil {
			localConfigObj["http_path"] = types.StringValue(*apiResp.JSON200.LocalConfig.HttpPath)
		}
		if apiResp.JSON200.LocalConfig.HttpPort != nil {
			localConfigObj["http_port"] = types.Int64Value(int64(*apiResp.JSON200.LocalConfig.HttpPort))
		}
		if apiResp.JSON200.LocalConfig.TransportType != nil {
			localConfigObj["transport_type"] = types.StringValue(string(*apiResp.JSON200.LocalConfig.TransportType))
		}

		localConfigAttrTypes := map[string]attr.Type{
			"command":        types.StringType,
			"arguments":      types.ListType{ElemType: types.StringType},
			"environment":    types.MapType{ElemType: types.StringType},
			"docker_image":   types.StringType,
			"transport_type": types.StringType,
			"http_port":      types.Int64Type,
			"http_path":      types.StringType,
		}

		data.LocalConfig, _ = types.ObjectValue(localConfigAttrTypes, localConfigObj)
	} else {
		data.LocalConfig = types.ObjectNull(map[string]attr.Type{
			"command":        types.StringType,
			"arguments":      types.ListType{ElemType: types.StringType},
			"environment":    types.MapType{ElemType: types.StringType},
			"docker_image":   types.StringType,
			"transport_type": types.StringType,
			"http_port":      types.Int64Type,
			"http_path":      types.StringType,
		})
	}

	// Map AuthFields from API response if present
	if apiResp.JSON200.AuthFields != nil && len(*apiResp.JSON200.AuthFields) > 0 {
		authFieldValues := make([]attr.Value, len(*apiResp.JSON200.AuthFields))
		authFieldAttrTypes := map[string]attr.Type{
			"name":        types.StringType,
			"label":       types.StringType,
			"type":        types.StringType,
			"required":    types.BoolType,
			"description": types.StringType,
		}

		for i, af := range *apiResp.JSON200.AuthFields {
			authFieldMap := map[string]attr.Value{
				"name":        types.StringValue(af.Name),
				"label":       types.StringValue(af.Label),
				"type":        types.StringValue(af.Type),
				"required":    types.BoolValue(af.Required),
				"description": types.StringNull(),
			}
			if af.Description != nil {
				authFieldMap["description"] = types.StringValue(*af.Description)
			}
			authFieldValues[i], _ = types.ObjectValue(authFieldAttrTypes, authFieldMap)
		}
		data.AuthFields, _ = types.ListValue(types.ObjectType{AttrTypes: authFieldAttrTypes}, authFieldValues)
	} else {
		data.AuthFields = types.ListNull(types.ObjectType{AttrTypes: map[string]attr.Type{
			"name":        types.StringType,
			"label":       types.StringType,
			"type":        types.StringType,
			"required":    types.BoolType,
			"description": types.StringType,
		}})
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *MCPServerRegistryResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data MCPServerRegistryResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Parse UUID from state
	serverID, err := uuid.Parse(data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid ID", fmt.Sprintf("Unable to parse MCP server ID: %s", err))
		return
	}

	// Build the request body
	requestBody := client.UpdateInternalMcpCatalogItemJSONRequestBody{}

	// Set optional string fields
	if !data.Name.IsNull() {
		name := data.Name.ValueString()
		requestBody.Name = &name
	}
	if !data.Description.IsNull() {
		desc := data.Description.ValueString()
		requestBody.Description = &desc
	}
	if !data.DocsURL.IsNull() {
		url := data.DocsURL.ValueString()
		requestBody.DocsUrl = &url
	}
	if !data.InstallationCommand.IsNull() {
		cmd := data.InstallationCommand.ValueString()
		requestBody.InstallationCommand = &cmd
	}
	if !data.AuthDescription.IsNull() {
		desc := data.AuthDescription.ValueString()
		requestBody.AuthDescription = &desc
	}

	// Handle LocalConfig
	if !data.LocalConfig.IsNull() {
		var localConfig LocalConfigModel
		resp.Diagnostics.Append(data.LocalConfig.As(ctx, &localConfig, basetypes.ObjectAsOptions{})...)
		if resp.Diagnostics.HasError() {
			return
		}

		lcStruct := struct {
			Arguments   *[]string `json:"arguments,omitempty"`
			Command     *string   `json:"command,omitempty"`
			DockerImage *string   `json:"dockerImage,omitempty"`
			Environment *[]struct {
				Description          *string                                                               `json:"description,omitempty"`
				Key                  string                                                                `json:"key"`
				PromptOnInstallation bool                                                                  `json:"promptOnInstallation"`
				Required             *bool                                                                 `json:"required,omitempty"`
				Type                 client.UpdateInternalMcpCatalogItemJSONBodyLocalConfigEnvironmentType `json:"type"`
				Value                *string                                                               `json:"value,omitempty"`
			} `json:"environment,omitempty"`
			HttpPath      *string                                                              `json:"httpPath,omitempty"`
			HttpPort      *float32                                                             `json:"httpPort,omitempty"`
			TransportType *client.UpdateInternalMcpCatalogItemJSONBodyLocalConfigTransportType `json:"transportType,omitempty"`
		}{}

		// Command
		if !localConfig.Command.IsNull() {
			cmd := localConfig.Command.ValueString()
			lcStruct.Command = &cmd
		}

		// Arguments
		if !localConfig.Arguments.IsNull() {
			var args []string
			resp.Diagnostics.Append(localConfig.Arguments.ElementsAs(ctx, &args, false)...)
			if resp.Diagnostics.HasError() {
				return
			}
			lcStruct.Arguments = &args
		}

		// Environment - convert map[string]string to new struct format
		if !localConfig.Environment.IsNull() {
			var env map[string]string
			resp.Diagnostics.Append(localConfig.Environment.ElementsAs(ctx, &env, false)...)
			if resp.Diagnostics.HasError() {
				return
			}
			envSlice := make([]struct {
				Description          *string                                                               `json:"description,omitempty"`
				Key                  string                                                                `json:"key"`
				PromptOnInstallation bool                                                                  `json:"promptOnInstallation"`
				Required             *bool                                                                 `json:"required,omitempty"`
				Type                 client.UpdateInternalMcpCatalogItemJSONBodyLocalConfigEnvironmentType `json:"type"`
				Value                *string                                                               `json:"value,omitempty"`
			}, 0, len(env))
			for k, v := range env {
				val := v
				envSlice = append(envSlice, struct {
					Description          *string                                                               `json:"description,omitempty"`
					Key                  string                                                                `json:"key"`
					PromptOnInstallation bool                                                                  `json:"promptOnInstallation"`
					Required             *bool                                                                 `json:"required,omitempty"`
					Type                 client.UpdateInternalMcpCatalogItemJSONBodyLocalConfigEnvironmentType `json:"type"`
					Value                *string                                                               `json:"value,omitempty"`
				}{
					Key:   k,
					Value: &val,
					Type:  "string",
				})
			}
			lcStruct.Environment = &envSlice
		}

		// Optional fields
		if !localConfig.DockerImage.IsNull() {
			img := localConfig.DockerImage.ValueString()
			lcStruct.DockerImage = &img
		}
		if !localConfig.HTTPPath.IsNull() {
			path := localConfig.HTTPPath.ValueString()
			lcStruct.HttpPath = &path
		}
		if !localConfig.HTTPPort.IsNull() {
			port := float32(localConfig.HTTPPort.ValueInt64())
			lcStruct.HttpPort = &port
		}
		if !localConfig.TransportType.IsNull() {
			tt := client.UpdateInternalMcpCatalogItemJSONBodyLocalConfigTransportType(localConfig.TransportType.ValueString())
			lcStruct.TransportType = &tt
		}

		requestBody.LocalConfig = &lcStruct
	}

	// Handle AuthFields
	if !data.AuthFields.IsNull() {
		var authFields []AuthFieldModel
		resp.Diagnostics.Append(data.AuthFields.ElementsAs(ctx, &authFields, false)...)
		if resp.Diagnostics.HasError() {
			return
		}

		afSlice := make([]struct {
			Description *string `json:"description,omitempty"`
			Label       string  `json:"label"`
			Name        string  `json:"name"`
			Required    bool    `json:"required"`
			Type        string  `json:"type"`
		}, len(authFields))

		for i, af := range authFields {
			afSlice[i].Name = af.Name.ValueString()
			afSlice[i].Label = af.Label.ValueString()
			afSlice[i].Type = af.Type.ValueString()
			afSlice[i].Required = af.Required.ValueBool()
			if !af.Description.IsNull() {
				desc := af.Description.ValueString()
				afSlice[i].Description = &desc
			}
		}

		requestBody.AuthFields = &afSlice
	}

	// Call API
	apiResp, err := r.client.UpdateInternalMcpCatalogItemWithResponse(ctx, serverID, requestBody)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to update MCP server, got error: %s", err))
		return
	}

	// Check response
	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Expected 200 OK, got status %d: %s", apiResp.StatusCode(), string(apiResp.Body)),
		)
		return
	}

	// Read back the updated resource
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)

	// Trigger a read to get the full updated state
	readReq := resource.ReadRequest{State: resp.State}
	readResp := resource.ReadResponse{State: resp.State}
	r.Read(ctx, readReq, &readResp)
	resp.Diagnostics.Append(readResp.Diagnostics...)
	resp.State = readResp.State
}

func (r *MCPServerRegistryResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data MCPServerRegistryResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Parse UUID from state
	serverID, err := uuid.Parse(data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid ID", fmt.Sprintf("Unable to parse MCP server ID: %s", err))
		return
	}

	// Call API
	apiResp, err := r.client.DeleteInternalMcpCatalogItemWithResponse(ctx, serverID)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to delete MCP server, got error: %s", err))
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

func (r *MCPServerRegistryResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
