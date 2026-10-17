package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRequiredCollectionTags(t *testing.T) {
	file := filepath.Join(t.TempDir(), "types.go")
	source := "package example\ntype PolicyParameters struct {\n Required []string `json:\"required,omitempty\" tf:\"required,omitempty\"`\n Optional []string `json:\"optional,omitempty\" tf:\"optional,omitempty\"`\n}\ntype PolicyInitParameters struct {\n Required []string `tf:\"required,omitempty\"`\n}\n"
	if err := os.WriteFile(file, []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	if err := rewriteRequiredCollectionTags(file, "PolicyParameters", map[string]bool{"required": true}); err != nil {
		t.Fatal(err)
	}
	result, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	text := string(result)
	for _, expected := range []string{`json:"required" tf:"required"`, `json:"optional,omitempty" tf:"optional,omitempty"`, `tf:"required,omitempty"`} {
		if !strings.Contains(text, expected) {
			t.Fatalf("missing contract %q: %s", expected, text)
		}
	}
	if err := rewriteRequiredCollectionTags(file, "PolicyParameters", map[string]bool{"required": true}); err != nil {
		t.Fatal(err)
	}
	again, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if string(again) != text {
		t.Fatal("rewrite is not idempotent")
	}
}
