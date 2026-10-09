// Purpose: extract Go API-mux registrations without importing or executing handlers.
// Depends on: Go stdlib AST, filesystem and strings only; no DB/API/network mutations.
// Used by: CI-DRIFT scanGo; source definitions, callers and unknown spans feed changed-line gates.
// Invariant: a reassignment invalidates its old value; finite branches are a union, never a guessed runtime choice.
package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
)

type goValue struct {
	Text     []string
	Items    []goValue
	Fields   map[string]goValue
	Refs     []location
	Unknown  bool
	Sequence bool
}
type goEnv []map[string]goValue
type goFile struct {
	ImportRefs map[string]location
	AST        *ast.File
	HTTP       string
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
	Seen    map[token.Pos]bool
	Root    string
	Set     *token.FileSet
	Out     inventory
	Package *goPackage
	File    *goFile
	Stack   map[string]bool
	Callers []location
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
			file := &goFile{AST: f, ImportRefs: map[string]location{}}
			for _, im := range f.Imports {
				v, _ := strconv.Unquote(im.Path.Value)
				alias := filepath.Base(v)
				if im.Name != nil {
					alias = im.Name.Name
				}
				file.ImportRefs[alias] = s.loc(im)
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
	for key := range packages {
		keys = append(keys, key)
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
				if call, ok := n.(*ast.CallExpr); ok && goRegistration(call) {
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
func goRefs(groups ...[]location) []location {
	var out []location
	seen := map[location]bool{}
	for _, g := range groups {
		for _, r := range g {
			if !seen[r] {
				seen[r] = true
				out = append(out, r)
			}
		}
	}
	return out
}
func (e goEnv) lookup(name string) (goValue, bool) {
	for i := len(e) - 1; i >= 0; i-- {
		if v, ok := e[i][name]; ok {
			return v, true
		}
	}
	return goValue{Unknown: true}, false
}
func (e goEnv) get(name string) goValue { v, _ := e.lookup(name); return v }
func (e goEnv) set(name string, v goValue, define bool) {
	if !define {
		for i := len(e) - 1; i >= 0; i-- {
			if _, ok := e[i][name]; ok {
				e[i][name] = v
				return
			}
		}
	}
	e[len(e)-1][name] = v
}
func goClone(e goEnv) goEnv {
	out := make(goEnv, len(e))
	for i, m := range e {
		out[i] = map[string]goValue{}
		for k, v := range m {
			out[i][k] = v
		}
	}
	return out
}
func goUnion(a, b goValue) goValue {
	if reflect.DeepEqual(a, b) {
		return a
	}
	out := goValue{Unknown: a.Unknown || b.Unknown, Sequence: a.Sequence || b.Sequence, Refs: goRefs(a.Refs, b.Refs)}
	seen := map[string]bool{}
	for _, text := range append(append([]string{}, a.Text...), b.Text...) {
		if !seen[text] {
			seen[text] = true
			out.Text = append(out.Text, text)
		}
	}
	out.Items = goMergeItems(a.Items, b.Items)
	if a.Fields != nil || b.Fields != nil {
		out.Fields = map[string]goValue{}
		for k, v := range a.Fields {
			out.Fields[k] = v
		}
		for k, v := range b.Fields {
			if old, ok := out.Fields[k]; ok {
				v = goUnion(old, v)
			}
			out.Fields[k] = v
		}
	}
	return out
}
func goValueKey(v goValue) string {
	fields := make([]string, 0, len(v.Fields))
	for k := range v.Fields {
		fields = append(fields, k)
	}
	sort.Strings(fields)
	key := fmt.Sprintf("%t:%t:%q", v.Unknown, v.Sequence, v.Text)
	for _, k := range fields {
		key += "|" + k + "=" + goValueKey(v.Fields[k])
	}
	for _, item := range v.Items {
		key += "[" + goValueKey(item) + "]"
	}
	return key
}
func goMergeItems(a, b []goValue) []goValue {
	out := append([]goValue{}, a...)
	seen := map[string]int{}
	for i, v := range out {
		seen[goValueKey(v)] = i
	}
	for _, v := range b {
		key := goValueKey(v)
		if i, ok := seen[key]; ok {
			out[i] = goUnion(out[i], v)
		} else {
			seen[key] = len(out)
			out = append(out, v)
		}
	}
	return out
}
func goMerge(dst, a, b goEnv) {
	for i := range dst {
		for k := range dst[i] {
			dst[i][k] = goUnion(a[i][k], b[i][k])
		}
	}
}
func goStructNames(expr ast.Expr) []string {
	st, ok := expr.(*ast.StructType)
	if !ok {
		return nil
	}
	var names []string
	for _, f := range st.Fields.List {
		for _, name := range f.Names {
			names = append(names, name.Name)
		}
	}
	return names
}
func (s *goScanner) typeNames(expr ast.Expr) []string {
	if id, ok := expr.(*ast.Ident); ok {
		return s.Package.Types[id.Name]
	}
	return goStructNames(expr)
}
func goCallName(expr ast.Expr) string {
	switch x := expr.(type) {
	case *ast.Ident:
		return x.Name
	case *ast.IndexExpr:
		return goCallName(x.X)
	case *ast.IndexListExpr:
		return goCallName(x.X)
	}
	return ""
}
func goRegistration(c *ast.CallExpr) bool {
	sel, ok := c.Fun.(*ast.SelectorExpr)
	return ok && (sel.Sel.Name == "HandleFunc" || sel.Sel.Name == "Handle") && len(c.Args) >= 2
}
func (s *goScanner) expr(expr ast.Expr, e goEnv) goValue {
	if expr == nil {
		return goValue{Unknown: true}
	}
	switch x := expr.(type) {
	case *ast.BasicLit:
		if x.Kind == token.STRING {
			v, err := strconv.Unquote(x.Value)
			if err == nil {
				return goValue{Text: []string{v}, Refs: []location{s.loc(x)}}
			}
		}
	case *ast.Ident:
		return e.get(x.Name)
	case *ast.ParenExpr:
		return s.expr(x.X, e)
	case *ast.SelectorExpr:
		if id, ok := x.X.(*ast.Ident); ok && id.Name == s.File.HTTP && s.File.HTTP != "" {
			if _, shadowed := e.lookup(id.Name); !shadowed {
				methods := map[string]string{"MethodGet": "GET", "MethodHead": "HEAD", "MethodPost": "POST", "MethodPut": "PUT", "MethodPatch": "PATCH", "MethodDelete": "DELETE", "MethodOptions": "OPTIONS", "MethodConnect": "CONNECT", "MethodTrace": "TRACE"}
				if m, ok := methods[x.Sel.Name]; ok {
					return goValue{Text: []string{m}, Refs: goRefs([]location{s.loc(x)}, []location{s.File.ImportRefs[id.Name]})}
				}
			}
		}
		v := s.expr(x.X, e)
		if field, ok := v.Fields[x.Sel.Name]; ok {
			field.Refs = goRefs(v.Refs, field.Refs)
			return field
		}
		if id, ok := x.X.(*ast.Ident); ok {
			if ref, imported := s.File.ImportRefs[id.Name]; imported {
				if _, shadowed := e.lookup(id.Name); !shadowed {
					return goValue{Unknown: true, Refs: goRefs(v.Refs, []location{s.loc(x), ref})}
				}
			}
		}
		return goValue{Unknown: true, Refs: goRefs(v.Refs, []location{s.loc(x)})}
	case *ast.BinaryExpr:
		if x.Op == token.ADD {
			a, b := s.expr(x.X, e), s.expr(x.Y, e)
			v := goValue{Unknown: a.Unknown || b.Unknown, Sequence: a.Sequence || b.Sequence, Refs: goRefs(a.Refs, b.Refs)}
			for _, left := range a.Text {
				for _, right := range b.Text {
					v.Text = append(v.Text, left+right)
				}
			}
			return v
		}
	case *ast.CompositeLit:
		return s.composite(x, x.Type, e)
	case *ast.CallExpr:
		if goCallName(x.Fun) == "append" && len(x.Args) > 1 {
			v := s.expr(x.Args[0], e)
			for _, arg := range x.Args[1:] {
				a := s.expr(arg, e)
				if x.Ellipsis.IsValid() {
					v.Items = append(v.Items, a.Items...)
				} else {
					v.Items = append(v.Items, a)
				}
				v.Refs = goRefs(v.Refs, a.Refs)
			}
			return v
		}
	}
	return goValue{Unknown: true, Refs: []location{s.loc(expr)}}
}
func (s *goScanner) composite(x *ast.CompositeLit, typ ast.Expr, e goEnv) goValue {
	v := goValue{Refs: []location{s.loc(x)}}
	if array, ok := typ.(*ast.ArrayType); ok {
		v.Sequence = true
		for _, elem := range x.Elts {
			if kv, ok := elem.(*ast.KeyValueExpr); ok {
				elem = kv.Value
			}
			if c, ok := elem.(*ast.CompositeLit); ok {
				et := c.Type
				if et == nil {
					et = array.Elt
				}
				v.Items = append(v.Items, s.composite(c, et, e))
			} else {
				v.Items = append(v.Items, s.expr(elem, e))
			}
		}
		return v
	}
	names := s.typeNames(typ)
	v.Fields = map[string]goValue{}
	for i, elem := range x.Elts {
		name := ""
		if kv, ok := elem.(*ast.KeyValueExpr); ok {
			if id, ok := kv.Key.(*ast.Ident); ok {
				name = id.Name
			}
			elem = kv.Value
		} else if i < len(names) {
			name = names[i]
		}
		if name != "" {
			v.Fields[name] = s.expr(elem, e)
		}
	}
	return v
}
func (s *goScanner) declare(d *ast.GenDecl, e goEnv) {
	var previous []ast.Expr
	for _, spec := range d.Specs {
		switch x := spec.(type) {
		case *ast.TypeSpec:
			s.Package.Types[x.Name.Name] = goStructNames(x.Type)
		case *ast.ValueSpec:
			values := x.Values
			if len(values) == 0 && d.Tok == token.CONST {
				values = previous
			}
			previous = values
			for i, name := range x.Names {
				v := goValue{Unknown: true}
				if i < len(values) {
					v = s.expr(values[i], e)
				}
				v.Refs = goRefs(v.Refs, []location{s.loc(x)})
				e.set(name.Name, v, true)
			}
		}
	}
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
		if len(items) == 0 {
			items = []goValue{{Unknown: true, Refs: seq.Refs}}
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
		if fn := s.Package.Named[goCallName(c.Fun)]; fn != nil && s.Package.Routes[fn.Key] {
			args := make([]goValue, len(c.Args))
			for i, arg := range c.Args {
				args[i] = s.expr(arg, e)
			}
			ref := s.loc(c)
			s.function(fn, args, &ref)
			return false
		}
		return true
	})
}
func (s *goScanner) fallback(expr ast.Expr) bool {
	if goFallback(expr) {
		return true
	}
	call, ok := expr.(*ast.CallExpr)
	if !ok || len(call.Args) != 1 {
		return false
	}
	fn := s.Package.Named[goCallName(call.Fun)]
	return fn != nil && goHeaderDecorator(fn.AST) && s.fallback(call.Args[0])
}

// A syntactically transparent header-only decorator preserves its child's 405.
func goHeaderDecorator(fn *ast.FuncDecl) bool {
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
			return ok && id.Name == param
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || (sel.Sel.Name != "Set" && sel.Sel.Name != "Add" && sel.Sel.Name != "Del") {
			return false
		}
		receiver, ok := sel.X.(*ast.CallExpr)
		if !ok {
			return false
		}
		header, ok := receiver.Fun.(*ast.SelectorExpr)
		if !ok || header.Sel.Name != "Header" {
			return false
		}
	}
	return false
}
func goFallback(expr ast.Expr) bool {
	if fn, ok := expr.(*ast.FuncLit); ok && len(fn.Body.List) == 1 {
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
		status, ok := call.Args[0].(*ast.SelectorExpr)
		return ok && status.Sel.Name == "StatusMethodNotAllowed"
	}
	call, ok := expr.(*ast.CallExpr)
	if !ok {
		return false
	}
	name := goCallName(call.Fun)
	if len(call.Args) > 0 && strings.HasSuffix(strings.ToLower(name), "route") {
		if lit, ok := call.Args[0].(*ast.BasicLit); ok {
			v, _ := strconv.Unquote(lit.Value)
			return v == ""
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
		if len(fields) == 1 && strings.HasPrefix(fields[0], "/") {
			s.Out.Notes = append(s.Out.Notes, finding{Kind: "GO_MOUNT_OR_FALLBACK", Path: normalizePath(fields[0]), Detail: "methodless registration; child paths remain absolute unless StripPrefix is explicit", Locations: refs})
			if sel, ok := c.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "HandleFunc" && !s.fallback(c.Args[1]) {
				method := "EXACT"
				if strings.HasSuffix(fields[0], "/") {
					method = "PREFIX"
				}
				s.Out.Unresolved = append(s.Out.Unresolved, finding{Kind: "UNRESOLVED", Method: method, Path: normalizePath(fields[0]), Detail: "opaque Go methodless handler; business methods unknown", Locations: refs})
			}
			continue
		}
		if len(fields) == 2 && strings.HasPrefix(fields[1], "/") {
			if s.fallback(c.Args[1]) {
				s.Out.Notes = append(s.Out.Notes, finding{Kind: "GO_405_FALLBACK", Method: fields[0], Path: normalizePath(fields[1]), Detail: "explicit wrong-method fallback", Locations: refs})
			} else {
				addRoute(&s.Out, fields[0], fields[1], refs...)
			}
			continue
		}
		s.Out.Unresolved = append(s.Out.Unresolved, finding{Kind: "UNRESOLVED", Detail: "unsupported Go ServeMux pattern " + pattern, Locations: refs})
	}
}

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
