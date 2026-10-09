// Purpose: bind drift decisions to the PR merge base plus staged/working/untracked source changes.
// Depends on: Go stdlib os/exec and git --no-ext-diff; missing refs fail closed.
// Used by: baseline ratchet; no fetch, shell, checkout or repository mutation.
package main

import (
	"fmt"
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
}

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
	data, err = gitRead(root, "diff", "--no-ext-diff", "--no-color", "--no-renames", "-U0", c.Base, "--")
	if err != nil {
		return c, err
	}
	parseDiff(data, &c)
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
