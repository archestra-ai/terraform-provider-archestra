package provider

import (
	"fmt"

	ds "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	rs "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

// Lookups expose the same types and descriptions as their owning resource.
func lookupAttributes(source map[string]rs.Attribute) map[string]ds.Attribute {
	result := make(map[string]ds.Attribute, len(source))
	for name, attribute := range source {
		result[name] = lookupAttribute(attribute)
	}
	return result
}
func lookupAttribute(source rs.Attribute) ds.Attribute {
	description, sensitive := source.GetMarkdownDescription(), source.IsSensitive()
	switch value := source.(type) {
	case rs.ListAttribute:
		return ds.ListAttribute{Computed: true, MarkdownDescription: description, Sensitive: sensitive, CustomType: value.CustomType, ElementType: value.ElementType}
	case rs.SetAttribute:
		return ds.SetAttribute{Computed: true, MarkdownDescription: description, Sensitive: sensitive, CustomType: value.CustomType, ElementType: value.ElementType}
	case rs.MapAttribute:
		return ds.MapAttribute{Computed: true, MarkdownDescription: description, Sensitive: sensitive, CustomType: value.CustomType, ElementType: value.ElementType}
	case rs.ObjectAttribute:
		return ds.ObjectAttribute{Computed: true, MarkdownDescription: description, Sensitive: sensitive, CustomType: value.CustomType, AttributeTypes: value.AttributeTypes}

	case rs.SingleNestedAttribute:
		return ds.SingleNestedAttribute{Computed: true, MarkdownDescription: description, Sensitive: sensitive, Attributes: lookupAttributes(value.Attributes), CustomType: value.CustomType}
	case rs.SetNestedAttribute:
		return ds.SetNestedAttribute{Computed: true, MarkdownDescription: description, Sensitive: sensitive, NestedObject: ds.NestedAttributeObject{Attributes: lookupAttributes(value.NestedObject.Attributes)}, CustomType: value.CustomType}
	case rs.ListNestedAttribute:
		return ds.ListNestedAttribute{Computed: true, MarkdownDescription: description, Sensitive: sensitive, NestedObject: ds.NestedAttributeObject{Attributes: lookupAttributes(value.NestedObject.Attributes)}, CustomType: value.CustomType}
	case rs.MapNestedAttribute:
		return ds.MapNestedAttribute{Computed: true, MarkdownDescription: description, Sensitive: sensitive, NestedObject: ds.NestedAttributeObject{Attributes: lookupAttributes(value.NestedObject.Attributes)}, CustomType: value.CustomType}
	}
	switch value := source.GetType().(type) {
	case basetypes.StringTypable:
		return ds.StringAttribute{Computed: true, MarkdownDescription: description, Sensitive: sensitive, CustomType: value}
	case basetypes.BoolTypable:
		return ds.BoolAttribute{Computed: true, MarkdownDescription: description, Sensitive: sensitive, CustomType: value}
	case basetypes.Int64Typable:
		return ds.Int64Attribute{Computed: true, MarkdownDescription: description, Sensitive: sensitive, CustomType: value}
	case basetypes.Int32Typable:
		return ds.Int32Attribute{Computed: true, MarkdownDescription: description, Sensitive: sensitive, CustomType: value}
	case basetypes.Float64Typable:
		return ds.Float64Attribute{Computed: true, MarkdownDescription: description, Sensitive: sensitive, CustomType: value}
	case basetypes.Float32Typable:
		return ds.Float32Attribute{Computed: true, MarkdownDescription: description, Sensitive: sensitive, CustomType: value}
	case basetypes.NumberTypable:
		return ds.NumberAttribute{Computed: true, MarkdownDescription: description, Sensitive: sensitive, CustomType: value}
	case basetypes.DynamicTypable:
		return ds.DynamicAttribute{Computed: true, MarkdownDescription: description, Sensitive: sensitive, CustomType: value}
	}
	panic(fmt.Sprintf("unsupported lookup attribute type %T", source.GetType()))
}
