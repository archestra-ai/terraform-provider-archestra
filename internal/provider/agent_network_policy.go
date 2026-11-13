package provider

import (
	"context"
	"encoding/json"
	"regexp"

	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// The API normalizes an agent's egress policy before storing it: a missing
// egress mode means "restricted", and the domain preset and domain allowlist
// apply only to "restricted" (the CIDR allowlist to everything but "off").
// The schema defaults mirror that normalization and validation refuses what
// the API would silently drop, so the stored policy always equals the plan.

var networkPolicyAttrTypes = map[string]attr.Type{
	"egress_mode":     types.StringType,
	"domain_preset":   types.StringType,
	"allowed_domains": types.SetType{ElemType: types.StringType},
	"allowed_cidrs":   types.SetType{ElemType: types.StringType},
}

var agentNetworkPolicyAttrSpec = AttrSpec{
	TFName: "network_policy", JSONName: "networkPolicy", Kind: AtomicObject,
	Children: []AttrSpec{
		{TFName: "egress_mode", JSONName: "egressMode", Kind: Scalar},
		{TFName: "domain_preset", JSONName: "domainPreset", Kind: Scalar},
		{TFName: "allowed_domains", JSONName: "allowedDomains", Kind: Set},
		{TFName: "allowed_cidrs", JSONName: "allowedCidrs", Kind: Set},
	},
}

func agentNetworkPolicyAttribute() schema.SingleNestedAttribute {
	emptySet := types.SetValueMust(types.StringType, []attr.Value{})
	return schema.SingleNestedAttribute{
		Optional: true,
		MarkdownDescription: "Egress override for this agent's Agent Runtime and Code Sandbox. Omit to inherit the environment's policy; " +
			"removing it later returns the agent to that policy. Requires API 1.5.0-rc.20 or later.",
		Attributes: map[string]schema.Attribute{
			"egress_mode": schema.StringAttribute{
				Optional: true, Computed: true, Default: stringdefault.StaticString("restricted"),
				MarkdownDescription: "`off` (no egress), `restricted` (allowlists only, the default) or `unrestricted`.",
				Validators:          []validator.String{stringvalidator.OneOf("off", "restricted", "unrestricted")},
			},
			"domain_preset": schema.StringAttribute{
				Optional: true, Computed: true, Default: stringdefault.StaticString("none"),
				MarkdownDescription: "Extra domains allowed in `restricted` mode: `none` (default), `common_dependencies` or `package_managers`.",
				Validators:          []validator.String{stringvalidator.OneOf("none", "common_dependencies", "package_managers")},
			},
			"allowed_domains": schema.SetAttribute{
				Optional: true, Computed: true, ElementType: types.StringType, Default: setdefault.StaticValue(emptySet),
				MarkdownDescription: "Lowercase domains such as `api.example.com` or `*.example.com`, allowed in `restricted` mode.",
				Validators: []validator.Set{
					setvalidator.SizeAtMost(500),
					setvalidator.ValueStringsAre(stringvalidator.LengthBetween(1, 253), stringvalidator.RegexMatches(
						networkPolicyDomainRegexp, "Use a lowercase domain such as api.example.com or *.example.com.")),
				},
			},
			"allowed_cidrs": schema.SetAttribute{
				Optional: true, Computed: true, ElementType: types.StringType, Default: setdefault.StaticValue(emptySet),
				MarkdownDescription: "CIDR ranges such as `203.0.113.0/24` or `2001:db8::/32`, allowed unless `egress_mode` is `off`.",
				Validators: []validator.Set{
					setvalidator.SizeAtMost(500),
					setvalidator.ValueStringsAre(stringvalidator.RegexMatches(networkPolicyCIDRRegexp, "Use a CIDR such as 203.0.113.0/24 without surrounding spaces.")),
				},
			},
		},
	}
}

// validateAgentNetworkPolicy rejects combinations the API would normalize away.
func validateAgentNetworkPolicy(ctx context.Context, config tfsdk.Config, diags *diag.Diagnostics) {
	var policy types.Object
	diags.Append(config.GetAttribute(ctx, path.Root("network_policy"), &policy)...)
	if diags.HasError() || policy.IsNull() || policy.IsUnknown() {
		return
	}
	values := policy.Attributes()
	mode, ok := values["egress_mode"].(types.String)
	if !ok || mode.IsUnknown() {
		return
	}
	egress := "restricted"
	if !mode.IsNull() {
		egress = mode.ValueString()
	}
	base := path.Root("network_policy")
	if egress != "restricted" {
		if preset, ok := values["domain_preset"].(types.String); ok && !preset.IsNull() && !preset.IsUnknown() && preset.ValueString() != "none" {
			diags.AddAttributeError(base.AtName("domain_preset"), "Domain Preset Needs Restricted Egress",
				`domain_preset applies only when egress_mode = "restricted"; the API stores "none" otherwise.`)
		}
		if domains, ok := values["allowed_domains"].(types.Set); ok && !domains.IsNull() && !domains.IsUnknown() && len(domains.Elements()) > 0 {
			diags.AddAttributeError(base.AtName("allowed_domains"), "Allowed Domains Need Restricted Egress",
				`allowed_domains applies only when egress_mode = "restricted"; the API drops them otherwise.`)
		}
	}
	if egress == "off" {
		if cidrs, ok := values["allowed_cidrs"].(types.Set); ok && !cidrs.IsNull() && !cidrs.IsUnknown() && len(cidrs.Elements()) > 0 {
			diags.AddAttributeError(base.AtName("allowed_cidrs"), "Allowed CIDRs Need Egress",
				`allowed_cidrs has no effect when egress_mode = "off"; the API drops them.`)
		}
	}
}

// networkPolicyFromResponse reads an agent's stored egress override; null
// means it inherits the environment's policy (and on releases without it).
func networkPolicyFromResponse(body []byte, diags *diag.Diagnostics) types.Object {
	var wire struct {
		NetworkPolicy *struct {
			EgressMode     string   `json:"egressMode"`
			DomainPreset   string   `json:"domainPreset"`
			AllowedDomains []string `json:"allowedDomains"`
			AllowedCidrs   []string `json:"allowedCidrs"`
		} `json:"networkPolicy"`
	}
	if err := json.Unmarshal(body, &wire); err != nil {
		diags.AddError("Invalid Agent Response", "Unexpected networkPolicy value: "+err.Error())
		return types.ObjectNull(networkPolicyAttrTypes)
	}
	if wire.NetworkPolicy == nil {
		return types.ObjectNull(networkPolicyAttrTypes)
	}
	p := wire.NetworkPolicy
	domains, d := types.SetValueFrom(context.Background(), types.StringType, nonNilStrings(p.AllowedDomains))
	diags.Append(d...)
	cidrs, d := types.SetValueFrom(context.Background(), types.StringType, nonNilStrings(p.AllowedCidrs))
	diags.Append(d...)
	out, d := types.ObjectValue(networkPolicyAttrTypes, map[string]attr.Value{
		"egress_mode":     types.StringValue(p.EgressMode),
		"domain_preset":   types.StringValue(p.DomainPreset),
		"allowed_domains": domains,
		"allowed_cidrs":   cidrs,
	})
	diags.Append(d...)
	return out
}

var (
	networkPolicyDomainRegexp = regexp.MustCompile(`^(\*\.)?[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?(\.[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?)+$`)
	networkPolicyCIDRRegexp   = regexp.MustCompile(`^\S+/[0-9]{1,3}$`)
)

func nonNilStrings(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}
