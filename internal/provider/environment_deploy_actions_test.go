package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func grantActionsValid(t *testing.T, permissions schema.Attribute, actions ...string) bool {
	t.Helper()
	nested, ok := permissions.(schema.SetNestedAttribute)
	if !ok {
		t.Fatalf("permissions is %T", permissions)
	}
	attribute, ok := nested.NestedObject.Attributes["actions"].(schema.SetAttribute)
	if !ok {
		t.Fatal("actions is not a set attribute")
	}
	values := make([]attr.Value, 0, len(actions))
	for _, action := range actions {
		values = append(values, types.StringValue(action))
	}
	request := validator.SetRequest{Path: path.Root("actions"), ConfigValue: types.SetValueMust(types.StringType, values)}
	for _, v := range attribute.Validators {
		response := &validator.SetResponse{}
		v.ValidateSet(context.Background(), request, response)
		if response.Diagnostics.HasError() {
			return false
		}
	}
	return true
}

func TestEnvironmentDeployActionsOnlyOnEnvironments(t *testing.T) {
	var environment resource.SchemaResponse
	(&EnvironmentResource{}).Schema(t.Context(), resource.SchemaRequest{}, &environment)
	var agent resource.SchemaResponse
	(&AgentResource{}).Schema(t.Context(), resource.SchemaRequest{}, &agent)
	defaults, ok := defaultPermissionsAttribute().Attributes["environments"]
	if !ok {
		t.Fatal("default_permissions has no environments slot")
	}
	limited := []string{"read", "deploy-agent", "deploy-mcp-gateway", "deploy-mcp-registry", "deploy-skill", "deploy-app"}
	for name, permissions := range map[string]schema.Attribute{"environment": environment.Schema.Attributes["permissions"], "default_permissions.environments": defaults} {
		if !grantActionsValid(t, permissions, limited...) {
			t.Errorf("%s rejects a limited deploy grant", name)
		}
		if !grantActionsValid(t, permissions, "read", "use") {
			t.Errorf("%s rejects read and use", name)
		}
		if grantActionsValid(t, permissions, "deploy-everything") {
			t.Errorf("%s accepts an unknown action", name)
		}
	}
	if grantActionsValid(t, agent.Schema.Attributes["permissions"], "read", "deploy-agent") {
		t.Error("agent permissions accept an environment-only action")
	}
}
