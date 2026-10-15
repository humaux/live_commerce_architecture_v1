// Purpose: recognize narrow inline error refusals without inventing route coverage.
// Depends on: stdlib AST/constants/imported net/http constants and handler proof.
// Used by: goHandlerFrame.closure; never changes runtime handlers or route rows.
// Invariant: constant error-only leaves need a proven sibling in their table branch.

package main

import (
	"go/ast"
	"go/constant"
	"go/importer"
	"go/token"
	"go/types"
	"path/filepath"
)

// The complete recognized writer list. Keys use import identity, never spelling
// or inferred body behavior. httperror is the scanned module's internal package.
// Values are the status/message argument indexes; argument zero must be w.
var goRefusalWriters = map[string][2]int{
	"net/http.Error":           {2, 1},
	"internal/httperror.Write": {1, 2},
}

func (f *goHandlerFrame) refusalSymbol(expr ast.Expr) string {
	sel, ok := expr.(*ast.SelectorExpr)
	if !ok {
		return ""
	}
	id, ok := sel.X.(*ast.Ident)
	if !ok || id.Obj != nil {
		return ""
	}
	path := f.Func.File.ImportPaths[id.Name]
	if pkg := f.Proof.Scanner.Packages[path]; pkg != nil && pkg.Path == filepath.Join(f.Proof.Scanner.Root, "internal", "httperror") {
		path = "internal/httperror"
	}
	return path + "." + sel.Sel.Name
}

func (f *goHandlerFrame) refusal(expr ast.Expr) bool {
	switch node := expr.(type) {
	case *ast.ParenExpr:
		return f.refusal(node.X)
	case *ast.CallExpr:
		symbol := f.refusalSymbol(node.Fun)
		if (symbol == "net/http.HandlerFunc" || symbol == "internal/httperror.Middleware") && len(node.Args) == 1 {
			return f.refusal(node.Args[0])
		}
	case *ast.FuncLit:
		slots := goHandlerSlots(node.Type.Params)
		if len(slots) != 2 || slots[0].Name == nil || slots[1].Name == nil || node.Type.Results != nil {
			return false
		}
		if f.refusalSymbol(slots[0].Type) != "net/http.ResponseWriter" {
			return false
		}
		ptr, ok := slots[1].Type.(*ast.StarExpr)
		if !ok || f.refusalSymbol(ptr.X) != "net/http.Request" {
			return false
		}
		stmts := node.Body.List
		if len(stmts) == 2 {
			ret, ok := stmts[1].(*ast.ReturnStmt)
			if !ok || len(ret.Results) != 0 {
				return false
			}
			stmts = stmts[:1]
		}
		if len(stmts) != 1 {
			return false
		}
		stmt, ok := stmts[0].(*ast.ExprStmt)
		if !ok {
			return false
		}
		call, ok := stmt.X.(*ast.CallExpr)
		if !ok || len(call.Args) != 3 || call.Ellipsis.IsValid() {
			return false
		}
		args, ok := goRefusalWriters[f.refusalSymbol(call.Fun)]
		if !ok {
			return false
		}
		w, ok := call.Args[0].(*ast.Ident)
		if !ok || w.Obj != slots[0].Name.Obj {
			return false
		}
		requestUsed := false
		ast.Inspect(node.Body, func(n ast.Node) bool {
			if id, ok := n.(*ast.Ident); ok && id.Obj == slots[1].Name.Obj {
				requestUsed = true
			}
			return true
		})
		if requestUsed {
			return false
		}
		status := f.refusalConstant(call.Args[args[0]], map[*ast.Object]bool{})
		message := f.refusalConstant(call.Args[args[1]], map[*ast.Object]bool{})
		if status.Kind() != constant.Int {
			return false
		}
		n, integer := constant.Int64Val(status)
		return integer && n >= 400 && n <= 599 && message.Kind() == constant.String
	}
	return false
}

// Fail closed outside literals, explicit const bindings, net/http constants and
// simple constant arithmetic. Variables are never folded from their initializer.
func (f *goHandlerFrame) refusalConstant(expr ast.Expr, active map[*ast.Object]bool) constant.Value {
	unknown := constant.MakeUnknown()
	switch node := expr.(type) {
	case *ast.BasicLit:
		return constant.MakeFromLiteral(node.Value, node.Kind, 0)
	case *ast.ParenExpr:
		return f.refusalConstant(node.X, active)
	case *ast.Ident:
		if node.Obj == nil || node.Obj.Kind != ast.Con || active[node.Obj] {
			return unknown
		}
		spec, ok := node.Obj.Decl.(*ast.ValueSpec)
		if !ok || len(spec.Values) != len(spec.Names) {
			return unknown
		}
		active[node.Obj] = true
		defer delete(active, node.Obj)
		for i, id := range spec.Names {
			if id.Obj == node.Obj {
				return f.refusalConstant(spec.Values[i], active)
			}
		}
	case *ast.SelectorExpr:
		id, ok := node.X.(*ast.Ident)
		if !ok || id.Obj != nil || f.Func.File.ImportPaths[id.Name] != "net/http" {
			return unknown
		}
		if f.Proof.HTTP == nil {
			pkg, err := importer.Default().Import("net/http")
			if err != nil {
				return unknown
			}
			f.Proof.HTTP = pkg
		}
		if c, ok := f.Proof.HTTP.Scope().Lookup(node.Sel.Name).(*types.Const); ok {
			return c.Val()
		}
	case *ast.BinaryExpr:
		left, right := f.refusalConstant(node.X, active), f.refusalConstant(node.Y, active)
		if (left.Kind() == constant.Int && right.Kind() == constant.Int && (node.Op == token.ADD || node.Op == token.SUB || node.Op == token.MUL)) ||
			(left.Kind() == constant.String && right.Kind() == constant.String && node.Op == token.ADD) {
			return constant.BinaryOp(left, node.Op, right)
		}
	case *ast.CallExpr:
		// len of a fixed array literal is constant even when its elements refer to
		// r. Scan r separately above: constant folding must not erase request use.
		id, ok := node.Fun.(*ast.Ident)
		if !ok || id.Name != "len" || id.Obj != nil || len(node.Args) != 1 {
			return unknown
		}
		lit, ok := node.Args[0].(*ast.CompositeLit)
		if !ok {
			return unknown
		}
		array, ok := lit.Type.(*ast.ArrayType)
		if !ok || array.Len == nil {
			return unknown
		}
		safe := true
		ast.Inspect(lit, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.CallExpr:
				safe = false
			case *ast.UnaryExpr:
				if x.Op == token.ARROW {
					safe = false
				}
			}
			return true
		})
		if safe {
			return f.refusalConstant(array.Len, active)
		}
	}
	return unknown
}

// Recognize a positive table branch, its explicit else, or the continuation
// after a negative guard with an unconditional return. Other control flow is
// deliberately unsupported. Positions keep a helper/fallback outside the
// branch from being mistaken for a proven sibling of the refusal.
func (f *goHandlerFrame) refusalBranch(body *ast.BlockStmt, guards map[*ast.CallExpr]bool, refusal ast.Node, proven []ast.Node) bool {
	covered := false
	contains := func(start, end token.Pos) bool {
		if refusal.Pos() < start || refusal.End() > end {
			return false
		}
		for _, leaf := range proven {
			if leaf.Pos() >= start && leaf.End() <= end {
				return true
			}
		}
		return false
	}
	ast.Inspect(body, func(n ast.Node) bool {
		if _, nested := n.(*ast.FuncLit); nested {
			return false
		}
		block, ok := n.(*ast.BlockStmt)
		if !ok {
			return true
		}
		for _, stmt := range block.List {
			branch, ok := stmt.(*ast.IfStmt)
			if !ok {
				continue
			}
			cond := branch.Cond
			for {
				paren, ok := cond.(*ast.ParenExpr)
				if !ok {
					break
				}
				cond = paren.X
			}
			negative := false
			if not, ok := cond.(*ast.UnaryExpr); ok && not.Op == token.NOT {
				negative = true
				cond = not.X
			}
			call, ok := cond.(*ast.CallExpr)
			if !ok || !guards[call] {
				continue
			}
			if !negative {
				covered = contains(branch.Body.Pos(), branch.Body.End()) || covered
				continue
			}
			if branch.Else != nil {
				covered = contains(branch.Else.Pos(), branch.Else.End()) || covered
				continue
			}
			if len(branch.Body.List) > 0 {
				if ret, ok := branch.Body.List[len(branch.Body.List)-1].(*ast.ReturnStmt); ok && len(ret.Results) == 0 {
					covered = contains(branch.End(), block.End()) || covered
				}
			}
		}
		return true
	})
	return covered
}
