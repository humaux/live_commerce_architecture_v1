// Purpose: bind table coverage to an independently reviewed dispatcher closure.
// Depends on: stdlib AST, scanner, gofmt, SHA256 and the explicit JSON pin contract.
// Used by: resolveRouteTables and fixture pin helpers; never writes or updates pins.
// Invariant: only row data/row-only targets are excluded; referenced declarations freeze regardless of Git touches.

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
	Comment      string   `json:"_comment,omitempty"`
	Table        string   `json:"table"`
	SHA256       string   `json:"dispatcher_sha256"`
	Functions    []string `json:"functions"`
	Declarations []string `json:"declarations,omitempty"`
	Reviewed     string   `json:"reviewed"`
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
	for _, fn := range pkg.Funcs {
		if goDispatcherReceiver(fn) == table.TypeName || fn.AST.Recv == nil && fn.AST.Name.Name == "init" {
			roots = append(roots, fn)
		}
	}
	hash, names, decls, err := goDispatcherFingerprint(pkg, roots, table.Name, table.MountName)
	return goDispatcherPin{Table: pkg.Files[0].AST.Name.Name + "." + table.MountName, SHA256: hash, Functions: names, Declarations: decls}, err
}

func goDispatcherReceiver(fn *goFunc) string {
	if fn.AST.Recv == nil || len(fn.AST.Recv.List) != 1 {
		return ""
	}
	expr := fn.AST.Recv.List[0].Type
	for {
		switch e := expr.(type) {
		case *ast.StarExpr:
			expr = e.X
		case *ast.IndexExpr:
			expr = e.X
		case *ast.IndexListExpr:
			expr = e.X
		case *ast.Ident:
			return e.Name
		default:
			return ""
		}
	}
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
func goDispatcherFingerprint(pkg *goPackage, roots []*goFunc, rowVariable string, facadeNames ...string) (string, []string, []string, error) {
	type declaration struct {
		kind, name, key string
		node            ast.Node
		file            *goFile
	}
	counts, err := goDispatcherPackageNames(pkg)
	if err != nil {
		return "", nil, nil, err
	}
	specs := map[string]*declaration{}
	for fileIndex, file := range pkg.Files {
		for _, decl := range file.AST.Decls {
			gd, ok := decl.(*ast.GenDecl)
			if !ok || (gd.Tok != token.CONST && gd.Tok != token.VAR && gd.Tok != token.TYPE) {
				continue
			}
			// Freeze const blocks whole: iota positions and implicit expressions depend
			// on neighbouring specs, including entries not referenced by name.
			var blockNames []string
			for _, spec := range gd.Specs {
				var names []string
				switch v := spec.(type) {
				case *ast.ValueSpec:
					for _, name := range v.Names {
						names = append(names, name.Name)
					}
				case *ast.TypeSpec:
					names = []string{v.Name.Name}
				}
				blockNames = append(blockNames, names...)
				if gd.Tok == token.CONST {
					continue
				}
				if gd.Tok == token.VAR && len(names) == 1 && names[0] == rowVariable {
					continue
				}
				d := &declaration{kind: gd.Tok.String(), name: strings.Join(names, ","),
					key:  fmt.Sprintf("%08d:%012d", fileIndex, spec.Pos()),
					node: &ast.GenDecl{Tok: gd.Tok, Specs: []ast.Spec{spec}}, file: file}
				for _, name := range names {
					specs[name] = d
				}
			}
			if gd.Tok == token.CONST {
				sorted := append([]string{}, blockNames...)
				sort.Strings(sorted)
				d := &declaration{kind: "const", name: strings.Join(sorted, ","),
					key: fmt.Sprintf("%08d:%012d", fileIndex, gd.Pos()), node: gd, file: file}
				for _, name := range blockNames {
					specs[name] = d
				}
			}
		}
	}
	seenFuncs := map[*goFunc]bool{}
	seenSpecs := map[*declaration]bool{}
	queue := append([]*goFunc{}, roots...)
	var inspect func(ast.Node, *goFile)
	inspect = func(node ast.Node, file *goFile) {
		if node == nil {
			return
		}
		ast.Inspect(node, func(n ast.Node) bool {
			if sel, ok := n.(*ast.SelectorExpr); ok {
				if id, ok := sel.X.(*ast.Ident); ok && id.Obj == nil && file.ImportPaths[id.Name] != "" {
					return false
				}
				for _, callee := range pkg.ByName[sel.Sel.Name] {
					if callee.AST.Recv != nil {
						queue = append(queue, callee)
					}
				}
				inspect(sel.X, file)
				return false
			}
			if id, ok := n.(*ast.Ident); ok {
				// Local bindings are not package references. Unresolved cross-file names
				// remain conservative; imported implementations are explicit boundaries.
				if id.Obj != nil && file.AST.Scope.Lookup(id.Name) != id.Obj {
					return true
				}
				if callee := pkg.Named[id.Name]; callee != nil {
					queue = append(queue, callee)
				}
				if d := specs[id.Name]; d != nil && !seenSpecs[d] {
					seenSpecs[d] = true
					inspect(d.node, d.file)
				}
			}
			return true
		})
	}
	// Do not walk the row literal at all. A target referenced only from data never
	// enters this graph; the same target reached directly by any closure body does.
	for len(queue) > 0 {
		fn := queue[0]
		queue = queue[1:]
		if seenFuncs[fn] {
			continue
		}
		seenFuncs[fn] = true
		if fn.AST.Recv != nil {
			inspect(fn.AST.Recv, fn.File)
		}
		inspect(fn.AST.Type, fn.File)
		inspect(fn.AST.Body, fn.File)
	}
	// Name resolution maps are only an aid: never let a last-wins declaration
	// or a writer elsewhere in the package become an authority for frozen inputs.
	for fn := range seenFuncs {
		name := goDispatcherName(fn)
		if name != "init" && counts[name] > 1 {
			return "", nil, nil, fmt.Errorf("duplicate frozen name %s at %s", name, goDispatcherLocation(fn.File, fn.AST))
		}
	}
	closure := map[ast.Node]bool{}
	for fn := range seenFuncs {
		closure[fn.AST] = true
	}
	for d := range seenSpecs {
		for _, spec := range d.node.(*ast.GenDecl).Specs {
			closure[spec] = true
		}
	}
	for d := range seenSpecs {
		for _, name := range strings.Split(d.name, ",") {
			if name != "_" && counts[name] > 1 {
				return "", nil, nil, fmt.Errorf("duplicate frozen name %s at %s", name, goDispatcherLocation(d.file, d.node))
			}
			if d.kind == "var" {
				if err := goDispatcherVarAuthority(pkg, name, d.node.(*ast.GenDecl).Specs[0], d.file, facadeNames, closure); err != nil {
					return "", nil, nil, err
				}
			}
		}
	}
	var decls []*declaration
	for fn := range seenFuncs {
		fileIndex := 0
		for i, file := range pkg.Files {
			if file == fn.File {
				fileIndex = i
				break
			}
		}
		key := fmt.Sprintf("%08d:%012d:%s", fileIndex, fn.AST.Pos(), fn.Key)
		decls = append(decls, &declaration{kind: "func", name: goDispatcherName(fn), key: key, node: fn.AST, file: fn.File})
	}
	for d := range seenSpecs {
		decls = append(decls, d)
	}
	sort.SliceStable(decls, func(i, j int) bool {
		a, b := decls[i], decls[j]
		if a.kind != b.kind {
			return a.kind < b.kind
		}
		if a.name != b.name {
			return a.name < b.name
		}
		return a.key < b.key
	})
	var canonical bytes.Buffer
	names, declNames := []string{}, []string{}
	for _, d := range decls {
		normalized, err := goDispatcherCanonical(d.node)
		if err != nil {
			return "", nil, nil, err
		}
		name := d.kind + " " + d.name
		declNames = append(declNames, name)
		if d.kind == "func" {
			names = append(names, d.name)
		}
		canonical.WriteString(name + "\n")
		canonical.Write(normalized)
		canonical.WriteByte('\n')
	}
	sum := sha256.Sum256(canonical.Bytes())
	return hex.EncodeToString(sum[:]), names, declNames, nil
}

// Re-tokenize with comments disabled and canonical whitespace before gofmt.
// Merely clearing Doc/Comment fields leaves comment-induced blank-line positions
// in the AST printer, making a documentation edit change the hash.
func goDispatcherCanonical(node ast.Node) ([]byte, error) {
	var decl bytes.Buffer
	if err := format.Node(&decl, token.NewFileSet(), node); err != nil {
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
