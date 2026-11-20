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

func virtualKeyModelPoolIDsAttribute() schema.SetAttribute {
	return schema.SetAttribute{
		Optional:            true,
		ElementType:         types.StringType,
		Validators:          []validator.Set{setvalidator.SizeAtMost(500), setvalidator.ValueStringsAre(stringvalidator.RegexMatches(canonicalUUIDRegexp, "Use lowercase canonical model pool UUIDs."))},
		MarkdownDescription: "`archestra_model_pool` IDs the key may call by name; an empty set clears them, and removing the attribute leaves them unmanaged. Standard keys only; a key with pools needs no provider keys. Read back from each pool's `virtualApiKeyIds`.",
	}
}

// readVirtualKeyModelPools refreshes managed pool grants. The key's own
// response does not report them, so they are collected from the pools.
// Unmanaged grants (null) never call the model pool API.
func readVirtualKeyModelPools(ctx context.Context, api *client.ClientWithResponses, data *VirtualAPIKeyResourceModel, diags *diag.Diagnostics) {
	if data.ModelPoolIDs.IsNull() || data.ModelPoolIDs.IsUnknown() || diags.HasError() {
		return
	}
	pools, err := client.NewModelPoolClient(api)
	if err != nil {
		diags.AddError("Model Pool Client Configuration Failed", err.Error())
		return
	}
	list, err := pools.List(ctx)
	if err != nil {
		diags.AddError("Model Pool Grants Read Failed", err.Error())
		return
	}
	ids := []string{}
	for _, pool := range list {
		for _, granted := range pool.VirtualAPIKeyIDs {
			if granted == data.ID.ValueString() {
				ids = append(ids, pool.ID)
				break
			}
		}
	}
	value, d := types.SetValueFrom(ctx, types.StringType, ids)
	diags.Append(d...)
	data.ModelPoolIDs = value
}
