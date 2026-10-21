package provider

import (
	"context"
	"encoding/json"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

type schemaResource interface {
	Schema(context.Context, resource.SchemaRequest, *resource.SchemaResponse)
}

// Preserve existing bindings and secret values while dropping retired schema
// fields. This transforms state only; it never calls the API or deletes objects.
func v3StateUpgrade(r schemaResource, agent bool, transform ...func(map[string]json.RawMessage)) map[int64]resource.StateUpgrader {
	return map[int64]resource.StateUpgrader{0: {StateUpgrader: func(ctx context.Context, req resource.UpgradeStateRequest, resp *resource.UpgradeStateResponse) {
		if req.RawState == nil || len(req.RawState.JSON) == 0 {
			resp.Diagnostics.AddError("Missing Previous State", "Expected JSON state from provider v2.")
			return
		}
		var values map[string]json.RawMessage
		if err := json.Unmarshal(req.RawState.JSON, &values); err != nil {
			resp.Diagnostics.AddError("Invalid Previous State", "Unable to decode provider v2 state.")
			return
		}
		var id string
		if err := json.Unmarshal(values["id"], &id); err != nil || id == "" {
			resp.Diagnostics.AddError("Invalid Previous State", "Existing string ID is required; restore the state backup before retrying.")
			return
		}
		if agent {
			if old := values["built_in_agent_config"]; len(old) > 0 && string(old) != "null" {
				encoded, err := json.Marshal(string(old))
				if err != nil {
					resp.Diagnostics.AddError("State Upgrade Failed", "Unable to preserve built-in metadata.")
					return
				}
				values["built_in_agent_config"] = encoded
			}
		}
		for _, convert := range transform {
			convert(values)
		}
		var current resource.SchemaResponse
		r.Schema(ctx, resource.SchemaRequest{}, &current)
		resp.Diagnostics.Append(current.Diagnostics...)
		if resp.Diagnostics.HasError() {
			return
		}
		encoded, err := json.Marshal(values)
		if err == nil {
			encoded, err = projectStateJSON(encoded, current.Schema.Type().TerraformType(ctx))
		}
		if err != nil {
			resp.Diagnostics.AddError("State Upgrade Failed", "Previous state does not match the supported v2 schema; restore the backup and inspect the source provider version.")
			return
		}
		upgraded, err := (tfprotov6.RawState{JSON: encoded}).Unmarshal(current.Schema.Type().TerraformType(ctx))
		if err != nil {
			resp.Diagnostics.AddError("State Upgrade Failed", "Previous state has incompatible values; restore the backup and inspect the source provider version.")
			return
		}
		resp.State.Schema = current.Schema
		resp.State.Raw = upgraded
	}}}
}

func projectStateJSON(raw json.RawMessage, kind tftypes.Type) (json.RawMessage, error) {
	if string(raw) == "null" {
		return raw, nil
	}
	switch t := kind.(type) {
	case tftypes.Object:
		var prior map[string]json.RawMessage
		if err := json.Unmarshal(raw, &prior); err != nil {
			return nil, err
		}
		out := make(map[string]json.RawMessage, len(t.AttributeTypes))
		for name, child := range t.AttributeTypes {
			value, ok := prior[name]
			if !ok {
				value = json.RawMessage("null")
			}
			var err error
			out[name], err = projectStateJSON(value, child)
			if err != nil {
				return nil, err
			}
		}
		return json.Marshal(out)
	case tftypes.List:
		return projectStateSequence(raw, t.ElementType)
	case tftypes.Set:
		return projectStateSequence(raw, t.ElementType)
	case tftypes.Map:
		var values map[string]json.RawMessage
		if err := json.Unmarshal(raw, &values); err != nil {
			return nil, err
		}
		for k, v := range values {
			var err error
			values[k], err = projectStateJSON(v, t.ElementType)
			if err != nil {
				return nil, err
			}
		}
		return json.Marshal(values)
	default:
		return raw, nil
	}
}
func projectStateSequence(raw json.RawMessage, kind tftypes.Type) (json.RawMessage, error) {
	var values []json.RawMessage
	if err := json.Unmarshal(raw, &values); err != nil {
		return nil, err
	}
	for i, v := range values {
		var err error
		values[i], err = projectStateJSON(v, kind)
		if err != nil {
			return nil, err
		}
	}
	return json.Marshal(values)
}

func (r *AgentResource) UpgradeState(context.Context) map[int64]resource.StateUpgrader {
	return v3StateUpgrade(r, true)
}
func (r *McpGatewayResource) UpgradeState(context.Context) map[int64]resource.StateUpgrader {
	return v3StateUpgrade(r, false)
}
func (r *MCPServerRegistryResource) UpgradeState(context.Context) map[int64]resource.StateUpgrader {
	return v3StateUpgrade(r, false)
}
func (r *TeamResource) UpgradeState(context.Context) map[int64]resource.StateUpgrader {
	return v3StateUpgrade(r, false)
}
func (r *OrganizationSettingsResource) UpgradeState(context.Context) map[int64]resource.StateUpgrader {
	return v3StateUpgrade(r, false)
}
func (r *IdentityProviderResource) UpgradeState(context.Context) map[int64]resource.StateUpgrader {
	return v3StateUpgrade(r, false)
}

// v2 mixed discovered modalities with managed inputs. Preserve the discovered
// values as metadata, leaving undeclared inputs unmanaged during upgrade.
func (r *LlmModelResource) UpgradeState(context.Context) map[int64]resource.StateUpgrader {
	return v3StateUpgrade(r, false, func(values map[string]json.RawMessage) {
		for _, kind := range []string{"input", "output"} {
			name := kind + "_modalities"
			if old := values[name]; len(old) > 0 {
				values["effective_"+name] = old
			}
			values[name] = json.RawMessage("null")
		}
	})
}
