package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// Upjet observes before creating, with no external ID. Missing IDs must mean
// absent resources, rather than UUID errors or calls with empty identifiers.
func TestInitialObserveWithoutIdentifier(t *testing.T) {
	p := New("test")()
	for _, constructor := range p.Resources(t.Context()) {
		r := constructor()
		var meta resource.MetadataResponse
		r.Metadata(t.Context(), resource.MetadataRequest{ProviderTypeName: "archestra"}, &meta)
		if meta.TypeName == "archestra_organization_settings" {
			continue
		} // Existing singleton is read/adopted.
		t.Run(meta.TypeName, func(t *testing.T) {
			var schema resource.SchemaResponse
			r.Schema(t.Context(), resource.SchemaRequest{}, &schema)
			object, ok := schema.Schema.Type().TerraformType(t.Context()).(tftypes.Object)
			if !ok {
				t.Fatal("object schema expected")
			}
			for _, id := range []any{nil, "", tftypes.UnknownValue} {
				values := map[string]tftypes.Value{}
				for name, kind := range object.AttributeTypes {
					values[name] = tftypes.NewValue(kind, nil)
				}
				values["id"] = tftypes.NewValue(tftypes.String, id)
				state := tfsdk.State{Schema: schema.Schema, Raw: tftypes.NewValue(object, values)}
				response := resource.ReadResponse{State: state}
				r.Read(t.Context(), resource.ReadRequest{State: state}, &response)
				if response.Diagnostics.HasError() {
					t.Fatal(response.Diagnostics)
				}
				if !response.State.Raw.IsNull() {
					t.Fatal("blank identifier did not remove the resource")
				}
			}
		})
	}
}
