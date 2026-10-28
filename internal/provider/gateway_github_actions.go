package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var githubActionsAccessAttrTypes = map[string]attr.Type{
	"act_as":                    types.StringType,
	"resolve_review_threads":    types.BoolType,
	"github_app_credential_key": types.StringType,
	"repositories":              types.ListType{ElemType: types.StringType},
	"workflows":                 types.ListType{ElemType: types.StringType},
}

var githubActionsAccessAttrSpec = AttrSpec{
	TFName: "github_actions_access", JSONName: "githubActionsAccess", Kind: AtomicObject,
	Children: []AttrSpec{
		{TFName: "act_as", JSONName: "actAs", Kind: Scalar},
		{TFName: "resolve_review_threads", JSONName: "resolveReviewThreads", Kind: Scalar},
		{TFName: "github_app_credential_key", JSONName: "githubAppCredentialKey", Kind: Scalar},
		{TFName: "repositories", JSONName: "repositories", Kind: List},
		{TFName: "workflows", JSONName: "workflows", Kind: List},
	},
}

func githubActionsAccessSchema() schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		Optional: true, MarkdownDescription: "GitHub Actions token exchange access for this gateway.",
		Attributes: map[string]schema.Attribute{
			"act_as":                    schema.StringAttribute{Optional: true, Validators: []validator.String{stringvalidator.OneOf("author", "actor")}, MarkdownDescription: "Account used by a run: author (default) or the actor who started it."},
			"resolve_review_threads":    schema.BoolAttribute{Optional: true, MarkdownDescription: "Whether repository tokens can resolve review threads."},
			"github_app_credential_key": schema.StringAttribute{Required: true, MarkdownDescription: "Credential key of an accessible GitHub App."},
			"repositories":              schema.ListAttribute{Required: true, ElementType: types.StringType, MarkdownDescription: "Allowed owner/repository names or owner/* patterns."},
			"workflows":                 schema.ListAttribute{Required: true, ElementType: types.StringType, MarkdownDescription: "Trusted workflow references as owner/repo/.github/workflows/file.yml@refs/heads/branch, refs/tags/tag or a full commit SHA."},
		},
	}
}

func githubActionsAccessFromResponse(body []byte, diags *diag.Diagnostics) types.Object {
	var wire struct {
		Access *struct {
			ActAs                *string  `json:"actAs"`
			ResolveReviewThreads *bool    `json:"resolveReviewThreads"`
			Credential           string   `json:"githubAppCredentialKey"`
			Repositories         []string `json:"repositories"`
			Workflows            []string `json:"workflows"`
		} `json:"githubActionsAccess"`
	}
	empty := types.ObjectNull(githubActionsAccessAttrTypes)
	if err := json.Unmarshal(body, &wire); err != nil {
		diags.AddError("Invalid Gateway Response", "Unable to decode GitHub Actions access: "+err.Error())
		return empty
	}
	if wire.Access == nil {
		return empty
	}
	stringsToList := func(values []string) types.List {
		elements := make([]attr.Value, len(values))
		for i, value := range values {
			elements[i] = types.StringValue(value)
		}
		list, ds := types.ListValue(types.StringType, elements)
		diags.Append(ds...)
		return list
	}
	value, ds := types.ObjectValue(githubActionsAccessAttrTypes, map[string]attr.Value{
		"act_as":                    types.StringPointerValue(wire.Access.ActAs),
		"resolve_review_threads":    types.BoolPointerValue(wire.Access.ResolveReviewThreads),
		"github_app_credential_key": types.StringValue(wire.Access.Credential),
		"repositories":              stringsToList(wire.Access.Repositories),
		"workflows":                 stringsToList(wire.Access.Workflows),
	})
	diags.Append(ds...)
	return value
}

func (r *McpGatewayResource) writeGithubActionsAccess(ctx context.Context, data *McpGatewayResourceModel, access any, diags *diag.Diagnostics) {
	id, err := uuid.Parse(data.ID.ValueString())
	if err != nil {
		diags.AddError("Invalid Gateway ID", err.Error())
		return
	}
	body, err := json.Marshal(map[string]any{"githubActionsAccess": access})
	if err != nil {
		diags.AddError("Marshal Error", err.Error())
		return
	}
	result, err := r.client.UpdateAgentWithBodyWithResponse(ctx, id, "application/json", bytes.NewReader(body))
	if err != nil {
		diags.AddError("GitHub Actions Access Update Failed", err.Error())
		return
	}
	if result.JSON200 == nil {
		diags.AddError("GitHub Actions Access Update Failed", fmt.Sprintf("Expected 200 OK, got status %d: %s", result.StatusCode(), result.Body))
		return
	}
	r.flatten(ctx, data, result.Body, diags)
}
