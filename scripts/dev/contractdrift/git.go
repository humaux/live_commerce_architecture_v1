// Purpose: bind drift decisions to the PR merge base plus staged/working/untracked source changes.
// Depends on: Go stdlib os/exec and raw text git diffs with fixed prefixes; missing provenance fails closed.
// Used by: baseline ratchet; no fetch, shell, checkout or repository mutation.
package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

type lineRange struct{ First, Last int }
type changes struct {
	Current, Previous map[string][]lineRange
	Base              string
	Edits             map[string][]lineEdit
}
type lineEdit struct{ OldStart, OldCount, NewStart, NewCount int }

func gitRead(root string, args ...string) ([]byte, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	return cmd.Output()
}

var diffHunk = regexp.MustCompile(`^@@ -([0-9]+)(?:,([0-9]+))? \+([0-9]+)(?:,([0-9]+))? @@`)

func diffPath(raw string) string {
	if strings.HasPrefix(raw, `"`) {
		s, e := strconv.Unquote(raw)
		if e == nil {
			raw = s
		}
	}
	return strings.TrimPrefix(strings.TrimPrefix(raw, "a/"), "b/")
}
func parseDiff(data []byte, c *changes) {
	old, newPath := "", ""
	inHunk := false
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "diff --git ") {
			old, newPath, inHunk = "", "", false
			continue
		}
		// A removed '-- ' or added '++ ' source line looks like a file header inside a hunk.
		// Only the header block of a new diff --git section can set file ownership.
		if !inHunk && strings.HasPrefix(line, "--- ") {
			old = diffPath(line[4:])
		}
		if !inHunk && strings.HasPrefix(line, "+++ ") {
			newPath = diffPath(line[4:])
		}
		m := diffHunk.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		inHunk = true
		oldStart, _ := strconv.Atoi(m[1])
		newStart, _ := strconv.Atoi(m[3])
		oldCount, newCount := 1, 1
		if m[2] != "" {
			oldCount, _ = strconv.Atoi(m[2])
		}
		if m[4] != "" {
			newCount, _ = strconv.Atoi(m[4])
		}
		if old != "" && old != "/dev/null" {
			if c.Edits == nil {
				c.Edits = map[string][]lineEdit{}
			}
			c.Edits[old] = append(c.Edits[old], lineEdit{oldStart, oldCount, newStart, newCount})
		}
		for i, dst := range []map[string][]lineRange{c.Previous, c.Current} {
			start, _ := strconv.Atoi(m[1+i*2])
			count := 1
			if m[2+i*2] != "" {
				count, _ = strconv.Atoi(m[2+i*2])
			}
			name := old
			if i == 1 {
				name = newPath
			}
			if count > 0 && name != "/dev/null" {
				dst[name] = append(dst[name], lineRange{start, start + count - 1})
			}
		}
	}
}
func gitChanges(root, base string) (changes, error) {
	c := changes{Current: map[string][]lineRange{}, Previous: map[string][]lineRange{}}
	data, err := gitRead(root, "merge-base", "--", base, "HEAD")
	if err != nil {
		return c, fmt.Errorf("merge-base %s unavailable: %w", base, err)
	}
	c.Base = strings.TrimSpace(string(data))
	if !regexp.MustCompile(`^[0-9a-f]{40,64}$`).MatchString(c.Base) {
		return c, fmt.Errorf("invalid merge base")
	}
	// Diff the merge base against final files: committed/staged/working edits share one coordinate system.
	// When clean this is the PR's merge-base...HEAD diff; the working supplement catches local mutations too.
	data, err = gitRead(root, "diff", "--patch", "--text", "--no-textconv", "--no-ext-diff", "--no-color", "--no-renames", "--no-relative", "--src-prefix=a/", "--dst-prefix=b/", "-U0", c.Base, "--")
	if err != nil {
		return c, err
	}
	parseDiff(data, &c)
	// An independently enumerated changed source must have raw line provenance, even if Git attributes/config suppress output.
	names, err := gitRead(root, "diff", "--name-only", "-z", "--no-ext-diff", "--no-textconv", "--no-renames", "--no-relative", c.Base, "--")
	if err != nil {
		return c, err
	}
	for _, name := range strings.Split(string(names), "\x00") {
		if !apiSourceFile(name) || len(c.Current[name])+len(c.Previous[name]) > 0 {
			continue
		}
		before, e := gitRead(root, "show", c.Base+":"+name)
		if e != nil {
			if _, exists := gitRead(root, "cat-file", "-e", c.Base+":"+name); exists == nil {
				return c, e
			}
		}
		after, e := os.ReadFile(filepath.Join(root, filepath.FromSlash(name)))
		if e != nil && !os.IsNotExist(e) {
			return c, e
		}
		if !bytes.Equal(before, after) {
			return c, fmt.Errorf("changed API source lacks raw line provenance: %s", name)
		}
	}
	data, err = gitRead(root, "ls-files", "--others", "--exclude-standard", "-z")
	if err != nil {
		return c, err
	}
	for _, name := range strings.Split(string(data), "\x00") {
		if name == "" {
			continue
		}
		// Untracked source is entirely changed; do not read unrelated binary evidence just to count its lines.
		c.Current[name] = append(c.Current[name], lineRange{1, 1 << 30})
	}
	return c, nil
}

func apiSourceFile(name string) bool {
	ext := filepath.Ext(name)
	return name == "go.mod" || (strings.HasPrefix(name, "internal/") || strings.HasPrefix(name, "cmd/api/")) && ext == ".go" || (strings.HasPrefix(name, "apps/") || strings.HasPrefix(name, "packages/")) && (ext == ".ts" || ext == ".tsx" || ext == ".js" || ext == ".jsx" || ext == ".mjs" || strings.HasSuffix(name, "/package.json")) || strings.HasPrefix(name, "contracts/") && (ext == ".md" || strings.HasSuffix(name, "-openapi.json"))
}

func previousLine(line int, edits []lineEdit) (int, bool) {
	delta := 0
	for _, h := range edits {
		if h.OldCount > 0 && line >= h.OldStart && line < h.OldStart+h.OldCount {
			return 0, false
		}
		if h.OldCount == 0 && line > h.OldStart || h.OldCount > 0 && line >= h.OldStart+h.OldCount {
			delta += h.NewCount - h.OldCount
		}
	}
	return line + delta, true
}

// Persisting unknown regions retain their merge-base identity even when a hunk has no new-side lines.
// A completely removed old region has no surviving lines and cannot taint an unrelated remaining unknown.
func unresolvedTouched(f finding, base baseline, c changes) bool {
	if intersects(f.Locations, c.Current) {
		return true
	}
	if len(f.Locations) == 0 {
		return false
	}
	current := f.Locations[0]
	end := current.End
	if end < current.Line {
		end = current.Line
	}
	equivalent := false
	for _, old := range base.Entries {
		if old.Kind != "UNRESOLVED" || len(old.Locations) == 0 || old.Method != f.Method || old.Path != f.Path || old.Detail != f.Detail {
			continue
		}
		ref := old.Locations[0]
		if ref.File != current.File {
			continue
		}
		last := ref.End
		if last < ref.Line {
			last = ref.Line
		}
		for line := ref.Line; line <= last; line++ {
			mapped, ok := previousLine(line, c.Edits[ref.File])
			if ok && mapped >= current.Line && mapped <= end {
				// Same file/signature is insufficient: an unrelated old unknown must not hide a newly opaque region.
				equivalent = true
				if intersects(old.Locations, c.Previous) {
					return true
				}
				break
			}
		}
	}
	// Deleting a previously valid field may introduce a new unknown with no Current hunk.
	return !equivalent && len(c.Previous[current.File]) > 0
}
func intersects(refs []location, set map[string][]lineRange) bool {
	for _, ref := range refs {
		end := ref.End
		if end < ref.Line {
			end = ref.Line
		}
		for _, r := range set[ref.File] {
			if ref.Line <= r.Last && end >= r.First {
				return true
			}
		}
	}
	return false
}
func touched(f finding, b baseline, c changes) bool {
	if intersects(f.Locations, c.Current) {
		return true
	}
	if old, ok := baselineMap(b)[findingKey(f)]; ok {
		return intersects(old.Locations, c.Previous)
	}
	return false
}
func baseBaseline(root, base, path string) (baseline, bool, error) {
	rel, err := filepath.Rel(root, path)
	if err != nil || strings.HasPrefix(rel, "..") {
		return baseline{}, false, fmt.Errorf("baseline must be inside root")
	}
	data, err := gitRead(root, "show", base+":"+filepath.ToSlash(rel))
	if err != nil {
		// Distinguish missing initial baseline from malformed repository/configuration.
		_, existsErr := gitRead(root, "cat-file", "-e", base+":"+filepath.ToSlash(rel))
		if existsErr != nil {
			return baseline{Version: 1}, false, nil
		}
		return baseline{}, false, err
	}
	b, err := decodeBaseline(data)
	return b, true, err
}
