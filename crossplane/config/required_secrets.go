package config

import (
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	ujconfig "github.com/crossplane/upjet/v2/pkg/config"
	"github.com/crossplane/upjet/v2/pkg/types/name"
)

// PreserveRequiredSecretSources makes required scalar Secret inputs available
// from either parameter source. Upjet emits these selectors as value fields
// and requires forProvider even though initProvider has the same input. An
// omitted value selector also serializes an empty Secret and overrides init.
func preserveRequiredSecretSources(root, scope string, r *ujconfig.Resource, shape collectionSchemaResource) error {
	fields := map[string]bool{}
	for key, attribute := range shape.Block.Attributes {
		if attribute.Required && attribute.Sensitive && string(attribute.Type) == `"string"` {
			fields[name.NewFromSnake(key).Camel+"SecretRef"] = true
		}
	}
	if len(fields) == 0 {
		return nil
	}
	files, err := filepath.Glob(filepath.Join(root, "apis", scope, r.ShortGroup, "*", "zz_"+strings.ToLower(r.Kind)+"_types.go"))
	if err != nil {
		return err
	}
	for _, file := range files {
		if err := rewriteRequiredSecretSources(file, fields); err != nil {
			return err
		}
	}
	return nil
}

func rewriteRequiredSecretSources(file string, fields map[string]bool) error {
	// #nosec G304 -- Only generated type paths from the controlled apis glob reach this function.
	raw, err := os.ReadFile(file)
	if err != nil {
		return err
	}
	source := string(raw)
	for field := range fields {
		pattern := regexp.MustCompile(`(?m)^\t` + regexp.QuoteMeta(field) + `\s+(\*?[\w.]+) ` + "`" + `json:"([^",]+)(,omitempty)?" tf:"-"` + "`" + `$`)
		matches := pattern.FindAllStringSubmatch(source, -1)
		if len(matches) != 2 {
			return fmt.Errorf("expected both required Secret parameter fields for %s in %s, found %d", field, file, len(matches))
		}
		jsonName := matches[0][2]
		source = pattern.ReplaceAllStringFunc(source, func(line string) string {
			parts := pattern.FindStringSubmatch(line)
			if strings.HasPrefix(parts[1], "*") {
				return line
			}
			return "\t// +kubebuilder:validation:Optional\n\t" + field + " *" + parts[1] + " `json:\"" + jsonName + ",omitempty\" tf:\"-\"`"
		})
		old := `|| has(self.forProvider.` + jsonName + `)",message=`
		updated := `|| has(self.forProvider.` + jsonName + `) || (has(self.initProvider) && has(self.initProvider.` + jsonName + `))",message=`
		if strings.Contains(source, old) {
			source = strings.ReplaceAll(source, old, updated)
		} else if !strings.Contains(source, updated) {
			return fmt.Errorf("required Secret validation was not found for %s in %s", field, file)
		}
	}
	formatted, err := format.Source([]byte(source))
	if err != nil {
		return err
	}
	return os.WriteFile(file, formatted, 0o600)
}
