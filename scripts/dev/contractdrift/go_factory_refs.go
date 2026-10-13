// Purpose: project proven startup admission statements out of factory routing provenance.
// Depends on: bounded admission proof and Go AST; never executes application code.
// Used by: opaque inventories and expression provenance, in both sides of the Git ratchet.
// Invariant: unknown effects retain the full span; helper, argument and import refs survive.
package main

import (
	"go/ast"
	"go/token"
	"os"
	"strings"
)

func (s *goScanner) factoryRefs(fn *goFunc) ([]location, map[ast.Node]bool) {
	full := s.loc(fn.AST)
	skipped := map[ast.Node]bool{}
	if fn.AST.Type.Results == nil || fn.AST.Body == nil {
		return []location{full}, skipped
	}
	handler := false
	for _, field := range fn.AST.Type.Results.List {
		if sel, ok := field.Type.(*ast.SelectorExpr); ok {
			if id, ok := sel.X.(*ast.Ident); ok && fn.File.ImportPaths[id.Name] == "net/http" && sel.Sel.Name == "Handler" {
				handler = true
			}
		}
	}
	if !handler || len(fn.AST.Type.Results.List) < 2 {
		return []location{full}, skipped
	}
	p := &goAdmissionProof{s: s, pkg: s.Package, file: fn.File, active: map[*goFunc]bool{}}
	env := p.params(fn.AST.Type.Params, nil)
	// A borrowed real pool is trusted only when every source caller has a proved origin.
	i := 0
	for _, field := range fn.AST.Type.Params.List {
		for _, name := range field.Names {
			if p.poolType(field.Type) && p.borrowedPool(fn, i) {
				v := env[name.Name]
				v.kind = "pool"
				v.refs = goRefs(v.refs, p.refs)
				env[name.Name] = v
			}
			if p.contextType(field.Type) && p.borrowedContext(fn, i, map[*goFunc]bool{}) {
				v := env[name.Name]
				v.kind = "ctx"
				v.refs = goRefs(v.refs, p.refs)
				env[name.Name] = v
			}
			i++
		}
	}
	removed := map[int]bool{}
	var refs []location
	for i, stmt := range fn.AST.Body.List {
		candidate := false
		switch stmt.(type) {
		case *ast.IfStmt, *ast.RangeStmt:
			candidate = true
		}
		trial := admissionClone(env)
		p.factory, p.overwrite, p.aborts, p.probes = true, admissionOverwrite(fn.AST.Body.List[i+1:]), 0, 0
		oldRefs := p.refs
		p.refs = nil
		accepted := candidate && p.stmt(stmt, trial) && p.aborts > 0 && p.probes > 0
		// Blank identifier placeholders have no routing effect, but their source dependencies do.
		if a, ok := stmt.(*ast.AssignStmt); ok && a.Tok == token.ASSIGN && len(a.Lhs) == 1 && len(a.Rhs) == 1 {
			left, l := a.Lhs[0].(*ast.Ident)
			right, r := a.Rhs[0].(*ast.Ident)
			if l && r && left.Name == "_" {
				if v, known := env[right.Name]; known {
					accepted = true
					p.refs = goRefs(p.refs, v.refs)
				}
			}
		}
		if accepted && s.admissionWholeLines(stmt) {
			skipped[stmt] = true
			loc := s.loc(stmt)
			for line := loc.Line; line <= loc.End; line++ {
				removed[line] = true
			}
			refs = goRefs(refs, p.refs)
		}
		p.refs = oldRefs
		// Retained construction is not declared pure. Only infer closed origins and local bindings.
		p.bind(stmt, env)
	}
	var spans []location
	start := full.Line
	for line := full.Line; line <= full.End+1; line++ {
		if removed[line] || line == full.End+1 {
			if start < line {
				spans = append(spans, location{File: full.File, Line: start, End: line - 1})
			}
			start = line + 1
		}
	}
	return goRefs(spans, refs), skipped
}

// A sibling statement sharing either boundary line cannot be subtracted.
func (s *goScanner) admissionWholeLines(n ast.Node) bool {
	a, b := s.Set.Position(n.Pos()), s.Set.Position(n.End())
	data, err := os.ReadFile(a.Filename)
	if err != nil {
		return false
	}
	start := strings.LastIndexByte(string(data[:a.Offset]), '\n') + 1
	end := b.Offset
	for end < len(data) && data[end] != '\n' {
		end++
	}
	return strings.TrimSpace(string(data[start:a.Offset])) == "" && strings.TrimSpace(string(data[b.Offset:end])) == ""
}

// Only an immediate unconditional overwrite proves that an existing error binding is dead.
func admissionOverwrite(rest []ast.Stmt) string {
	if len(rest) == 0 {
		return ""
	}
	a, ok := rest[0].(*ast.AssignStmt)
	if !ok || (a.Tok != token.ASSIGN && a.Tok != token.DEFINE) {
		return ""
	}
	for _, lhs := range a.Lhs {
		id, ok := lhs.(*ast.Ident)
		if !ok || id.Name == "_" {
			continue
		}
		read := false
		for _, rhs := range a.Rhs {
			ast.Inspect(rhs, func(n ast.Node) bool {
				if use, ok := n.(*ast.Ident); ok && use.Name == id.Name {
					read = true
				}
				return true
			})
		}
		if !read && id.Name == "err" {
			return id.Name
		}
	}
	return ""
}
