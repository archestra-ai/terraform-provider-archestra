package config

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"

	ujconfig "github.com/crossplane/upjet/v2/pkg/config"
)

type collectionSchemaAttribute struct {
	Optional   bool            `json:"optional"`
	Computed   bool            `json:"computed"`
	Required   bool            `json:"required"`
	Type       json.RawMessage `json:"type"`
	NestedType *struct {
		NestingMode string `json:"nesting_mode"`
	} `json:"nested_type"`
}
type collectionSchemaResource struct {
	Block struct {
		Attributes map[string]collectionSchemaAttribute `json:"attributes"`
	} `json:"block"`
}

// PreserveRequiredCollections retains explicit empty authoritative collections
// in both Terraform parameters and Kubernetes updates instead of dropping them.
func PreserveRequiredCollections(root string, providers ...*ujconfig.Provider) error {
	var snapshot struct {
		ProviderSchemas map[string]struct {
			ResourceSchemas map[string]collectionSchemaResource `json:"resource_schemas"`
		} `json:"provider_schemas"`
	}
	if err := json.Unmarshal([]byte(providerSchema), &snapshot); err != nil {
		return err
	}
	resources := snapshot.ProviderSchemas["registry.terraform.io/archestra-ai/archestra"].ResourceSchemas
	for i, pc := range providers {
		scope := "cluster"
		if i == 1 {
			scope = "namespaced"
		}
		for name, r := range pc.Resources {
			if err := preserveResourceCollections(root, scope, r, requiredCollectionNames(resources[name])); err != nil {
				return err
			}
		}
	}
	return nil
}
func preserveResourceCollections(root, scope string, r *ujconfig.Resource, required map[string]bool) error {
	if len(required) == 0 {
		return nil
	}
	files, err := filepath.Glob(filepath.Join(root, "apis", scope, r.ShortGroup, "*", "zz_"+strings.ToLower(r.Kind)+"_types.go"))
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return fmt.Errorf("no generated %s %s types", scope, r.Kind)
	}
	for _, file := range files {
		if err := rewriteRequiredCollectionTags(file, r.Kind+"Parameters", required); err != nil {
			return err
		}
		if required["initial_grants"] {
			if err := rewriteRequiredCollectionTags(file, r.Kind+"InitParameters", map[string]bool{"initial_grants": true}); err != nil {
				return err
			}
			if err := normalizeCreationGrantParameterMaps(strings.Replace(file, "_types.go", "_terraformed.go", 1)); err != nil {
				return err
			}
		}
	}
	return nil
}
func collectionMode(attribute collectionSchemaAttribute) string {
	if attribute.NestedType != nil {
		return attribute.NestedType.NestingMode
	}
	var shape []json.RawMessage
	if json.Unmarshal(attribute.Type, &shape) != nil || len(shape) == 0 {
		return ""
	}
	var mode string
	_ = json.Unmarshal(shape[0], &mode)
	return mode
}

func requiredCollectionNames(resource collectionSchemaResource) map[string]bool {
	result := map[string]bool{}
	for name, attribute := range resource.Block.Attributes {
		// Empty creation grants must not fall through to API defaults. Null remains unmanaged.
		creationGrants := name == "initial_grants" && attribute.Optional && attribute.Computed
		if !attribute.Required && !creationGrants {
			continue
		}
		mode := collectionMode(attribute)
		if mode == "set" || mode == "list" || mode == "map" {
			result[name] = true
		}
	}
	return result
}
func preserveRequiredFieldTag(field *ast.Field, required map[string]bool) bool {
	if field.Tag == nil {
		return false
	}
	raw, err := strconv.Unquote(field.Tag.Value)
	if err != nil {
		return false
	}
	tag := reflect.StructTag(raw).Get("tf")
	name, _, _ := strings.Cut(tag, ",")
	if !required[name] {
		return false
	}
	jsonTag := reflect.StructTag(raw).Get("json")
	tfReplacement, jsonReplacement := strings.ReplaceAll(tag, ",omitempty", ""), strings.ReplaceAll(jsonTag, ",omitempty", "")
	if name == "initial_grants" {
		jsonReplacement = strings.ReplaceAll(jsonTag, ",omitempty", ",omitzero")
	}
	if tfReplacement == tag && jsonReplacement == jsonTag {
		return false
	}
	raw = strings.Replace(raw, `tf:"`+tag+`"`, `tf:"`+tfReplacement+`"`, 1)
	raw = strings.Replace(raw, `json:"`+jsonTag+`"`, `json:"`+jsonReplacement+`"`, 1)
	field.Tag.Value = "`" + raw + "`"
	return true
}
func rewriteRequiredCollectionTags(file, parameters string, required map[string]bool) error {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, file, nil, parser.ParseComments)
	if err != nil {
		return err
	}
	changed := false
	ast.Inspect(node, func(n ast.Node) bool {
		spec, ok := n.(*ast.TypeSpec)
		if !ok || spec.Name.Name != parameters {
			return true
		}
		structure, ok := spec.Type.(*ast.StructType)
		if !ok {
			return false
		}
		for _, field := range structure.Fields.List {
			if preserveRequiredFieldTag(field, required) {
				changed = true
			}
			if comment := markOptionalInitialGrants(parameters, field, required); comment != nil {
				node.Comments = append(node.Comments, comment)
				changed = true
			}
		}
		return false
	})
	if !changed {
		return nil
	}
	sort.Slice(node.Comments, func(i, j int) bool { return node.Comments[i].Pos() < node.Comments[j].Pos() })
	return writeGeneratedCollectionTypes(file, fset, node)
}
func markOptionalInitialGrants(parameters string, field *ast.Field, required map[string]bool) *ast.CommentGroup {
	if !strings.HasSuffix(parameters, "InitParameters") || len(field.Names) == 0 || field.Names[0].Name != "InitialGrants" || !required["initial_grants"] || field.Doc != nil {
		return nil
	}
	field.Doc = &ast.CommentGroup{List: []*ast.Comment{{Slash: field.Pos() - 1, Text: "// +kubebuilder:validation:Optional"}}}
	return field.Doc
}

func writeGeneratedCollectionTypes(file string, fset *token.FileSet, node *ast.File) error {
	root, err := os.OpenRoot(filepath.Dir(file))
	if err != nil {
		return err
	}
	defer func() { _ = root.Close() }()
	out, err := root.OpenFile(filepath.Base(file), os.O_WRONLY|os.O_TRUNC, 0)
	if err != nil {
		return err
	}
	if err := format.Node(out, fset, node); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}

// Omitted creation grants must be absent from parameter maps. Upjet compares
// initProvider collection types before merging, so a present nil value panics.
func normalizeCreationGrantParameterMaps(file string) error {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, file, nil, parser.ParseComments)
	if err != nil {
		return err
	}
	snippet, err := parser.ParseFile(token.NewFileSet(), "normalize.go", `package normalize
func normalize() {
 err = json.TFParser.Unmarshal(p, &base)
 if base["initial_grants"] == nil { delete(base, "initial_grants") }
 return base, err
}`, 0)
	if err != nil {
		return err
	}
	statements := snippet.Decls[0].(*ast.FuncDecl).Body.List
	changed := false
	for _, declaration := range node.Decls {
		method, ok := declaration.(*ast.FuncDecl)
		if !ok || (method.Name.Name != "GetParameters" && method.Name.Name != "GetInitParameters") {
			continue
		}
		replaced, err := replaceCreationGrantReturn(method, statements)
		if err != nil {
			return fmt.Errorf("%s: %w", file, err)
		}
		changed = changed || replaced
	}
	if !changed {
		return nil
	}
	return writeGeneratedCollectionTypes(file, fset, node)
}

func replaceCreationGrantReturn(method *ast.FuncDecl, statements []ast.Stmt) (bool, error) {
	last := len(method.Body.List) - 1
	result, ok := method.Body.List[last].(*ast.ReturnStmt)
	if !ok || len(result.Results) != 2 {
		return false, fmt.Errorf("unexpected generated parameter method %s", method.Name.Name)
	}
	if _, ok := result.Results[1].(*ast.CallExpr); !ok {
		return false, nil
	}
	method.Body.List = append(method.Body.List[:last], statements...)
	return true, nil
}
