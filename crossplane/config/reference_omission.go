package config

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"path/filepath"
)

// PreserveReferenceCollectionOmission prevents reference.ToPtrValues(nil) from
// turning an unmanaged omission into an authoritative empty collection.
func PreserveReferenceCollectionOmission(root string) error {
	files, err := filepath.Glob(filepath.Join(root, "apis", "*", "*", "*", "zz_generated.resolvers.go"))
	if err != nil {
		return err
	}
	for _, file := range files {
		if err := preserveResolverOmission(file); err != nil {
			return err
		}
	}
	return nil
}

func resolverExpression(value ast.Expr) string {
	var result bytes.Buffer
	_ = format.Node(&result, token.NewFileSet(), value)
	return result.String()
}

func resolverNilGuard(value ast.Expr) ast.Expr {
	return &ast.BinaryExpr{X: value, Op: token.NEQ, Y: ast.NewIdent("nil")}
}

func resolverCollectionGuard(value ast.Expr) bool {
	outer, ok := value.(*ast.BinaryExpr)
	if !ok || outer.Op != token.LOR {
		return false
	}
	inner, ok := outer.X.(*ast.BinaryExpr)
	if !ok || inner.Op != token.LOR {
		return false
	}
	var fields []string
	for _, expression := range []ast.Expr{inner.X, inner.Y, outer.Y} {
		comparison, ok := expression.(*ast.BinaryExpr)
		if !ok || comparison.Op != token.NEQ || resolverExpression(comparison.Y) != "nil" {
			return false
		}
		fields = append(fields, resolverExpression(comparison.X))
	}
	return fields[1] == fields[0]+"Refs" && fields[2] == fields[0]+"Selector"
}

type referenceOmissionRewriter struct {
	changed bool
	err     error
	file    string
}

func preserveResolverOmission(file string) error {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, file, nil, parser.ParseComments)
	if err != nil {
		return err
	}
	rewriter := referenceOmissionRewriter{file: file}
	ast.Inspect(node, rewriter.visit)
	if rewriter.err != nil {
		return rewriter.err
	}
	if !rewriter.changed {
		return nil
	}
	return writeGeneratedCollectionTypes(file, fset, node)
}
func (r *referenceOmissionRewriter) visit(n ast.Node) bool {
	if conditional, ok := n.(*ast.IfStmt); ok && resolverCollectionGuard(conditional.Cond) {
		return false
	}
	block, ok := n.(*ast.BlockStmt)
	if !ok {
		return true
	}
	for index := 0; index < len(block.List); index++ {
		request := multiReferenceRequest(block.List[index])
		if request == nil {
			continue
		}
		guard, err := multiReferenceGuard(request)
		if err != nil {
			r.err = fmt.Errorf("%s: %w", r.file, err)
			return false
		}
		if !referenceSequenceValid(block.List[index:]) {
			r.err = fmt.Errorf("%s: unexpected generated reference assignment sequence", r.file)
			return false
		}
		body := append([]ast.Stmt(nil), block.List[index:index+4]...)
		rest := append([]ast.Stmt(nil), block.List[index+4:]...)
		block.List = append(block.List[:index], &ast.IfStmt{Cond: guard, Body: &ast.BlockStmt{List: body}})
		block.List = append(block.List, rest...)
		r.changed = true
	}
	return true
}
func multiReferenceRequest(statement ast.Stmt) *ast.CompositeLit {
	assignment, ok := statement.(*ast.AssignStmt)
	if !ok || len(assignment.Rhs) != 1 {
		return nil
	}
	call, ok := assignment.Rhs[0].(*ast.CallExpr)
	if !ok {
		return nil
	}
	method, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || method.Sel.Name != "ResolveMultiple" || len(call.Args) != 2 {
		return nil
	}
	request, _ := call.Args[1].(*ast.CompositeLit)
	return request
}
func referenceRequestFields(request *ast.CompositeLit) map[string]ast.Expr {
	fields := map[string]ast.Expr{}
	for _, element := range request.Elts {
		pair, ok := element.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		key, ok := pair.Key.(*ast.Ident)
		if ok {
			fields[key.Name] = pair.Value
		}
	}
	return fields
}
func multiReferenceGuard(request *ast.CompositeLit) (ast.Expr, error) {
	fields := referenceRequestFields(request)
	current, ok := fields["CurrentValues"].(*ast.CallExpr)
	if !ok || len(current.Args) != 1 || fields["References"] == nil || fields["Selector"] == nil {
		return nil, fmt.Errorf("unexpected generated multi-reference resolution")
	}
	guard := &ast.BinaryExpr{X: &ast.BinaryExpr{X: resolverNilGuard(current.Args[0]), Op: token.LOR, Y: resolverNilGuard(fields["References"])}, Op: token.LOR, Y: resolverNilGuard(fields["Selector"])}
	if !resolverCollectionGuard(guard) {
		return nil, fmt.Errorf("unexpected generated reference field names")
	}
	return guard, nil
}
func referenceSequenceValid(statements []ast.Stmt) bool {
	if len(statements) < 4 {
		return false
	}
	if _, ok := statements[1].(*ast.IfStmt); !ok {
		return false
	}
	for _, statement := range statements[2:4] {
		if _, ok := statement.(*ast.AssignStmt); !ok {
			return false
		}
	}
	return true
}
