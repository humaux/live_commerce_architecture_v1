// Purpose: prove opaque factories from the provenance of every returned HTTP handler.
// Depends on: consumed route tables, stdlib AST bindings and existing opaque findings.
// Used by: resolveRouteTables after all dispatchers and mounts have been considered.
// Invariant: unknown alternatives, alias writes and unproven path dispatch fail closed;
// successful proofs retain the original whole-function/dependency spans as notes.

package main

import (
	"go/ast"
	"go/token"
	"strings"
)

type goHandlerSlot struct {
	Type ast.Expr
	Name *ast.Ident
}

func goHandlerSlots(fields *ast.FieldList) []goHandlerSlot {
	var slots []goHandlerSlot
	if fields != nil {
		for _, field := range fields.List {
			if len(field.Names) == 0 {
				slots = append(slots, goHandlerSlot{Type: field.Type})
			}
			for _, name := range field.Names {
				slots = append(slots, goHandlerSlot{Type: field.Type, Name: name})
			}
		}
	}
	return slots
}

func goHTTPHandlerType(expr ast.Expr, file *goFile) bool {
	sel, ok := expr.(*ast.SelectorExpr)
	if !ok || (sel.Sel.Name != "Handler" && sel.Sel.Name != "HandlerFunc") {
		return false
	}
	id, ok := sel.X.(*ast.Ident)
	return ok && id.Obj == nil && file.ImportPaths[id.Name] == "net/http"
}

type goHandlerProof struct {
	Scanner  *goScanner
	Tables   []goRouteTable
	Owners   map[*goFunc]*goPackage
	Dispatch map[*goFunc]bool
	Active   map[*goFunc]bool
	Refs     []location
}

type goHandlerOrigin struct {
	Expr   ast.Expr
	Result int
}

type goHandlerFrame struct {
	Proof  *goHandlerProof
	Func   *goFunc
	Pkg    *goPackage
	Params map[*ast.Object]bool
	Values map[*ast.Object][]goHandlerOrigin
	Seen   map[*ast.Object]bool
}

func (s *goScanner) resolveHandlerFactories(tables []goRouteTable) {
	p := &goHandlerProof{Scanner: s, Tables: tables, Owners: map[*goFunc]*goPackage{}, Dispatch: map[*goFunc]bool{}, Active: map[*goFunc]bool{}}
	byLocation := map[location]*goFunc{}
	for _, pkg := range s.Packages {
		for _, fn := range pkg.Funcs {
			p.Owners[fn] = pkg
			byLocation[s.loc(fn.AST)] = fn
			for _, table := range tables {
				if pkg == table.Package && (table.Exact && goTableExactDispatcher(fn, table.Consumers[1]) ||
					!table.Exact && fn.AST.Recv != nil && fn.AST.Name.Name == "ServeHTTP" && s.goTableReaches(table, fn)) {
					p.Dispatch[fn] = true
				}
			}
		}
	}
	remaining := s.Out.Unresolved[:0]
	for _, finding := range s.Out.Unresolved {
		var fn *goFunc
		if finding.Method == "" && len(finding.Locations) > 0 && strings.HasPrefix(finding.Detail, "opaque Go handler dispatch ") {
			fn = byLocation[finding.Locations[0]]
		}
		known, handlers := fn != nil, 0
		p.Refs = nil
		if fn != nil {
			params := map[*ast.Object]bool{}
			for _, slot := range goHandlerSlots(fn.AST.Type.Params) {
				if slot.Name != nil && goHTTPHandlerType(slot.Type, fn.File) {
					params[slot.Name.Obj] = true // supplied fallback/child; actual call sites bind their own arguments below
				}
			}
			for i, slot := range goHandlerSlots(fn.AST.Type.Results) {
				if goHTTPHandlerType(slot.Type, fn.File) {
					handlers++
					known = p.function(fn, i, params) && known
				}
			}
		}
		if known && handlers > 0 {
			note := finding
			note.Kind = "GO_ROUTE_TABLE"
			note.Detail = "handler factory " + fn.AST.Name.Name + " resolved from returned handler provenance"
			note.Locations = goRefs(finding.Locations, p.Refs)
			s.Out.Notes = append(s.Out.Notes, note)
		} else {
			remaining = append(remaining, finding)
		}
	}
	s.Out.Unresolved = remaining
}

func (p *goHandlerProof) function(fn *goFunc, result int, params map[*ast.Object]bool) bool {
	results := goHandlerSlots(fn.AST.Type.Results)
	if result >= len(results) || !goHTTPHandlerType(results[result].Type, fn.File) || p.Active[fn] {
		return false
	}
	p.Refs = goRefs(p.Refs, []location{p.Scanner.loc(fn.AST)})
	if p.Dispatch[fn] { // only the actual, consumed table dispatcher is a trusted leaf
		return true
	}
	p.Active[fn] = true
	defer delete(p.Active, fn)
	f := &goHandlerFrame{Proof: p, Func: fn, Pkg: p.Owners[fn], Params: params, Values: map[*ast.Object][]goHandlerOrigin{}, Seen: map[*ast.Object]bool{}}
	// Flow-insensitive union is deliberate: every assignment is a possible
	// source. AST objects distinguish shadowed locals. Include closure writes to
	// captured handlers; never mistake one good assignment for all return paths.
	ast.Inspect(fn.AST.Body, func(n ast.Node) bool {
		var names []ast.Expr
		var values []ast.Expr
		switch node := n.(type) {
		case *ast.AssignStmt:
			names, values = node.Lhs, node.Rhs
		case *ast.ValueSpec:
			for _, name := range node.Names {
				names = append(names, name)
			}
			values = node.Values
		case *ast.RangeStmt:
			for _, expr := range []ast.Expr{node.Key, node.Value} {
				if id, ok := expr.(*ast.Ident); ok && id.Obj != nil {
					f.Values[id.Obj] = append(f.Values[id.Obj], goHandlerOrigin{})
				}
			}
		case *ast.UnaryExpr:
			if node.Op == token.AND {
				if id, ok := node.X.(*ast.Ident); ok && id.Obj != nil {
					f.Values[id.Obj] = append(f.Values[id.Obj], goHandlerOrigin{}) // address escape is not a proven assignment
				}
			}
		}
		for i, name := range names {
			id, ok := name.(*ast.Ident)
			if !ok || id.Obj == nil {
				continue
			}
			origin := goHandlerOrigin{}
			if len(values) == len(names) {
				origin.Expr = values[i]
			} else if len(values) == 1 {
				origin = goHandlerOrigin{Expr: values[0], Result: i}
			} else if len(values) == 0 {
				origin.Expr = ast.NewIdent("nil")
			}
			f.Values[id.Obj] = append(f.Values[id.Obj], origin)
		}
		return true
	})
	found, valid := false, true
	ast.Inspect(fn.AST.Body, func(n ast.Node) bool {
		if _, nested := n.(*ast.FuncLit); nested {
			return false
		}
		if ret, ok := n.(*ast.ReturnStmt); ok {
			found = true
			switch {
			case len(ret.Results) == len(results):
				valid = f.value(ret.Results[result], 0, false) && valid
			case len(ret.Results) == 1:
				valid = f.value(ret.Results[0], result, false) && valid
			case len(ret.Results) == 0 && results[result].Name != nil:
				valid = f.value(results[result].Name, 0, false) && valid
			default:
				valid = false
			}
		}
		return true
	})
	return found && valid
}

func (f *goHandlerFrame) callee(expr ast.Expr) *goFunc {
	if id, ok := expr.(*ast.Ident); ok && (id.Obj == nil || id.Obj.Kind == ast.Fun) {
		return f.Pkg.Named[id.Name]
	}
	if sel, ok := expr.(*ast.SelectorExpr); ok {
		if id, ok := sel.X.(*ast.Ident); ok && id.Obj == nil {
			if pkg := f.Proof.Scanner.Packages[f.Func.File.ImportPaths[id.Name]]; pkg != nil {
				f.Proof.Refs = goRefs(f.Proof.Refs, []location{f.Func.File.ImportRefs[id.Name]})
				return pkg.Named[sel.Sel.Name]
			}
		}
	}
	return nil
}

func (f *goHandlerFrame) value(expr ast.Expr, result int, terminal bool) bool {
	switch node := expr.(type) {
	case *ast.Ident:
		if node.Obj == nil {
			return node.Name == "nil"
		}
		if f.Seen[node.Obj] {
			return false
		}
		f.Seen[node.Obj] = true
		defer delete(f.Seen, node.Obj)
		valid, found := f.Params[node.Obj]
		for _, origin := range f.Values[node.Obj] {
			good := f.value(origin.Expr, origin.Result, terminal)
			if !found {
				valid, found = good, true
			} else {
				valid = good && valid
			}
		}
		if !found {
			if field, ok := node.Obj.Decl.(*ast.Field); ok {
				return goHTTPHandlerType(field.Type, f.Func.File) // zero-valued named result
			}
		}
		return found && valid
	case *ast.ParenExpr:
		return f.value(node.X, result, terminal)
	case *ast.UnaryExpr:
		return node.Op == token.AND && f.value(node.X, result, terminal)
	case *ast.CompositeLit:
		typ, ok := node.Type.(*ast.Ident)
		if !ok {
			return false
		}
		for fn := range f.Proof.Dispatch {
			if f.Proof.Owners[fn] != f.Pkg || fn.AST.Recv == nil {
				continue
			}
			receiver := fn.AST.Recv.List[0].Type
			if ptr, ok := receiver.(*ast.StarExpr); ok {
				receiver = ptr.X
			}
			if id, ok := receiver.(*ast.Ident); ok && id.Name == typ.Name {
				f.Proof.Refs = goRefs(f.Proof.Refs, []location{f.Proof.Scanner.loc(fn.AST)})
				return true
			}
		}
	case *ast.FuncLit:
		return result == 0 && f.closure(node, terminal)
	case *ast.CallExpr:
		if result == 0 && goHTTPHandlerType(node.Fun, f.Func.File) && len(node.Args) == 1 {
			return f.value(node.Args[0], 0, terminal)
		}
		if target := f.callee(node.Fun); target != nil {
			params := map[*ast.Object]bool{}
			for i, slot := range goHandlerSlots(target.AST.Type.Params) {
				if goHTTPHandlerType(slot.Type, target.File) {
					if slot.Name == nil || i >= len(node.Args) {
						return false
					}
					params[slot.Name.Obj] = f.value(node.Args[i], 0, terminal)
					if !params[slot.Name.Obj] {
						return false
					}
				}
			}
			return f.Proof.function(target, result, params)
		}
	}
	return false
}

func (f *goHandlerFrame) tableCondition(call *ast.CallExpr) bool {
	for _, table := range f.Proof.Tables {
		if f.Proof.Scanner.tableMountCall(f.Func, call, table) {
			// A local/import alias with the same spelling cannot act as the table.
			method := call.Fun.(*ast.SelectorExpr)
			variable := method.X.(*ast.SelectorExpr)
			alias := variable.X.(*ast.Ident)
			if alias.Obj == nil {
				f.Proof.Refs = goRefs(f.Proof.Refs, []location{table.Ref})
				return true
			}
		}
	}
	return false
}

func (f *goHandlerFrame) closure(lit *ast.FuncLit, terminal bool) bool {
	valid, forwarded, mounted := true, false, false
	guards := map[*ast.CallExpr]bool{}
	var condition func(ast.Expr) bool
	condition = func(expr ast.Expr) bool {
		switch node := expr.(type) {
		case *ast.ParenExpr:
			return condition(node.X)
		case *ast.UnaryExpr:
			return node.Op == token.NOT && condition(node.X)
		case *ast.BinaryExpr:
			return (node.Op == token.LOR || node.Op == token.LAND) && condition(node.X) && condition(node.Y)
		case *ast.CallExpr:
			if f.tableCondition(node) {
				guards[node] = true
				return true
			}
		}
		return false
	}
	ast.Inspect(lit.Body, func(n ast.Node) bool {
		if _, nested := n.(*ast.FuncLit); nested {
			return false
		}
		if stmt, ok := n.(*ast.IfStmt); ok && condition(stmt.Cond) {
			mounted = true
		}
		return true
	})
	seen := map[*goFunc]bool{}
	var inspect func(ast.Node) bool
	inspect = func(n ast.Node) bool {
		if sel, ok := n.(*ast.SelectorExpr); ok && (sel.Sel.Name == "Path" || strings.HasPrefix(sel.Sel.Name, "Method")) {
			valid = false
		}
		if call, ok := n.(*ast.CallExpr); ok {
			if guards[call] {
				return false // only this proven consumer owns these Path selectors
			}
			if sel, ok := call.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "ServeHTTP" {
				forwarded = true
				valid = f.value(sel.X, 0, terminal || mounted) && valid
				for _, arg := range call.Args {
					ast.Inspect(arg, inspect)
				}
				return false
			}
			// A named helper accepting the request is part of the dispatch proof.
			// Unknown request consumers and local function variables fail closed.
			requestArg := false
			for _, arg := range call.Args {
				if id, ok := arg.(*ast.Ident); ok && id.Obj != nil {
					if field, ok := id.Obj.Decl.(*ast.Field); ok {
						if ptr, ok := field.Type.(*ast.StarExpr); ok {
							if sel, ok := ptr.X.(*ast.SelectorExpr); ok && sel.Sel.Name == "Request" {
								requestArg = true
							}
						}
					}
				}
			}
			if target := f.callee(call.Fun); requestArg && target != nil && !seen[target] {
				seen[target] = true
				f.Proof.Refs = goRefs(f.Proof.Refs, []location{f.Proof.Scanner.loc(target.AST)})
				ast.Inspect(target.AST.Body, inspect)
			} else if requestArg && target == nil {
				valid = false
			}
			if id, ok := call.Fun.(*ast.Ident); ok && id.Obj != nil && id.Obj.Kind == ast.Var {
				valid = false
			}
		}
		return true
	}
	ast.Inspect(lit.Body, inspect)
	return valid && (forwarded || terminal)
}
