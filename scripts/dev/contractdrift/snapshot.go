// Purpose: recover merge-base provenance from real source snapshots, independent of seed-era baseline line numbers.
// Depends on: git archive and Go stdlib tar/io/fs; owns a task-local temporary tree and always removes it.
// Used by: initial seeding proof and deletion/touched ratchet; no network, source checkout or handler execution.
package main

import (
	"archive/tar"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func mergeBaseFindings(root, sha string) ([]finding, error) {
	dirs, err := gitRead(root, "ls-tree", "--name-only", sha, "--", "internal", "cmd", "apps", "contracts", "go.mod")
	if err != nil {
		return nil, err
	}
	names := strings.Fields(string(dirs))
	if len(names) == 0 {
		return nil, fmt.Errorf("merge base has no API source trees")
	}
	args := append([]string{"archive", sha, "--"}, names...)
	data, err := gitRead(root, args...)
	if err != nil {
		return nil, err
	}
	parent := filepath.Join(root, "output", "playwright", "ci-contract-drift")
	if err := os.MkdirAll(parent, 0700); err != nil {
		return nil, err
	}
	tmp, err := os.MkdirTemp(parent, "base-source-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmp)
	tr := tar.NewReader(bytes.NewReader(data))
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if h.Typeflag != tar.TypeReg {
			continue
		}
		ext := filepath.Ext(h.Name)
		if h.Name != "go.mod" && ext != ".go" && ext != ".ts" && ext != ".tsx" && ext != ".js" && ext != ".mjs" && ext != ".json" && ext != ".md" {
			continue
		}
		clean := filepath.Clean(h.Name)
		if filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
			return nil, fmt.Errorf("unsafe source archive entry")
		}
		dst := filepath.Join(tmp, clean)
		if err := os.MkdirAll(filepath.Dir(dst), 0700); err != nil {
			return nil, err
		}
		f, err := os.OpenFile(dst, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil {
			return nil, err
		}
		_, copyErr := io.Copy(f, tr)
		closeErr := f.Close()
		if copyErr != nil {
			return nil, copyErr
		}
		if closeErr != nil {
			return nil, closeErr
		}
	}
	g, err := scanGo(tmp)
	if err != nil {
		return nil, err
	}
	d, err := scanContracts(tmp)
	if err != nil {
		return nil, err
	}
	b, err := scanBFF(tmp)
	if err != nil {
		return nil, err
	}
	return compare(g, d, b), nil
}
