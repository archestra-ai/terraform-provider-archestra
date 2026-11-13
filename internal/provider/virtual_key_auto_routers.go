package provider

import (
	"context"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func virtualKeyAutoRouterIDsAttribute() schema.SetAttribute {
	return schema.SetAttribute{
		Optional:            true,
		ElementType:         types.StringType,
		Validators:          []validator.Set{setvalidator.SizeAtMost(500), setvalidator.ValueStringsAre(stringvalidator.RegexMatches(canonicalUUIDRegexp, "Use lowercase canonical auto router UUIDs."))},
		MarkdownDescription: "ALPHA. `archestra_auto_router` IDs the key may call by name; an empty set clears them, and removing the attribute leaves them unmanaged. Standard keys only. Requires API 1.5.0-rc.29 or later with auto routing enabled; read back from each router's `virtual_api_key_ids`.",
	}
}

// readVirtualKeyAutoRouters refreshes managed router grants. The key's own
// response does not report them, so they are collected from the routers.
// Unmanaged grants (null) never call the auto-routing API, which keeps keys
// working on releases or deployments without auto routing.
func readVirtualKeyAutoRouters(ctx context.Context, api *client.ClientWithResponses, data *VirtualAPIKeyResourceModel, diags *diag.Diagnostics) {
	if data.AutoRouterIDs.IsNull() || data.AutoRouterIDs.IsUnknown() || diags.HasError() {
		return
	}
	routers, err := client.NewAutoRouterClient(api)
	if err != nil {
		diags.AddError("Auto Router Client Configuration Failed", err.Error())
		return
	}
	list, err := routers.List(ctx)
	if err != nil {
		diags.AddError("Auto Router Grants Read Failed", err.Error())
		return
	}
	data.AutoRouterIDs = virtualKeyAutoRouterIDs(ctx, list, data.ID.ValueString(), diags)
}

func virtualKeyAutoRouterIDs(ctx context.Context, routers []client.AutoRouter, keyID string, diags *diag.Diagnostics) types.Set {
	ids := []string{}
	for _, router := range routers {
		for _, granted := range router.VirtualAPIKeyIDs {
			if granted == keyID {
				ids = append(ids, router.ID)
				break
			}
		}
	}
	value, d := types.SetValueFrom(ctx, types.StringType, ids)
	diags.Append(d...)
	return value
}
