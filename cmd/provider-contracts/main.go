// Export the actual registered Terraform schemas and wire mappings for audits.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/types"
	"golang.org/x/tools/go/packages"
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
	ResourceSource    string            `json:"resource_source"`
	ImportImplemented bool              `json:"import_implemented"`
	APIMethods        []string          `json:"api_methods"`
	WireFields        map[string]string `json:"wire_fields"`
	SyntheticFields   []string          `json:"synthetic_fields"`
}

func main() {
	ctx := context.Background()
	p := provider.New("audit")()
	contracts := map[string]contract{}
	methods, sources := resourceAPIMethods()
	for _, newResource := range p.Resources(ctx) {
		r := newResource()
		var meta resource.MetadataResponse
		r.Metadata(ctx, resource.MetadataRequest{ProviderTypeName: "archestra"}, &meta)
		_, imports := r.(resource.ResourceWithImportState)
		entry := contract{ResourceSource: sources[reflect.TypeOf(r).Elem().Name()], ImportImplemented: imports, APIMethods: methods[reflect.TypeOf(r).Elem().Name()], WireFields: map[string]string{}, SyntheticFields: []string{}}
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

// Follow typed helper calls so inline owners retain their complete API contract.
func resourceAPIMethods() (map[string][]string, map[string]string) {
	loaded, err := packages.Load(&packages.Config{Mode: packages.NeedName | packages.NeedFiles | packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo | packages.NeedImports | packages.NeedDeps}, "./internal/provider")
	if err != nil {
		panic(err)
	}
	if packages.PrintErrors(loaded) > 0 || len(loaded) != 1 {
		panic("unable to load provider source contract")
	}
	pkg := loaded[0]
	type calls struct {
		api     map[string]bool
		helpers []*types.Func
	}
	graph := map[*types.Func]calls{}
	owners := map[string][]*types.Func{}
	sources := map[string]string{}
	for index, tree := range pkg.Syntax {
		for _, declaration := range tree.Decls {
			fn, ok := declaration.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			object, ok := pkg.TypesInfo.Defs[fn.Name].(*types.Func)
			if !ok {
				continue
			}
			info := calls{api: map[string]bool{}}
			if signature, ok := object.Type().(*types.Signature); ok && signature.Recv() != nil {
				receiver := signature.Recv().Type()
				if pointer, ok := receiver.(*types.Pointer); ok {
					receiver = pointer.Elem()
				}
				if named, ok := receiver.(*types.Named); ok {
					kind := named.Obj().Name()
					owners[kind] = append(owners[kind], object)
					if fn.Name.Name == "Metadata" {
						sources[kind] = "internal/provider/" + filepath.Base(pkg.GoFiles[index])
					}
				}
			}
			ast.Inspect(fn.Body, func(node ast.Node) bool {
				call, ok := node.(*ast.CallExpr)
				if !ok {
					return true
				}
				var target *types.Func
				switch callee := call.Fun.(type) {
				case *ast.SelectorExpr:
					if selection := pkg.TypesInfo.Selections[callee]; selection != nil {
						target, _ = selection.Obj().(*types.Func)
					} else {
						target, _ = pkg.TypesInfo.Uses[callee.Sel].(*types.Func)
					}
				case *ast.Ident:
					target, _ = pkg.TypesInfo.Uses[callee].(*types.Func)
				}
				if target == nil || target.Pkg() == nil {
					return true
				}
				switch target.Pkg().Path() {
				case pkg.PkgPath:
					info.helpers = append(info.helpers, target)
				case pkg.PkgPath[:strings.LastIndex(pkg.PkgPath, "/")] + "/client":
					name := target.Name()
					if strings.HasSuffix(name, "WithResponse") {
						info.api[strings.TrimSuffix(strings.TrimSuffix(name, "WithResponse"), "WithBody")] = true
					}
				}
				return true
			})
			graph[object] = info
		}
	}
	result := map[string][]string{}
	for kind, roots := range owners {
		seen := map[*types.Func]bool{}
		api := map[string]bool{}
		var visit func(*types.Func)
		visit = func(fn *types.Func) {
			if seen[fn] {
				return
			}
			seen[fn] = true
			for name := range graph[fn].api {
				api[name] = true
			}
			for _, helper := range graph[fn].helpers {
				visit(helper)
			}
		}
		for _, root := range roots {
			visit(root)
		}
		for name := range api {
			result[kind] = append(result[kind], name)
		}
		sort.Strings(result[kind])
	}
	return result, sources
}
