// Purpose: prove narrow returned-handler 405 factories from actual writer/status flow, never naming conventions.
// Depends on: Go AST and scanner-local package/import maps; no handler execution or external dependencies.
// Used by: Go route classification; both successful and failed proofs retain all visited source dependencies.
package main

import (
	"go/ast"
	"go/token"
	"strconv"
)

type go405Value struct {
	Writer, Request, Empty bool
	Status                 int64
}
type go405Proof struct {
	Scanner *goScanner
	Refs    []location
	Active  map[*goFunc]bool
}

func (p *go405Proof) value(x ast.Expr, file *goFile, env map[string]go405Value) go405Value {
	switch x := x.(type) {
	case *ast.Ident:
		return env[x.Name]
	case *ast.ParenExpr:
		return p.value(x.X, file, env)
	case *ast.BasicLit:
		if x.Kind == token.INT {
			status, _ := strconv.ParseInt(x.Value, 0, 64)
			return go405Value{Status: status}
		}
		if x.Kind == token.STRING {
			text, err := strconv.Unquote(x.Value)
			return go405Value{Empty: err == nil && text == ""}
		}
	case *ast.SelectorExpr:
		if id, ok := x.X.(*ast.Ident); ok && id.Name == file.HTTP && file.HTTP != "" {
			if _, shadowed := env[id.Name]; !shadowed && x.Sel.Name == "StatusMethodNotAllowed" {
				p.Refs = goRefs(p.Refs, []location{file.ImportRefs[id.Name]})
				return go405Value{Status: 405}
			}
		}
	}
	return go405Value{}
}

func go405Params(params *ast.FieldList, file *goFile, args []go405Value, handler bool) map[string]go405Value {
	env := map[string]go405Value{}
	i := 0
	if params == nil {
		return env
	}
	for _, field := range params.List {
		for _, name := range field.Names {
			v := go405Value{}
			if i < len(args) {
				v = args[i]
			}
			if handler {
				typ := field.Type
				pointer, isPointer := typ.(*ast.StarExpr)
				if isPointer {
					typ = pointer.X
				}
				if sel, ok := typ.(*ast.SelectorExpr); ok {
					if pkg, ok := sel.X.(*ast.Ident); ok && pkg.Name == file.HTTP && file.HTTP != "" {
						v.Writer = !isPointer && sel.Sel.Name == "ResponseWriter"
						v.Request = isPointer && sel.Sel.Name == "Request"
					}
				}
			}
			env[name.Name] = v
			i++
		}
	}
	return env
}

func (p *go405Proof) callee(expr ast.Expr, pkg *goPackage, file *goFile, env map[string]go405Value) (*goFunc, *goPackage) {
	if id, ok := expr.(*ast.Ident); ok {
		if _, shadowed := env[id.Name]; !shadowed {
			return pkg.Named[id.Name], pkg
		}
	}
	if sel, ok := expr.(*ast.SelectorExpr); ok {
		if id, ok := sel.X.(*ast.Ident); ok {
			if _, shadowed := env[id.Name]; !shadowed {
				if child := p.Scanner.Packages[file.ImportPaths[id.Name]]; child != nil {
					p.Refs = goRefs(p.Refs, []location{file.ImportRefs[id.Name]})
					return child.Named[sel.Sel.Name], child
				}
			}
		}
	}
	return nil, nil
}

func go405Header(call *ast.CallExpr, env map[string]go405Value) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || (sel.Sel.Name != "Set" && sel.Sel.Name != "Add" && sel.Sel.Name != "Del") {
		return false
	}
	receiver, ok := sel.X.(*ast.CallExpr)
	if !ok || len(receiver.Args) != 0 {
		return false
	}
	header, ok := receiver.Fun.(*ast.SelectorExpr)
	if !ok || header.Sel.Name != "Header" {
		return false
	}
	w, ok := header.X.(*ast.Ident)
	if !ok || !env[w.Name].Writer {
		return false
	}
	for _, arg := range call.Args {
		pure := true
		ast.Inspect(arg, func(n ast.Node) bool {
			if _, ok := n.(*ast.CallExpr); ok {
				pure = false
			}
			return pure
		})
		if !pure {
			return false
		}
	}
	return true
}

// Until the first status is committed, unsupported control flow, writes or calls fail closed.
// After a proven first WriteHeader(405), later body encoding cannot change that status.
func (p *go405Proof) writes405(block *ast.BlockStmt, pkg *goPackage, file *goFile, env map[string]go405Value) bool {
	for _, stmt := range block.List {
		if expr, ok := stmt.(*ast.ExprStmt); ok {
			if call, ok := expr.X.(*ast.CallExpr); ok {
				if go405Header(call, env) {
					continue
				}
				if sel, ok := call.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "WriteHeader" && len(call.Args) == 1 {
					if w, ok := sel.X.(*ast.Ident); ok && env[w.Name].Writer {
						return p.value(call.Args[0], file, env).Status == 405
					}
				}
				fn, child := p.callee(call.Fun, pkg, file, env)
				if fn == nil || p.Active[fn] || len(p.Active) >= 8 {
					return false
				}
				p.Refs = goRefs(p.Refs, []location{p.Scanner.loc(fn.AST)})
				args := make([]go405Value, len(call.Args))
				for i, arg := range call.Args {
					pure := true
					ast.Inspect(arg, func(n ast.Node) bool {
						if _, ok := n.(*ast.CallExpr); ok {
							pure = false
						}
						return pure
					})
					if !pure {
						return false
					}
					args[i] = p.value(arg, file, env)
				}
				p.Active[fn] = true
				proved := p.writes405(fn.AST.Body, child, fn.File, go405Params(fn.AST.Type.Params, fn.File, args, false))
				delete(p.Active, fn)
				return proved
			}
		}
		pure := true
		ast.Inspect(stmt, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.CallExpr, *ast.ReturnStmt, *ast.BranchStmt, *ast.DeferStmt, *ast.GoStmt, *ast.ForStmt, *ast.RangeStmt:
				pure = false
			case *ast.Ident:
				v := env[x.Name]
				if v.Writer || v.Request || v.Status != 0 || v.Empty {
					pure = false
				}
			}
			return pure
		})
		if !pure {
			return false
		}
	}
	return false
}

func (s *goScanner) factory405(call *ast.CallExpr, e goEnv) (bool, []location) {
	p := &go405Proof{Scanner: s, Active: map[*goFunc]bool{}}
	env := map[string]go405Value{}
	for _, scope := range e {
		for name, value := range scope {
			env[name] = go405Value{Empty: !value.Unknown && len(value.Text) == 1 && value.Text[0] == ""}
		}
	}
	fn, pkg := p.callee(call.Fun, s.Package, s.File, env)
	if fn == nil {
		return false, p.Refs
	}
	p.Refs = goRefs(p.Refs, []location{s.loc(fn.AST)})
	args := make([]go405Value, len(call.Args))
	for i, arg := range call.Args {
		args[i] = p.value(arg, s.File, env)
	}
	if len(fn.AST.Body.List) != 1 {
		return false, p.Refs
	}
	ret, ok := fn.AST.Body.List[0].(*ast.ReturnStmt)
	if !ok || len(ret.Results) != 1 {
		return false, p.Refs
	}
	lit, ok := ret.Results[0].(*ast.FuncLit)
	if !ok {
		return false, p.Refs
	}
	bound := go405Params(fn.AST.Type.Params, fn.File, args, false)
	for name, v := range go405Params(lit.Type.Params, fn.File, nil, true) {
		bound[name] = v
	}
	if p.writes405(lit.Body, pkg, fn.File, bound) {
		return true, p.Refs
	}
	for _, stmt := range lit.Body.List {
		if expr, ok := stmt.(*ast.ExprStmt); ok {
			if header, ok := expr.X.(*ast.CallExpr); ok && go405Header(header, bound) {
				continue
			}
		}
		guard, ok := stmt.(*ast.IfStmt)
		if !ok || guard.Init != nil || guard.Else != nil || len(guard.Body.List) != 2 {
			return false, p.Refs
		}
		condition, ok := guard.Cond.(*ast.BinaryExpr)
		if !ok || condition.Op != token.NEQ {
			return false, p.Refs
		}
		request, ok := condition.X.(*ast.SelectorExpr)
		if !ok || request.Sel.Name != "Method" {
			return false, p.Refs
		}
		id, ok := request.X.(*ast.Ident)
		if !ok || !bound[id.Name].Request || !p.value(condition.Y, fn.File, bound).Empty {
			return false, p.Refs
		}
		end, ok := guard.Body.List[1].(*ast.ReturnStmt)
		if !ok || len(end.Results) != 0 {
			return false, p.Refs
		}
		// Incoming net/http requests have a nonempty Method: the empty-bound guard always rejects.
		return p.writes405(&ast.BlockStmt{List: guard.Body.List[:1]}, pkg, fn.File, bound), p.Refs
	}
	return false, p.Refs
}
