package provider

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// parseCatalogResp wraps a JSON body in a fake 200 response and runs it
// through the same generated parser the real Read path uses, so tests
// exercise wire-format parsing identically to production.
func parseCatalogResp(t *testing.T, body string) *client.GetInternalMcpCatalogItemResponse {
	t.Helper()
	rsp := &http.Response{
		StatusCode: 200,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
	parsed, err := client.ParseGetInternalMcpCatalogItemResponse(rsp)
	if err != nil {
		t.Fatalf("ParseGetInternalMcpCatalogItemResponse: %v", err)
	}
	if parsed.JSON200 == nil {
		t.Fatal("JSON200 nil; check Content-Type and status in test fixture")
	}
	return parsed
}

// Typed-assertion helpers. attr.Value is the framework's interface type;
// production code uses comma-ok everywhere via the framework's typed APIs,
// but in test extraction we routinely walk attribute maps. These helpers
// keep the lint clean (no forcetypeassert) and surface mismatches as a
// test failure instead of a panic.
func asString(t *testing.T, v attr.Value) types.String {
	t.Helper()
	s, ok := v.(types.String)
	if !ok {
		t.Fatalf("expected types.String, got %T", v)
	}
	return s
}

func asBool(t *testing.T, v attr.Value) types.Bool {
	t.Helper()
	b, ok := v.(types.Bool)
	if !ok {
		t.Fatalf("expected types.Bool, got %T", v)
	}
	return b
}

func asInt64(t *testing.T, v attr.Value) types.Int64 {
	t.Helper()
	i, ok := v.(types.Int64)
	if !ok {
		t.Fatalf("expected types.Int64, got %T", v)
	}
	return i
}

func asList(t *testing.T, v attr.Value) types.List {
	t.Helper()
	l, ok := v.(types.List)
	if !ok {
		t.Fatalf("expected types.List, got %T", v)
	}
	return l
}

func asSet(t *testing.T, v attr.Value) types.Set {
	t.Helper()
	s, ok := v.(types.Set)
	if !ok {
		t.Fatalf("expected types.Set, got %T", v)
	}
	return s
}

func asObject(t *testing.T, v attr.Value) types.Object {
	t.Helper()
	o, ok := v.(types.Object)
	if !ok {
		t.Fatalf("expected types.Object, got %T", v)
	}
	return o
}

func TestMapCatalogEnterpriseManagedConfig(t *testing.T) {
	t.Parallel()

	t.Run("nil returns nil", func(t *testing.T) {
		got := mapCatalogEnterpriseManagedConfig(parseCatalogResp(t, `{}`))
		if got != nil {
			t.Errorf("got %+v, want nil", got)
		}
	})

	t.Run("populated maps each kind of field", func(t *testing.T) {
		body := `{
			"enterpriseManagedConfig": {
				"identityProviderId": "11111111-1111-1111-1111-111111111111",
				"resourceType": "api",
				"resourceIdentifier": "https://api.example.com",
				"scopes": ["read", "write"],
				"cacheTtlSeconds": 3600,
				"fallbackMode": "fail",
				"audience": "aud-x"
			}
		}`
		got := mapCatalogEnterpriseManagedConfig(parseCatalogResp(t, body))
		if got == nil {
			t.Fatal("got nil, want populated")
		}
		// stringValueOrNull on *string
		if got.IdentityProviderId.ValueString() != "11111111-1111-1111-1111-111111111111" {
			t.Errorf("IdentityProviderId = %q", got.IdentityProviderId.ValueString())
		}
		if got.Audience.ValueString() != "aud-x" {
			t.Errorf("Audience = %q", got.Audience.ValueString())
		}
		// inline if/else on *enum (typed string alias)
		if got.ResourceType.ValueString() != "api" {
			t.Errorf("ResourceType = %q", got.ResourceType.ValueString())
		}
		if got.FallbackMode.ValueString() != "fail" {
			t.Errorf("FallbackMode = %q", got.FallbackMode.ValueString())
		}
		// inline if/else on *int
		if got.CacheTtlSeconds.ValueInt64() != 3600 {
			t.Errorf("CacheTtlSeconds = %d", got.CacheTtlSeconds.ValueInt64())
		}
		// *[]string via ListValueFrom
		if got.Scopes.IsNull() || len(got.Scopes.Elements()) != 2 {
			t.Errorf("Scopes = %v, want list of 2", got.Scopes)
		}
		// fields not in the JSON should be null, not empty string
		if !got.HeaderName.IsNull() {
			t.Errorf("HeaderName = %v, want null", got.HeaderName)
		}
	})
}

func TestMapCatalogTeams(t *testing.T) {
	t.Parallel()

	t.Run("empty list", func(t *testing.T) {
		got := mapCatalogTeams(parseCatalogResp(t, `{}`))
		if !got.IsNull() {
			t.Errorf("expected null list, got %v", got)
		}
	})

	t.Run("two teams", func(t *testing.T) {
		body := `{"teams":[{"id":"t1","name":"Team 1"},{"id":"t2","name":"Team 2"}]}`
		got := mapCatalogTeams(parseCatalogResp(t, body))
		if got.IsNull() || len(got.Elements()) != 2 {
			t.Fatalf("expected 2 elements, got %v", got)
		}
		// list values are types.String — only the id is mapped, name dropped.
		first := asString(t, got.Elements()[0]).ValueString()
		if first != "t1" {
			t.Errorf("first team = %q, want t1", first)
		}
	})
}

func TestMapCatalogLabels(t *testing.T) {
	t.Parallel()

	t.Run("empty list", func(t *testing.T) {
		got := mapCatalogLabels(parseCatalogResp(t, `{}`))
		if !got.IsNull() {
			t.Errorf("expected null, got %v", got)
		}
	})

	t.Run("two labels", func(t *testing.T) {
		body := `{"labels":[{"key":"env","value":"prod"},{"key":"team","value":"infra"}]}`
		got := mapCatalogLabels(parseCatalogResp(t, body))
		if got.IsNull() || len(got.Elements()) != 2 {
			t.Fatalf("expected 2 elements, got %v", got)
		}
	})
}

func TestMapCatalogAuthFields(t *testing.T) {
	t.Parallel()

	t.Run("nil pointer", func(t *testing.T) {
		got := mapCatalogAuthFields(parseCatalogResp(t, `{}`))
		if !got.IsNull() {
			t.Errorf("expected null, got %v", got)
		}
	})

	t.Run("description optional", func(t *testing.T) {
		body := `{"authFields":[
			{"name":"token","label":"Token","type":"string","required":true,"description":"API token"},
			{"name":"region","label":"Region","type":"string","required":false}
		]}`
		got := mapCatalogAuthFields(parseCatalogResp(t, body))
		if got.IsNull() || len(got.Elements()) != 2 {
			t.Fatalf("expected 2 elements, got %v", got)
		}
		first := asObject(t, got.Elements()[0]).Attributes()
		if asString(t, first["description"]).ValueString() != "API token" {
			t.Errorf("first description = %v", first["description"])
		}
		second := asObject(t, got.Elements()[1]).Attributes()
		if !asString(t, second["description"]).IsNull() {
			t.Errorf("second description = %v, want null", second["description"])
		}
	})
}

func TestMapCatalogLocalConfig(t *testing.T) {
	t.Parallel()

	priorNull := types.ObjectNull(localConfigAttrTypes)

	t.Run("nil returns ObjectNull", func(t *testing.T) {
		got := mapCatalogLocalConfig(t.Context(), parseCatalogResp(t, `{}`), priorNull)
		if !got.IsNull() {
			t.Errorf("expected null, got %v", got)
		}
	})

	t.Run("minimal: just command, optionals null", func(t *testing.T) {
		body := `{"localConfig":{"command":"npx"}}`
		got := mapCatalogLocalConfig(t.Context(), parseCatalogResp(t, body), priorNull)
		if got.IsNull() {
			t.Fatal("got null, want populated")
		}
		attrs := got.Attributes()
		if asString(t, attrs["command"]).ValueString() != "npx" {
			t.Errorf("command = %v", attrs["command"])
		}
		if !asList(t, attrs["arguments"]).IsNull() {
			t.Errorf("arguments = %v, want null", attrs["arguments"])
		}
		if !asSet(t, attrs["environment"]).IsNull() {
			t.Errorf("environment = %v, want null", attrs["environment"])
		}
	})

	t.Run("environment with default scalar collapse", func(t *testing.T) {
		// envVar.Default is *any in the wire — strings collapse via JSON unquote,
		// non-strings keep their JSON encoding. This test pins both branches.
		body := `{"localConfig":{"environment":[
			{"key":"FOO","type":"value","default":"hello","promptOnInstallation":false},
			{"key":"PORT","type":"value","default":8080,"promptOnInstallation":false}
		]}}`
		got := mapCatalogLocalConfig(t.Context(), parseCatalogResp(t, body), priorNull)
		envSet := asSet(t, got.Attributes()["environment"])
		elements := envSet.Elements()
		if len(elements) != 2 {
			t.Fatalf("env elements = %d, want 2", len(elements))
		}
		// Set ordering isn't deterministic — collect into a key→default map.
		defaults := map[string]string{}
		for _, e := range elements {
			fields := asObject(t, e).Attributes()
			defaults[asString(t, fields["key"]).ValueString()] = asString(t, fields["default"]).ValueString()
		}
		if defaults["FOO"] != "hello" {
			t.Errorf("FOO default = %q, want unquoted 'hello'", defaults["FOO"])
		}
		if defaults["PORT"] != "8080" {
			t.Errorf("PORT default = %q, want JSON-encoded '8080'", defaults["PORT"])
		}
	})
}

func TestMapCatalogImagePullSecrets(t *testing.T) {
	t.Parallel()

	t.Run("no IPS in body", func(t *testing.T) {
		got := mapCatalogImagePullSecrets(t.Context(), []byte(`{"localConfig":{}}`), types.ObjectNull(localConfigAttrTypes))
		if !got.IsNull() {
			t.Errorf("expected null, got %v", got)
		}
	})

	t.Run("malformed body returns null (not panic)", func(t *testing.T) {
		got := mapCatalogImagePullSecrets(t.Context(), []byte(`not json at all`), types.ObjectNull(localConfigAttrTypes))
		if !got.IsNull() {
			t.Errorf("expected null, got %v", got)
		}
	})

	t.Run("populated, no prior state — passwords are null", func(t *testing.T) {
		body := []byte(`{"localConfig":{"imagePullSecrets":[
			{"source":"inline","server":"registry.example.com","username":"u1","email":"u1@example.com"}
		]}}`)
		got := mapCatalogImagePullSecrets(t.Context(), body, types.ObjectNull(localConfigAttrTypes))
		if got.IsNull() || len(got.Elements()) != 1 {
			t.Fatalf("expected 1 element, got %v", got)
		}
		fields := asObject(t, got.Elements()[0]).Attributes()
		if !asString(t, fields["password"]).IsNull() {
			t.Errorf("password = %v, want null (backend doesn't echo)", fields["password"])
		}
		if asString(t, fields["server"]).ValueString() != "registry.example.com" {
			t.Errorf("server = %v", fields["server"])
		}
	})

	t.Run("populated WITH prior state — password preserved", func(t *testing.T) {
		body := []byte(`{"localConfig":{"imagePullSecrets":[
			{"source":"inline","server":"registry.example.com","username":"u1"}
		]}}`)
		prior := mustPriorLocalConfigWithIPS(t, "registry.example.com", "u1", "preserved-secret")
		got := mapCatalogImagePullSecrets(t.Context(), body, prior)
		if got.IsNull() || len(got.Elements()) != 1 {
			t.Fatalf("expected 1 element, got %v", got)
		}
		fields := asObject(t, got.Elements()[0]).Attributes()
		if asString(t, fields["password"]).ValueString() != "preserved-secret" {
			t.Errorf("password = %v, want 'preserved-secret' from prior state", fields["password"])
		}
	})

	t.Run("prior state with mismatched server — password not transferred", func(t *testing.T) {
		body := []byte(`{"localConfig":{"imagePullSecrets":[
			{"source":"inline","server":"registry.example.com","username":"u1"}
		]}}`)
		prior := mustPriorLocalConfigWithIPS(t, "different.registry.com", "u1", "secret-for-other-registry")
		got := mapCatalogImagePullSecrets(t.Context(), body, prior)
		fields := asObject(t, got.Elements()[0]).Attributes()
		if !asString(t, fields["password"]).IsNull() {
			t.Errorf("password = %v, want null (key mismatch)", fields["password"])
		}
	})
}

func TestMapCatalogRemoteConfig(t *testing.T) {
	t.Parallel()

	t.Run("not remote returns null", func(t *testing.T) {
		body := `{"serverType":"local","serverUrl":"https://x.example.com"}`
		got := mapCatalogRemoteConfig(parseCatalogResp(t, body))
		if !got.IsNull() {
			t.Errorf("expected null, got %v", got)
		}
	})

	t.Run("remote without serverUrl returns null", func(t *testing.T) {
		got := mapCatalogRemoteConfig(parseCatalogResp(t, `{"serverType":"remote"}`))
		if !got.IsNull() {
			t.Errorf("expected null, got %v", got)
		}
	})

	t.Run("remote without oauth — oauth_config null", func(t *testing.T) {
		body := `{"serverType":"remote","serverUrl":"https://x.example.com"}`
		got := mapCatalogRemoteConfig(parseCatalogResp(t, body))
		if got.IsNull() {
			t.Fatal("got null, want populated")
		}
		attrs := got.Attributes()
		if asString(t, attrs["url"]).ValueString() != "https://x.example.com" {
			t.Errorf("url = %v", attrs["url"])
		}
		if !asObject(t, attrs["oauth_config"]).IsNull() {
			t.Errorf("oauth_config should be null when omitted")
		}
	})
}

func TestMapCatalogOauthConfig(t *testing.T) {
	t.Parallel()

	t.Run("nil returns ObjectNull", func(t *testing.T) {
		got := mapCatalogOauthConfig(parseCatalogResp(t, `{}`))
		if !got.IsNull() {
			t.Errorf("expected null, got %v", got)
		}
	})

	t.Run("populated with optional fields and lists", func(t *testing.T) {
		// OauthConfig sub-struct uses snake_case JSON tags (unlike the
		// camelCase top-level catalog item — quirk of the generated parser).
		body := `{"oauthConfig":{
			"client_id":"cid",
			"client_secret":"sec",
			"supports_resource_metadata":true,
			"redirect_uris":["https://a/cb","https://b/cb"],
			"scopes":["read","write"],
			"default_scopes":["read"],
			"streamable_http_port":8443,
			"generic_oauth":true
		}}`
		got := mapCatalogOauthConfig(parseCatalogResp(t, body))
		if got.IsNull() {
			t.Fatal("got null, want populated")
		}
		attrs := got.Attributes()
		if asString(t, attrs["client_id"]).ValueString() != "cid" {
			t.Errorf("client_id = %v", attrs["client_id"])
		}
		if asString(t, attrs["client_secret"]).ValueString() != "sec" {
			t.Errorf("client_secret = %v", attrs["client_secret"])
		}
		if !asBool(t, attrs["supports_resource_metadata"]).ValueBool() {
			t.Error("supports_resource_metadata should be true")
		}
		if l := asList(t, attrs["redirect_uris"]); l.IsNull() || len(l.Elements()) != 2 {
			t.Errorf("redirect_uris = %v", l)
		}
		if asInt64(t, attrs["streamable_http_port"]).ValueInt64() != 8443 {
			t.Errorf("streamable_http_port = %v", attrs["streamable_http_port"])
		}
		if !asBool(t, attrs["generic_oauth"]).ValueBool() {
			t.Error("generic_oauth should be true")
		}
		// Fields absent from JSON should be null, not zero.
		if !asBool(t, attrs["browser_auth"]).IsNull() {
			t.Errorf("browser_auth = %v, want null", attrs["browser_auth"])
		}
		if !asString(t, attrs["grant_type"]).IsNull() {
			t.Errorf("grant_type = %v, want null", attrs["grant_type"])
		}
	})
}

// End-to-end through mapGetResponseToState — proves the helpers compose correctly.
func TestMapGetResponseToState_LocalServer(t *testing.T) {
	t.Parallel()

	body := `{
		"name": "filesystem",
		"description": "FS access",
		"requiresAuth": false,
		"scope": "public",
		"serverType": "local",
		"localConfig": {
			"command": "npx",
			"arguments": ["-y","@modelcontextprotocol/server-filesystem"]
		},
		"teams": [{"id":"team-1","name":"x"}]
	}`
	apiResp := parseCatalogResp(t, body)

	var data MCPServerRegistryResourceModel
	data.LocalConfig = types.ObjectNull(localConfigAttrTypes)
	var diags diag.Diagnostics

	r := &MCPServerRegistryResource{}
	r.mapGetResponseToState(t.Context(), &data, apiResp, &diags)

	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if data.Name.ValueString() != "filesystem" {
		t.Errorf("Name = %v", data.Name)
	}
	if data.Description.ValueString() != "FS access" {
		t.Errorf("Description = %v", data.Description)
	}
	if data.LocalConfig.IsNull() {
		t.Fatal("LocalConfig should be populated")
	}
	lcCmd := asString(t, data.LocalConfig.Attributes()["command"])
	if lcCmd.ValueString() != "npx" {
		t.Errorf("LocalConfig.command = %v", lcCmd)
	}
	if data.RemoteConfig.IsNull() != true {
		t.Errorf("RemoteConfig should be null for local servers")
	}
}

// mustPriorLocalConfigWithIPS builds a types.Object shaped like LocalConfig
// where image_pull_secrets contains a single entry. Used to exercise the
// password-preservation path in mapCatalogImagePullSecrets.
func mustPriorLocalConfigWithIPS(t *testing.T, server, username, password string) types.Object {
	t.Helper()
	ipsObjType := types.ObjectType{AttrTypes: ipSecretAttrTypes}

	ipsElem, _ := types.ObjectValue(ipSecretAttrTypes, map[string]attr.Value{
		"source":   types.StringValue("inline"),
		"name":     types.StringNull(),
		"server":   types.StringValue(server),
		"username": types.StringValue(username),
		"password": types.StringValue(password),
		"email":    types.StringNull(),
	})
	ipsList, d := types.ListValue(ipsObjType, []attr.Value{ipsElem})
	if d.HasError() {
		t.Fatalf("ListValue: %v", d)
	}

	fields := map[string]attr.Value{
		"command":            types.StringNull(),
		"arguments":          types.ListNull(types.StringType),
		"environment":        types.SetNull(types.ObjectType{AttrTypes: envVariableAttrTypes}),
		"env_from":           types.ListNull(types.ObjectType{AttrTypes: envFromAttrTypes}),
		"docker_image":       types.StringNull(),
		"transport_type":     types.StringNull(),
		"http_port":          types.Int64Null(),
		"http_path":          types.StringNull(),
		"service_account":    types.StringNull(),
		"node_port":          types.Int64Null(),
		"image_pull_secrets": ipsList,
	}
	obj, d := types.ObjectValue(localConfigAttrTypes, fields)
	if d.HasError() {
		t.Fatalf("ObjectValue: %v", d)
	}
	return obj
}
