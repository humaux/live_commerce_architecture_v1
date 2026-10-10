// Purpose: discover API producer packages and preserve route/source inventory.
// Depends on: Go stdlib only; no DB/API/network mutations.
// Used by: CI-DRIFT scanGo and its parser gates.
// Invariant: extraction never executes handlers or discards unresolved registration coverage.
package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

type goValue struct {
	Text      []string
	Items     []goValue
	Fields    map[string]goValue
	Refs      []location
	Unknown   bool
	Sequence  bool
	Registrar bool // Method values remain opaque even when their aliases are statically known.
	Mux       bool // Static ServeMux type only, never proof of a particular child mux or its routes.
}
type goEnv []map[string]goValue
type goFile struct {
	ImportRefs  map[string]location
	ImportPaths map[string]string
	AST         *ast.File
	HTTP        string
}
type goFunc struct {
	AST  *ast.FuncDecl
	File *goFile
	Key  string
}
type goPackage struct {
	Path    string
	ByName  map[string][]*goFunc
	Imports []string
	Files   []*goFile
	Funcs   []*goFunc
	Named   map[string]*goFunc
	Types   map[string][]string
	Globals goEnv
	Routes  map[string]bool
}
type goScanner struct {
	Packages map[string]*goPackage
	Seen     map[token.Pos]bool
	Root     string
	Set      *token.FileSet
	Out      inventory
	Package  *goPackage
	File     *goFile
	Stack    map[string]bool
	Callers  []location
}

func scanGo(root string) (inventory, error) {
	s := &goScanner{Root: root, Set: token.NewFileSet(), Stack: map[string]bool{}, Seen: map[token.Pos]bool{}}
	packages := map[string]*goPackage{}
	for _, tree := range []string{"internal", "cmd/api"} {
		base := filepath.Join(root, tree)
		err := filepath.WalkDir(base, func(path string, d fs.DirEntry, err error) error {
			if os.IsNotExist(err) {
				return nil
			}
			if err != nil {
				return err
			}
			if d.IsDir() {
				if d.Name() == "testdata" {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			f, err := parser.ParseFile(s.Set, path, nil, 0)
			if err != nil {
				return fmt.Errorf("Go route AST %s: %w", path, err)
			}
			key := filepath.Dir(path) + ":" + f.Name.Name
			p := packages[key]
			if p == nil {
				p = &goPackage{Named: map[string]*goFunc{}, ByName: map[string][]*goFunc{}, Path: filepath.ToSlash(filepath.Dir(path)), Types: map[string][]string{}, Globals: goEnv{{}}, Routes: map[string]bool{}}
				packages[key] = p
			}
			file := &goFile{AST: f, ImportRefs: map[string]location{}, ImportPaths: map[string]string{}}
			for _, im := range f.Imports {
				v, _ := strconv.Unquote(im.Path.Value)
				alias := filepath.Base(v)
				if im.Name != nil {
					alias = im.Name.Name
				}
				file.ImportRefs[alias] = s.loc(im)
				file.ImportPaths[alias] = v
				if v == "net/http" {
					file.HTTP = "http"
					if im.Name != nil {
						file.HTTP = im.Name.Name
					}
				}
			}
			p.Files = append(p.Files, file)
			for _, im := range f.Imports {
				v, _ := strconv.Unquote(im.Path.Value)
				p.Imports = append(p.Imports, v)
			}
			for _, decl := range f.Decls {
				if fn, ok := decl.(*ast.FuncDecl); ok && fn.Body != nil {
					key := fn.Name.Name
					if fn.Recv != nil {
						key = s.Set.Position(fn.Pos()).String()
					}
					entry := &goFunc{AST: fn, File: file, Key: key}
					p.Funcs = append(p.Funcs, entry)
					p.ByName[fn.Name.Name] = append(p.ByName[fn.Name.Name], entry)
					if fn.Recv == nil {
						p.Named[fn.Name.Name] = entry
					}
				}
				if gd, ok := decl.(*ast.GenDecl); ok {
					for _, spec := range gd.Specs {
						if ty, ok := spec.(*ast.TypeSpec); ok {
							p.Types[ty.Name.Name] = goStructNames(ty.Type)
						}
					}
				}
			}
			return nil
		})
		if err != nil {
			return inventory{}, err
		}
	}
	keys := make([]string, 0, len(packages))
	s.Packages = map[string]*goPackage{}
	module := ""
	if data, err := os.ReadFile(filepath.Join(root, "go.mod")); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			if fields := strings.Fields(line); len(fields) == 2 && fields[0] == "module" {
				module = fields[1]
			}
		}
	}
	for key := range packages {
		keys = append(keys, key)
		if module != "" {
			rel, _ := filepath.Rel(root, packages[key].Path)
			s.Packages[module+"/"+filepath.ToSlash(rel)] = packages[key]
		}
	}
	sort.Strings(keys)
	reachable := goAPIReachable(root, packages)
	for _, key := range keys {
		if reachable != nil && !reachable[key] {
			continue
		}
		p := packages[key]
		s.Package = p
		// Const references may point forward or into another file of the same package.
		for pass := 0; pass <= len(p.Files)+2; pass++ {
			for _, file := range p.Files {
				s.File = file
				for _, decl := range file.AST.Decls {
					if gd, ok := decl.(*ast.GenDecl); ok && (gd.Tok == token.CONST || gd.Tok == token.VAR) {
						s.declare(gd, p.Globals)
					}
				}
			}
		}
		for _, fn := range p.Funcs {
			ast.Inspect(fn.AST.Body, func(n ast.Node) bool {
				if expr, ok := n.(ast.Expr); ok && goRegistrarSelector(expr) {
					p.Routes[fn.Key] = true
				}
				if call, ok := n.(*ast.CallExpr); ok && p.Globals.get(goCallName(call.Fun)).Registrar {
					p.Routes[fn.Key] = true
				}
				return true
			})
		}
		for changed := true; changed; {
			changed = false
			for _, fn := range p.Funcs {
				if p.Routes[fn.Key] {
					continue
				}
				ast.Inspect(fn.AST.Body, func(n ast.Node) bool {
					if c, ok := n.(*ast.CallExpr); ok {
						if callee := p.Named[goCallName(c.Fun)]; callee != nil && p.Routes[callee.Key] {
							p.Routes[fn.Key] = true
							changed = true
						}
					}
					return true
				})
			}
		}
		called := map[string]bool{}
		for _, fn := range p.Funcs {
			if p.Routes[fn.Key] {
				ast.Inspect(fn.AST.Body, func(n ast.Node) bool {
					if c, ok := n.(*ast.CallExpr); ok {
						if callee := p.Named[goCallName(c.Fun)]; callee != nil && p.Routes[callee.Key] {
							called[callee.Key] = true
						}
					}
					return true
				})
			}
		}
		for _, fn := range p.Funcs {
			if p.Routes[fn.Key] && !called[fn.Key] {
				s.function(fn, nil, nil)
			}
			if !p.Routes[fn.Key] {
				s.opaque(fn)
			}
		}
		for _, fn := range p.Funcs {
			ast.Inspect(fn.AST.Body, func(n ast.Node) bool {
				if c, ok := n.(*ast.CallExpr); ok && goRegistration(c) && !s.Seen[c.Pos()] {
					s.Out.Unresolved = append(s.Out.Unresolved, finding{Kind: "UNRESOLVED", Detail: "unsupported Go registration execution scope", Locations: []location{s.loc(c), s.loc(fn.AST)}})
					s.Seen[c.Pos()] = true
				}
				return true
			})
		}
	}
	s.Out.Routes = goDedupRoutes(s.Out.Routes)
	s.Out.Unresolved = goDedupFindings(s.Out.Unresolved)
	goBindOpaqueMountSpans(packages, s.Root, s.Set, &s.Out)
	s.Out.Notes = goDedupFindings(s.Out.Notes)
	return s.Out, nil
}

func (s *goScanner) loc(n ast.Node) location {
	a, b := s.Set.Position(n.Pos()), s.Set.Position(n.End())
	file, _ := filepath.Rel(s.Root, a.Filename)
	return location{File: filepath.ToSlash(file), Line: a.Line, End: b.Line}
}

func goAPIReachable(root string, packages map[string]*goPackage) map[string]bool {
	data, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return nil
	}
	module := ""
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[0] == "module" {
			module = fields[1]
			break
		}
	}
	if module == "" {
		return nil
	}
	byImport := map[string]string{}
	var queue []string
	for key, p := range packages {
		rel, _ := filepath.Rel(root, p.Path)
		byImport[module+"/"+filepath.ToSlash(rel)] = key
		if filepath.ToSlash(rel) == "cmd/api" {
			queue = append(queue, key)
		}
	}
	if len(queue) == 0 {
		return nil
	}
	out := map[string]bool{}
	for len(queue) > 0 {
		key := queue[0]
		queue = queue[1:]
		if out[key] {
			continue
		}
		out[key] = true
		for _, im := range packages[key].Imports {
			if next, ok := byImport[im]; ok {
				queue = append(queue, next)
			}
		}
	}
	return out
}

func goDedupRoutes(in []route) []route {
	out := []route{}
	index := map[string]int{}
	for _, r := range in {
		key := r.Method + " " + r.Path
		if i, ok := index[key]; ok {
			out[i].Locations = goRefs(out[i].Locations, r.Locations)
		} else {
			index[key] = len(out)
			out = append(out, r)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path+out[i].Method < out[j].Path+out[j].Method })
	return out
}

func goDedupFindings(in []finding) []finding {
	out := []finding{}
	index := map[string]int{}
	for _, f := range in {
		key := f.Kind + " " + f.Method + " " + f.Path + " " + f.Detail
		if f.Kind == "UNRESOLVED" && len(f.Locations) > 0 {
			r := f.Locations[0]
			key += fmt.Sprintf(" %s:%d", r.File, r.Line)
		}
		if i, ok := index[key]; ok {
			out[i].Locations = goRefs(out[i].Locations, f.Locations)
		} else {
			index[key] = len(out)
			out = append(out, f)
		}
	}
	return out
}
