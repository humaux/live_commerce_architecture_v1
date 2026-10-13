// Purpose: prove a closed routing-effect grammar for error-only startup admission.
// Depends on: source package/import resolution and explicit stdlib/PGx effect contracts.
// Used by: factoryRefs; unsupported syntax, foreign receivers and escapes fail closed.
// Invariant: a PGx type alone never proves callback-free ownership; fresh origins are required.
package main

import (
	"go/ast"
	"go/token"
	"strconv"
)

type admissionValue struct {
	kind, text string
	local bool
	items []admissionValue
	closure *ast.FuncLit
	refs []location
}
type goAdmissionProof struct {
	s *goScanner
	pkg *goPackage
	file *goFile
	active map[*goFunc]bool
	refs []location
	factory bool
	failure bool
	overwrite string
	aborts, probes int
}

func admissionClone(env map[string]admissionValue) map[string]admissionValue {
	out := map[string]admissionValue{}
	for k, v := range env { out[k] = v }
	return out
}
func (p *goAdmissionProof) poolType(x ast.Expr) bool {
	star, ok := x.(*ast.StarExpr)
	if !ok { return false }
	sel, ok := star.X.(*ast.SelectorExpr)
	if !ok { return false }
	id, ok := sel.X.(*ast.Ident)
	return ok && id.Obj == nil && sel.Sel.Name == "Pool" && p.file.ImportPaths[id.Name] == "github.com/jackc/pgx/v5/pgxpool"
}
func (p *goAdmissionProof) params(fields *ast.FieldList, args []admissionValue) map[string]admissionValue {
	env := map[string]admissionValue{}
	i := 0
	if fields == nil { return env }
	for _, f := range fields.List {
		for _, name := range f.Names {
			v := admissionValue{kind: "borrowed", refs: []location{p.s.loc(f)}}
			if i < len(args) { v = args[i] }
			v.local = false
			env[name.Name] = v
			i++
		}
	}
	return env
}
func (p *goAdmissionProof) callee(x ast.Expr, env map[string]admissionValue) (*goFunc, *goPackage) {
	if id, ok := x.(*ast.Ident); ok {
		if _, shadow := env[id.Name]; !shadow { return p.pkg.Named[id.Name], p.pkg }
	}
	if sel, ok := x.(*ast.SelectorExpr); ok {
		if id, ok := sel.X.(*ast.Ident); ok {
			if _, shadow := env[id.Name]; !shadow {
				if child := p.s.Packages[p.file.ImportPaths[id.Name]]; child != nil {
					p.refs = goRefs(p.refs, []location{p.file.ImportRefs[id.Name]})
					return child.Named[sel.Sel.Name], child
				}
			}
		}
	}
	return nil, nil
}
func (p *goAdmissionProof) expr(x ast.Expr, env map[string]admissionValue) admissionValue {
	switch x := x.(type) {
	case *ast.Ident:
		if v, ok := env[x.Name]; ok { p.refs = goRefs(p.refs, v.refs); return v }
		if x.Name == "nil" && x.Obj == nil { return admissionValue{kind: "nil"} }
		if (x.Name == "true" || x.Name == "false") && x.Obj == nil { return admissionValue{kind: "scalar", text: x.Name} }
		// Source globals are only accepted when initialized by the real errors.New.
		for _, file := range p.pkg.Files {
			for _, decl := range file.AST.Decls {
				if gd, ok := decl.(*ast.GenDecl); ok {
					for _, spec := range gd.Specs {
						if v, ok := spec.(*ast.ValueSpec); ok && len(v.Names) == 1 && v.Names[0].Name == x.Name && len(v.Values) == 1 {
							if c, ok := v.Values[0].(*ast.CallExpr); ok {
								if sel, ok := c.Fun.(*ast.SelectorExpr); ok {
									if id, ok := sel.X.(*ast.Ident); ok && file.ImportPaths[id.Name] == "errors" && sel.Sel.Name == "New" && len(c.Args) == 1 {
										if _, ok := c.Args[0].(*ast.BasicLit); ok && p.immutableGlobal(x.Name) { p.refs = goRefs(p.refs, []location{p.s.loc(v), file.ImportRefs[id.Name]}); return admissionValue{kind: "nonnull"} }
									}
								}
							}
						}
					}
				}
			}
		}
	case *ast.BasicLit:
		v := admissionValue{kind: "scalar", text: x.Value}
		if x.Kind == token.STRING { v.text, _ = strconv.Unquote(x.Value) }
		return v
	case *ast.ParenExpr: return p.expr(x.X, env)
	case *ast.UnaryExpr:
		v := p.expr(x.X, env)
		if x.Op == token.AND && v.local && v.kind == "scalar" { return admissionValue{kind: "address"} }
		if x.Op == token.NOT && v.kind == "scalar" { return v }
	case *ast.BinaryExpr:
		a, b := p.expr(x.X, env), p.expr(x.Y, env)
		if a.kind != "" && b.kind != "" {
			if x.Op == token.EQL || x.Op == token.NEQ { return admissionValue{kind: "scalar"} }
			if a.kind == "scalar" && b.kind == "scalar" { return admissionValue{kind: "scalar"} }
		}
	case *ast.SelectorExpr:
		if id, ok := x.X.(*ast.Ident); ok {
			if _, shadow := env[id.Name]; !shadow && p.file.ImportPaths[id.Name] == "time" && x.Sel.Name == "Second" { p.refs = goRefs(p.refs, []location{p.file.ImportRefs[id.Name]}); return admissionValue{kind: "scalar"} }
		}
	case *ast.SliceExpr:
		v := p.expr(x.X, env)
		if v.kind == "buffer" && v.local && x.Low == nil && x.High == nil && x.Max == nil { return v }
	case *ast.CompositeLit:
		if _, ok := x.Type.(*ast.ArrayType); ok && len(x.Elts) <= 16 {
			v := admissionValue{kind: "collection"}
			for _, e := range x.Elts { a := p.expr(e, env); if a.kind == "" { return admissionValue{} }; v.items = append(v.items, a) }
			return v
		}
		if len(x.Elts) == 0 { return admissionValue{kind: "zeroComposite"} }
	case *ast.FuncLit: return admissionValue{kind: "closure", closure: x}
	case *ast.CallExpr:
		v := p.call(x, env)
		if len(v) == 1 { return v[0] }
	}
	return admissionValue{}
}

func (p *goAdmissionProof) call(c *ast.CallExpr, env map[string]admissionValue) []admissionValue {
	var args []admissionValue
	for _, x := range c.Args { v := p.expr(x, env); if v.kind == "" || v.kind == "closure" { return nil }; args = append(args, v) }
	if id, ok := c.Fun.(*ast.Ident); ok {
		if v, ok := env[id.Name]; ok {
			if v.kind == "cancel" && len(args) == 0 { return []admissionValue{{kind: "nil"}} }
			if v.kind == "closure" && v.closure != nil {
				e := admissionClone(env)
				for name, arg := range p.params(v.closure.Type.Params, args) { e[name] = arg }
				if p.block(v.closure.Body, e) { return []admissionValue{{kind: "nil"}} }
				return nil
			}
			return nil
		}
		if (id.Name == "int64" || id.Name == "uint64") && id.Obj == nil && p.pkg.Named[id.Name] == nil && len(args) == 1 && args[0].kind == "scalar" { return args }
	}
	if sel, ok := c.Fun.(*ast.SelectorExpr); ok {
		if id, ok := sel.X.(*ast.Ident); ok {
			if _, shadow := env[id.Name]; !shadow {
				path := p.file.ImportPaths[id.Name]
				p.refs = goRefs(p.refs, []location{p.file.ImportRefs[id.Name]})
				switch {
				case path == "errors" && sel.Sel.Name == "New" && len(args) == 1 && args[0].kind == "scalar": return []admissionValue{{kind: "nonnull"}}
				case path == "context" && sel.Sel.Name == "Background" && len(args) == 0: return []admissionValue{{kind: "ctx"}}
				case path == "context" && sel.Sel.Name == "WithTimeout" && len(args) == 2 && args[0].kind == "ctx" && args[1].kind == "scalar": return []admissionValue{{kind: "ctx"}, {kind: "cancel"}}
				case path == "crypto/rand" && sel.Sel.Name == "Read" && len(args) == 1 && args[0].kind == "buffer" && args[0].local: return []admissionValue{{kind: "scalar"}, {kind: "error"}}
				}
			}
		}
		if endian, ok := sel.X.(*ast.SelectorExpr); ok && endian.Sel.Name == "BigEndian" && sel.Sel.Name == "Uint64" {
			if id, ok := endian.X.(*ast.Ident); ok {
				if _, shadow := env[id.Name]; !shadow && p.file.ImportPaths[id.Name] == "encoding/binary" && len(args) == 1 && args[0].kind == "buffer" && args[0].local { p.refs = goRefs(p.refs, []location{p.file.ImportRefs[id.Name]}); return []admissionValue{{kind: "scalar"}} }
			}
		}
		recv := p.expr(sel.X, env)
		switch {
		case recv.kind == "ctx" && sel.Sel.Name == "Err" && len(args) == 0: return []admissionValue{{kind: "error"}}
		case recv.kind == "pool" && sel.Sel.Name == "Close" && len(args) == 0 && p.failure: return []admissionValue{{kind: "nil"}}
		case recv.kind == "pool" && sel.Sel.Name == "Begin" && len(args) == 1 && args[0].kind == "ctx": return []admissionValue{{kind: "tx"}, {kind: "error"}}
		case recv.kind == "tx" && sel.Sel.Name == "Rollback" && len(args) == 1 && args[0].kind == "ctx": return []admissionValue{{kind: "error"}}
		case recv.kind == "tx" && sel.Sel.Name == "QueryRow" && len(args) == 3 && args[0].kind == "ctx" && args[1].text == "SELECT pg_catalog.pg_try_advisory_xact_lock($1::bigint)" && args[2].kind == "scalar": return []admissionValue{{kind: "row"}}
		case recv.kind == "row" && sel.Sel.Name == "Scan" && len(args) == 1 && args[0].kind == "address": return []admissionValue{{kind: "error"}}
		}
	}
	fn, pkg := p.callee(c.Fun, env)
	if fn == nil || p.active[fn] || len(p.active) >= 8 || fn.AST.Type.Results == nil { return nil }
	// Only source-analyzed error results, never handler/resource-producing calls, are admissions.
	if len(fn.AST.Type.Results.List) != 1 { return nil }
	errType, ok := fn.AST.Type.Results.List[0].Type.(*ast.Ident)
	if !ok || errType.Name != "error" { return nil }
	oldPkg, oldFile, oldFactory := p.pkg, p.file, p.factory
	p.pkg, p.file, p.factory = pkg, fn.File, false
	e := p.params(fn.AST.Type.Params, args)
	for _, field := range fn.AST.Type.Results.List { for _, name := range field.Names { e[name.Name] = admissionValue{kind: "error", local: true} } }
	p.active[fn] = true
	p.refs = goRefs(p.refs, []location{p.s.loc(fn.AST)})
	safe := p.block(fn.AST.Body, e)
	delete(p.active, fn)
	p.pkg, p.file, p.factory = oldPkg, oldFile, oldFactory
	if safe { p.probes++; return []admissionValue{{kind: "error"}} }
	return nil
}

func (p *goAdmissionProof) assign(a *ast.AssignStmt, env map[string]admissionValue) bool {
	var values []admissionValue
	if len(a.Rhs) == 1 { if c, ok := a.Rhs[0].(*ast.CallExpr); ok { values = p.call(c, env) } }
	if values == nil { for _, rhs := range a.Rhs { values = append(values, p.expr(rhs, env)) } }
	if len(values) != len(a.Lhs) { return false }
	for i, lhs := range a.Lhs {
		id, ok := lhs.(*ast.Ident)
		if !ok || values[i].kind == "" { return false }
		if id.Name == "_" { continue }
		old, exists := env[id.Name]
		if a.Tok != token.DEFINE && (!exists || (!old.local && !(p.factory && id.Name == p.overwrite && values[i].kind == "error"))) { return false }
		v := values[i]; v.local = true
		env[id.Name] = v
	}
	return true
}
func (p *goAdmissionProof) block(b *ast.BlockStmt, env map[string]admissionValue) bool {
	if b == nil { return false }
	for _, stmt := range b.List { if !p.stmt(stmt, env) { return false } }
	return true
}
func (p *goAdmissionProof) stmt(stmt ast.Stmt, env map[string]admissionValue) bool {
	switch x := stmt.(type) {
	case *ast.AssignStmt: return p.assign(x, env)
	case *ast.DeclStmt:
		gd, ok := x.Decl.(*ast.GenDecl); if !ok || gd.Tok != token.VAR { return false }
		for _, spec := range gd.Specs {
			v, ok := spec.(*ast.ValueSpec); if !ok { return false }
			if len(v.Values) != 0 { return false }
			kind := ""
			if id, ok := v.Type.(*ast.Ident); ok && id.Obj == nil && (id.Name == "bool" || id.Name == "int64") { kind = "scalar" }
			if a, ok := v.Type.(*ast.ArrayType); ok { if id, ok := a.Elt.(*ast.Ident); ok && id.Obj == nil && id.Name == "byte" && a.Len != nil { kind = "buffer" } }
			if kind == "" { return false }
			for _, name := range v.Names { env[name.Name] = admissionValue{kind: kind, local: true} }
		}
		return true
	case *ast.ExprStmt:
		c, ok := x.X.(*ast.CallExpr); return ok && len(p.call(c, env)) > 0
	case *ast.DeferStmt: return !p.factory && len(p.call(x.Call, env)) > 0
	case *ast.IfStmt:
		e := admissionClone(env)
		if x.Init != nil && !p.stmt(x.Init, e) { return false }
		if p.expr(x.Cond, e).kind != "scalar" { return false }
		// A != nil test refines only the tested error on the true branch.
		if b, ok := x.Cond.(*ast.BinaryExpr); ok && b.Op == token.NEQ {
			if id, ok := b.X.(*ast.Ident); ok { if nilID, ok := b.Y.(*ast.Ident); ok && nilID.Name == "nil" && e[id.Name].kind == "error" { v := e[id.Name]; v.kind = "nonnull"; e[id.Name] = v } }
		}
		oldFailure := p.failure
		if len(x.Body.List) > 0 {
			if r, ok := x.Body.List[len(x.Body.List)-1].(*ast.ReturnStmt); ok && len(r.Results) > 0 {
				p.failure = p.expr(r.Results[len(r.Results)-1], e).kind == "nonnull" && (!p.factory || (len(r.Results) >= 2 && p.expr(r.Results[0], e).kind == "nil"))
			}
		}
		safe := p.block(x.Body, e)
		p.failure = oldFailure
		if !safe { return false }
		if x.Else != nil { return p.stmt(x.Else, admissionClone(env)) }
		return true
	case *ast.BlockStmt: return p.block(x, env)
	case *ast.RangeStmt:
		v := p.expr(x.X, env)
		if v.kind != "collection" || len(v.items) == 0 || len(v.items) > 16 || x.Tok != token.DEFINE { return false }
		key, ok := x.Key.(*ast.Ident); if !ok || key.Name != "_" { return false }
		id, ok := x.Value.(*ast.Ident); if !ok { return false }
		for _, item := range v.items { e := admissionClone(env); item.local = true; e[id.Name] = item; if !p.block(x.Body, e) { return false } }
		return true
	case *ast.ForStmt:
		// The helper's scalar retry loop may affect latency, but cannot affect routing state.
		return !p.factory && x.Init == nil && x.Post == nil && p.expr(x.Cond, env).kind == "scalar" && p.block(x.Body, admissionClone(env))
	case *ast.ReturnStmt:
		if p.factory {
			if len(x.Results) < 2 || p.expr(x.Results[0], env).kind != "nil" || p.expr(x.Results[len(x.Results)-1], env).kind != "nonnull" { return false }
			for _, result := range x.Results[1:len(x.Results)-1] { v := p.expr(result, env); if v.kind != "nil" && v.kind != "zeroComposite" { return false } }
			p.aborts++; return true
		}
		if len(x.Results) == 0 { return true }
		if len(x.Results) != 1 { return false }
		v := p.expr(x.Results[0], env); return v.kind == "nil" || v.kind == "nonnull" || v.kind == "error"
	}
	return false
}

func (p *goAdmissionProof) immutableGlobal(name string) bool {
	safe := true
	for _, fn := range p.pkg.Funcs {
		ast.Inspect(fn.AST.Body, func(n ast.Node) bool {
			if a, ok := n.(*ast.AssignStmt); ok {
				for _, lhs := range a.Lhs { if id, ok := lhs.(*ast.Ident); ok && id.Name == name { safe = false } }
			}
			if u, ok := n.(*ast.UnaryExpr); ok && u.Op == token.AND { if id, ok := u.X.(*ast.Ident); ok && id.Name == name { safe = false } }
			return true
		})
	}
	return safe
}

// Retained factory statements can supply values, but are never themselves exempted.
func (p *goAdmissionProof) bind(stmt ast.Stmt, env map[string]admissionValue) {
	if a, ok := stmt.(*ast.AssignStmt); ok {
		var values []admissionValue
		if len(a.Rhs) == 1 { if c, ok := a.Rhs[0].(*ast.CallExpr); ok { values = p.call(c, env) } }
		for i, lhs := range a.Lhs {
			if id, ok := lhs.(*ast.Ident); ok && id.Name != "_" {
				v := admissionValue{kind: "borrowed"}
				if i < len(a.Rhs) { v = p.origin(a.Rhs[i], env, 0) }
				if i < len(values) { v = values[i] }
				if v.kind == "" { v.kind = "borrowed" }
				v.refs = goRefs(v.refs, []location{p.s.loc(a)})
				v.local = false
				env[id.Name] = v
			}
		}
	}
	if d, ok := stmt.(*ast.DeclStmt); ok {
		if gd, ok := d.Decl.(*ast.GenDecl); ok { for _, spec := range gd.Specs { if v, ok := spec.(*ast.ValueSpec); ok { for _, id := range v.Names { kind := "borrowed"; if p.poolType(v.Type) && len(v.Values) == 0 { kind = "pool" }; env[id.Name] = admissionValue{kind: kind, refs: []location{p.s.loc(v)}} } } } }
	}
	// Do not carry an origin through a retained conditional overwrite or alias write.
	ast.Inspect(stmt, func(n ast.Node) bool {
		if a, ok := n.(*ast.AssignStmt); ok && n != stmt {
			for _, lhs := range a.Lhs {
				if id, ok := lhs.(*ast.Ident); ok && a.Tok == token.ASSIGN { if _, exists := env[id.Name]; exists { env[id.Name] = admissionValue{kind: "borrowed", refs: []location{p.s.loc(a)}} } }
			}
		}
		if u, ok := n.(*ast.UnaryExpr); ok && u.Op == token.AND { if id, ok := u.X.(*ast.Ident); ok { if _, exists := env[id.Name]; exists { env[id.Name] = admissionValue{kind: "borrowed", refs: []location{p.s.loc(u)}} } } }
		return true
	})
}
func (p *goAdmissionProof) origin(x ast.Expr, env map[string]admissionValue, depth int) admissionValue {
	if depth > 6 { return admissionValue{} }
	if u, ok := x.(*ast.UnaryExpr); ok && u.Op == token.AND {
		if c, ok := u.X.(*ast.CompositeLit); ok && len(c.Elts) == 0 {
			if p.poolType(&ast.StarExpr{X: c.Type}) { return admissionValue{kind: "pool", refs: []location{p.s.loc(c)}} }
		}
	}
	if c, ok := x.(*ast.CallExpr); ok {
		fn, pkg := p.callee(c.Fun, env)
		if fn != nil && !p.active[fn] {
			oldPkg, oldFile := p.pkg, p.file; p.pkg, p.file = pkg, fn.File
			p.active[fn] = true
			v := p.poolConstructor(fn, c.Args, env, depth+1)
			delete(p.active, fn)
			p.pkg, p.file = oldPkg, oldFile
			if v.kind == "pool" { return v }
		}
		if fn != nil && len(fn.AST.Body.List) == 1 {
			if r, ok := fn.AST.Body.List[0].(*ast.ReturnStmt); ok && len(r.Results) > 0 {
				oldPkg, oldFile := p.pkg, p.file; p.pkg, p.file = pkg, fn.File
				v := p.origin(r.Results[0], map[string]admissionValue{}, depth+1)
				p.pkg, p.file = oldPkg, oldFile
				v.refs = goRefs(v.refs, []location{p.s.loc(fn.AST)})
				return v
			}
		}
	}
	return p.expr(x, env)
}

// Constructor purity is not required: its entire body remains a dependency.
// This proves only fresh PGx configuration ownership. The fresh config may flow
// exclusively into NewWithConfig; hook writes, aliases and unknown escapes fail.
func (p *goAdmissionProof) poolConstructor(fn *goFunc, args []ast.Expr, outer map[string]admissionValue, depth int) admissionValue {
	if depth > 6 { return admissionValue{} }
	env := p.params(fn.AST.Type.Params, nil)
	config := ""
	allowed := map[*ast.Ident]bool{}
	pool := ""
	var poolObject *ast.Object
	poolDefinitions := map[*ast.Ident]bool{}
	safe := true
	var result admissionValue
	library := func(c *ast.CallExpr, method string) bool {
		sel, ok := c.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != method { return false }
		id, ok := sel.X.(*ast.Ident)
		if !ok || p.file.ImportPaths[id.Name] != "github.com/jackc/pgx/v5/pgxpool" { return false }
		_, shadow := env[id.Name]
		return !shadow && id.Obj == nil
	}
	for _, stmt := range fn.AST.Body.List {
		if a, ok := stmt.(*ast.AssignStmt); ok {
			if len(a.Rhs) == 1 && len(a.Lhs) == 2 && a.Tok == token.DEFINE {
				if c, ok := a.Rhs[0].(*ast.CallExpr); ok {
					if library(c, "ParseConfig") && len(c.Args) == 1 {
						if id, ok := a.Lhs[0].(*ast.Ident); ok && config == "" { config = id.Name; allowed[id] = true }
					}
					if library(c, "NewWithConfig") && len(c.Args) == 2 {
						if id, ok := c.Args[1].(*ast.Ident); ok && config != "" && id.Name == config { allowed[id] = true; if name, ok := a.Lhs[0].(*ast.Ident); ok { pool = name.Name; poolObject = name.Obj; poolDefinitions[name] = true } }
					}
				}
			}
			if len(a.Lhs) == 1 && len(a.Rhs) == 1 && a.Tok == token.ASSIGN {
				if sel, ok := a.Lhs[0].(*ast.SelectorExpr); ok && sel.Sel.Name == "MaxConns" {
					if id, ok := sel.X.(*ast.Ident); ok && config != "" && id.Name == config && p.expr(a.Rhs[0], env).kind == "scalar" { allowed[id] = true }
				}
			}
		}
		if r, ok := stmt.(*ast.ReturnStmt); ok && len(r.Results) > 0 {
			if id, ok := r.Results[0].(*ast.Ident); ok && poolObject != nil && id.Obj == poolObject {
				result.kind = "pool"
			} else if c, ok := r.Results[0].(*ast.CallExpr); ok && library(c, "NewWithConfig") && len(c.Args) == 2 {
				if id, ok := c.Args[1].(*ast.Ident); ok && config != "" && id.Name == config { allowed[id] = true; result.kind = "pool" }
			} else {
				v := p.origin(r.Results[0], env, depth+1)
				if v.kind == "pool" { result = v }
			}
		}
		// Bind only local names, without pretending arbitrary calls are effect-free.
		if a, ok := stmt.(*ast.AssignStmt); ok { for _, lhs := range a.Lhs { if id, ok := lhs.(*ast.Ident); ok { env[id.Name] = admissionValue{kind: "borrowed"} } } }
	}
	ast.Inspect(fn.AST.Body, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok && config != "" && id.Name == config && !allowed[id] { safe = false }
		if _, ok := n.(*ast.FuncLit); ok { safe = false }
		if r, ok := n.(*ast.ReturnStmt); ok {
			if len(r.Results) == 0 { safe = false } else {
				v := admissionValue{}
				if id, ok := r.Results[0].(*ast.Ident); ok && poolObject != nil && id.Obj == poolObject { v.kind = "pool" } else if c, ok := r.Results[0].(*ast.CallExpr); ok && library(c, "NewWithConfig") && len(c.Args) == 2 {
					if id, ok := c.Args[1].(*ast.Ident); ok && id.Name == config && allowed[id] { v.kind = "pool" }
				} else { v = p.origin(r.Results[0], env, depth+1) }
				if v.kind != "pool" && v.kind != "nil" { safe = false }
			}
		}
		if u, ok := n.(*ast.UnaryExpr); ok && u.Op == token.AND { if id, ok := u.X.(*ast.Ident); ok && id.Name == pool { safe = false } }
		if a, ok := n.(*ast.AssignStmt); ok { for _, lhs := range a.Lhs { if id, ok := lhs.(*ast.Ident); ok && poolObject != nil && id.Obj == poolObject && !poolDefinitions[id] { safe = false } } }
		return true
	})
	if !safe { return admissionValue{} }
	result.refs = goRefs(result.refs, []location{p.s.loc(fn.AST)})
	return result
}
func (p *goAdmissionProof) borrowedPool(fn *goFunc, index int) bool {
	found, safe := false, true
	for _, caller := range p.pkg.Funcs {
		if caller == fn { continue }
		env := p.params(caller.AST.Type.Params, nil)
		oldFile := p.file; p.file = caller.File
		for _, stmt := range caller.AST.Body.List {
			ast.Inspect(stmt, func(n ast.Node) bool {
				if c, ok := n.(*ast.CallExpr); ok { if id, ok := c.Fun.(*ast.Ident); ok && p.pkg.Named[id.Name] == fn && (id.Obj == nil || id.Obj.Kind == ast.Fun) { found = true; if index >= len(c.Args) { safe = false } else { v := p.origin(c.Args[index], env, 0); if v.kind != "pool" && v.kind != "nil" { safe = false }; p.refs = goRefs(p.refs, v.refs) } } }
				return true
			})
			p.bind(stmt, env)
		}
		p.file = oldFile
	}
	return found && safe
}

func (p *goAdmissionProof) contextType(x ast.Expr) bool {
	sel, ok := x.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "Context" { return false }
	id, ok := sel.X.(*ast.Ident)
	return ok && id.Obj == nil && p.file.ImportPaths[id.Name] == "context"
}
func (p *goAdmissionProof) borrowedContext(fn *goFunc, index int, active map[*goFunc]bool) bool {
	if active[fn] || len(active) >= 8 { return false }
	active[fn] = true
	defer delete(active, fn)
	found, safe := false, true
	for _, caller := range p.pkg.Funcs {
		if caller == fn { continue }
		// An unreferenced private wrapper cannot be a source of runtime arguments.
		used := caller.AST.Name.Name == "main"
		for _, other := range p.pkg.Funcs {
			ast.Inspect(other.AST.Body, func(n ast.Node) bool { if id, ok := n.(*ast.Ident); ok && id.Name == caller.AST.Name.Name && (id.Obj == nil || id.Obj.Kind == ast.Fun) { used = true }; return true })
		}
		if !used { continue }
		oldFile := p.file; p.file = caller.File
		env := p.params(caller.AST.Type.Params, nil)
		i := 0
		for _, field := range caller.AST.Type.Params.List {
			for _, name := range field.Names { if p.contextType(field.Type) && p.borrowedContext(caller, i, active) { env[name.Name] = admissionValue{kind: "ctx"} }; i++ }
		}
		for _, stmt := range caller.AST.Body.List {
			ast.Inspect(stmt, func(n ast.Node) bool {
				if c, ok := n.(*ast.CallExpr); ok { if id, ok := c.Fun.(*ast.Ident); ok && p.pkg.Named[id.Name] == fn && (id.Obj == nil || id.Obj.Kind == ast.Fun) { found = true; if index >= len(c.Args) || p.origin(c.Args[index], env, 0).kind != "ctx" { safe = false } } }
				return true
			})
			p.bind(stmt, env)
		}
		p.file = oldFile
	}
	return found && safe
}
