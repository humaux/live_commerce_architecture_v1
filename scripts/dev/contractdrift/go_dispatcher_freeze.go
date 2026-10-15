// Purpose: bind table coverage to an independently reviewed dispatcher closure.
// Depends on: stdlib AST, scanner, gofmt, SHA256 and the explicit JSON pin contract.
// Used by: resolveRouteTables and fixture pin helpers; never writes or updates pins.
// Invariant: row data and row target bodies are excluded; changed/unproved pins fail regardless of Git touches.

package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/format"
	"go/scanner"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const goDispatcherPinFile = "contracts/route-table-dispatchers.json"

type goDispatcherPin struct {
	Comment   string   `json:"_comment,omitempty"`
	Table     string   `json:"table"`
	SHA256    string   `json:"dispatcher_sha256"`
	Functions []string `json:"functions"`
	Reviewed  string   `json:"reviewed"`
}

func readGoDispatcherPins(root string) (map[string]goDispatcherPin, error) {
	out := map[string]goDispatcherPin{}
	data, err := os.ReadFile(filepath.Join(root, goDispatcherPinFile))
	if os.IsNotExist(err) {
		return out, nil
	}
	if err != nil {
		return out, err
	}
	var pins []goDispatcherPin
	if err = json.Unmarshal(data, &pins); err != nil {
		return out, err
	}
	for _, pin := range pins {
		raw, err := hex.DecodeString(pin.SHA256)
		if pin.Table == "" || err != nil || len(raw) != sha256.Size || len(pin.Functions) == 0 || pin.Reviewed == "" {
			return out, fmt.Errorf("incomplete dispatcher pin %q", pin.Table)
		}
		if _, exists := out[pin.Table]; exists {
			return out, fmt.Errorf("duplicate dispatcher pin %q", pin.Table)
		}
		out[pin.Table] = pin
	}
	return out, nil
}

func (s *goScanner) dispatcherError(ref location, detail, method, path string) {
	s.Out.Unresolved = append(s.Out.Unresolved, finding{Kind: "UNRESOLVED", Fatal: true, Method: method, Path: path, Detail: detail, Locations: []location{ref}})
}

func goDispatcherFunctionsEqual(a, b []string) bool {
	return strings.Join(a, "\n") == strings.Join(b, "\n")
}

// The public facade identifies the table contract; private row variable renames
// are still visible in the normalized consumer declarations.
func goTableDispatcherFingerprint(table goRouteTable, entries []*goFunc) (goDispatcherPin, error) {
	pkg := table.Package
	roots := append(append([]*goFunc{}, entries...), table.Consumers...)
	excluded := map[*goFunc]bool{}
	for _, file := range pkg.Files {
		for _, decl := range file.AST.Decls {
			gd, ok := decl.(*ast.GenDecl)
			if !ok || gd.Tok != token.VAR {
				continue
			}
			for _, spec := range gd.Specs {
				vs, ok := spec.(*ast.ValueSpec)
				if !ok || len(vs.Names) != 1 || vs.Names[0].Name != table.Name {
					continue
				}
				// Named function/method values in rows are terminal handler targets.
				// Calls constructing row values are not terminal targets.
				for _, value := range vs.Values {
					ast.Inspect(value, func(n ast.Node) bool {
						if _, call := n.(*ast.CallExpr); call {
							return false
						}
						if id, ok := n.(*ast.Ident); ok && pkg.Named[id.Name] != nil {
							excluded[pkg.Named[id.Name]] = true
						}
						return true
					})
				}
			}
		}
	}
	for _, fn := range pkg.Funcs {
		if goDispatcherReceiver(fn) == table.TypeName {
			roots = append(roots, fn)
		}
	}
	hash, names, err := goDispatcherFingerprint(pkg, roots, excluded, table.Name)
	return goDispatcherPin{Table: pkg.Files[0].AST.Name.Name + "." + table.MountName, SHA256: hash, Functions: names}, err
}

func goDispatcherReceiver(fn *goFunc) string {
	if fn.AST.Recv == nil || len(fn.AST.Recv.List) != 1 {
		return ""
	}
	expr := fn.AST.Recv.List[0].Type
	if ptr, ok := expr.(*ast.StarExpr); ok {
		expr = ptr.X
	}
	if id, ok := expr.(*ast.Ident); ok {
		return id.Name
	}
	return ""
}

func goDispatcherName(fn *goFunc) string {
	if receiver := goDispatcherReceiver(fn); receiver != "" {
		return receiver + "." + fn.AST.Name.Name
	}
	return fn.AST.Name.Name
}

// Conservative same-package reference closure: include function values as well
// as calls, and every same-name method candidate on an unresolved receiver.
// Imported package selectors are boundaries; no cross-package call is followed.
// This can freeze extra same-package methods, but cannot miss a receiver helper
// just because the syntax-only scanner could not infer its type.
func goDispatcherFingerprint(pkg *goPackage, roots []*goFunc, excluded map[*goFunc]bool, rowVariable string) (string, []string, error) {
	// Follow package variable initializers too: calls through aliases, maps or
	// stored function values must retain the target body in the closure. Global
	// function literals receive a stable declaration name for canonicalization.
	type initializer struct {
		expr ast.Expr
		file *goFile
	}
	initializers := map[string][]initializer{}
	literals := map[*ast.FuncLit]*goFunc{}
	for _, file := range pkg.Files {
		for _, decl := range file.AST.Decls {
			gd, ok := decl.(*ast.GenDecl)
			if !ok || gd.Tok != token.VAR {
				continue
			}
			for _, spec := range gd.Specs {
				vs, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for _, name := range vs.Names {
					index := 0
					for _, value := range vs.Values {
						initializers[name.Name] = append(initializers[name.Name], initializer{value, file})
						ast.Inspect(value, func(n ast.Node) bool {
							if lit, ok := n.(*ast.FuncLit); ok {
								index++
								literals[lit] = &goFunc{File: file, AST: &ast.FuncDecl{Name: ast.NewIdent(fmt.Sprintf("__global_%s_%d", name.Name, index)), Type: lit.Type, Body: lit.Body}}
								return false // nested literals belong to this body's canonical source
							}
							return true
						})
					}
				}
			}
		}
	}
	seen := map[*goFunc]bool{}
	expanded := map[string]bool{}
	queue := append([]*goFunc{}, roots...)
	var inspect func(ast.Node, *goFile)
	inspect = func(node ast.Node, file *goFile) {
		ast.Inspect(node, func(n ast.Node) bool {
			if kv, ok := n.(*ast.KeyValueExpr); ok {
				inspect(kv.Value, file)
				return false
			}
			if lit, ok := n.(*ast.FuncLit); ok && literals[lit] != nil {
				queue = append(queue, literals[lit])
				return false
			}
			if sel, ok := n.(*ast.SelectorExpr); ok {
				if id, ok := sel.X.(*ast.Ident); ok && id.Obj == nil && file.ImportPaths[id.Name] != "" {
					return false
				}
				for _, callee := range pkg.ByName[sel.Sel.Name] {
					if callee.AST.Recv != nil {
						queue = append(queue, callee)
					}
				}
				return true
			}
			if id, ok := n.(*ast.Ident); ok {
				if callee := pkg.Named[id.Name]; callee != nil {
					queue = append(queue, callee)
				}
				if id.Name != rowVariable && !expanded[id.Name] {
					expanded[id.Name] = true
					for _, init := range initializers[id.Name] {
						inspect(init.expr, init.file)
					}
				}
			}
			return true
		})
	}
	// Resolve row target aliases with the same graph, then discard the targets'
	// bodies. The row literal is never followed while traversing dispatchers.
	queue = nil
	for _, init := range initializers[rowVariable] {
		inspect(init.expr, init.file)
	}
	for _, target := range queue {
		excluded[target] = true
	}
	queue = append([]*goFunc{}, roots...)
	expanded = map[string]bool{}
	for len(queue) > 0 {
		fn := queue[0]
		queue = queue[1:]
		if seen[fn] || excluded[fn] {
			continue
		}
		seen[fn] = true
		inspect(fn.AST.Body, fn.File)
	}
	funcs := make([]*goFunc, 0, len(seen))
	for fn := range seen {
		funcs = append(funcs, fn)
	}
	sort.Slice(funcs, func(i, j int) bool { return goDispatcherName(funcs[i]) < goDispatcherName(funcs[j]) })
	var canonical bytes.Buffer
	names := []string{}
	for _, fn := range funcs {
		name := goDispatcherName(fn)
		normalized, err := goDispatcherCanonical(fn)
		if err != nil {
			return "", nil, err
		}
		names = append(names, name)
		canonical.WriteString(name + "\n")
		canonical.Write(normalized)
		canonical.WriteByte('\n')
	}
	sum := sha256.Sum256(canonical.Bytes())
	return hex.EncodeToString(sum[:]), names, nil
}

// Re-tokenize with comments disabled and canonical whitespace before gofmt.
// Merely clearing Doc/Comment fields leaves comment-induced blank-line positions
// in the AST printer, making a documentation edit change the hash.
func goDispatcherCanonical(fn *goFunc) ([]byte, error) {
	var decl bytes.Buffer
	if err := format.Node(&decl, token.NewFileSet(), fn.AST); err != nil {
		return nil, err
	}
	set := token.NewFileSet()
	file := set.AddFile("dispatcher.go", -1, decl.Len())
	var scan scanner.Scanner
	var scanErr error
	scan.Init(file, decl.Bytes(), func(_ token.Position, message string) { scanErr = fmt.Errorf("dispatcher tokens: %s", message) }, 0)
	var tokens bytes.Buffer
	tokens.WriteString("package dispatcher\n")
	for {
		_, tok, lit := scan.Scan()
		if tok == token.EOF {
			break
		}
		if tok == token.SEMICOLON {
			tokens.WriteByte('\n')
			continue
		}
		if lit == "" {
			lit = tok.String()
		}
		tokens.WriteString(lit)
		tokens.WriteByte(' ')
	}
	if scanErr != nil {
		return nil, scanErr
	}
	return format.Source(tokens.Bytes())
}
