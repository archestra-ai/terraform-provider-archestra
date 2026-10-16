package provider

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestAgentInlineToolsPreserveDerivedSubagentAssignments(t *testing.T) {
	for _, incomplete := range []bool{false, true} {
		t.Run(fmt.Sprint(incomplete), func(t *testing.T) {
			const parent = "11111111-1111-4111-8111-111111111111"
			const ordinary = "22222222-2222-4222-8222-222222222222"
			const local = "33333333-3333-4333-8333-333333333333"
			const remote = "44444444-4444-4444-8444-444444444444"
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if r.Method != http.MethodGet {
					t.Error("ownership read attempted mutation")
					w.WriteHeader(500)
					return
				}
				if strings.HasPrefix(r.URL.Path, "/api/agents/") {
					tools := fmt.Sprintf(`{"id":%q,"delegateToAgentId":null,"delegateToA2aConnectionId":null},`, ordinary)
					if incomplete {
						tools = ""
					}
					fmt.Fprintf(w, `{"id":%q,"tools":[%s{"id":%q,"delegateToAgentId":%q,"delegateToA2aConnectionId":null},{"id":%q,"delegateToAgentId":null,"delegateToA2aConnectionId":%q}]}`, parent, tools, local, local, remote, remote)
					return
				}
				fmt.Fprintf(w, `{"data":[{"id":%q,"agent":{"id":%q,"name":"fixture"},"tool":{"id":%q,"name":"ordinary"},"mcpServerId":null,"credentialResolutionMode":"dynamic"},{"id":%q,"agent":{"id":%q,"name":"fixture"},"tool":{"id":%q,"name":"local"},"mcpServerId":null,"credentialResolutionMode":"dynamic"},{"id":%q,"agent":{"id":%q,"name":"fixture"},"tool":{"id":%q,"name":"remote"},"mcpServerId":null,"credentialResolutionMode":"dynamic"}],"pagination":{"hasNext":false}}`, ordinary, parent, ordinary, local, parent, local, remote, parent, remote)
			}))
			defer server.Close()
			c, err := client.NewClientWithResponses(server.URL)
			if err != nil {
				t.Fatal(err)
			}
			r := AgentResource{client: c}
			var diagnostics diag.Diagnostics
			rows, _ := r.ordinaryToolAssignments(t.Context(), uuid.MustParse(parent), &diagnostics)
			if incomplete {
				if !diagnostics.HasError() {
					t.Fatal("incomplete ownership visibility was accepted")
				}
				return
			}
			if diagnostics.HasError() || len(rows) != 1 || rows[0].ToolID != ordinary {
				t.Fatal("derived subagents were included in ordinary tool ownership", rows, diagnostics)
			}
		})
	}
}
func TestAccAgentInlineTools(t *testing.T)   { testAccInlineTools(t, "archestra_agent") }
func TestAccGatewayInlineTools(t *testing.T) { testAccInlineTools(t, "archestra_mcp_gateway") }
func testAccInlineTools(t *testing.T, kind string) {
	address := kind + ".test"
	name := acctest.RandomWithPrefix("tf-inline-tools")
	var id, toolID, serverID string
	reservedIDs := map[string]bool{}
	catalogConfig := `local_config={command="npx",arguments=["-y","@modelcontextprotocol/server-filesystem","/tmp"]}`
	if endpoint := os.Getenv("ARCHESTRA_TEST_MCP_URL"); endpoint != "" {
		catalogConfig = fmt.Sprintf(`remote_config={url=%q}`, endpoint)
	}
	base := fmt.Sprintf(`
 resource "archestra_mcp_registry_catalog_item" "test" {
 name=%q
 multitenant=false
 %s
 }
 resource "archestra_mcp_server_installation" "test" {
 name=%q
 catalog_id=archestra_mcp_registry_catalog_item.test.id
 scope="org"
 }
 resource "archestra_agent" "child" {name=%q}
 resource "archestra_remote_agent" "remote" {source=%q}
 `, name+"-catalog", catalogConfig, name+"-install", name+"-child", testRemoteCard(name+"-remote"))
	config := func(fields string) string {
		return base + fmt.Sprintf(`
 resource "%s" "test" {
 name=%q
 subagent_ids=[archestra_agent.child.id]
 remote_agent_ids=[archestra_remote_agent.remote.id]
 %s
 }`, kind, name, fields)
	}
	initial := config(`tools=[for tool in archestra_mcp_server_installation.test.tools:{tool_id=tool.id,mcp_server_id=archestra_mcp_server_installation.test.id,credential_resolution_mode="static"}]
 excluded_tool_ids=[]`)
	updated := config(`tools=[{tool_id=archestra_mcp_server_installation.test.tools[0].id,credential_resolution_mode="dynamic"}]
 excluded_tool_ids=[archestra_mcp_server_installation.test.tools[1].id]`)
	api := func() *client.ClientWithResponses {
		c, err := client.NewClientWithResponses(os.Getenv("ARCHESTRA_BASE_URL"), client.WithRequestEditorFn(func(_ context.Context, r *http.Request) error {
			r.Header.Set("Authorization", os.Getenv("ARCHESTRA_API_KEY"))
			return nil
		}))
		if err != nil {
			t.Fatal(err)
		}
		return c
	}
	same := func(s *terraform.State) error {
		if s.RootModule().Resources[address].Primary.ID != id {
			return fmt.Errorf("tool selection replaced the agent")
		}
		response, err := api().GetAgentWithResponse(t.Context(), uuid.MustParse(id))
		if err != nil || response.JSON200 == nil {
			return fmt.Errorf("read reserved tools: %v", err)
		}
		present := map[string]bool{}
		for _, tool := range response.JSON200.Tools {
			present[tool.Id.String()] = true
		}
		for key := range reservedIDs {
			if !present[key] {
				return fmt.Errorf("reserved tool %s was removed", key)
			}
		}
		return nil
	}
	resource.Test(t, resource.TestCase{PreCheck: func() { testAccPreCheck(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{
		{Config: initial, Check: resource.ComposeTestCheckFunc(resource.TestCheckResourceAttrPair(address, "tools.#", "archestra_mcp_server_installation.test", "tools.#"), resource.TestCheckResourceAttr(address, "subagent_ids.#", "1"), resource.TestCheckResourceAttr(address, "remote_agent_ids.#", "1"), func(s *terraform.State) error {
			id = s.RootModule().Resources[address].Primary.ID
			serverID = s.RootModule().Resources["archestra_mcp_server_installation.test"].Primary.ID
			toolID = s.RootModule().Resources["archestra_mcp_server_installation.test"].Primary.Attributes["tools.0.id"]
			response, err := api().GetAgentWithResponse(t.Context(), uuid.MustParse(id))
			if err != nil || response.JSON200 == nil {
				return fmt.Errorf("read reserved tools: %v", err)
			}
			for _, tool := range response.JSON200.Tools {
				if tool.CatalogId != nil && requiredPlatformTool(tool.CatalogId.String(), tool.Name) {
					reservedIDs[tool.Id.String()] = true
				}
			}
			if len(reservedIDs) == 0 {
				return fmt.Errorf("fixture has no platform-required recovery tools")
			}

			return nil
		})},
		{ResourceName: address, ImportState: true, ImportStateVerify: true},
		{Config: initial, PlanOnly: true},
		{Config: updated, Check: resource.ComposeTestCheckFunc(same, resource.TestCheckResourceAttr(address, "tools.#", "1"), resource.TestCheckResourceAttr(address, "excluded_tool_ids.#", "1"))},
		{Config: updated, PlanOnly: true},
		{PreConfig: func() {
			r, err := api().AssignToolToAgentWithBodyWithResponse(t.Context(), uuid.MustParse(id), uuid.MustParse(toolID), "application/json", strings.NewReader(fmt.Sprintf(`{"mcpServerId":%q,"credentialResolutionMode":"static"}`, serverID)))
			if err != nil || r.JSON200 == nil {
				t.Fatal("unable to modify owned tool assignment", err)
			}
		}, Config: updated, PlanOnly: true, ExpectNonEmptyPlan: true},
		{Config: updated, Check: same},
		{Config: config(""), Check: resource.ComposeTestCheckFunc(same, resource.TestCheckResourceAttr(address, "tools.#", "1"), resource.TestCheckResourceAttr(address, "excluded_tool_ids.#", "1"))},
		{Config: config(`tools=[]
 excluded_tool_ids=[]`), Check: resource.ComposeTestCheckFunc(same, resource.TestCheckResourceAttr(address, "tools.#", "0"), resource.TestCheckResourceAttr(address, "excluded_tool_ids.#", "0"), resource.TestCheckResourceAttr(address, "subagent_ids.#", "1"), resource.TestCheckResourceAttr(address, "remote_agent_ids.#", "1"))},
		{Config: config(`tools=[]
 excluded_tool_ids=[]`), PlanOnly: true},
	}})
}

func TestRequiredPlatformToolOwnership(t *testing.T) {
	const catalog = "00000000-0000-4000-8000-000000000001"
	for _, name := range []string{"get_remedy_plans", "execute_remedy_plan", "list_peer_messages", "read_peer_message", "yell"} {
		if !requiredPlatformTool(catalog, name) || !requiredPlatformTool(catalog, "custom_brand__"+name) {
			t.Errorf("required tool %s was not protected", name)
		}
		if requiredPlatformTool("11111111-1111-4111-8111-111111111111", "custom_brand__"+name) {
			t.Errorf("custom tool %s was incorrectly protected", name)
		}
	}
	if requiredPlatformTool(catalog, "archestra__search") {
		t.Fatal("ordinary built-in tool was protected")
	}
}
