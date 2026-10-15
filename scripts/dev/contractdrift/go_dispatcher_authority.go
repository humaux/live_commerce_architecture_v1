// Purpose: reject writable or platform-selected inputs to a dispatcher freeze.
// Depends on: stdlib AST and parser source positions; no build-tag evaluation.
// Used by: goDispatcherFingerprint before emitting a candidate pin.
// Invariant: frozen private vars have no outside writers or address escapes;
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

func goDispatcherVarAuthority(pkg *goPackage, name string, own ast.Spec, declared *goFile, facadeNames []string) error {
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
	}
	return nil
}
