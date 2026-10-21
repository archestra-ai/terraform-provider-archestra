package config

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"

	ujconfig "github.com/crossplane/upjet/v2/pkg/config"
	"github.com/crossplane/upjet/v2/pkg/types/name"
)

// ignoreInitOnlySecretSources keeps Secret inputs given only in initProvider
// creation-only. Upjet drops Secret selectors (tf:"-") from the init parameter
// map, so their Terraform argument never enters ignore_changes and the Secret
// value is re-applied on every reconcile. Exposing the argument key makes
// upjet ignore it; the Secret value still replaces the placeholder at creation.
func ignoreInitOnlySecretSources(root, scope string, r *ujconfig.Resource, shape collectionSchemaResource) error {
	arguments := sensitiveStringArguments(shape)
	if len(arguments) == 0 {
		return nil
	}
	files, err := filepath.Glob(filepath.Join(root, "apis", scope, r.ShortGroup, "*", "zz_"+strings.ToLower(r.Kind)+"_types.go"))
	if err != nil {
		return err
	}
	for _, file := range files {
		fields, err := initSecretSelectorFields(file, arguments)
		if err != nil {
			return err
		}
		if len(fields) == 0 {
			continue
		}
		if err := markInitOnlySecretArguments(strings.Replace(file, "_types.go", "_terraformed.go", 1), arguments, fields); err != nil {
			return err
		}
	}
	return nil
}

func sensitiveStringArguments(shape collectionSchemaResource) []string {
	var arguments []string
	for key, attribute := range shape.Block.Attributes {
		if attribute.Sensitive && string(attribute.Type) == `"string"` && (attribute.Required || attribute.Optional) {
			arguments = append(arguments, key)
		}
	}
	sort.Strings(arguments)
	return arguments
}

// initSecretSelectorFields maps arguments to Secret selectors that are
// optional pointers in both forProvider and initProvider.
func initSecretSelectorFields(file string, arguments []string) (map[string]string, error) {
	// #nosec G304 -- Only generated type paths from the controlled apis glob reach this function.
	raw, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	fields := map[string]string{}
	for _, argument := range arguments {
		field := name.NewFromSnake(argument).Camel + "SecretRef"
		pointer := regexp.MustCompile(`(?m)^\t` + regexp.QuoteMeta(field) + `\s+\*[\w.]+ ` + "`" + `json:"[^"]+" tf:"-"` + "`" + `$`)
		if len(pointer.FindAllString(string(raw), -1)) == 2 {
			fields[argument] = field
		}
	}
	return fields, nil
}

func markInitOnlySecretArguments(file string, arguments []string, fields map[string]string) error {
	statements, err := initSecretMarkStatements(arguments, fields)
	if err != nil {
		return err
	}
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, file, nil, parser.ParseComments)
	if err != nil {
		return err
	}
	for _, declaration := range node.Decls {
		method, ok := declaration.(*ast.FuncDecl)
		if !ok || method.Name.Name != "GetInitParameters" {
			continue
		}
		last := len(method.Body.List) - 1
		result, ok := method.Body.List[last].(*ast.ReturnStmt)
		if !ok || len(result.Results) != 2 {
			return fmt.Errorf("%s: unexpected generated GetInitParameters", file)
		}
		body := slices.Clone(method.Body.List[:last])
		if call, ok := result.Results[1].(*ast.CallExpr); ok {
			body = append(body, &ast.AssignStmt{Lhs: []ast.Expr{ast.NewIdent("err")}, Tok: token.ASSIGN, Rhs: []ast.Expr{call}})
		}
		body = append(body, statements...)
		method.Body.List = body
		return writeGeneratedCollectionTypes(file, fset, node)
	}
	return fmt.Errorf("%s: GetInitParameters not found", file)
}

func initSecretMarkStatements(arguments []string, fields map[string]string) ([]ast.Stmt, error) {
	var checks strings.Builder
	for _, argument := range arguments {
		if field, ok := fields[argument]; ok {
			fmt.Fprintf(&checks, " if tr.Spec.InitProvider.%[1]s != nil && tr.Spec.ForProvider.%[1]s == nil { base[%[2]s] = \"\" }\n", field, strconv.Quote(argument))
		}
	}
	snippet, err := parser.ParseFile(token.NewFileSet(), "mark.go", "package mark\nfunc mark() {\n"+checks.String()+" return base, err\n}", 0)
	if err != nil {
		return nil, err
	}
	return snippet.Decls[0].(*ast.FuncDecl).Body.List, nil
}
