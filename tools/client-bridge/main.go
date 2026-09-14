// Extract new operations from a generated subset without duplicating the pinned client.
package main

import (
	"bytes"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"strings"
)

func key(decl ast.Decl) string {
	switch d := decl.(type) {
	case *ast.FuncDecl:
		receiver := ""
		if d.Recv != nil {
			var b bytes.Buffer
			_ = format.Node(&b, token.NewFileSet(), d.Recv.List[0].Type)
			receiver = b.String()
		}
		return receiver + "." + d.Name.Name
	case *ast.GenDecl:
		if d.Tok == token.IMPORT {
			return "imports"
		}
		if len(d.Specs) == 1 {
			if t, ok := d.Specs[0].(*ast.TypeSpec); ok {
				return "type." + t.Name.Name
			}
		}
	}
	return ""
}

func main() {
	fs := token.NewFileSet()
	base, err := parser.ParseFile(fs, os.Args[1], nil, parser.ParseComments)
	if err != nil {
		panic(err)
	}
	subset, err := parser.ParseFile(fs, os.Args[2], nil, parser.ParseComments)
	if err != nil {
		panic(err)
	}
	existing := map[string]bool{}
	for _, d := range base.Decls {
		existing[key(d)] = true
	}
	var out bytes.Buffer
	out.WriteString("// Code generated from the platform credential/tool-exclusion OpenAPI subset by oapi-codegen and tools/client-bridge. DO NOT EDIT.\npackage client\n\n")
	for _, d := range subset.Decls {
		k := key(d)
		if k != "imports" && k != "" && existing[k] {
			continue
		}
		if err := format.Node(&out, fs, d); err != nil {
			panic(err)
		}
		out.WriteString("\n\n")
	}
	result := strings.ReplaceAll(out.String(), "rsp, err := c.", "rsp, err := c.ClientInterface.(*Client).")
	formatted, err := format.Source([]byte(result))
	if err != nil {
		panic(err)
	}
	if err := os.WriteFile(os.Args[3], formatted, 0644); err != nil {
		panic(err)
	}
}
