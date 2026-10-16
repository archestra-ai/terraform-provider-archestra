package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestPermissionPolicyConflictDoesNotRetry(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		calls++
		if req.Method != "PUT" {
			t.Errorf("method=%s", req.Method)
		}
		var body struct {
			Revision int64                 `json:"revision"`
			Grants   []permissionGrantWire `json:"grants"`
		}
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		if body.Revision != 7 {
			t.Errorf("revision=%d", body.Revision)
		}
		if body.Grants == nil {
			t.Error("revoke must encode an empty array, not null")
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(409)
		fmt.Fprint(w, `{"error":{"message":"Revision conflict"}}`)
	}))
	defer server.Close()
	c, err := client.NewClientWithResponses(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	r := ResourcePermissionPolicyResource{client: c}
	state := ResourcePermissionPolicyResourceModel{ResourceType: types.StringValue("environment"), ResourceID: types.StringValue("*")}
	var diags diag.Diagnostics
	got := r.writePolicy(t.Context(), &state, 7, []permissionGrantWire{}, &diags)
	if got != nil || !diags.HasError() {
		t.Fatalf("expected conflict: %v", diags)
	}
	if calls != 1 {
		t.Fatalf("concurrent change was retried %d times", calls)
	}
	if !strings.Contains(diags.Errors()[0].Summary(), "Revision Conflict") {
		t.Fatalf("unexpected error: %v", diags)
	}
}

func TestAccResourcePermissionPolicyResource(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("acceptance tests are disabled")
	}
	testAccPreCheck(t)
	ctx := context.Background()
	c, err := client.NewClientWithResponses(os.Getenv("ARCHESTRA_BASE_URL"), client.WithRequestEditorFn(func(_ context.Context, req *http.Request) error {
		req.Header.Set("Authorization", os.Getenv("ARCHESTRA_API_KEY"))
		return nil
	}))
	if err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(map[string]string{"name": "tf-policy-" + uuid.NewString()[:8]})
	created, err := c.CreateEnvironmentWithBodyWithResponse(ctx, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if created.JSON200 == nil {
		t.Fatalf("create test environment status %d", created.StatusCode())
	}
	id := created.JSON200.Id
	t.Cleanup(func() {
		deleted, err := c.DeleteEnvironmentWithResponse(ctx, id)
		if err != nil {
			t.Errorf("delete fixture: %v", err)
		} else if deleted.StatusCode() != 200 && deleted.StatusCode() != 404 {
			t.Errorf("delete fixture status %d", deleted.StatusCode())
		}
	})
	configuration := func(actions string) string {
		return fmt.Sprintf(`
resource "archestra_resource_permission_policy" "test" {
 resource_type = "environment"
 resource_id = %q
 grants = [{subject_type="organization", subject_id="*", actions=%s}]
}`, id.String(), actions)
	}
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(_ *terraform.State) error {
			response, err := c.GetResourcePermissionsWithResponse(ctx, "environment", id.String())
			if err != nil {
				return err
			}
			if response.StatusCode() != 200 {
				return fmt.Errorf("target environment must still exist, got %d", response.StatusCode())
			}
			var policy permissionPolicyWire
			if err := json.Unmarshal(response.Body, &policy); err != nil {
				return err
			}
			if len(policy.Grants) != 0 {
				return fmt.Errorf("destroy left %d direct grants", len(policy.Grants))
			}
			return nil
		},
		Steps: []resource.TestStep{
			{Config: configuration(`["read","use"]`), Check: resource.ComposeTestCheckFunc(resource.TestCheckResourceAttr("archestra_resource_permission_policy.test", "id", "environment/"+id.String()), resource.TestCheckResourceAttr("archestra_resource_permission_policy.test", "grants.#", "1"))},
			{ResourceName: "archestra_resource_permission_policy.test", ImportState: true, ImportStateVerify: true},
			{Config: configuration(`["read"]`)},
			{Config: configuration(`["read"]`), PlanOnly: true},
		},
	})
}
