// Purpose: reject writable or platform-selected inputs to a dispatcher freeze.
// Depends on: stdlib AST and parser source positions; no build-tag evaluation.
// Used by: goDispatcherFingerprint before emitting a candidate pin.
// Invariant: frozen private vars have no outside references or value escapes;
// duplicate frozen names and platform-dependent package files fail closed.

package main

import (
	"fmt"
	"go/ast"
	"go/token"
	"path/filepath"
	"strings"
)

func goDispatcherLocation(file *goFile, node ast.Node) string {
	// Frozen var/type specs are wrapped in a synthetic GenDecl for hashing;
	// diagnostics must use the original spec's position, not its zero TokPos.
	if gd, ok := node.(*ast.GenDecl); ok && !gd.TokPos.IsValid() && len(gd.Specs) > 0 {
		node = gd.Specs[0]
	}
	if file.Set == nil {
		return file.Path
	}
	return fmt.Sprintf("%s:%d", file.Path, file.Set.Position(node.Pos()).Line)
}

// Match Go's filename selection vocabulary without choosing this host's build.
// Also reject legacy build lines: evaluating either syntax would hide variants.
func goDispatcherPackageNames(pkg *goPackage) (map[string]int, error) {
	counts := map[string]int{}
	osNames := " aix android darwin dragonfly freebsd hurd illumos ios js linux nacl netbsd openbsd plan9 solaris wasip1 windows zos "
	archNames := " 386 amd64 amd64p32 arm armbe arm64 arm64be loong64 mips mipsle mips64 mips64le mips64p32 mips64p32le ppc ppc64 ppc64le riscv riscv64 s390 s390x sparc sparc64 wasm "
	for _, file := range pkg.Files {
		base := strings.TrimSuffix(filepath.Base(file.Path), ".go")
		parts := strings.Split(base, "_")
		if len(parts) > 1 {
			last := " " + parts[len(parts)-1] + " "
			if strings.Contains(osNames, last) || strings.Contains(archNames, last) {
				return nil, fmt.Errorf("build constraint filename %s", file.Path)
			}
		}
		for _, group := range file.AST.Comments {
			for _, comment := range group.List {
				if strings.HasPrefix(comment.Text, "//go:build") || strings.HasPrefix(comment.Text, "// +build") {
					return nil, fmt.Errorf("build constraint at %s", goDispatcherLocation(file, comment))
				}
			}
		}
		for _, decl := range file.AST.Decls {
			switch node := decl.(type) {
			case *ast.FuncDecl:
				fn := &goFunc{AST: node}
				counts[goDispatcherName(fn)]++
			case *ast.GenDecl:
				for _, spec := range node.Specs {
					switch v := spec.(type) {
					case *ast.ValueSpec:
						for _, name := range v.Names {
							if name.Name != "_" {
								counts[name.Name]++
							}
						}
					case *ast.TypeSpec:
						counts[v.Name.Name]++
					}
				}
			}
		}
	}
	return counts, nil
}

func goDispatcherVarAuthority(pkg *goPackage, name string, own ast.Spec, declared *goFile, facadeNames []string, closure map[ast.Node]bool) error {
	if name == "_" {
		return nil
	}
	// The fieldless facade is the sole public-value exception. It carries no
	// writable dispatch state, as proved by goTableMountType's existing grammar.
	if ast.IsExported(name) {
		for _, facade := range facadeNames {
			if name == facade && goTableMountType(pkg, name) != "" {
				return nil
			}
		}
		return fmt.Errorf("exported frozen var %s at %s", name, goDispatcherLocation(declared, own))
	}
	for _, file := range pkg.Files {
		// Resolve same-file locals by parser object identity. An unresolved name in
		// another file remains a conservative package reference, never a local alias.
		var target func(ast.Expr) bool
		target = func(expr ast.Expr) bool {
			switch e := expr.(type) {
			case *ast.Ident:
				return e.Name == name && (e.Obj == nil || file.AST.Scope.Lookup(name) == e.Obj)
			case *ast.ParenExpr:
				return target(e.X)
			case *ast.SelectorExpr:
				return target(e.X)
			case *ast.IndexExpr:
				return target(e.X)
			case *ast.SliceExpr:
				return target(e.X)
			case *ast.StarExpr:
				return target(e.X)
			}
			return false
		}
		var invalid error
		ast.Inspect(file.AST, func(n ast.Node) bool {
			if n == own || invalid != nil {
				return false
			}
			reason := ""
			switch node := n.(type) {
			case *ast.AssignStmt:
				for _, lhs := range node.Lhs {
					if target(lhs) {
						reason = "assignment"
					}
				}
			case *ast.RangeStmt:
				if node.Tok == token.ASSIGN && (target(node.Key) || target(node.Value)) {
					reason = "range assignment"
				}
			case *ast.IncDecStmt:
				if target(node.X) {
					reason = "increment/decrement"
				}
			case *ast.UnaryExpr:
				if node.Op == token.AND && target(node.X) {
					reason = "address taken"
				}
			case *ast.CallExpr:
				if id, ok := node.Fun.(*ast.Ident); ok && len(node.Args) > 0 && target(node.Args[0]) {
					switch id.Name {
					case "append", "copy", "clear", "delete":
						reason = id.Name + " mutation/escape"
					}
				}
			}
			if reason != "" {
				invalid = fmt.Errorf("frozen var %s: %s at %s", name, reason, goDispatcherLocation(file, n))
			}
			return invalid == nil
		})
		if invalid != nil {
			return invalid
		}
		// Check every reference, rather than attempting to enumerate writes through
		// aliases. The allowed contexts consume the value immediately and cannot
		// retain it. Unrecognised contexts fail closed (including local aliases).
		var stack []ast.Node
		ast.Inspect(file.AST, func(n ast.Node) bool {
			if n == nil {
				stack = stack[:len(stack)-1]
				return false
			}
			if invalid != nil {
				return false
			}
			id, ok := n.(*ast.Ident)
			if ok && target(id) {
				if goDispatcherDeclarationName(stack, id) {
					return false
				}
				inside := false
				for _, ancestor := range stack {
					inside = inside || closure[ancestor]
				}
				reason := "reference outside hashed closure"
				if inside {
					reason = "value escape or unsupported read"
					if goDispatcherReadOnlyReference(pkg, file, declared, own, id, stack) {
						reason = ""
					}
					for _, ancestor := range stack {
						switch ancestor.(type) {
						case *ast.GoStmt, *ast.FuncLit:
							reason = "goroutine/closure capture"
						}
					}
				}
				if reason != "" {
					invalid = fmt.Errorf("frozen var %s: %s at %s", name, reason, goDispatcherLocation(file, id))
					return false
				}
			}
			stack = append(stack, n)
			return true
		})
		if invalid != nil {
			return invalid
		}
	}
	return nil
}

func goDispatcherDeclarationName(stack []ast.Node, id *ast.Ident) bool {
	if len(stack) == 0 {
		return false
	}
	switch p := stack[len(stack)-1].(type) {
	case *ast.ValueSpec:
		for _, name := range p.Names {
			if name == id {
				return true
			}
		}
	case *ast.SelectorExpr:
		return p.Sel == id // a field/method name is not a package-var reference
	}
	return false
}

// This intentionally small allowlist is documented in docs/engineering/contractdrift.md.
// Package identity and concrete receiver provenance are required, not method names.
func goDispatcherReadOnlyReference(pkg *goPackage, file, declared *goFile, own ast.Spec, id *ast.Ident, stack []ast.Node) bool {
	var value ast.Node = id
	i := len(stack) - 1
	for i >= 0 {
		if p, ok := stack[i].(*ast.ParenExpr); ok {
			value = p
			i--
			continue
		}
		break
	}
	if i < 0 {
		return false
	}
	switch p := stack[i].(type) {
	case *ast.AssignStmt:
		// Discard is not an alias, even for a reference-typed value.
		if len(p.Lhs) == 1 && len(p.Rhs) == 1 && p.Rhs[0] == value {
			lhs, ok := p.Lhs[0].(*ast.Ident)
			return ok && lhs.Name == "_"
		}
	case *ast.BinaryExpr:
		return p.Op == token.EQL || p.Op == token.NEQ // only a boolean escapes
	case *ast.SelectorExpr:
		if p.X != value || i == 0 {
			return false
		}
		call, ok := stack[i-1].(*ast.CallExpr)
		if !ok || call.Fun != p || p.Sel.Name != "MatchString" {
			return false
		}
		v, ok := own.(*ast.ValueSpec)
		if !ok || len(v.Values) != 1 {
			return false
		}
		init, ok := v.Values[0].(*ast.CallExpr)
		return ok && goDispatcherStdlibSelector(declared, init.Fun, "regexp", "MustCompile")
	case *ast.CallExpr:
		if fun, ok := p.Fun.(*ast.Ident); ok && fun.Obj == nil && (fun.Name == "len" || fun.Name == "cap") {
			for _, source := range pkg.Files {
				if source.AST.Scope.Lookup(fun.Name) != nil {
					return false
				}
			}
			return len(p.Args) == 1 && p.Args[0] == value
		}
		// encoding/csv.Writer.Write reads record fields and copies them to its
		// own buffer; it neither stores nor writes the caller's []string.
		sel, ok := p.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "Write" || len(p.Args) != 1 || p.Args[0] != value {
			return false
		}
		receiver, ok := sel.X.(*ast.Ident)
		if !ok || receiver.Obj == nil {
			return false
		}
		field, ok := receiver.Obj.Decl.(*ast.Field)
		if !ok {
			return false
		}
		ptr, ok := field.Type.(*ast.StarExpr)
		return ok && goDispatcherStdlibSelector(file, ptr.X, "encoding/csv", "Writer")
	}
	return false
}

func goDispatcherStdlibSelector(file *goFile, expr ast.Expr, path, member string) bool {
	sel, ok := expr.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != member {
		return false
	}
	id, ok := sel.X.(*ast.Ident)
	return ok && id.Obj == nil && file.ImportPaths[id.Name] == path && file.AST.Scope.Lookup(id.Name) == nil
}
