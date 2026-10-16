// Export the actual registered Terraform schemas and wire mappings for audits.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/provider"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

type wireSpec interface{ AttrSpecs() []provider.AttrSpec }

type contract struct {
	APIMethods      []string          `json:"api_methods"`
	WireFields      map[string]string `json:"wire_fields"`
	SyntheticFields []string          `json:"synthetic_fields"`
}

func main() {
	ctx := context.Background()
	p := provider.New("audit")()
	contracts := map[string]contract{}
	methods := resourceAPIMethods()
	for _, newResource := range p.Resources(ctx) {
		r := newResource()
		var meta resource.MetadataResponse
		r.Metadata(ctx, resource.MetadataRequest{ProviderTypeName: "archestra"}, &meta)
		entry := contract{APIMethods: methods[reflect.TypeOf(r).Elem().Name()], WireFields: map[string]string{}, SyntheticFields: []string{}}
		if specified, ok := r.(wireSpec); ok {
			for _, spec := range specified.AttrSpecs() {
				if spec.Kind == provider.Synthetic {
					entry.SyntheticFields = append(entry.SyntheticFields, spec.TFName)
					continue
				}
				if spec.JSONName != "" {
					entry.WireFields[spec.JSONName] = spec.TFName
				}
			}
		}
		contracts[meta.TypeName] = entry
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(contracts); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func resourceAPIMethods() map[string][]string {
	result := map[string][]string{}
	files, err := filepath.Glob("internal/provider/*.go")
	if err != nil {
		panic(err)
	}
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		tree, err := parser.ParseFile(token.NewFileSet(), file, nil, 0)
		if err != nil {
			panic(err)
		}
		for _, declaration := range tree.Decls {
			fn, ok := declaration.(*ast.FuncDecl)
			if !ok || fn.Recv == nil || len(fn.Recv.List) != 1 || len(fn.Recv.List[0].Names) != 1 {
				continue
			}
			receiver := fn.Recv.List[0]
			pointer, ok := receiver.Type.(*ast.StarExpr)
			if !ok {
				continue
			}
			kind, ok := pointer.X.(*ast.Ident)
			if !ok {
				continue
			}
			ast.Inspect(fn.Body, func(node ast.Node) bool {
				selector, ok := node.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				owner, ok := selector.X.(*ast.SelectorExpr)
				if !ok || owner.Sel.Name != "client" {
					return true
				}
				variable, ok := owner.X.(*ast.Ident)
				if !ok || variable.Name != receiver.Names[0].Name {
					return true
				}
				name := selector.Sel.Name
				if !strings.HasSuffix(name, "WithResponse") {
					return true
				}
				name = strings.TrimSuffix(strings.TrimSuffix(name, "WithResponse"), "WithBody")
				result[kind.Name] = append(result[kind.Name], name)
				return true
			})
		}
	}
	for name, methods := range result {
		sort.Strings(methods)
		unique := []string{}
		for _, method := range methods {
			if len(unique) == 0 || method != unique[len(unique)-1] {
				unique = append(unique, method)
			}
		}
		result[name] = unique
	}
	return result
}
