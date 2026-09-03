package provider

import (
	"context"
	"encoding/json"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// runtimeSchema returns the `runtime` nested attribute for `archestra_agent`.
// Kept beside its model/flatten/encode
// helpers so a wire-shape change touches one file.
func runtimeSchema() schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		Optional: true,
		MarkdownDescription: "Dedicated runtime used for interactive, delegated, and long-running work. " +
			"Requires an Agent Runtime backend on the platform (e.g. the Kubernetes orchestrator).",
		Attributes: map[string]schema.Attribute{
			"image": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Container image reference the run starts from",
			},
			"command": schema.ListAttribute{
				Optional:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Container command override; omit to use the image's default entrypoint",
			},
			"inference_protocol": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Wire protocol the image uses to reach Archestra's model router: `openai_responses`, `openai_chat`, or `anthropic`",
				Validators: []validator.String{
					stringvalidator.OneOf("openai_responses", "openai_chat", "anthropic"),
				},
			},
			"backend": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("kubernetes"),
				MarkdownDescription: "Runtime backend the run is scheduled on. Only `kubernetes` today (default).",
				Validators:          []validator.String{stringvalidator.OneOf("kubernetes")},
			},
			"steer_mode": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "How a steer message reaches the running process: `pipe` (runtime-agent FIFO) or `tmux_keys` (typed into the tmux session, for CLIs that own their input loop)",
				Validators:          []validator.String{stringvalidator.OneOf("pipe", "tmux_keys")},
			},
			"privileged": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
				MarkdownDescription: "Whether the container runs privileged (default `false`)",
			},
			"resources": schema.SingleNestedAttribute{
				Optional:            true,
				MarkdownDescription: "Kubernetes-style resource requests/limits for the run",
				Attributes: map[string]schema.Attribute{
					"cpu_request":    schema.StringAttribute{Optional: true, MarkdownDescription: "CPU request, e.g. `2`"},
					"memory_request": schema.StringAttribute{Optional: true, MarkdownDescription: "Memory request, e.g. `4Gi`"},
					"cpu_limit":      schema.StringAttribute{Optional: true, MarkdownDescription: "CPU limit; omitted by default so an agent loop is never throttled mid-turn"},
					"memory_limit":   schema.StringAttribute{Optional: true, MarkdownDescription: "Memory limit, e.g. `20Gi`"},
				},
			},
			"environment": schema.ListNestedAttribute{
				Optional:            true,
				MarkdownDescription: "Plain (non-secret) environment variables injected into the run",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"key":   schema.StringAttribute{Required: true, MarkdownDescription: "Environment variable name"},
						"value": schema.StringAttribute{Required: true, MarkdownDescription: "Environment variable value"},
					},
				},
			},
			"credentials": schema.ListNestedAttribute{
				Optional:            true,
				MarkdownDescription: "Runtime credentials the run needs, injected as environment variables. Values are supplied through `archestra_runtime_credential` (organization scope) or by each user (personal scope) — never inline here.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"key": schema.StringAttribute{
							Required:            true,
							MarkdownDescription: "Environment variable the resolved value is injected under (`A-Z`, `0-9`, `_`)",
						},
						"scope": schema.StringAttribute{
							Required:            true,
							MarkdownDescription: "`shared` (one organization-level value serves every user) or `per_user` (each invoking user supplies their own)",
							Validators:          []validator.String{stringvalidator.OneOf("shared", "per_user")},
						},
						"credential_id": schema.StringAttribute{
							Optional:            true,
							MarkdownDescription: "Stable connection identifier — declarations sharing it (and scope) reuse one stored secret. Pass `archestra_runtime_credential.<n>.key`.",
						},
						"label": schema.StringAttribute{
							Required:            true,
							MarkdownDescription: "Human label shown when prompting a user to supply the credential",
						},
						"description": schema.StringAttribute{
							Optional:            true,
							MarkdownDescription: "How to obtain the credential, e.g. \"Run `claude setup-token` and paste the result\"",
						},
						"required": schema.BoolAttribute{
							Required:            true,
							MarkdownDescription: "Whether a run is blocked until this credential resolves",
						},
					},
				},
			},
			"ttl_hours": schema.Int64Attribute{
				Optional:            true,
				MarkdownDescription: "Hard lifetime cap for a run in hours (1–720); omit for no cap",
			},
			"max_cost_usd": schema.Int64Attribute{
				Optional:            true,
				MarkdownDescription: "Hard LLM spend ceiling in USD for the short-lived virtual key backing one run; omit for no ceiling",
			},
			"idle_timeout_minutes": schema.Int64Attribute{
				Optional:            true,
				MarkdownDescription: "Minutes of inactivity after which the run is paused (1–1440); omit for no idle timeout",
			},
		},
	}
}

// AgentRuntimeModel mirrors the `runtime` nested attribute.
type AgentRuntimeModel struct {
	Image             types.String                `tfsdk:"image"`
	Command           types.List                  `tfsdk:"command"`
	InferenceProtocol types.String                `tfsdk:"inference_protocol"`
	Backend           types.String                `tfsdk:"backend"`
	SteerMode         types.String                `tfsdk:"steer_mode"`
	Privileged        types.Bool                  `tfsdk:"privileged"`
	Resources         *AgentRuntimeResourcesModel `tfsdk:"resources"`
	// Environment and Credentials are types.List (of the entry object types
	// below) rather than Go slices: the framework must be able to hand the
	// model an UNKNOWN list — e.g. while a same-plan referenced resource is
	// still pending import — and a raw slice cannot represent that (the
	// "Received unknown value" conversion panic).
	Environment        types.List  `tfsdk:"environment"`
	Credentials        types.List  `tfsdk:"credentials"`
	TtlHours           types.Int64 `tfsdk:"ttl_hours"`
	MaxCostUsd         types.Int64 `tfsdk:"max_cost_usd"`
	IdleTimeoutMinutes types.Int64 `tfsdk:"idle_timeout_minutes"`
}

type AgentRuntimeResourcesModel struct {
	CpuRequest    types.String `tfsdk:"cpu_request"`
	MemoryRequest types.String `tfsdk:"memory_request"`
	CpuLimit      types.String `tfsdk:"cpu_limit"`
	MemoryLimit   types.String `tfsdk:"memory_limit"`
}

type AgentRuntimeEnvEntryModel struct {
	Key   types.String `tfsdk:"key"`
	Value types.String `tfsdk:"value"`
}

type AgentRuntimeCredentialModel struct {
	Key          types.String `tfsdk:"key"`
	Scope        types.String `tfsdk:"scope"`
	CredentialId types.String `tfsdk:"credential_id"`
	Label        types.String `tfsdk:"label"`
	Description  types.String `tfsdk:"description"`
	Required     types.Bool   `tfsdk:"required"`
}

var runtimeEnvEntryType = types.ObjectType{AttrTypes: map[string]attr.Type{
	"key":   types.StringType,
	"value": types.StringType,
}}

var runtimeCredentialType = types.ObjectType{AttrTypes: map[string]attr.Type{
	"key":           types.StringType,
	"scope":         types.StringType,
	"credential_id": types.StringType,
	"label":         types.StringType,
	"description":   types.StringType,
	"required":      types.BoolType,
}}

// runtimeAttrSpec is the Children spec of the `runtime`
// AtomicObject entry in agentAttrSpec.
var runtimeAttrSpec = []AttrSpec{
	{TFName: "image", JSONName: "image", Kind: Scalar},
	{TFName: "command", JSONName: "command", Kind: List},
	{TFName: "inference_protocol", JSONName: "inferenceProtocol", Kind: Scalar},
	{TFName: "backend", JSONName: "backend", Kind: Scalar},
	{TFName: "steer_mode", JSONName: "steerMode", Kind: Scalar},
	{TFName: "privileged", JSONName: "privileged", Kind: Scalar},
	{
		TFName: "resources", JSONName: "resources", Kind: AtomicObject,
		Children: []AttrSpec{
			{TFName: "cpu_request", JSONName: "cpuRequest", Kind: Scalar},
			{TFName: "memory_request", JSONName: "memoryRequest", Kind: Scalar},
			{TFName: "cpu_limit", JSONName: "cpuLimit", Kind: Scalar},
			{TFName: "memory_limit", JSONName: "memoryLimit", Kind: Scalar},
		},
	},
	{
		TFName: "environment", JSONName: "environment", Kind: List,
		Children: []AttrSpec{
			{TFName: "key", JSONName: "key", Kind: Scalar},
			{TFName: "value", JSONName: "value", Kind: Scalar},
		},
	},
	{
		TFName: "credentials", JSONName: "credentials", Kind: List,
		Children: []AttrSpec{
			{TFName: "key", JSONName: "key", Kind: Scalar},
			{TFName: "scope", JSONName: "scope", Kind: Scalar},
			{TFName: "credential_id", JSONName: "credentialId", Kind: Scalar},
			{TFName: "label", JSONName: "label", Kind: Scalar},
			{TFName: "description", JSONName: "description", Kind: Scalar},
			{TFName: "required", JSONName: "required", Kind: Scalar},
		},
	},
	{TFName: "ttl_hours", JSONName: "ttlHours", Kind: Scalar},
	{TFName: "max_cost_usd", JSONName: "maxCostUsd", Kind: Scalar},
	{TFName: "idle_timeout_minutes", JSONName: "idleTimeoutMinutes", Kind: Scalar},
}

// encodeRuntime fills in the object keys the backend zod schema
// declares as required-but-nullable. The generic AtomicObject encoder drops
// null sub-attributes entirely, but `AgentRuntimeSchema` rejects
// a missing `command`/`resources`/`environment`/`credentials`/`ttlHours`/
// `idleTimeoutMinutes` key ("received undefined") while accepting an explicit
// null. `maxCostUsd` is genuinely optional and stays omitted when unset.
func encodeRuntime(v any) any {
	m, ok := v.(map[string]any)
	if !ok || m == nil {
		return v
	}
	for _, key := range []string{"command", "resources", "environment", "credentials", "ttlHours", "idleTimeoutMinutes"} {
		if _, present := m[key]; !present {
			m[key] = nil
		}
	}
	return m
}

// runtimeAPI mirrors the wire shape of an agent's `runtime` payload so the
// create, read, and update response variants share one decoder.
type runtimeAPI struct {
	Image             string    `json:"image"`
	Command           *[]string `json:"command"`
	InferenceProtocol string    `json:"inferenceProtocol"`
	Backend           string    `json:"backend"`
	SteerMode         string    `json:"steerMode"`
	Privileged        bool      `json:"privileged"`
	Resources         *struct {
		CpuRequest    *string `json:"cpuRequest"`
		MemoryRequest *string `json:"memoryRequest"`
		CpuLimit      *string `json:"cpuLimit"`
		MemoryLimit   *string `json:"memoryLimit"`
	} `json:"resources"`
	Environment *[]struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	} `json:"environment"`
	Credentials *[]struct {
		Key          string  `json:"key"`
		Scope        string  `json:"scope"`
		CredentialId *string `json:"credentialId"`
		Label        string  `json:"label"`
		Description  *string `json:"description"`
		Required     bool    `json:"required"`
	} `json:"credentials"`
	TtlHours           *int64 `json:"ttlHours"`
	MaxCostUsd         *int64 `json:"maxCostUsd"`
	IdleTimeoutMinutes *int64 `json:"idleTimeoutMinutes"`
}

// runtimeFromResponse parses `runtime` out of a raw
// agent response body into the nested model. Returns nil when the agent has
// no dedicated runtime configured.
func runtimeFromResponse(ctx context.Context, responseBody []byte, diags *diag.Diagnostics) *AgentRuntimeModel {
	var raw struct {
		Runtime *runtimeAPI `json:"runtime"`
	}
	if err := json.Unmarshal(responseBody, &raw); err != nil || raw.Runtime == nil {
		return nil
	}
	api := raw.Runtime

	out := &AgentRuntimeModel{
		Image:              types.StringValue(api.Image),
		Command:            types.ListNull(types.StringType),
		Environment:        types.ListNull(runtimeEnvEntryType),
		Credentials:        types.ListNull(runtimeCredentialType),
		InferenceProtocol:  types.StringValue(api.InferenceProtocol),
		Backend:            types.StringValue(api.Backend),
		SteerMode:          types.StringValue(api.SteerMode),
		Privileged:         types.BoolValue(api.Privileged),
		TtlHours:           types.Int64PointerValue(api.TtlHours),
		MaxCostUsd:         types.Int64PointerValue(api.MaxCostUsd),
		IdleTimeoutMinutes: types.Int64PointerValue(api.IdleTimeoutMinutes),
	}

	if api.Command != nil {
		list, d := types.ListValueFrom(ctx, types.StringType, *api.Command)
		diags.Append(d...)
		out.Command = list
	}

	if api.Resources != nil {
		out.Resources = &AgentRuntimeResourcesModel{
			CpuRequest:    types.StringPointerValue(api.Resources.CpuRequest),
			MemoryRequest: types.StringPointerValue(api.Resources.MemoryRequest),
			CpuLimit:      types.StringPointerValue(api.Resources.CpuLimit),
			MemoryLimit:   types.StringPointerValue(api.Resources.MemoryLimit),
		}
	}

	if api.Environment != nil {
		env := make([]AgentRuntimeEnvEntryModel, len(*api.Environment))
		for i, e := range *api.Environment {
			env[i] = AgentRuntimeEnvEntryModel{
				Key:   types.StringValue(e.Key),
				Value: types.StringValue(e.Value),
			}
		}
		list, d := types.ListValueFrom(ctx, runtimeEnvEntryType, env)
		diags.Append(d...)
		out.Environment = list
	}

	if api.Credentials != nil {
		creds := make([]AgentRuntimeCredentialModel, len(*api.Credentials))
		for i, c := range *api.Credentials {
			creds[i] = AgentRuntimeCredentialModel{
				Key:          types.StringValue(c.Key),
				Scope:        types.StringValue(c.Scope),
				CredentialId: types.StringPointerValue(c.CredentialId),
				Label:        types.StringValue(c.Label),
				Description:  types.StringPointerValue(c.Description),
				Required:     types.BoolValue(c.Required),
			}
		}
		list, d := types.ListValueFrom(ctx, runtimeCredentialType, creds)
		diags.Append(d...)
		out.Credentials = list
	}

	return out
}
