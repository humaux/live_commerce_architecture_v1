// Purpose: retain unknown custom dispatcher coverage and link actual mount guards to child spans.
// Depends on: Go stdlib only; no DB/API/network mutations.
// Used by: CI-DRIFT scanGo and its parser gates.
// Invariant: extraction never executes handlers or discards unresolved registration coverage.

package main

import (
	"go/ast"
	"go/token"
	"path/filepath"
	"strconv"
	"strings"
)

// ponytail: custom dispatch is conservatively opaque, not inferred from unrelated path literals.
// These spans keep edits to buyer/webhook/feed/TLS routers inside the progressive unknown gate.
func (s *goScanner) opaque(fn *goFunc) {
	returnsHandler := fn.AST.Name.Name == "ServeHTTP"
	if fn.AST.Type.Results != nil {
		for _, r := range fn.AST.Type.Results.List {
			if sel, ok := r.Type.(*ast.SelectorExpr); ok && (sel.Sel.Name == "Handler" || sel.Sel.Name == "HandlerFunc") {
				returnsHandler = true
			}
		}
	}
	if !returnsHandler {
		return
	}
	// A HandlerFunc decorating an explicit mux registration already has a known pattern.
	// Method validation in such a decorator is not a second router inventory.
	if fn.AST.Type.Results != nil {
		for _, r := range fn.AST.Type.Results.List {
			if sel, ok := r.Type.(*ast.SelectorExpr); ok && sel.Sel.Name == "HandlerFunc" {
				return
			}
		}
	}
	// Follow references with their owning package. Function identity, not a bare
	// name, breaks cycles without losing same-name helpers in imported packages.
	owner, file := s.Package, s.File
	defer func() { s.Package, s.File = owner, file }()
	type dependency struct {
		pkg *goPackage
		fn  *goFunc
	}
	seen := map[*goFunc]bool{}
	seenGlobals := map[ast.Node]bool{}
	queue := []dependency{{owner, fn}}
	dispatch := false
	refs := []location{s.loc(fn.AST)}
	var inspect func(ast.Node, *goPackage, *goFile)
	inspect = func(node ast.Node, pkg *goPackage, source *goFile) {
		ast.Inspect(node, func(n ast.Node) bool {
			if sel, ok := n.(*ast.SelectorExpr); ok {
				if sel.Sel.Name == "Path" || sel.Sel.Name == "Method" || strings.HasPrefix(sel.Sel.Name, "Method") {
					dispatch = true
				}
				if id, ok := sel.X.(*ast.Ident); ok && id.Obj == nil && source.ImportPaths[id.Name] != "" {
					if child := s.Packages[source.ImportPaths[id.Name]]; child != nil {
						if callee := child.Named[sel.Sel.Name]; callee != nil {
							refs = goRefs(refs, []location{source.ImportRefs[id.Name]})
							queue = append(queue, dependency{child, callee})
						}
					}
					return false // selector members are not unqualified package references
				}
				// Resolve syntactically typed receivers; never treat an arbitrary
				// selector member as every same-name function in this package.
				var receiver ast.Expr
				if id, ok := sel.X.(*ast.Ident); ok && id.Obj != nil {
					switch decl := id.Obj.Decl.(type) {
					case *ast.Field:
						receiver = decl.Type
					case *ast.ValueSpec:
						receiver = decl.Type
						if receiver == nil && len(decl.Names) == len(decl.Values) {
							for i, name := range decl.Names {
								if name.Obj == id.Obj {
									receiver = decl.Values[i]
								}
							}
						}
					case *ast.AssignStmt:
						if decl.Tok == token.DEFINE && len(decl.Lhs) == len(decl.Rhs) {
							for i, lhs := range decl.Lhs {
								if name, ok := lhs.(*ast.Ident); ok && name.Obj == id.Obj {
									receiver = decl.Rhs[i]
								}
							}
						}
					}
				}
				if receiver == nil {
					if literal, ok := sel.X.(*ast.CompositeLit); ok {
						receiver = literal
					}
				}
				// Only direct static declarations/literals establish the receiver
				// type here; do not infer types through calls or alias flow.
				if address, ok := receiver.(*ast.UnaryExpr); ok && address.Op == token.AND {
					receiver = address.X
				}
				if literal, ok := receiver.(*ast.CompositeLit); ok {
					receiver = literal.Type
				}
				if pointer, ok := receiver.(*ast.StarExpr); ok {
					receiver = pointer.X
				}
				if typ, ok := receiver.(*ast.Ident); ok {
					for _, callee := range pkg.ByName[sel.Sel.Name] {
						if goDispatcherReceiver(callee) == typ.Name {
							queue = append(queue, dependency{pkg, callee})
						}
					}
				}
				inspect(sel.X, pkg, source)
				return false
			}
			if id, ok := n.(*ast.Ident); ok {
				if id.Obj != nil && source.AST.Scope.Lookup(id.Name) != id.Obj {
					return true
				}
				if callee := pkg.Named[id.Name]; callee != nil {
					queue = append(queue, dependency{pkg, callee})
				}
				// Read declarations directly: imported package evaluation order must not
				// determine provenance, and non-path/unknown values can affect a handler.
				for _, f := range pkg.Files {
					for _, decl := range f.AST.Decls {
						gd, ok := decl.(*ast.GenDecl)
						if !ok || (gd.Tok != token.VAR && gd.Tok != token.CONST) {
							continue
						}
						for _, spec := range gd.Specs {
							v, ok := spec.(*ast.ValueSpec)
							if !ok {
								continue
							}
							for _, name := range v.Names {
								if name.Name == id.Name && !seenGlobals[v] {
									seenGlobals[v] = true
									refs = goRefs(refs, []location{s.loc(v)})
									for _, value := range v.Values {
										inspect(value, pkg, f)
									}
								}
							}
						}
					}
				}
			}
			return true
		})
	}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		if seen[current.fn] {
			continue
		}
		seen[current.fn] = true
		refs = goRefs(refs, []location{s.loc(current.fn.AST)})
		inspect(current.fn.AST.Body, current.pkg, current.fn.File)
	}
	s.Package, s.File = owner, fn.File
	// Only actual custom mount guards provide coverage; provider API strings in
	// underlying handlers cannot define this API's served namespace.
	if !dispatch {
		return
	}
	ref := s.loc(fn.AST)
	detail := "opaque Go handler dispatch " + fn.AST.Name.Name + "; route/method coverage is unknown"
	// Retain the complete finding first. After all route tables are consumed,
	// resolveHandlerFactories can prove every returned handler's provenance;
	// failed proofs keep this finding and every dependency span unchanged.
	if strings.HasPrefix(ref.File, "cmd/api/") && strings.HasPrefix(fn.AST.Name.Name, "mount") {
		guards := s.mountGuards(fn)
		if len(guards) > 0 {
			for _, g := range guards {
				s.Out.Unresolved = append(s.Out.Unresolved, finding{Kind: "UNRESOLVED", Method: g.Method, Path: normalizePath(g.Path), Detail: detail, Locations: goRefs(refs, g.Locations)})
			}
			return
		}
	}
	s.Out.Unresolved = append(s.Out.Unresolved, finding{Kind: "UNRESOLVED", Detail: detail, Locations: refs})
}

func goPathExpression(expr ast.Expr) bool {
	found := false
	ast.Inspect(expr, func(n ast.Node) bool {
		if sel, ok := n.(*ast.SelectorExpr); ok && sel.Sel.Name == "Path" {
			found = true
		}
		return true
	})
	return found
}

func (s *goScanner) mountGuards(fn *goFunc) []finding {
	s.File = fn.File
	var out []finding
	ast.Inspect(fn.AST.Body, func(n ast.Node) bool {
		var value goValue
		method := ""
		if c, ok := n.(*ast.CallExpr); ok {
			if sel, ok := c.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "HasPrefix" && len(c.Args) == 2 && goPathExpression(c.Args[0]) {
				value = s.expr(c.Args[1], s.Package.Globals)
				method = "PREFIX"
			}
		}
		if b, ok := n.(*ast.BinaryExpr); ok && (b.Op == token.EQL || b.Op == token.NEQ) {
			if goPathExpression(b.X) {
				value = s.expr(b.Y, s.Package.Globals)
				method = "EXACT"
			} else if goPathExpression(b.Y) {
				value = s.expr(b.X, s.Package.Globals)
				method = "EXACT"
			}
		}
		if method != "" && !value.Unknown {
			for _, path := range value.Text {
				if strings.HasPrefix(path, "/") {
					out = append(out, finding{Method: method, Path: path, Locations: value.Refs})
				}
			}
		}
		return true
	})
	return goDedupFindings(out)
}

// Associate mount guards with opaque dispatchers through existing dependency roots
// in the assembly package, retaining the direct imported-package boundary.
func goBindOpaqueMountSpans(packages map[string]*goPackage, root string, set *token.FileSet, out *inventory) {
	imports := map[string][]string{}
	owners := map[string]*goPackage{}
	for _, p := range packages {
		for _, file := range p.Files {
			rel, _ := filepath.Rel(root, set.Position(file.AST.Pos()).Filename)
			rel = filepath.ToSlash(rel)
			owners[rel] = p
			for _, im := range file.AST.Imports {
				path, _ := strconv.Unquote(im.Path.Value)
				if i := strings.Index(path, "/internal/"); i >= 0 {
					imports[rel] = append(imports[rel], path[i+1:]+"/")
				}
			}
		}
	}
	for i := range out.Unresolved {
		f := &out.Unresolved[i]
		if (f.Method != "PREFIX" && f.Method != "EXACT" && f.Method != "MOUNT") || len(f.Locations) == 0 {
			continue
		}
		assembly := f.Locations[0].File
		if !strings.HasPrefix(assembly, "cmd/api/") {
			continue
		}
		for _, child := range out.Unresolved {
			if child.Method != "" || len(child.Locations) == 0 {
				continue
			}
			file := child.Locations[0].File
			belongs := false
			if owners[file] != nil && owners[file] == owners[assembly] {
				// Existing qualified dependency closures retain their root first.
				// A mount must reference this child, or a child builder must
				// reference this guard; sharing an assembly package is not an edge.
				for _, ref := range f.Locations {
					if ref == child.Locations[0] {
						belongs = true
					}
				}
				for _, ref := range child.Locations {
					if ref == f.Locations[0] {
						belongs = true
					}
				}
			}
			for _, pkg := range imports[assembly] {
				if filepath.ToSlash(filepath.Dir(file))+"/" == pkg {
					belongs = true
				}
			}
			if belongs {
				f.Locations = goRefs(f.Locations, child.Locations)
			}
		}
	}
}
