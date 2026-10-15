// Purpose: widen Go extraction for explicit, executable custom-router inventories.
// Depends on: the stdlib AST/value evaluator and existing opaque mount provenance.
// Used by: scanGo after ordinary registration/opaque discovery, before mount binding.
// Invariant: only a consumed, immutable marked table resolves its own dispatch coverage;
// unmarked, partial, mutated and unrelated custom routers retain their opaque findings.
package main

import (
	"go/ast"
	"go/token"
	"strings"
)

// Marker grammar (on a single package-level array/slice composite literal):
//
//	//contractdrift:routes /namespace pathMatcher methodChecker
//	var Routes = [...]NamedRoute{{Method: http.MethodGet, Path: "/namespace/{id}", ...}}
//
// Method/Path must each resolve to one string; parameters occupy whole segments.
// Both named consumers must consist of a FULL range over this table plus a closed
// fallback return. The path consumer calls a pattern helper for each entry and
// returns its match; the method consumer compares entry fields to its arguments.
// The marker declares the complete namespace inventory. Pattern-helper semantics
// and service-kind exhaustiveness are runtime invariants, gated by router tests.
// This is deliberately not an inference from arbitrary literals or an exemption.
//
// Exact split mount/handler grammar adds a final "exact" marker field. All rows
// must have the SAME literal path, equal to the marker path. The two consumers
// are full-table boolean equality walks (Path == path and Method == method).
// A table receiver wraps the path consumer for the owning mount, while the
// returned HandlerFunc starts with a closed !methodConsumer(r.Method) guard.
// It must not dispatch on another path. This resolves a GET/HEAD feed without
// treating a namespace-prefix check or an unrelated opaque handler as exact.
type goRouteTable struct {
	Package      *goPackage
	Name, Prefix string
	TypeName     string
	Exact        bool
	Consumers    []*goFunc
	Routes       []route
	Ref          location
}

func (s *goScanner) resolveRouteTables(packages map[string]*goPackage) {
	var tables []goRouteTable
	for _, p := range packages {
		// Only API-reachable packages were initialized by scanGo.
		if len(p.Globals[0]) == 0 {
			continue
		}
		s.Package = p
		for _, file := range p.Files {
			s.File = file
			for _, decl := range file.AST.Decls {
				gd, ok := decl.(*ast.GenDecl)
				if !ok || gd.Tok != token.VAR || gd.Doc == nil {
					continue
				}
				for _, comment := range gd.Doc.List {
					if !strings.HasPrefix(comment.Text, "//contractdrift:routes") {
						continue
					}
					fields := strings.Fields(comment.Text)
					table, valid := s.readRouteTable(gd, fields)
					if !valid {
						addUnresolved(&s.Out, s.loc(gd), "invalid or incompletely consumed contractdrift route table")
						continue
					}
					tables = append(tables, table)
				}
			}
		}
	}
	for _, table := range tables {
		resolved := map[location]bool{}
		for _, fn := range table.Package.Funcs {
			if !table.Exact && goTableReaches(table.Package, fn, table.Consumers) ||
				table.Exact && goTableExactDispatcher(fn, table.Consumers[1]) {
				resolved[s.loc(fn.AST)] = true
			}
		}
		// A declaration without an actual opaque HTTP entrypoint does not invent routes.
		used := false
		for _, f := range s.Out.Unresolved {
			if len(f.Locations) > 0 && f.Method == "" && resolved[f.Locations[0]] {
				used = true
			}
		}
		if table.Exact && used {
			mounted := false
			for _, p := range packages {
				for _, fn := range p.Funcs {
					if s.tableMount(fn, table) {
						mounted = true
					}
				}
			}
			used = mounted
		}
		if !used {
			addUnresolved(&s.Out, table.Ref, "route table has no consuming HTTP dispatcher")
			continue
		}
		s.Out.Routes = append(s.Out.Routes, table.Routes...)
		remaining := s.Out.Unresolved[:0]
		for _, f := range s.Out.Unresolved {
			known := len(f.Locations) > 0 && f.Method == "" && resolved[f.Locations[0]]
			// The owning mount must reference the actual exported table's namespace
			// method. Merely importing a package never proves that it is dispatched.
			if f.Method == "" && len(f.Locations) > 0 {
				for _, p := range packages {
					for _, fn := range p.Funcs {
						if s.loc(fn.AST) == f.Locations[0] && s.tableMount(fn, table) {
							known = true
						}
					}
				}
			}
			if known {
				path := f.Path
				if path == "" {
					path = table.Prefix
				}
				s.Out.Notes = append(s.Out.Notes, finding{Kind: "GO_ROUTE_TABLE", Method: f.Method, Path: path,
					Detail: "dispatch resolved from consumed route table " + table.Name, Locations: goRefs([]location{table.Ref}, f.Locations)})
			} else {
				remaining = append(remaining, f)
			}
		}
		s.Out.Unresolved = remaining
	}
}

func (s *goScanner) readRouteTable(gd *ast.GenDecl, fields []string) (goRouteTable, bool) {
	table := goRouteTable{Package: s.Package, Ref: s.loc(gd)}
	table.Exact = len(fields) == 5 && fields[4] == "exact"
	if (len(fields) != 4 && !table.Exact) || fields[0] != "//contractdrift:routes" || !strings.HasPrefix(fields[1], "/") || len(gd.Specs) != 1 {
		return table, false
	}
	table.Prefix = fields[1]
	vs, ok := gd.Specs[0].(*ast.ValueSpec)
	if !ok || len(vs.Names) != 1 || len(vs.Values) != 1 {
		return table, false
	}
	table.Name = vs.Names[0].Name
	lit, ok := vs.Values[0].(*ast.CompositeLit)
	if !ok {
		return table, false
	}
	array, ok := lit.Type.(*ast.ArrayType)
	if named, namedOK := lit.Type.(*ast.Ident); namedOK {
		table.TypeName = named.Name
		for _, file := range s.Package.Files {
			for _, decl := range file.AST.Decls {
				if gd, yes := decl.(*ast.GenDecl); yes {
					for _, spec := range gd.Specs {
						if ty, yes := spec.(*ast.TypeSpec); yes && ty.Name.Name == named.Name {
							array, ok = ty.Type.(*ast.ArrayType)
						}
					}
				}
			}
		}
	}
	if !ok {
		return table, false
	}
	if _, ok := array.Elt.(*ast.Ident); !ok {
		return table, false
	}
	value := s.composite(lit, array, s.Package.Globals)
	if !value.Sequence || len(value.Items) == 0 || len(value.Items) != len(lit.Elts) {
		return table, false
	}
	seen := map[string]bool{}
	for i, item := range value.Items {
		method, path := item.Fields["Method"], item.Fields["Path"]
		if method.Unknown || path.Unknown || len(method.Text) != 1 || len(path.Text) != 1 {
			return table, false
		}
		m, p := method.Text[0], path.Text[0]
		pathOK := strings.HasPrefix(p, table.Prefix+"/")
		if table.Exact {
			pathOK = p == table.Prefix && !strings.ContainsAny(p, "{}")
		}
		if !bffHTTP(m) || !pathOK || !goTablePattern(p) || seen[m+" "+p] {
			return table, false
		}
		seen[m+" "+p] = true
		table.Routes = append(table.Routes, route{Method: m, Path: normalizePath(p), Locations: goRefs([]location{s.loc(lit.Elts[i])}, method.Refs, path.Refs, []location{table.Ref})})
	}
	for i, name := range fields[2:4] {
		fn := s.Package.Named[name]
		valid := fn != nil
		if valid && table.Exact {
			valid = goTableExactConsumer(fn, table.Name, i == 1)
		}
		if valid && !table.Exact {
			valid = goTableConsumer(fn, table.Name, i == 1)
		}
		if !valid {
			return table, false
		}
		table.Consumers = append(table.Consumers, fn)
	}
	// Every reference to the global must be the full range expression of a consumer.
	// This rejects aliases, append, reassignment, index writes, address escape and slices.
	reads := map[*ast.Ident]bool{}
	for _, fn := range table.Consumers {
		loop := fn.AST.Body.List[0].(*ast.RangeStmt)
		reads[loop.X.(*ast.Ident)] = true
	}
	for _, file := range s.Package.Files {
		valid := true
		ast.Inspect(file.AST, func(n ast.Node) bool {
			if n == gd {
				return false
			}
			if id, ok := n.(*ast.Ident); ok && id.Name == table.Name && !reads[id] {
				valid = false
			}
			return true
		})
		if !valid {
			return table, false
		}
	}
	return table, true
}

func goTablePattern(path string) bool {
	for i, part := range strings.Split(path, "/") {
		if i == 0 {
			continue
		}
		if part == "" || part == "." || part == ".." || strings.ContainsAny(part, "?%\\") {
			return false
		}
		if strings.ContainsAny(part, "{}") && !(strings.HasPrefix(part, "{") && strings.HasSuffix(part, "}") && strings.Count(part, "{") == 1 && strings.Count(part, "}") == 1) {
			return false
		}
	}
	return true
}

// The deliberately small supported consumer grammar fails closed on filtering,
// truncation or an extra route-selection branch outside the table walk.
func goTableConsumer(fn *goFunc, name string, methods bool) bool {
	body := fn.AST.Body.List
	if len(body) != 2 {
		return false
	}
	loop, ok := body[0].(*ast.RangeStmt)
	if !ok || loop.Tok != token.DEFINE || len(loop.Body.List) != 1 {
		return false
	}
	x, ok := loop.X.(*ast.Ident)
	if !ok || x.Name != name {
		return false
	}
	entry, ok := loop.Value.(*ast.Ident)
	if !ok || entry.Name == "_" {
		return false
	}
	branch, ok := loop.Body.List[0].(*ast.IfStmt)
	if !ok || branch.Else != nil || len(branch.Body.List) != 1 {
		return false
	}
	ret, ok := branch.Body.List[0].(*ast.ReturnStmt)
	if !ok || len(ret.Results) != 1 {
		return false
	}
	fallback, ok := body[1].(*ast.ReturnStmt)
	if !ok || len(fallback.Results) != 1 {
		return false
	}
	if methods {
		yes, a := ret.Results[0].(*ast.Ident)
		no, b := fallback.Results[0].(*ast.Ident)
		if !a || !b || yes.Name != "true" || no.Name != "false" || branch.Init != nil {
			return false
		}
		foundMethod := false
		valid := true
		ast.Inspect(branch.Cond, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.BinaryExpr:
				if n.Op != token.EQL && n.Op != token.LAND {
					valid = false
				}
			case *ast.BasicLit, *ast.CallExpr:
				valid = false
			case *ast.SelectorExpr:
				id, ok := n.X.(*ast.Ident)
				if !ok || id.Name != entry.Name {
					valid = false
				}
				if n.Sel.Name == "Method" {
					foundMethod = true
				}
			}
			return true
		})
		return valid && foundMethod
	}
	init, ok := branch.Init.(*ast.AssignStmt)
	if !ok || init.Tok != token.DEFINE || len(init.Lhs) != 2 || len(init.Rhs) != 1 {
		return false
	}
	call, ok := init.Rhs[0].(*ast.CallExpr)
	if !ok || len(call.Args) != 2 {
		return false
	}
	arg, ok := call.Args[0].(*ast.Ident)
	if !ok || arg.Name != entry.Name {
		return false
	}
	value, a := init.Lhs[0].(*ast.Ident)
	matched, b := init.Lhs[1].(*ast.Ident)
	cond, c := branch.Cond.(*ast.Ident)
	result, d := ret.Results[0].(*ast.Ident)
	closed, e := fallback.Results[0].(*ast.CompositeLit)
	return a && b && c && d && e && cond.Name == matched.Name && result.Name == value.Name && len(closed.Elts) == 0
}

func goTableReaches(p *goPackage, entry *goFunc, consumers []*goFunc) bool {
	seen := map[*goFunc]bool{}
	queue := []*goFunc{entry}
	for len(queue) > 0 {
		fn := queue[0]
		queue = queue[1:]
		if seen[fn] {
			continue
		}
		seen[fn] = true
		ast.Inspect(fn.AST.Body, func(n ast.Node) bool {
			if call, ok := n.(*ast.CallExpr); ok {
				if callee := p.Named[goCallName(call.Fun)]; callee != nil {
					queue = append(queue, callee)
				}
			}
			return true
		})
	}
	for _, fn := range consumers {
		if !seen[fn] {
			return false
		}
	}
	return true
}

// tableMount recognizes only a call on the imported table to a receiver method
// whose entire body returns strings.HasPrefix(path, the declared namespace).
// It cannot bless an unrelated custom dispatcher in the same assembly file.
func (s *goScanner) tableMount(fn *goFunc, table goRouteTable) bool {
	found := false
	ast.Inspect(fn.AST.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || len(call.Args) != 1 || !goPathExpression(call.Args[0]) {
			return true
		}
		method, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		variable, ok := method.X.(*ast.SelectorExpr)
		if !ok || variable.Sel.Name != table.Name {
			return true
		}
		pkg, ok := variable.X.(*ast.Ident)
		if !ok || s.Packages[fn.File.ImportPaths[pkg.Name]] != table.Package {
			return true
		}
		for _, target := range table.Package.ByName[method.Sel.Name] {
			if target.AST.Recv == nil || len(target.AST.Body.List) != 1 {
				continue
			}
			receiver, ok := target.AST.Recv.List[0].Type.(*ast.Ident)
			if !ok || receiver.Name != table.TypeName || table.TypeName == "" {
				continue
			}
			ret, ok := target.AST.Body.List[0].(*ast.ReturnStmt)
			if !ok || len(ret.Results) != 1 {
				continue
			}
			if table.Exact {
				call, ok := ret.Results[0].(*ast.CallExpr)
				if !ok || len(call.Args) != 1 || goCallName(call.Fun) != table.Consumers[0].AST.Name.Name ||
					target.AST.Type.Params == nil || len(target.AST.Type.Params.List) != 1 || len(target.AST.Type.Params.List[0].Names) != 1 {
					continue
				}
				helper, direct := call.Fun.(*ast.Ident)
				arg, ok := call.Args[0].(*ast.Ident)
				if direct && helper.Name == table.Consumers[0].AST.Name.Name && ok && arg.Name == target.AST.Type.Params.List[0].Names[0].Name {
					found = true
				}
				continue
			}
			prefix, ok := ret.Results[0].(*ast.CallExpr)
			if !ok || len(prefix.Args) != 2 {
				continue
			}
			if target.AST.Type.Params == nil || len(target.AST.Type.Params.List) != 1 || len(target.AST.Type.Params.List[0].Names) != 1 {
				continue
			}
			argument, ok := prefix.Args[0].(*ast.Ident)
			if !ok || argument.Name != target.AST.Type.Params.List[0].Names[0].Name {
				continue
			}
			sel, ok := prefix.Fun.(*ast.SelectorExpr)
			if !ok || sel.Sel.Name != "HasPrefix" {
				continue
			}
			alias, ok := sel.X.(*ast.Ident)
			if !ok || target.File.ImportPaths[alias.Name] != "strings" {
				continue
			}
			s.Package, s.File = table.Package, target.File
			value := s.expr(prefix.Args[1], table.Package.Globals)
			if !value.Unknown && len(value.Text) == 1 && value.Text[0] == table.Prefix {
				found = true
			}
		}
		return true
	})
	return found
}

// Exact consumers cannot filter or truncate the table, or accept a value outside it.
func goTableExactConsumer(fn *goFunc, name string, methods bool) bool {
	body := fn.AST.Body.List
	if len(body) != 2 || fn.AST.Type.Params == nil || len(fn.AST.Type.Params.List) != 1 || len(fn.AST.Type.Params.List[0].Names) != 1 {
		return false
	}
	loop, ok := body[0].(*ast.RangeStmt)
	if !ok || loop.Tok != token.DEFINE || len(loop.Body.List) != 1 {
		return false
	}
	source, ok := loop.X.(*ast.Ident)
	if !ok || source.Name != name {
		return false
	}
	entry, ok := loop.Value.(*ast.Ident)
	if !ok || entry.Name == "_" {
		return false
	}
	branch, ok := loop.Body.List[0].(*ast.IfStmt)
	if !ok || branch.Init != nil || branch.Else != nil || len(branch.Body.List) != 1 {
		return false
	}
	cond, ok := branch.Cond.(*ast.BinaryExpr)
	if !ok || cond.Op != token.EQL {
		return false
	}
	field, ok := cond.X.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	owner, ok := field.X.(*ast.Ident)
	if !ok || owner.Name != entry.Name {
		return false
	}
	want := "Path"
	if methods {
		want = "Method"
	}
	arg, ok := cond.Y.(*ast.Ident)
	if !ok || field.Sel.Name != want || arg.Name != fn.AST.Type.Params.List[0].Names[0].Name {
		return false
	}
	yes, ok := branch.Body.List[0].(*ast.ReturnStmt)
	if !ok || len(yes.Results) != 1 {
		return false
	}
	no, ok := body[1].(*ast.ReturnStmt)
	if !ok || len(no.Results) != 1 {
		return false
	}
	y, ok := yes.Results[0].(*ast.Ident)
	n, noOK := no.Results[0].(*ast.Ident)
	return ok && noOK && y.Name == "true" && n.Name == "false"
}

// Only the direct returned HandlerFunc's first, closed method guard can claim
// the method half of an exact table. A path-dispatching handler stays opaque.
func goTableExactDispatcher(fn *goFunc, consumer *goFunc) bool {
	body := fn.AST.Body.List
	if len(body) != 1 {
		return false
	}
	ret, ok := body[0].(*ast.ReturnStmt)
	if !ok || len(ret.Results) != 1 {
		return false
	}
	wrapper, ok := ret.Results[0].(*ast.CallExpr)
	if !ok || len(wrapper.Args) != 1 {
		return false
	}
	sel, ok := wrapper.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "HandlerFunc" {
		return false
	}
	pkg, ok := sel.X.(*ast.Ident)
	if !ok || fn.File.ImportPaths[pkg.Name] != "net/http" {
		return false
	}
	handler, ok := wrapper.Args[0].(*ast.FuncLit)
	if !ok || len(handler.Body.List) == 0 {
		return false
	}
	guard, ok := handler.Body.List[0].(*ast.IfStmt)
	if !ok || guard.Init != nil || guard.Else != nil || len(guard.Body.List) == 0 {
		return false
	}
	neg, ok := guard.Cond.(*ast.UnaryExpr)
	if !ok || neg.Op != token.NOT {
		return false
	}
	call, ok := neg.X.(*ast.CallExpr)
	if !ok || goCallName(call.Fun) != consumer.AST.Name.Name || len(call.Args) != 1 {
		return false
	}
	helper, direct := call.Fun.(*ast.Ident)
	if !direct || helper.Name != consumer.AST.Name.Name {
		return false
	}
	method, ok := call.Args[0].(*ast.SelectorExpr)
	if !ok || method.Sel.Name != "Method" {
		return false
	}
	if handler.Type.Params == nil || len(handler.Type.Params.List) != 2 || len(handler.Type.Params.List[1].Names) != 1 {
		return false
	}
	request, ok := method.X.(*ast.Ident)
	if !ok || request.Name != handler.Type.Params.List[1].Names[0].Name {
		return false
	}
	closed, ok := guard.Body.List[len(guard.Body.List)-1].(*ast.ReturnStmt)
	if !ok || len(closed.Results) != 0 {
		return false
	}
	valid := true
	ast.Inspect(handler.Body, func(n ast.Node) bool {
		if sel, ok := n.(*ast.SelectorExpr); ok && sel.Sel.Name == "Path" {
			valid = false
		}
		return true
	})
	return valid
}
