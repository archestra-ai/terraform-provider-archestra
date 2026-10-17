package provider

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

type AgentDataSource struct{ client *client.ClientWithResponses }

func NewAgentDataSource() datasource.DataSource { return &AgentDataSource{} }
func (d *AgentDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_agent"
}
func agentLookupSchema(ctx context.Context) schema.Schema {
	var source resource.SchemaResponse
	(&AgentResource{}).Schema(ctx, resource.SchemaRequest{}, &source)
	attributes := lookupAttributes(source.Schema.Attributes)
	for name, description := range map[string]string{"permissions": "Current additional direct sharing grants; owner and inherited access are excluded.", "subagent_ids": "UUIDs of assigned local subagents.", "remote_agent_ids": "UUIDs of assigned remote-agent registrations.", "excluded_subagent_ids": "Local agents excluded from automatic access.", "excluded_connector_ids": "Knowledge connectors excluded from automatic access.", "tools": "Explicit MCP tool assignments, excluding subagents and platform-required recovery tools.", "excluded_tool_ids": "Tools excluded from automatic access.", "skills": "Current skill activation mode and allowed/excluded references.", "hooks": "Current event hooks and their file configuration."} {
		switch value := attributes[name].(type) {
		case schema.SetAttribute:
			value.MarkdownDescription = description
			attributes[name] = value
		case schema.SetNestedAttribute:
			value.MarkdownDescription = description
			attributes[name] = value
		case schema.SingleNestedAttribute:
			value.MarkdownDescription = description
			attributes[name] = value
		}
	}

	attributes["id"] = schema.StringAttribute{Optional: true, Computed: true, MarkdownDescription: "Agent UUID; configure either id or name.", Validators: []validator.String{stringvalidator.ExactlyOneOf(path.MatchRoot("name")), stringvalidator.RegexMatches(canonicalUUIDRegexp, "Use a lowercase canonical agent UUID.")}}
	attributes["name"] = schema.StringAttribute{Optional: true, Computed: true, MarkdownDescription: "Exact agent name; duplicate matches require an explicit id.", Validators: []validator.String{stringvalidator.ExactlyOneOf(path.MatchRoot("id")), stringvalidator.LengthAtLeast(1)}}
	return schema.Schema{MarkdownDescription: "Read an existing agent by ID or exact name, including its subagents, remote agents, tools, skills, hooks, knowledge access and runtime settings.", Attributes: attributes}
}
func (d *AgentDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = agentLookupSchema(ctx)
}
func (d *AgentDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.ClientWithResponses)
	if !ok {
		resp.Diagnostics.AddError("Unexpected lookup configuration", fmt.Sprintf("Expected *client.ClientWithResponses, got %T.", req.ProviderData))
		return
	}
	d.client = c
}
func (d *AgentDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var id, name types.String
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("id"), &id)...)
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("name"), &name)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if id.IsNull() || id.IsUnknown() {
		if name.IsNull() || name.IsUnknown() {
			resp.Diagnostics.AddError("Unknown agent lookup", "Configure one known id or exact name.")
			return
		}
		selected, err := d.agentIDByName(ctx, name.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Agent lookup failed", err.Error())
			return
		}
		id = types.StringValue(selected)
	}
	owner := &AgentResource{client: d.client}
	var ownerSchema resource.SchemaResponse
	owner.Schema(ctx, resource.SchemaRequest{}, &ownerSchema)
	values := map[string]tftypes.Value{}
	for key, attribute := range ownerSchema.Schema.Attributes {
		values[key] = tftypes.NewValue(attribute.GetType().TerraformType(ctx), nil)
	}
	values["id"] = tftypes.NewValue(tftypes.String, id.ValueString())
	ownerState := tfsdk.State{Schema: ownerSchema.Schema, Raw: tftypes.NewValue(ownerSchema.Schema.Type().TerraformType(ctx), values)}
	read := resource.ReadResponse{State: ownerState}
	owner.Read(ctx, resource.ReadRequest{State: ownerState}, &read)
	resp.Diagnostics.Append(read.Diagnostics...)
	if resp.Diagnostics.HasError() {
		return
	}
	if read.State.Raw.IsNull() {
		resp.Diagnostics.AddError("Agent not found", "The selected agent no longer exists or is not visible.")
		return
	}
	if err := read.State.Raw.As(&values); err != nil {
		resp.Diagnostics.AddError("Invalid agent lookup state", err.Error())
		return
	}
	if !name.IsNull() && !name.IsUnknown() {
		var observed string
		if err := values["name"].As(&observed); err != nil {
			resp.Diagnostics.AddError("Invalid agent name", err.Error())
			return
		}
		if observed != name.ValueString() {
			resp.Diagnostics.AddError("Agent changed during lookup", "Refresh the configuration or select the agent by id.")
			return
		}
	}
	projected := map[string]tftypes.Value{}
	for name, attribute := range resp.State.Schema.GetAttributes() {
		value, present := values[name]
		if !present || !value.Type().Equal(attribute.GetType().TerraformType(ctx)) {
			resp.Diagnostics.AddError("Agent lookup schema mismatch", "The resource read did not provide the expected field "+name+".")
			return
		}
		projected[name] = value
	}
	resp.State.Raw = tftypes.NewValue(resp.State.Schema.Type().TerraformType(ctx), projected)
}

func (d *AgentDataSource) agentIDByName(ctx context.Context, name string) (string, error) {
	limit, offset := 100, 0
	kind := client.GetAgentsParamsAgentType("agent")
	includeBuiltIn := true
	ids := map[string]bool{}
	for {
		response, err := d.client.GetAgentsWithResponse(ctx, &client.GetAgentsParams{Name: &name, AgentType: &kind, IncludeBuiltIn: &includeBuiltIn, Limit: &limit, Offset: &offset})
		if err != nil {
			return "", err
		}
		if response.StatusCode() != 200 {
			return "", fmt.Errorf("agent listing returned API status %d", response.StatusCode())
		}
		var page struct {
			Data []struct {
				ID        string `json:"id"`
				Name      string `json:"name"`
				AgentType string `json:"agentType"`
			} `json:"data"`
			Pagination struct {
				HasNext bool `json:"hasNext"`
			} `json:"pagination"`
		}
		if err := json.Unmarshal(response.Body, &page); err != nil {
			return "", err
		}
		for _, row := range page.Data {
			if row.Name == name && row.AgentType == "agent" {
				ids[row.ID] = true
			}
		}
		if !page.Pagination.HasNext {
			break
		}
		if len(page.Data) == 0 {
			return "", fmt.Errorf("agent listing returned an empty page with more results")
		}
		offset += limit
	}
	if len(ids) == 0 {
		return "", fmt.Errorf("no visible agent has the exact name %q; select an existing id or add a dependency if it is created in this configuration", name)
	}
	if len(ids) > 1 {
		return "", fmt.Errorf("multiple visible agents have the exact name %q; select one by id", name)
	}
	for id := range ids {
		return id, nil
	}
	return "", nil
}
