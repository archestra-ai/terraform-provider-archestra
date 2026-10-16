package provider

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestDefaultLimitFailedAdditionPreservesExistingDefaults(t *testing.T) {
	creates, deletes := 0, 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch req.Method {
		case "GET":
			fmt.Fprint(w, `[{"id":"11111111-1111-4111-8111-111111111111","environmentId":null,"limitValue":10,"model":[],"cleanupInterval":"calendar_month"}]`)
		case "POST":
			creates++
			w.WriteHeader(500)
			fmt.Fprint(w, `{"error":{"message":"synthetic failure"}}`)
		case "DELETE":
			deletes++
			w.WriteHeader(204)
		default:
			t.Errorf("unexpected method %s", req.Method)
		}
	}))
	defer server.Close()
	api, err := client.NewClientWithResponses(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	models, d := types.ListValueFrom(context.Background(), types.StringType, []string{})
	if d.HasError() {
		t.Fatal(d)
	}
	var diagnostics diag.Diagnostics
	(&OrganizationSettingsResource{client: api}).reconcileUserDefaultLimits(t.Context(), map[string]userDefaultLimitModel{"22222222-2222-4222-8222-222222222222": {LimitValue: types.Int64Value(5), Models: models, CleanupInterval: types.StringValue("1h")}}, &diagnostics)
	if !diagnostics.HasError() || creates != 1 || deletes != 0 {
		t.Fatalf("failed addition removed existing defaults: creates=%d deletes=%d diagnostics=%v", creates, deletes, diagnostics)
	}
}
