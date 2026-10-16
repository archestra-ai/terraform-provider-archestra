package provider

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func TestMcpToolCallsPagination(t *testing.T) {
	for _, tc := range []struct {
		name      string
		cap       int64
		pages     int
		next      string
		hasNext   bool
		wantCount int64
		truncated bool
		wantError bool
	}{
		{"two populated pages", 10, 2, "next", true, 2, false, false},
		{"cap stops requests", 1, 1, "next", true, 1, true, false},
		{"exact cap exhausted", 1, 1, "", false, 1, false, false},
		{"missing cursor", 10, 1, "", true, 0, false, true},
		{"repeated cursor", 10, 2, "same", true, 0, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				calls++
				if req.URL.Query().Get("mcpServerName") != "fixture" {
					t.Error("server filter was not sent")
				}
				if calls > 1 && req.URL.Query().Get("cursor") != tc.next {
					t.Error("next cursor was not sent")
				}
				next := tc.hasNext && (calls == 1 || tc.name == "repeated cursor")
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprintf(w, `{"data":[{"id":"00000000-0000-0000-0000-00000000000%d","createdAt":"2026-01-01T00:00:00Z","method":"tools/call","mcpServerName":"fixture","ownerType":"organization","appName":"app","toolCall":{"name":"tool","arguments":{"x":1},"extension":true},"toolResult":{"result":"ok"}}],"pagination":{"limit":100,"hasNext":%t,"nextCursor":%q}}`, calls, next, tc.next)
			}))
			defer server.Close()
			c, err := client.NewClientWithResponses(server.URL)
			if err != nil {
				t.Fatal(err)
			}
			d := McpToolCallsDataSource{client: c}
			ctx := t.Context()
			var schema datasource.SchemaResponse
			d.Schema(ctx, datasource.SchemaRequest{}, &schema)
			typ := schema.Schema.Type().TerraformType(ctx).(tftypes.Object)
			values := map[string]tftypes.Value{}
			for name, typ := range typ.AttributeTypes {
				values[name] = tftypes.NewValue(typ, nil)
			}
			values["max_records"] = tftypes.NewValue(tftypes.Number, tc.cap)
			values["mcp_server_name"] = tftypes.NewValue(tftypes.String, "fixture")
			response := datasource.ReadResponse{State: tfsdk.State{Schema: schema.Schema}}
			d.Read(ctx, datasource.ReadRequest{Config: tfsdk.Config{Schema: schema.Schema, Raw: tftypes.NewValue(typ, values)}}, &response)
			if response.Diagnostics.HasError() != tc.wantError {
				t.Fatalf("diagnostics=%v", response.Diagnostics)
			}
			if calls != tc.pages {
				t.Fatalf("requests=%d, want %d", calls, tc.pages)
			}
			if tc.wantError {
				return
			}
			var result McpToolCallsDataSourceModel
			if ds := response.State.Get(ctx, &result); ds.HasError() {
				t.Fatal(ds)
			}
			if result.ReturnedCount.ValueInt64() != tc.wantCount || result.Truncated.ValueBool() != tc.truncated {
				t.Fatalf("count=%v truncated=%v", result.ReturnedCount, result.Truncated)
			}
			object := result.Calls.Elements()[0].(types.Object).Attributes()
			var wire map[string]any
			if err := json.Unmarshal([]byte(object["tool_call_json"].(types.String).ValueString()), &wire); err != nil || wire["extension"] != true {
				t.Fatal("tool payload extension lost")
			}
			if object["app_name"].(types.String).ValueString() != "app" {
				t.Fatal("app metadata lost")
			}
		})
	}
}

func TestMcpToolCallAcceptsPolymorphicPayload(t *testing.T) {
	value, ds := flattenMcpToolCall(map[string]any{"toolCall": "opaque", "id": "00000000-0000-0000-0000-000000000001"})
	if ds.HasError() {
		t.Fatal(ds)
	}
	attrs := value.(types.Object).Attributes()
	if attrs["tool_call_json"].(types.String).ValueString() != `"opaque"` || !attrs["tool_name"].IsNull() {
		t.Fatal("polymorphic payload was not preserved")
	}
}
