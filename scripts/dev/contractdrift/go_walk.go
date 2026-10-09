// Purpose: walk registration control flow and classify business routes versus fallbacks.
// Depends on: Go stdlib only; no DB/API/network mutations.
// Used by: CI-DRIFT scanGo and its parser gates.
// Invariant: extraction never executes handlers or discards unresolved registration coverage.
package main

import (
	"go/ast"
	"go/token"
	"strconv"
	"strings"
)

func goRegistration(c *ast.CallExpr) bool {
	return goRegistrarSelector(c.Fun) && len(c.Args) >= 2
}

func goRegistrarSelector(expr ast.Expr) bool {
	sel, ok := expr.(*ast.SelectorExpr)
	return ok && (sel.Sel.Name == "HandleFunc" || sel.Sel.Name == "Handle")
}

func (s *goScanner) function(fn *goFunc, args []goValue, caller *location) {
	if s.Stack[fn.Key] {
		ref := s.loc(fn.AST)
		if caller != nil {
			ref = *caller
		}
		addUnresolved(&s.Out, ref, "recursive Go registration wrapper "+fn.AST.Name.Name)
		return
	}
	oldFile, oldCallers := s.File, s.Callers
	s.File = fn.File
	if caller != nil {
		s.Callers = append(append([]location{}, s.Callers...), *caller)
	}
	e := append(goClone(s.Package.Globals), map[string]goValue{})
	i := 0
	for _, param := range fn.AST.Type.Params.List {
		for _, name := range param.Names {
			v := goValue{Unknown: true, Refs: []location{s.loc(param)}}
			if i < len(args) {
				v = args[i]
				v.Refs = goRefs(v.Refs, []location{s.loc(param)})
			}
			if pointer, ok := param.Type.(*ast.StarExpr); ok {
				if typ, ok := pointer.X.(*ast.SelectorExpr); ok && typ.Sel.Name == "ServeMux" {
					if pkg, ok := typ.X.(*ast.Ident); ok && pkg.Name == s.File.HTTP && s.File.HTTP != "" {
						v.Mux = true
						v.Refs = goRefs(v.Refs, []location{s.File.ImportRefs[pkg.Name]})
					}
				}
			}
			e.set(name.Name, v, true)
			i++
		}
	}
	s.Stack[fn.Key] = true
	s.block(fn.AST.Body, e, false)
	delete(s.Stack, fn.Key)
	s.File, s.Callers = oldFile, oldCallers
}

func (s *goScanner) block(block *ast.BlockStmt, e goEnv, scope bool) {
	if block == nil {
		return
	}
	if scope {
		e = append(e, map[string]goValue{})
	}
	for _, stmt := range block.List {
		s.stmt(stmt, e)
	}
}

func (s *goScanner) stmt(stmt ast.Stmt, e goEnv) {
	switch x := stmt.(type) {
	case *ast.DeclStmt:
		if d, ok := x.Decl.(*ast.GenDecl); ok {
			s.declare(d, e)
		}
	case *ast.AssignStmt:
		values := make([]goValue, len(x.Rhs))
		for i, rhs := range x.Rhs {
			values[i] = s.expr(rhs, e)
			s.calls(rhs, e)
		}
		for i, lhs := range x.Lhs {
			if name, ok := lhs.(*ast.Ident); ok {
				v := goValue{Unknown: true}
				if i < len(values) {
					v = values[i]
				}
				v.Refs = goRefs(v.Refs, []location{s.loc(x)})
				if x.Tok == token.ADD_ASSIGN {
					old := e.get(name.Name)
					merged := goValue{Unknown: old.Unknown || v.Unknown, Refs: goRefs(old.Refs, v.Refs)}
					for _, a := range old.Text {
						for _, b := range v.Text {
							merged.Text = append(merged.Text, a+b)
						}
					}
					v = merged
				}
				define := x.Tok == token.DEFINE
				if define {
					_, define = e[len(e)-1][name.Name]
					define = !define
				}
				e.set(name.Name, v, define)
			}
		}
	case *ast.ExprStmt:
		s.calls(x.X, e)
	case *ast.ReturnStmt:
		for _, r := range x.Results {
			s.calls(r, e)
		}
	case *ast.BlockStmt:
		s.block(x, e, true)
	case *ast.IfStmt:
		scope := append(e, map[string]goValue{})
		if x.Init != nil {
			s.stmt(x.Init, scope)
		}
		a, b := goClone(scope), goClone(scope)
		s.block(x.Body, a, true)
		if x.Else != nil {
			s.stmt(x.Else, b)
		}
		goMerge(scope, a, b)
	case *ast.RangeStmt:
		seq := s.expr(x.X, e)

		items := seq.Items
		if len(items) == 0 && seq.Sequence && !seq.Unknown {
			ast.Inspect(x.Body, func(n ast.Node) bool {
				if c, ok := n.(*ast.CallExpr); ok && goRegistration(c) {
					s.Seen[c.Pos()] = true
				}
				return true
			})
			return
		}
		before := goClone(e)
		if seq.Unknown || len(items) == 0 {
			// Scan unknown alternatives even when a branch/append also supplied known items.
			items = append(append([]goValue{}, items...), goValue{Unknown: true, Refs: goRefs(seq.Refs, []location{s.loc(x.X)})})
		}
		for _, item := range items {
			scope := append(e, map[string]goValue{})
			if id, ok := x.Value.(*ast.Ident); ok {
				item.Refs = goRefs(seq.Refs, item.Refs)
				scope.set(id.Name, item, true)
			}
			if id, ok := x.Key.(*ast.Ident); ok {
				scope.set(id.Name, goValue{Unknown: true, Refs: seq.Refs}, true)
			}
			s.block(x.Body, scope, false)
		}
		if seq.Unknown {
			goMerge(e, before, goClone(e))
		}
	case *ast.ForStmt:
		before := goClone(e)
		scope := append(e, map[string]goValue{})
		if x.Init != nil {
			s.stmt(x.Init, scope)
		}
		s.block(x.Body, scope, false)
		goMerge(e, before, goClone(e))
	case *ast.SwitchStmt:
		scope := append(e, map[string]goValue{})
		if x.Init != nil {
			s.stmt(x.Init, scope)
		}
		s.cases(x.Body.List, scope)
	case *ast.TypeSwitchStmt:
		scope := append(e, map[string]goValue{})
		if x.Init != nil {
			s.stmt(x.Init, scope)
		}
		if x.Assign != nil {
			s.stmt(x.Assign, scope)
		}
		s.cases(x.Body.List, scope)
	case *ast.DeferStmt:
		s.calls(x.Call, e)
	case *ast.GoStmt:
		s.calls(x.Call, e)
	case *ast.LabeledStmt:
		s.stmt(x.Stmt, e)
	}
}

func (s *goScanner) cases(cases []ast.Stmt, e goEnv) {
	before := goClone(e)
	merged := goClone(e)
	hasDefault := false
	first := true
	for _, stmt := range cases {
		c, ok := stmt.(*ast.CaseClause)
		if !ok {
			continue
		}
		if c.List == nil {
			hasDefault = true
		}
		branch := goClone(e)
		scope := append(branch, map[string]goValue{})
		for _, body := range c.Body {
			s.stmt(body, scope)
		}
		if first {
			merged = branch
			first = false
		} else {
			goMerge(merged, goClone(merged), branch)
		}
	}
	if !hasDefault {
		goMerge(merged, before, goClone(merged))
	}
	for i := range e {
		for k, v := range merged[i] {
			e[i][k] = v
		}
	}
}

func (s *goScanner) calls(expr ast.Expr, e goEnv) {
	ast.Inspect(expr, func(n ast.Node) bool {
		if _, ok := n.(*ast.FuncLit); ok {
			return false
		}
		c, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		if goRegistration(c) {
			s.registration(c, e)
			return false
		}
		callee := s.expr(c.Fun, e)
		if callee.Registrar {
			refs := goRefs([]location{s.loc(c)}, callee.Refs, s.Callers)
			var pattern goValue
			for i, arg := range c.Args {
				value := s.expr(arg, e)
				refs = goRefs(refs, value.Refs)
				if i == 0 {
					pattern = value
				}
			}
			f := finding{Kind: "UNRESOLVED", Detail: "opaque Go registration through a method value", Locations: refs}
			if len(pattern.Text) == 1 {
				fields := strings.Fields(pattern.Text[0])
				if len(fields) == 2 {
					f.Method, f.Path = fields[0], normalizePath(fields[1])
				} else if len(fields) == 1 && strings.HasPrefix(fields[0], "/") {
					f.Method, f.Path = "MOUNT", normalizePath(fields[0])
				}
			}
			s.Out.Unresolved = append(s.Out.Unresolved, f)
			return false
		}
		if fn := s.Package.Named[goCallName(c.Fun)]; fn != nil {
			args := make([]goValue, len(c.Args))
			registration := s.Package.Routes[fn.Key]
			for i, arg := range c.Args {
				args[i] = s.expr(arg, e)
				registration = registration || args[i].Registrar
			}
			if !registration {
				return true
			}
			ref := s.loc(c)
			s.function(fn, args, &ref)
			return false
		}
		return true
	})
}

func (s *goScanner) fallback(expr ast.Expr, e goEnv) (bool, []location) {
	if s.goFallback(expr, e) {
		return true, s.expr(expr, e).Refs
	}
	call, ok := expr.(*ast.CallExpr)
	if !ok {
		return false, nil
	}
	// Only the unshadowed net/http conversion is transparent; constructor names prove nothing.
	if sel, ok := call.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "HandlerFunc" && len(call.Args) == 1 {
		if id, ok := sel.X.(*ast.Ident); ok && id.Name == s.File.HTTP && s.File.HTTP != "" {
			if _, shadowed := e.lookup(id.Name); !shadowed {
				if _, literal := call.Args[0].(*ast.FuncLit); literal {
					proof, refs := s.fallback(call.Args[0], e)
					return proof, goRefs(refs, []location{s.loc(call), s.File.ImportRefs[id.Name]})
				}
			}
		}
	}
	fn := s.Package.Named[goCallName(call.Fun)]
	if _, shadowed := e.lookup(goCallName(call.Fun)); shadowed {
		fn = nil
	}
	if len(call.Args) == 1 && fn != nil && goHeaderDecorator(fn) {
		proof, refs := s.fallback(call.Args[0], e)
		return proof, goRefs(refs, []location{s.loc(fn.AST)})
	}
	return s.factory405(call, e)
}

// A syntactically transparent header-only decorator preserves its child's 405.
func goHeaderDecorator(source *goFunc) bool {
	fn := source.AST
	if fn.Type.Params == nil || len(fn.Type.Params.List) != 1 || len(fn.Type.Params.List[0].Names) != 1 || len(fn.Body.List) != 1 {
		return false
	}
	param := fn.Type.Params.List[0].Names[0].Name
	ret, ok := fn.Body.List[0].(*ast.ReturnStmt)
	if !ok || len(ret.Results) != 1 {
		return false
	}
	lit, ok := ret.Results[0].(*ast.FuncLit)
	if !ok || len(lit.Body.List) == 0 {
		return false
	}
	bound := go405Params(lit.Type.Params, source.File, nil, true)
	for i, stmt := range lit.Body.List {
		e, ok := stmt.(*ast.ExprStmt)
		if !ok {
			return false
		}
		call, ok := e.X.(*ast.CallExpr)
		if !ok {
			return false
		}
		if i == len(lit.Body.List)-1 {
			id, ok := call.Fun.(*ast.Ident)
			if !ok || id.Name != param || len(call.Args) != 2 {
				return false
			}
			w, writer := call.Args[0].(*ast.Ident)
			r, request := call.Args[1].(*ast.Ident)
			return writer && request && bound[w.Name].Writer && bound[r.Name].Request
		}
		// Header arguments may call a writer before the 405 delegate. Only a proven pure Header operation is transparent.
		if !go405Header(call, bound) {
			return false
		}
	}
	return false
}

func (s *goScanner) goFallback(expr ast.Expr, e goEnv) bool {
	if fn, ok := expr.(*ast.FuncLit); ok && len(fn.Body.List) == 1 {
		if fn.Type.Params == nil || len(fn.Type.Params.List) == 0 || len(fn.Type.Params.List[0].Names) != 1 {
			return false
		}
		writer := fn.Type.Params.List[0]
		typ, ok := writer.Type.(*ast.SelectorExpr)
		if !ok || typ.Sel.Name != "ResponseWriter" {
			return false
		}
		pkg, ok := typ.X.(*ast.Ident)
		if !ok || pkg.Name != s.File.HTTP || s.File.HTTP == "" {
			return false
		}
		stmt, ok := fn.Body.List[0].(*ast.ExprStmt)
		if !ok {
			return false
		}
		call, ok := stmt.X.(*ast.CallExpr)
		if !ok {
			return false
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "WriteHeader" || len(call.Args) != 1 {
			return false
		}
		receiver, ok := sel.X.(*ast.Ident)
		if !ok || receiver.Name != writer.Names[0].Name {
			return false
		}
		if status, ok := call.Args[0].(*ast.SelectorExpr); ok {
			alias, ok := status.X.(*ast.Ident)
			if !ok || alias.Name != s.File.HTTP || status.Sel.Name != "StatusMethodNotAllowed" {
				return false
			}
			if _, shadowed := e.lookup(alias.Name); shadowed {
				return false
			}
			for _, param := range fn.Type.Params.List {
				for _, name := range param.Names {
					if name.Name == alias.Name {
						return false
					}
				}
			}
			return true
		}
		if status, ok := call.Args[0].(*ast.BasicLit); ok && status.Kind == token.INT {
			value, err := strconv.ParseInt(status.Value, 0, 64)
			return err == nil && value == 405
		}
	}
	return false
}

func (s *goScanner) registration(c *ast.CallExpr, e goEnv) {
	s.Seen[c.Pos()] = true
	value := s.expr(c.Args[0], e)
	ref := s.loc(c)

	refs := goRefs([]location{ref}, value.Refs, s.Callers)
	if value.Unknown || len(value.Text) == 0 {
		f := finding{Kind: "UNRESOLVED", Detail: "Go mux registration pattern cannot be statically resolved", Locations: refs}
		s.Out.Unresolved = append(s.Out.Unresolved, f)
	}
	for _, pattern := range value.Text {
		fields := strings.Fields(pattern)
		fallback, proofRefs := s.fallback(c.Args[1], e)
		patternRefs := goRefs(refs, proofRefs)
		if len(fields) == 1 && strings.HasPrefix(fields[0], "/") {
			// ponytail: no mux identity is tracked; a constructor name or sibling routes cannot prove this child.
			mountRefs := goRefs(patternRefs, s.expr(c.Args[1], e).Refs)
			s.Out.Notes = append(s.Out.Notes, finding{Kind: "GO_MOUNT_OR_FALLBACK", Path: normalizePath(fields[0]), Detail: "methodless registration; child paths remain absolute unless StripPrefix is explicit", Locations: mountRefs})
			if sel, ok := c.Fun.(*ast.SelectorExpr); ok && !fallback {
				method, detail := "MOUNT", "opaque Go methodless mount; child route identity is unproven"
				// MOUNT cannot justify child coverage in compare: even '/' may be an empty/404 handler.
				// Existing explicit custom guards and HandleFunc paths keep their EXACT/PREFIX evidence.
				if sel.Sel.Name == "HandleFunc" {
					method, detail = "EXACT", "opaque Go methodless handler; business methods unknown"
					if strings.HasSuffix(fields[0], "/") {
						method = "PREFIX"
					}
				}
				s.Out.Unresolved = append(s.Out.Unresolved, finding{Kind: "UNRESOLVED", Method: method, Path: normalizePath(fields[0]), Detail: detail, Locations: mountRefs})
			}
			continue
		}
		if len(fields) == 2 && strings.HasPrefix(fields[1], "/") {
			if fallback {
				s.Out.Notes = append(s.Out.Notes, finding{Kind: "GO_405_FALLBACK", Method: fields[0], Path: normalizePath(fields[1]), Detail: "explicit wrong-method fallback", Locations: patternRefs})
			} else {
				addRoute(&s.Out, fields[0], fields[1], patternRefs...)
			}
			continue
		}
		s.Out.Unresolved = append(s.Out.Unresolved, finding{Kind: "UNRESOLVED", Detail: "unsupported Go ServeMux pattern " + pattern, Locations: refs})
	}
}
