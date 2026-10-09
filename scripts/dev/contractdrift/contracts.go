// Purpose: extract contract method/path facts with exact JSON/Markdown source positions.
// Depends on: Go stdlib JSON tokens and regexp; only contracts/*-openapi.json and *.md are read.
// Used by: CI-DRIFT CLI; relative Markdown routes require a preceding Route base: /v1/... directive.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type jsonNode struct {
	Value  any
	Pos    int
	Fields map[string]*jsonNode
}

func jsonTree(d *json.Decoder, b []byte) (*jsonNode, error) {
	pos := int(d.InputOffset())
	for pos < len(b) && (b[pos] == ' ' || b[pos] == '\n' || b[pos] == '\t' || b[pos] == '\r' || b[pos] == ',' || b[pos] == ':') {
		pos++
	}
	tok, err := d.Token()
	if err != nil {
		return nil, err
	}
	n := &jsonNode{Value: tok, Pos: pos}
	if delim, ok := tok.(json.Delim); ok {
		if delim == '{' {
			n.Fields = map[string]*jsonNode{}
			for d.More() {
				kp := int(d.InputOffset())
				for kp < len(b) && (b[kp] == ' ' || b[kp] == '\n' || b[kp] == '\t' || b[kp] == '\r' || b[kp] == ',') {
					kp++
				}
				k, err := d.Token()
				if err != nil {
					return nil, err
				}
				name, ok := k.(string)
				if !ok {
					return nil, fmt.Errorf("object key is not a string")
				}
				if _, dup := n.Fields[name]; dup {
					return nil, fmt.Errorf("duplicate JSON key at byte%d", kp)
				}
				v, err := jsonTree(d, b)
				if err != nil {
					return nil, err
				}
				v.Pos = kp
				n.Fields[name] = v
			}
		} else if delim == '[' {
			for d.More() {
				if _, err := jsonTree(d, b); err != nil {
					return nil, err
				}
			}
		} else {
			return nil, fmt.Errorf("unexpected delimiter")
		}
		if _, err := d.Token(); err != nil {
			return nil, err
		}
	}
	return n, nil
}
func contractLoc(file string, b []byte, pos int) location {
	return location{File: file, Line: 1 + bytes.Count(b[:pos], []byte{'\n'})}
}

var mdBase = regexp.MustCompile("^\\s*(?:`)?Route base:\\s*([^`\\s]+)")
var mdRoute = regexp.MustCompile("\\b(GET|POST|PUT|PATCH|DELETE|HEAD|OPTIONS)(?:/(GET|POST|PUT|PATCH|DELETE|HEAD|OPTIONS))*[ \\t]+([^ \\t`<>|,;()]+)")
var httpMethods = map[string]bool{"GET": true, "POST": true, "PUT": true, "PATCH": true, "DELETE": true, "HEAD": true, "OPTIONS": true, "CONNECT": true, "TRACE": true}

func scanContracts(root string) (inventory, error) {
	var out inventory
	entries, err := os.ReadDir(filepath.Join(root, "contracts"))
	if os.IsNotExist(err) {
		return out, nil
	}
	if err != nil {
		return out, err
	}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".md") && !strings.HasSuffix(name, "-openapi.json") {
			continue
		}
		file := filepath.ToSlash(filepath.Join("contracts", name))
		b, err := os.ReadFile(filepath.Join(root, file))
		if err != nil {
			return out, err
		}
		if strings.HasSuffix(name, ".json") {
			d := json.NewDecoder(bytes.NewReader(b))
			tree, err := jsonTree(d, b)
			if err == nil {
				_, extra := d.Token()
				if extra != io.EOF {
					err = fmt.Errorf("trailing JSON")
				}
			}
			if err != nil {
				addUnresolved(&out, location{File: file, Line: 1, End: 1 + bytes.Count(b, []byte{'\n'})}, "invalid OpenAPI JSON: "+err.Error())
				continue
			}
			paths := tree.Fields["paths"]
			if paths == nil || paths.Fields == nil {
				addUnresolved(&out, location{File: file, Line: 1}, "OpenAPI has no paths object")
				continue
			}
			for path, node := range paths.Fields {
				if !strings.HasPrefix(path, "/") {
					addUnresolved(&out, contractLoc(file, b, node.Pos), "OpenAPI path must be absolute")
					continue
				}
				if node.Fields == nil {
					addUnresolved(&out, contractLoc(file, b, node.Pos), "OpenAPI path item must be an object")
					continue
				}
				if ref := node.Fields["$ref"]; ref != nil {
					addUnresolved(&out, contractLoc(file, b, ref.Pos), "OpenAPI path-item reference is not resolved")
					continue
				}
				for method, operation := range node.Fields {
					if !httpMethods[strings.ToUpper(method)] {
						continue
					}
					if operation.Fields == nil {
						addUnresolved(&out, contractLoc(file, b, operation.Pos), "OpenAPI operation must be an object")
						continue
					}
					addRoute(&out, method, path, contractLoc(file, b, node.Pos), contractLoc(file, b, operation.Pos))
				}
			}
			continue
		}
		base := ""
		var baseRef location
		for i, line := range strings.Split(string(b), "\n") {
			ref := location{File: file, Line: i + 1}
			if m := mdBase.FindStringSubmatch(line); m != nil {
				base = m[1]
				baseRef = ref
				if !strings.HasPrefix(base, "/v1/") {
					addUnresolved(&out, ref, "Route base must start /v1/")
					base = ""
				}
				continue
			}
			for _, m := range mdRoute.FindAllStringSubmatchIndex(line, -1) {
				text := line[m[0]:m[1]]
				at := strings.IndexAny(text, " \t")
				methods := strings.Split(text[:at], "/")
				path := strings.TrimSuffix(strings.TrimSpace(text[at:]), ".")
				refs := []location{ref}
				if strings.HasPrefix(path, "/v1/") {
				} else if strings.HasPrefix(path, ".../") || strings.HasPrefix(path, "/") && base != "" {
					if base == "" {
						addUnresolved(&out, ref, "relative Markdown route requires Route base")
						continue
					}
					refs = append(refs, baseRef)
					path = strings.TrimSuffix(base, "/") + "/" + strings.TrimPrefix(strings.TrimPrefix(path, "..."), "/")
				} else {
					continue
				} // Only explicit /v1 or declared-base paths describe this API; Graph/provider paths are separate contracts.
				if strings.Contains(path, "...") || strings.Contains(path, "*") {
					addUnresolved(&out, ref, "incomplete Markdown route path")
					continue
				}
				for _, method := range methods {
					addRoute(&out, method, path, refs...)
				}
			}
		}
	}
	return out, nil
}
