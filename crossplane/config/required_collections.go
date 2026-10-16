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
	"strconv"
	"strings"

	ujconfig "github.com/crossplane/upjet/v2/pkg/config"
)

type collectionSchemaAttribute struct {
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
			required := requiredCollectionNames(resources[name])
			if len(required) == 0 {
				continue
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
			}
		}
	}
	return nil
}
func requiredCollectionNames(resource collectionSchemaResource) map[string]bool {
	result := map[string]bool{}
	for name, attribute := range resource.Block.Attributes {
		if !attribute.Required {
			continue
		}
		mode := ""
		if attribute.NestedType != nil {
			mode = attribute.NestedType.NestingMode
		} else {
			var shape []json.RawMessage
			if json.Unmarshal(attribute.Type, &shape) == nil && len(shape) > 0 {
				_ = json.Unmarshal(shape[0], &mode)
			}
		}
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
		}
		return false
	})
	if !changed {
		return nil
	}
	return writeGeneratedCollectionTypes(file, fset, node)
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
