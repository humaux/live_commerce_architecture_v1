// Purpose: evaluate finite Go values, lexical scopes, unions and source provenance.
// Depends on: Go stdlib only; no DB/API/network mutations.
// Used by: CI-DRIFT scanGo and its parser gates.
// Invariant: extraction never executes handlers or discards unresolved registration coverage.
package main

import (
	"fmt"
	"go/ast"
	"go/token"
	"reflect"
	"sort"
	"strconv"
)

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
	out := goValue{Unknown: a.Unknown || b.Unknown, Sequence: a.Sequence || b.Sequence, Registrar: a.Registrar || b.Registrar, Mux: a.Mux || b.Mux, Refs: goRefs(a.Refs, b.Refs)}
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
	key := fmt.Sprintf("%t:%t:%t:%t:%q", v.Unknown, v.Sequence, v.Registrar, v.Mux, v.Text)
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
		if _, bound := e.lookup(x.Name); !bound {
			if fn := s.Package.Named[x.Name]; fn != nil {
				return goValue{Unknown: true, Refs: []location{s.loc(fn.AST)}}
			}
		}
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
		if v.Mux && goRegistrarSelector(x) {
			return goValue{Unknown: true, Registrar: true, Refs: goRefs(v.Refs, []location{s.loc(x)})}
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
		if sel, ok := x.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "NewServeMux" && len(x.Args) == 0 {
			if id, ok := sel.X.(*ast.Ident); ok && id.Name == s.File.HTTP && s.File.HTTP != "" {
				if _, shadowed := e.lookup(id.Name); !shadowed {
					return goValue{Unknown: true, Mux: true, Refs: goRefs([]location{s.loc(x)}, []location{s.File.ImportRefs[id.Name]})}
				}
			}
		}
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
		refs := goRefs([]location{s.loc(x)}, s.expr(x.Fun, e).Refs)
		for _, arg := range x.Args {
			refs = goRefs(refs, s.expr(arg, e).Refs)
		}
		return goValue{Unknown: true, Refs: refs}
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
