package provider

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
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
