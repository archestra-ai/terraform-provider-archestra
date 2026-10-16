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
		NestingMode string                               `json:"nesting_mode"`
		Attributes  map[string]collectionSchemaAttribute `json:"attributes"`
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
			if err := preserveResourceCollections(root, scope, r, resources[name]); err != nil {
				return err
			}
		}
	}
	return nil
}

func preserveResourceCollections(root, scope string, r *ujconfig.Resource, shape collectionSchemaResource) error {
	required := requiredCollectionNames(shape)
	optional := optionalOwnedCollections(r.Name, shape)
	for _, name := range optional {
		required[name] = false
	}
	files, err := filepath.Glob(filepath.Join(root, "apis", scope, r.ShortGroup, "*", "zz_"+strings.ToLower(r.Kind)+"_types.go"))
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return fmt.Errorf("no generated %s %s types", scope, r.Kind)
	}
	for _, file := range files {
		if err := preserveCollectionFile(file, r.Kind, shape, required, optional); err != nil {
			return err
		}
	}
	return nil
}
func optionalOwnedCollections(name string, shape collectionSchemaResource) []string {
	optional := optionalCredentialCollections(name)
	for name, attribute := range shape.Block.Attributes {
		if attribute.Optional && attribute.Computed && isCollection(attribute) && name != "initial_grants" {
			optional = append(optional, name)
		}
	}
	sort.Strings(optional)
	return optional
}
func isCollection(attribute collectionSchemaAttribute) bool {
	switch collectionMode(attribute) {
	case "set", "list", "map":
		return true
	}
	return false
}
func preserveCollectionFile(file, kind string, shape collectionSchemaResource, required map[string]bool, optional []string) error {
	if err := rewriteRequiredCollectionTags(file, kind+"Parameters", required); err != nil {
		return err
	}
	if err := rewriteNestedRequiredCollections(file, kind+"Parameters", shape.Block.Attributes); err != nil {
		return err
	}
	if !required["initial_grants"] && len(optional) == 0 {
		return nil
	}
	initCollections := map[string]bool{}
	if required["initial_grants"] {
		initCollections["initial_grants"] = true
	}
	for _, name := range optional {
		initCollections[name] = false
	}
	if err := rewriteRequiredCollectionTags(file, kind+"InitParameters", initCollections); err != nil {
		return err
	}
	if err := rewriteNestedRequiredCollections(file, kind+"InitParameters", shape.Block.Attributes); err != nil {
		return err
	}
	return normalizeCreationGrantParameterMaps(strings.Replace(file, "_types.go", "_terraformed.go", 1), optional...)
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
	if _, present := required[name]; !present {
		return false
	}
	jsonTag := reflect.StructTag(raw).Get("json")
	tfReplacement, jsonReplacement := strings.ReplaceAll(tag, ",omitempty", ""), strings.ReplaceAll(jsonTag, ",omitempty", "")
	if name == "initial_grants" || !required[name] {
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
	if field.Tag == nil {
		return nil
	}
	raw, err := strconv.Unquote(field.Tag.Value)
	if err != nil {
		return nil
	}
	name, _, _ := strings.Cut(reflect.StructTag(raw).Get("tf"), ",")
	isRequired, present := required[name]
	if !present || (isRequired && (name != "initial_grants" || !strings.HasSuffix(parameters, "InitParameters"))) {
		return nil
	}
	if field.Doc != nil && strings.Contains(field.Doc.Text(), "+kubebuilder:validation:Optional") {
		return nil
	}
	comment := &ast.Comment{Slash: field.Pos() - 1, Text: "// +kubebuilder:validation:Optional"}
	if field.Doc != nil {
		field.Doc.List = append(field.Doc.List, comment)
		return nil
	}
	field.Doc = &ast.CommentGroup{List: []*ast.Comment{comment}}
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
func normalizeCreationGrantParameterMaps(file string, optional ...string) error {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, file, nil, parser.ParseComments)
	if err != nil {
		return err
	}
	keys := append([]string{"initial_grants"}, optional...)
	quoted := quotedCollectionKeys(keys)
	snippet, err := parser.ParseFile(token.NewFileSet(), "normalize.go", `package normalize
func normalize() {
 err = json.TFParser.Unmarshal(p, &base)
 for _, key := range []string{`+strings.Join(quoted, ",")+`} {
  if base[key] == nil { delete(base, key) }
 }
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

// These optional collections distinguish an unmanaged omission from an explicit empty set.
func optionalCredentialCollections(name string) []string {
	switch name {
	case "archestra_plugin", "archestra_git_plugin":
		return []string{"labels"}
	case "archestra_virtual_api_key":
		return []string{"provider_api_keys", "labels"}
	case "archestra_oauth_client":
		return []string{"provider_api_keys", "labels", "allowed_gateway_ids", "redirect_uris"}
	default:
		return nil
	}
}

// Nested authoritative sets must survive JSON and TF serialization when empty.

func rewriteNestedRequiredCollections(file, parameters string, attributes map[string]collectionSchemaAttribute) error {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, file, nil, parser.ParseComments)
	if err != nil {
		return err
	}
	rewriter := nestedCollectionRewriter{structures: generatedStructures(node)}
	rewriter.visit(parameters, attributes)
	if !rewriter.changed {
		return nil
	}
	return writeGeneratedCollectionTypes(file, fset, node)
}

type nestedCollectionRewriter struct {
	structures map[string]*ast.StructType
	changed    bool
}

func generatedStructures(node *ast.File) map[string]*ast.StructType {
	structures := map[string]*ast.StructType{}
	ast.Inspect(node, func(n ast.Node) bool {
		spec, ok := n.(*ast.TypeSpec)
		if !ok {
			return true
		}
		structure, ok := spec.Type.(*ast.StructType)
		if ok {
			structures[spec.Name.Name] = structure
		}
		return false
	})
	return structures
}
func generatedFieldName(field *ast.Field) string {
	if field.Tag == nil {
		return ""
	}
	raw, err := strconv.Unquote(field.Tag.Value)
	if err != nil {
		return ""
	}
	key, _, _ := strings.Cut(reflect.StructTag(raw).Get("tf"), ",")
	return key
}
func generatedFieldType(target ast.Expr) string {
	switch value := target.(type) {
	case *ast.ArrayType:
		return generatedFieldType(value.Elt)
	case *ast.StarExpr:
		return generatedFieldType(value.X)
	case *ast.Ident:
		return value.Name
	}
	return ""
}
func nestedRequiredCollections(attributes map[string]collectionSchemaAttribute) map[string]bool {
	required := map[string]bool{}
	for child, definition := range attributes {
		if definition.Required && isCollection(definition) {
			required[child] = true
		}
	}
	return required
}
func (r *nestedCollectionRewriter) visit(name string, attributes map[string]collectionSchemaAttribute) {
	structure := r.structures[name]
	if structure == nil {
		return
	}
	for _, field := range structure.Fields.List {
		attribute := attributes[generatedFieldName(field)]
		if attribute.NestedType == nil {
			continue
		}
		kind := generatedFieldType(field.Type)
		nested := r.structures[kind]
		if nested == nil {
			continue
		}
		required := nestedRequiredCollections(attribute.NestedType.Attributes)
		for _, child := range nested.Fields.List {
			if preserveRequiredFieldTag(child, required) {
				r.changed = true
			}
		}
		r.visit(kind, attribute.NestedType.Attributes)
	}
}

func quotedCollectionKeys(keys []string) []string {
	quoted := make([]string, len(keys))
	for i, key := range keys {
		quoted[i] = strconv.Quote(key)
	}
	return quoted
}
