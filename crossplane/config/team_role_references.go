package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// PreserveTeamRoleReferences avoids a generated team -> identity -> team Go
// import cycle without changing either CRD's public API group or ref markers.
func PreserveTeamRoleReferences(root string) error {
	target := regexp.MustCompile(`To: reference.To\{\s*List:\s*&[a-zA-Z0-9_]+\.RoleList\{\},\s*Managed:\s*&[a-zA-Z0-9_]+\.Role\{\},\s*\}`)
	for _, scope := range []string{"cluster", "namespaced"} {
		file := filepath.Join(root, "apis", scope, "team", "v1alpha1", "zz_generated.resolvers.go")
		data, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		source := string(data)
		if strings.Contains(source, "rolereference.Target(") {
			continue
		}
		if len(target.FindAllString(source, -1)) != 2 {
			return fmt.Errorf("%s: expected forProvider and initProvider role targets", file)
		}
		group := "identity.archestra.crossplane.io"
		if scope == "namespaced" {
			group = "identity.archestra.m.crossplane.io"
		}
		source = target.ReplaceAllString(source, `To: rolereference.Target("`+group+`")`)
		source = regexp.MustCompile(`(?m)^\s*[a-zA-Z0-9_]+ "github.com/archestra-ai/provider-archestra/apis/`+scope+`/identity/v1alpha1"\n`).ReplaceAllString(source, "")
		source = strings.Replace(source, `resource "github.com/crossplane/upjet/v2/pkg/resource"`, `rolereference "github.com/archestra-ai/provider-archestra/internal/rolereference"`, 1)
		source = strings.ReplaceAll(source, `resource.ExtractParamPath("role", true)`, `rolereference.Slug()`)
		source = strings.Replace(source, "reference.NewAPIResolver(c, mg)", "reference.NewAPIResolver(rolereference.Reader(c), mg)", 1)
		if err := os.WriteFile(file, []byte(source), 0644); err != nil {
			return err
		}
	}
	return nil
}
