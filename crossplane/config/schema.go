package config

import (
	"encoding/json"

	tfjson "github.com/hashicorp/terraform-json"
)

// Upjet's legacy block converter treats single blocks as lists. Framework
// single blocks have object values in both configuration and saved state.
func normalizedProviderSchema(raw string) []byte {
	var document tfjson.ProviderSchemas
	if err := json.Unmarshal([]byte(raw), &document); err != nil {
		panic(err)
	}
	for _, p := range document.Schemas {
		for _, resource := range p.ResourceSchemas {
			normalizeSingleBlocks(resource.Block)
		}
	}
	result, err := json.Marshal(document)
	if err != nil {
		panic(err)
	}
	return result
}

func normalizeSingleBlocks(block *tfjson.SchemaBlock) {
	if block == nil {
		return
	}
	for name, nested := range block.NestedBlocks {
		normalizeSingleBlocks(nested.Block)
		if nested.NestingMode != tfjson.SchemaNestingModeSingle && nested.NestingMode != tfjson.SchemaNestingModeGroup {
			continue
		}
		if block.Attributes == nil {
			block.Attributes = map[string]*tfjson.SchemaAttribute{}
		}
		block.Attributes[name] = attributeForBlock(nested)
		delete(block.NestedBlocks, name)
	}
}

func attributeForBlock(nested *tfjson.SchemaBlockType) *tfjson.SchemaAttribute {
	attributes := map[string]*tfjson.SchemaAttribute{}
	for name, attribute := range nested.Block.Attributes {
		attributes[name] = attribute
	}
	for name, child := range nested.Block.NestedBlocks {
		attributes[name] = attributeForBlock(child)
	}
	return &tfjson.SchemaAttribute{
		Optional: nested.MinItems == 0, Required: nested.MinItems > 0,
		Description: nested.Block.Description, DescriptionKind: nested.Block.DescriptionKind, Deprecated: nested.Block.Deprecated,
		AttributeNestedType: &tfjson.SchemaNestedAttributeType{Attributes: attributes, NestingMode: nested.NestingMode, MinItems: nested.MinItems, MaxItems: nested.MaxItems},
	}
}
