// Purpose: make CLI source mutations fail loudly when fixture anchors drift.
// Depends on: stdlib strings/testing; used by all contractdrift CLI fixtures.
// Invariant: every mutation changes the input and preserves its replacement count.

package main

import (
	"strings"
	"testing"
)

func mustReplace(t *testing.T, s, old, new string) string {
	t.Helper()
	if old == "" || !strings.Contains(s, old) {
		t.Fatalf("mutation anchor absent: %q", old)
	}
	result := strings.Replace(s, old, new, 1)
	if result == s {
		t.Fatalf("mutation made no change: %q", old)
	}
	return result
}

func mustReplaceAll(t *testing.T, s, old, new string) string {
	t.Helper()
	_ = mustReplace(t, s, old, new)
	return strings.ReplaceAll(s, old, new)
}
