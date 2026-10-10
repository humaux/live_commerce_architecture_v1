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
	// Follow local dispatch helpers, including method Handler -> serveWebhook adapters.
	seen := map[string]bool{}
	queue := []*goFunc{fn}
	dispatch := false
	refs := []location{s.loc(fn.AST)}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		if seen[current.Key] {
			continue
		}
		seen[current.Key] = true
		s.File = current.File
		signal := false
		ast.Inspect(current.AST.Body, func(n ast.Node) bool {
			if sel, ok := n.(*ast.SelectorExpr); ok && (sel.Sel.Name == "Path" || sel.Sel.Name == "Method" || strings.HasPrefix(sel.Sel.Name, "Method")) {
				signal = true
				dispatch = true
			}
			if lit, ok := n.(*ast.BasicLit); ok && lit.Kind == token.STRING {
				v, _ := strconv.Unquote(lit.Value)
				if strings.HasPrefix(v, "/") {
					signal = true

				}
			}
			if id, ok := n.(*ast.Ident); ok {
				for _, callee := range s.Package.ByName[id.Name] {
					queue = append(queue, callee)
				}
				v := s.Package.Globals.get(id.Name)
				for _, text := range v.Text {
					if strings.HasPrefix(text, "/") {
						signal = true
						refs = goRefs(refs, v.Refs)

					}
				}
			}
			if c, ok := n.(*ast.CallExpr); ok {
				name := goCallName(c.Fun)
				if sel, ok := c.Fun.(*ast.SelectorExpr); ok {
					name = sel.Sel.Name
				}
				for _, callee := range s.Package.ByName[name] {
					queue = append(queue, callee)
				}
			}
			return true
		})
		if signal {
			refs = goRefs(refs, []location{s.loc(current.AST)})
		}
	}
	// Only actual custom mount guards provide coverage; provider API strings in
	// underlying handlers cannot define this API's served namespace.
	if !dispatch {
		return
	}
	ref := s.loc(fn.AST)
	detail := "opaque Go handler dispatch " + fn.AST.Name.Name + "; route/method coverage is unknown"
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

// Associate mount guards with opaque dispatchers in the same assembly file and
// packages that file imports, so changes inside the child router touch its coverage.
func goBindOpaqueMountSpans(packages map[string]*goPackage, root string, set *token.FileSet, out *inventory) {
	imports := map[string][]string{}
	for _, p := range packages {
		for _, file := range p.Files {
			rel, _ := filepath.Rel(root, set.Position(file.AST.Pos()).Filename)
			rel = filepath.ToSlash(rel)
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
		if (f.Method != "PREFIX" && f.Method != "EXACT") || len(f.Locations) == 0 {
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
			belongs := file == assembly
			for _, pkg := range imports[assembly] {
				if strings.HasPrefix(file, pkg) {
					belongs = true
				}
			}
			if belongs {
				f.Locations = goRefs(f.Locations, child.Locations)
			}
		}
	}
}
