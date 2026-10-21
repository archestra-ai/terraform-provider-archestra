package provider

import (
	"encoding/json"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var connectionBaseURLType = types.ObjectType{AttrTypes: map[string]attr.Type{"url": types.StringType, "description": types.StringType, "is_default": types.BoolType, "visible": types.BoolType}}
var connectionBaseURLSpecs = []AttrSpec{{TFName: "url", JSONName: "url", Kind: Scalar}, {TFName: "description", JSONName: "description", Kind: Scalar}, {TFName: "is_default", JSONName: "isDefault", Kind: Scalar}, {TFName: "visible", JSONName: "visible", Kind: Scalar}}

func connectionBaseURLSchema() schema.ListNestedAttribute {
	return schema.ListNestedAttribute{Optional: true, Computed: true, PlanModifiers: []planmodifier.List{listplanmodifier.UseStateForUnknown()}, MarkdownDescription: "Connection endpoints shown to clients.", NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
		"url":         schema.StringAttribute{Required: true, MarkdownDescription: "Endpoint URL."},
		"description": schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""), MarkdownDescription: "Endpoint description."},
		"is_default":  schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), MarkdownDescription: "Whether this is the default endpoint."},
		"visible":     schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(true), MarkdownDescription: "Whether clients display this endpoint."},
	}}}
}
func flattenConnectionBaseURLs(target *types.List, raw json.RawMessage, diags *diag.Diagnostics) {
	if len(raw) == 0 || string(raw) == "null" {
		*target = types.ListNull(connectionBaseURLType)
		return
	}
	var rows []struct {
		URL         string `json:"url"`
		Description string `json:"description"`
		IsDefault   bool   `json:"isDefault"`
		Visible     *bool  `json:"visible"`
	}
	if err := json.Unmarshal(raw, &rows); err != nil {
		diags.AddError("Invalid Connection URLs", "Unable to decode connection endpoints.")
		return
	}
	values := make([]attr.Value, 0, len(rows))
	for _, row := range rows {
		visible := row.Visible == nil || *row.Visible
		value, ds := types.ObjectValue(connectionBaseURLType.AttrTypes, map[string]attr.Value{"url": types.StringValue(row.URL), "description": types.StringValue(row.Description), "is_default": types.BoolValue(row.IsDefault), "visible": types.BoolValue(visible)})
		diags.Append(ds...)
		values = append(values, value)
	}
	value, ds := types.ListValue(connectionBaseURLType, values)
	diags.Append(ds...)
	*target = value
}
