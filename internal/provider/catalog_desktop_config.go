package provider

import (
	"context"
	"regexp"
	"strings"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// A desktop MCP server runs on each user's device: the catalog item only
// tells the device how to start it (a command) or where it listens (a URL).
// Setting desktop_config makes the entry a `desktop_mcp` server, the third
// mode beside local_config and remote_config.

var desktopConfigAttrTypes = map[string]attr.Type{
	"command": types.StringType,
	"url":     types.StringType,
}

var catalogDesktopConfigAttrSpec = AttrSpec{
	TFName: "desktop_config", JSONName: "desktopConfig", Kind: AtomicObject, OmitOnNull: true,
	Children: []AttrSpec{
		{TFName: "command", JSONName: "command", Kind: Scalar},
		{TFName: "url", JSONName: "url", Kind: Scalar},
	},
}

func catalogDesktopConfigAttribute() schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		Optional: true,
		MarkdownDescription: "Makes this a desktop MCP server that runs on each user's device; set exactly one of `command` or `url`. " +
			"Conflicts with `local_config` and `remote_config`; adding or removing it replaces the entry. " +
			"Desktop MCP servers are alpha: they need API 1.5.0-rc.32 or later with `ARCHESTRA_BETA=true`.",
		PlanModifiers: []planmodifier.Object{objectplanmodifier.RequiresReplaceIf(desktopModeChanged,
			"Switching between a desktop MCP server and a local or remote one replaces the entry.",
			"Switching between a desktop MCP server and a local or remote one replaces the entry.")},
		Attributes: map[string]schema.Attribute{
			"command": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Command each device runs to start the server, such as `npx -y @acme/mcp-server`.",
				Validators:          []validator.String{stringvalidator.LengthAtLeast(1), trimmedStringValidator{}},
			},
			"url": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Address where the server already listens on each device, starting with `http://` or `https://`.",
				Validators: []validator.String{
					stringvalidator.RegexMatches(desktopURLRegexp, "The URL must start with http:// or https://."),
					trimmedStringValidator{},
				},
			},
		},
	}
}

// desktopConfigFromResponse reads the device instructions of a desktop MCP
// entry; null for local and remote entries.
func desktopConfigFromResponse(response *client.GetInternalMcpCatalogItemResponse) types.Object {
	j := response.JSON200
	if j == nil || string(j.ServerType) != "desktop_mcp" || j.DesktopConfig == nil {
		return types.ObjectNull(desktopConfigAttrTypes)
	}
	out, _ := types.ObjectValue(desktopConfigAttrTypes, map[string]attr.Value{
		"command": stringValueOrNull(j.DesktopConfig.Command),
		"url":     stringValueOrNull(j.DesktopConfig.Url),
	})
	return out
}

func desktopModeChanged(_ context.Context, req planmodifier.ObjectRequest, resp *objectplanmodifier.RequiresReplaceIfFuncResponse) {
	if req.State.Raw.IsNull() || req.PlanValue.IsUnknown() {
		return
	}
	resp.RequiresReplace = req.StateValue.IsNull() != req.PlanValue.IsNull()
}

// validateDesktopConfig requires exactly one of command or url.
func validateDesktopConfig(desktop types.Object) (string, bool) {
	if desktop.IsNull() || desktop.IsUnknown() {
		return "", true
	}
	values := desktop.Attributes()
	command, _ := values["command"].(types.String)
	url, _ := values["url"].(types.String)
	if command.IsUnknown() || url.IsUnknown() {
		return "", true
	}
	switch {
	case !command.IsNull() && !url.IsNull():
		return "set either command or url inside desktop_config, not both", false
	case command.IsNull() && url.IsNull():
		return "set command or url inside desktop_config", false
	}
	return "", true
}

var (
	desktopConfigPath = path.Root("desktop_config")
	desktopURLRegexp  = regexp.MustCompile(`^https?://\S+$`)
)

type trimmedStringValidator struct{}

func (trimmedStringValidator) Description(context.Context) string {
	return "Must not have leading or trailing whitespace."
}
func (v trimmedStringValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}
func (v trimmedStringValidator) ValidateString(ctx context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if !req.ConfigValue.IsNull() && !req.ConfigValue.IsUnknown() && strings.TrimSpace(req.ConfigValue.ValueString()) != req.ConfigValue.ValueString() {
		resp.Diagnostics.AddAttributeError(req.Path, "Untrimmed Value", v.Description(ctx))
	}
}
