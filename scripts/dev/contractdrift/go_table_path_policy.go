// Purpose: fail closed on every request-URL use outside the route-table allowlist.
// Depends on: stdlib AST/types, proven table consumers and refusal-leaf constants.
// Used by: goTableReaches; follows helpers with parameter and lexical-object taint.
// Invariant: aliases never erase URL provenance; unknown uses retain UNRESOLVED.

package main

import (
	"go/ast"
	"go/constant"
	"go/importer"
	"go/token"
	"go/types"
)

// URL-use allowlist (all other positions fail closed): (1) a plain argument to
// the proven path consumer; (2) ==/!= or switch equality with a constant exactly
// equal to a listed path; (3) the exact same-request EscapedPath()!=Path guard
// with a refusal and return; (4) a plain argument to a resolved helper, whose
// body is checked with the same parameter taint; (5) request forwarding to an
// already proven refusal/handler leaf. Local assignments are provenance edges,
// not sinks: only lexical variables qualify, never fields/globals/blank names.
// All reached bodies and closure captures are checked. Consumers are proof
// boundaries: their internal pattern algorithms are validated by table gates.
var goTableURLUseAllowlist = map[string]string{
	"consumer":  "proven path consumer argument",
	"listed":    "equality with a listed constant path",
	"canonical": "exact escaped-path refusal guard",
	"helper":    "resolved helper with propagated parameters",
	"forward":   "proven request refusal/handler leaf",
}

type goURLTaint uint8

const (
	goURLRequest goURLTaint = 1 << iota
	goURLObject
	goURLValue
)

type goURLPolicy struct {
	scanner *goScanner
	table   goRouteTable
	owners  map[*goFunc]*goPackage
	seeds   map[*goFunc]map[*ast.Object]goURLTaint
	values  map[*ast.Object]goURLTaint
	parents map[ast.Node]ast.Node
	locals  map[*ast.Object]bool
	seen    map[*goFunc]bool
	urlType types.Type
	bad     []ast.Node
}

func (s *goScanner) goTablePathPolicy(table goRouteTable, entry *goFunc) bool {
	p := &goURLPolicy{scanner: s, table: table, owners: map[*goFunc]*goPackage{},
		seeds: map[*goFunc]map[*ast.Object]goURLTaint{}, seen: map[*goFunc]bool{}}
	for _, pkg := range s.Packages {
		for _, fn := range pkg.Funcs {
			p.owners[fn] = pkg
		}
	}
	p.seeds[entry] = map[*ast.Object]goURLTaint{}
	// A monotone fixed point includes parameter taint added by later callers.
	for changed := true; changed; {
		changed = false
		for fn, seed := range p.seeds {
			p.prepare(fn, seed)
			ast.Inspect(fn.AST.Body, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				callee := p.callee(fn, call.Fun)
				if callee == nil {
					return true
				}
				p.seen[callee] = true
				for _, consumer := range table.Consumers {
					if consumer == callee {
						return true // never propagate inside a proven consumer
					}
				}
				if p.seeds[callee] == nil {
					p.seeds[callee] = map[*ast.Object]goURLTaint{}
					changed = true
				}
				slots := goHandlerSlots(callee.AST.Type.Params)
				for i, arg := range call.Args {
					if i < len(slots) && slots[i].Name != nil {
						obj, kind := slots[i].Name.Obj, p.kind(arg)
						if p.seeds[callee][obj]|kind != p.seeds[callee][obj] {
							p.seeds[callee][obj] |= kind
							changed = true
						}
					}
				}
				return true
			})
		}
	}
	for _, consumer := range table.Consumers {
		if !p.seen[consumer] {
			return false
		}
	}
	for fn, seed := range p.seeds {
		p.prepare(fn, seed)
		ast.Inspect(fn.AST.Body, func(n ast.Node) bool {
			if lit, ok := n.(*ast.FuncLit); ok && p.closureEscapes(lit) {
				p.bad = append(p.bad, lit)
			}
			if expr, ok := n.(ast.Expr); ok && p.kind(expr) != 0 && !p.allowed(fn, expr) {
				p.bad = append(p.bad, expr)
			}
			return true
		})
	}
	if entry.AST.Name.Name == "ServeHTTP" {
		for _, n := range p.bad {
			addUnresolved(&s.Out, s.loc(n), "URL-derived use outside route-table allowlist")
		}
	}
	return len(p.bad) == 0
}

func (p *goURLPolicy) prepare(fn *goFunc, seed map[*ast.Object]goURLTaint) {
	p.values, p.locals, p.parents = map[*ast.Object]goURLTaint{}, map[*ast.Object]bool{}, map[ast.Node]ast.Node{}
	for obj, kind := range seed {
		p.values[obj] = kind
	}
	var stack []ast.Node
	ast.Inspect(fn.AST, func(n ast.Node) bool {
		if n == nil {
			stack = stack[:len(stack)-1]
			return false
		}
		if len(stack) > 0 {
			p.parents[n] = stack[len(stack)-1]
		}
		stack = append(stack, n)
		if id, ok := n.(*ast.Ident); ok && id.Obj != nil && id.Obj.Kind == ast.Var {
			if decl, ok := id.Obj.Decl.(ast.Node); ok && decl.Pos() >= fn.AST.Pos() && decl.End() <= fn.AST.End() {
				p.locals[id.Obj] = true
			}
		}
		if field, ok := n.(*ast.Field); ok {
			ptr, ok := field.Type.(*ast.StarExpr)
			if ok {
				sel, ok := ptr.X.(*ast.SelectorExpr)
				if ok && sel.Sel.Name == "Request" {
					id, ok := sel.X.(*ast.Ident)
					if ok && id.Obj == nil && fn.File.ImportPaths[id.Name] == "net/http" {
						for _, name := range field.Names {
							p.values[name.Obj] |= goURLRequest
						}
					}
				}
			}
		}
		return true
	})
	for changed := true; changed; {
		changed = false
		bind := func(id *ast.Ident, expr ast.Expr) {
			if id != nil && p.locals[id.Obj] {
				kind := p.kind(expr)
				if p.values[id.Obj]|kind != p.values[id.Obj] {
					p.values[id.Obj] |= kind
					changed = true
				}
			}
		}
		ast.Inspect(fn.AST.Body, func(n ast.Node) bool {
			switch node := n.(type) {
			case *ast.AssignStmt:
				if len(node.Lhs) == len(node.Rhs) {
					for i, rhs := range node.Rhs {
						id, _ := node.Lhs[i].(*ast.Ident)
						bind(id, rhs)
					}
				}
			case *ast.ValueSpec:
				if len(node.Names) == len(node.Values) {
					for i, rhs := range node.Values {
						bind(node.Names[i], rhs)
					}
				}
			}
			return true
		})
	}
}

// Only identifiers, parentheses and URL field selections are plain values.
// Compound/transforming expressions are inspected at their operands instead.
func (p *goURLPolicy) kind(expr ast.Expr) goURLTaint {
	switch node := expr.(type) {
	case *ast.Ident:
		return p.values[node.Obj]
	case *ast.ParenExpr:
		return p.kind(node.X)
	case *ast.SelectorExpr:
		base := p.kind(node.X)
		if base&goURLRequest != 0 {
			if node.Sel.Name == "URL" {
				return goURLObject
			}
			if node.Sel.Name == "RequestURI" {
				return goURLValue
			}
		}
		if base&(goURLObject|goURLValue) != 0 {
			return goURLValue
		}
	}
	return 0
}

func (p *goURLPolicy) callee(fn *goFunc, expr ast.Expr) *goFunc {
	pkg := p.owners[fn]
	if pkg == nil {
		return nil
	}
	if id, ok := expr.(*ast.Ident); ok && (id.Obj == nil || id.Obj.Kind == ast.Fun) {
		return pkg.Named[id.Name]
	}
	if sel, ok := expr.(*ast.SelectorExpr); ok {
		if id, ok := sel.X.(*ast.Ident); ok && id.Obj == nil {
			if imported := p.scanner.Packages[fn.File.ImportPaths[id.Name]]; imported != nil {
				return imported.Named[sel.Sel.Name]
			}
		}
	}
	return nil // dynamic calls and methods are not guessed from their spelling
}

func (p *goURLPolicy) listed(fn *goFunc, expr ast.Expr) bool {
	f := &goHandlerFrame{Func: fn, Proof: &goHandlerProof{Scanner: p.scanner}}
	v := f.refusalConstant(expr, map[*ast.Object]bool{})
	if v.Kind() != constant.String {
		return false
	}
	for _, row := range p.table.Routes {
		if row.Path == constant.StringVal(v) {
			return true
		}
	}
	return false
}

func (p *goURLPolicy) field(sel *ast.SelectorExpr) bool {
	if p.kind(sel.X) != goURLObject {
		return false
	}
	if p.urlType == nil {
		pkg, err := importer.Default().Import("net/url")
		if err != nil {
			return false
		}
		p.urlType = pkg.Scope().Lookup("URL").Type()
	}
	obj, _, _ := types.LookupFieldOrMethod(p.urlType, true, nil, sel.Sel.Name)
	_, ok := obj.(*types.Var)
	return ok // method values are never alias/proven-consumer arguments
}

func (p *goURLPolicy) plain(expr ast.Expr) bool {
	switch node := expr.(type) {
	case *ast.ParenExpr:
		return p.plain(node.X)
	case *ast.Ident:
		return p.kind(node) != 0
	case *ast.SelectorExpr:
		return p.kind(node.X) == goURLRequest && (node.Sel.Name == "URL" || node.Sel.Name == "RequestURI") || p.field(node)
	}
	return false
}

func (p *goURLPolicy) allowed(fn *goFunc, expr ast.Expr) bool {
	parent := p.parents[expr]
	if paren, ok := parent.(*ast.ParenExpr); ok && paren.X == expr {
		return true
	}
	if sel, ok := parent.(*ast.SelectorExpr); ok && sel.X == expr {
		return p.kind(expr) == goURLRequest || p.field(sel) || p.canonicalUse(sel)
	}
	// Declaration and local-assignment identifiers are binding sites, not reads.
	if id, ok := expr.(*ast.Ident); ok {
		if field, ok := parent.(*ast.Field); ok {
			for _, name := range field.Names {
				if id == name {
					return true
				}
			}
		}
		if spec, ok := parent.(*ast.ValueSpec); ok {
			for _, name := range spec.Names {
				if id == name {
					return true
				}
			}
		}
	}
	rule := ""
	switch node := parent.(type) {
	case *ast.AssignStmt:
		if len(node.Lhs) == len(node.Rhs) && (node.Tok == token.ASSIGN || node.Tok == token.DEFINE) {
			for i, lhs := range node.Lhs {
				id, ok := lhs.(*ast.Ident)
				if ok && id.Name != "_" && p.locals[id.Obj] && (lhs == expr || node.Rhs[i] == expr && p.plain(expr)) {
					return true
				}
			}
		}
	case *ast.ValueSpec:
		if len(node.Names) == len(node.Values) {
			for i, rhs := range node.Values {
				if rhs == expr && node.Names[i].Name != "_" && p.locals[node.Names[i].Obj] && p.plain(expr) {
					return true
				}
			}
		}
	case *ast.BinaryExpr:
		if node.Op == token.EQL || node.Op == token.NEQ {
			if p.canonical(node) {
				rule = "canonical"
			} else if p.plain(expr) && (node.X == expr && p.listed(fn, node.Y) || node.Y == expr && p.listed(fn, node.X)) {
				rule = "listed"
			}
		}
	case *ast.SwitchStmt:
		if node.Tag == expr && p.plain(expr) {
			valid := true
			for _, stmt := range node.Body.List {
				for _, value := range stmt.(*ast.CaseClause).List {
					valid = valid && p.listed(fn, value)
				}
			}
			if valid {
				rule = "listed"
			}
		}
	case *ast.CallExpr:
		if node.Fun == expr {
			if p.canonicalUse(expr) {
				rule = "canonical"
			}
		} else if p.plain(expr) && !node.Ellipsis.IsValid() {
			if callee := p.callee(fn, node.Fun); callee != nil {
				if callee == p.table.Consumers[0] && len(node.Args) == 1 {
					rule = "consumer"
				} else if p.seeds[callee] != nil && len(node.Args) == len(goHandlerSlots(callee.AST.Type.Params)) {
					rule = "helper"
				}
			} else if p.kind(expr) == goURLRequest && p.refusalCall(fn, node) {
				rule = "forward"
			}
		}
	}
	for node := parent; node != nil; node = p.parents[node] {
		if _, ok := node.(*ast.GoStmt); ok {
			return false
		}
	}
	_, ok := goTableURLUseAllowlist[rule]
	return ok
}

// An escaping closure is an unknown helper carrying its lexical request/URL.
// Only a direct synchronous invocation stays inside this inspected body;
// assigned/returned/passed closures cannot be blessed by their current uses.
func (p *goURLPolicy) closureEscapes(lit *ast.FuncLit) bool {
	if call, ok := p.parents[lit].(*ast.CallExpr); ok && call.Fun == lit {
		if _, async := p.parents[call].(*ast.GoStmt); !async {
			return false
		}
	}
	derived := false
	ast.Inspect(lit.Body, func(n ast.Node) bool {
		if expr, ok := n.(ast.Expr); ok && p.kind(expr)&(goURLObject|goURLValue) != 0 {
			derived = true
		}
		if id, ok := n.(*ast.Ident); ok && p.kind(id) == goURLRequest {
			if decl, ok := id.Obj.Decl.(ast.Node); ok && (decl.Pos() < lit.Pos() || decl.End() > lit.End()) {
				derived = true
			}
		}
		return true
	})
	return derived
}

func (p *goURLPolicy) refusalCall(fn *goFunc, call *ast.CallExpr) bool {
	f := &goHandlerFrame{Func: fn, Proof: &goHandlerProof{Scanner: p.scanner}}
	return f.refusalSymbol(call.Fun) == "net/http.NotFound" && len(call.Args) == 2
}

func (p *goURLPolicy) canonicalUse(expr ast.Expr) bool {
	if call, ok := p.parents[expr].(*ast.CallExpr); ok && call.Fun == expr {
		if binary, ok := p.parents[call].(*ast.BinaryExpr); ok {
			return p.canonical(binary)
		}
	}
	return false
}

func (p *goURLPolicy) canonical(binary *ast.BinaryExpr) bool {
	if binary.Op != token.NEQ {
		return false
	}
	call, ok := binary.X.(*ast.CallExpr)
	if !ok || len(call.Args) != 0 {
		return false
	}
	method, ok := call.Fun.(*ast.SelectorExpr)
	path, pathOK := binary.Y.(*ast.SelectorExpr)
	if !ok || !pathOK || method.Sel.Name != "EscapedPath" || path.Sel.Name != "Path" {
		return false
	}
	left, leftOK := method.X.(*ast.SelectorExpr)
	right, rightOK := path.X.(*ast.SelectorExpr)
	if !leftOK || !rightOK || left.Sel.Name != "URL" || right.Sel.Name != "URL" {
		return false
	}
	x, xOK := left.X.(*ast.Ident)
	y, yOK := right.X.(*ast.Ident)
	if !xOK || !yOK || x.Obj == nil || x.Obj != y.Obj || p.kind(x) != goURLRequest {
		return false
	}
	guard, ok := p.parents[binary].(*ast.IfStmt)
	if !ok || guard.Cond != binary || guard.Init != nil || guard.Else != nil || len(guard.Body.List) != 2 {
		return false
	}
	stmt, ok := guard.Body.List[0].(*ast.ExprStmt)
	ret, retOK := guard.Body.List[1].(*ast.ReturnStmt)
	if !ok || !retOK || len(ret.Results) != 0 {
		return false
	}
	refusal, ok := stmt.X.(*ast.CallExpr)
	if !ok {
		return false
	}
	// Canonical refusal uses the same imported leaf identity as forwarding.
	for fn := range p.seeds {
		if guard.Pos() >= fn.AST.Pos() && guard.End() <= fn.AST.End() && p.refusalCall(fn, refusal) {
			return true
		}
	}
	return false
}
