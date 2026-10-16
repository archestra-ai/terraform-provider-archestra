package provider

import (
	"context"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"reflect"
	"testing"
)

func TestListBackedResourceTimestampImport(t *testing.T) {
	// The creation JSON and generated list response represent the same millisecond.
	bodies := []string{
		`{"id":"11111111-1111-4111-8111-111111111111","agentId":"22222222-2222-4222-8222-222222222222","model":[],"createdAt":"2026-10-16T13:30:50.600Z","updatedAt":"2026-10-16T13:30:50.600Z"}`,
		`{"id":"11111111-1111-4111-8111-111111111111","agentId":"22222222-2222-4222-8222-222222222222","model":[],"createdAt":"2026-10-16T13:30:50.6Z","updatedAt":"2026-10-16T13:30:50.6Z"}`,
	}
	for _, kind := range []string{"default limit", "hook", "directory", "service account token"} {
		t.Run(kind, func(t *testing.T) {
			var prior any
			for _, body := range bodies {
				var d diag.Diagnostics
				var current any
				switch kind {
				case "default limit":
					model := DefaultUserLimitResourceModel{}
					(&DefaultUserLimitResource{}).flatten(context.Background(), &model, []byte(body), &d)
					current = model
				case "hook":
					model := HookResourceModel{}
					(&HookResource{}).flatten(context.Background(), &model, []byte(body), &d)
					current = model
				case "service account token":
					model := ServiceAccountTokenResourceModel{}
					(&ServiceAccountTokenResource{}).flatten(&model, []byte(body), &d)
					current = model
				case "directory":
					model := KnowledgeDirectoryResourceModel{}
					(&KnowledgeDirectoryResource{}).flatten(context.Background(), &model, []byte(body), &d)
					current = model
				}
				if d.HasError() {
					t.Fatal(d)
				}
				if prior != nil && !reflect.DeepEqual(prior, current) {
					t.Fatalf("creation and import states differ: %#v / %#v", prior, current)
				}
				prior = current
			}
		})
	}
}
