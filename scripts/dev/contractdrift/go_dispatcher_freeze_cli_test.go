// Purpose: verify dispatcher freeze and its data/comment/row-handler boundaries through the built CLI.
// Depends on: real Git fixtures, scanGo candidates and the production in-package fingerprint function.
// Used by: CI only; owner forbids local Go test execution for BUYER-ROUTE-TABLE.
// Invariant: fixture pins precede mutations; literal hashes independently anchor positive canonicalization.

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const prefixDispatcherSHA = "bb891b718f9855ea78f60bb77fb3d23fca54c88f933304c21c80bad77198fc89"
const exactDispatcherSHA = "52e813a9f77a019c91e6ab9919db7977061658a1e2cc4e019fee4eb9337ee246"

// Explicit fixture review, never called after a negative mutation. scanGo uses
// goTableDispatcherFingerprint -> goDispatcherFingerprint, the production gate.
// Existing pins for other packages survive the two-table fixture setup.
func pinFixtureDispatcher(t *testing.T, root, table, literal string) goDispatcherPin {
	t.Helper()
	inv, err := scanGo(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, candidate := range inv.Dispatchers {
		if candidate.Table != table {
			continue
		}
		if literal != "" && candidate.SHA256 != literal {
			t.Fatalf("canonical fixture fingerprint changed: %s got %s want literal %s", table, candidate.SHA256, literal)
		}
		pins, err := readGoDispatcherPins(root)
		if err != nil {
			t.Fatal(err)
		}
		candidate.Reviewed = "2026-10-15 CI fixture review"
		pins[table] = candidate
		var all []goDispatcherPin
		for _, pin := range pins {
			all = append(all, pin)
		}
		data, err := json.MarshalIndent(all, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		trunkFactoryWrite(t, root, goDispatcherPinFile, string(data)+"\n")
		return candidate
	}
	t.Fatalf("no dispatcher candidate for %s", table)
	return goDispatcherPin{}
}

func assertFreezeCLI(t *testing.T, root string, resolved bool) string {
	t.Helper()
	code, out := goReviewProcessCLI(t, root)
	if resolved {
		if code != 0 || strings.Contains(out, "UNRESOLVED") || !strings.Contains(out, "GO_ROUTE_TABLE") {
			t.Fatalf("pinned table unresolved: exit%d %s", code, out)
		}
	} else {
		if code != 1 || !strings.Contains(out, "ERROR (dispatcher freeze) UNRESOLVED") || !strings.Contains(out, "route-table dispatcher changed or unpinned: router.Routes") || strings.Contains(out, "GO_ROUTE_TABLE") {
			t.Fatalf("changed/unpinned dispatcher accepted: exit%d %s", code, out)
		}
		// Every advertised row must carry the freeze diagnostic, not only the mount.
		for _, path := range []string{"GET /v1/table/one", "POST /v1/table/{}"} {
			if !strings.Contains(out, "ERROR (dispatcher freeze) UNRESOLVED "+path) {
				t.Errorf("missing per-route freeze failure %s: %s", path, out)
			}
		}
	}
	return out
}

func TestGoDispatcherFreezePinnedAndChangesCLI(t *testing.T) {
	cases := []struct {
		name, old, replacement string
		resolved               bool
	}{
		{"pinned", "", "", true},
		{"reached helper one byte", `id != ""`, `id == ""`, false},
		{"comment only", `if e.Path == path`, "// documentation only\n\n /* still documentation */\n if e.Path == path", true},
		{"doc comment", `func pattern`, "// New documentation\n// with a blank line\n\nfunc pattern", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root, p := routeTableCLIFixture(t)
			if tc.old != "" {
				mustWrite(t, p, strings.Replace(routeTableCLISource, tc.old, tc.replacement, 1))
			}
			assertFreezeCLI(t, root, tc.resolved)
		})
	}
}

func TestGoDispatcherFreezeMissingPinsCLI(t *testing.T) {
	for _, mode := range []string{"file", "entry", "wrong hash", "wrong function list", "unreviewed"} {
		t.Run(mode, func(t *testing.T) {
			root, _ := routeTableCLIFixture(t)
			p := filepath.Join(root, goDispatcherPinFile)
			switch mode {
			case "file":
				if err := os.Remove(p); err != nil {
					t.Fatal(err)
				}
			case "entry":
				mustWrite(t, p, "[]\n")
			default:
				pins, err := readGoDispatcherPins(root)
				if err != nil {
					t.Fatal(err)
				}
				pin := pins["router.Routes"]
				if mode == "wrong hash" {
					pin.SHA256 = strings.Repeat("0", 64)
				}
				if mode == "wrong function list" {
					pin.Functions = []string{"forged"}
				}
				if mode == "unreviewed" {
					pin.Reviewed = ""
				}
				data, err := json.Marshal([]goDispatcherPin{pin})
				if err != nil {
					t.Fatal(err)
				}
				mustWrite(t, p, string(data))
			}
			assertFreezeCLI(t, root, false)
		})
	}
}

func TestGoDispatcherFreezeUntouchedStillErrorsCLI(t *testing.T) {
	root, p := routeTableCLIFixture(t)
	mustWrite(t, p, strings.Replace(routeTableCLISource, `id != ""`, `id == ""`, 1))
	// Make both the changed dispatcher and its stale pin merge-base history: no
	// changed hunk remains. Freeze failure must never downgrade to legacy warning.
	git(t, root, "add", ".")
	git(t, root, "commit", "-qm", "stale dispatcher pin")
	git(t, root, "tag", "-f", "trunk")
	out := assertFreezeCLI(t, root, false)
	if strings.Contains(out, "TOUCHED") {
		t.Fatalf("untouched freeze relied on touched hunks: %s", out)
	}
	code, out := goReviewProcessCLI(t, root, "-write-baseline")
	if code != 1 || !strings.Contains(out, "cannot be grandfathered") {
		t.Fatalf("freeze failure seeded a baseline: exit%d %s", code, out)
	}
}

func TestGoDispatcherFreezeUnprovedPinErrorsCLI(t *testing.T) {
	for _, mode := range []string{"unmarked", "unconsumed", "orphan", "invalid rows"} {
		t.Run(mode, func(t *testing.T) {
			root, p := routeTableCLIFixture(t)
			switch mode {
			case "unmarked":
				mustWrite(t, p, strings.Replace(routeTableCLISource, "//contractdrift:routes /v1/table match allowed mount=Routes\n", "", 1))
			case "unconsumed":
				mustWrite(t, p, strings.Replace(routeTableCLISource, "selected := match(r.URL.Path)", "selected := route{}", 1))
			case "invalid rows":
				mustWrite(t, p, strings.Replace(routeTableCLISource, "range rows {", "range rows[:1] {", 1))
			case "orphan":
				pins, err := readGoDispatcherPins(root)
				if err != nil {
					t.Fatal(err)
				}
				orphan := pins["router.Routes"]
				orphan.Table = "router.Missing"
				data, err := json.Marshal([]goDispatcherPin{pins["router.Routes"], orphan})
				if err != nil {
					t.Fatal(err)
				}
				mustWrite(t, filepath.Join(root, goDispatcherPinFile), string(data))
			}
			code, out := goReviewProcessCLI(t, root)
			if code != 1 || !strings.Contains(out, "ERROR (dispatcher freeze) UNRESOLVED") || !strings.Contains(out, "pinned route table cannot be proved: router.") {
				t.Fatalf("unproved pin silently ignored: exit%d %s", code, out)
			}
			if mode != "orphan" && strings.Contains(out, "GO_ROUTE_TABLE") {
				t.Fatalf("unproved table resolved: %s", out)
			}
		})
	}
}

func TestGoDispatcherFreezeRowsAreDataCLI(t *testing.T) {
	for _, mode := range []string{"add", "remove"} {
		t.Run(mode, func(t *testing.T) {
			root, p := routeTableCLIFixture(t)
			source := routeTableCLISource
			contract := `{"openapi":"3.1.0","paths":{"/v1/table/one":{"get":{}}}}`
			if mode == "add" {
				source = strings.Replace(source, ` {Method: http.MethodPost, Path: "/v1/table/{id}", kind: "item"},`, ` {Method: http.MethodPost, Path: "/v1/table/{id}", kind: "item"},
 {Method: http.MethodGet, Path: "/v1/table/new", kind: "new"},`, 1)
				contract = `{"openapi":"3.1.0","paths":{"/v1/table/one":{"get":{}},"/v1/table/{id}":{"post":{}},"/v1/table/new":{"get":{}}}}`
			} else {
				source = strings.Replace(source, ` {Method: http.MethodPost, Path: "/v1/table/{id}", kind: "item"},`+"\n", "", 1)
			}
			mustWrite(t, p, source)
			mustWrite(t, filepath.Join(root, "contracts/route-table-openapi.json"), contract)
			out := assertFreezeCLI(t, root, true)
			want := "SUMMARY routes go=4 contract=4"
			if mode == "remove" {
				want = "SUMMARY routes go=2 contract=2"
			}
			if !strings.Contains(out, want) {
				t.Fatalf("rows did not change inventory: %s", out)
			}
		})
	}
}

func TestGoDispatcherFreezeRowHandlerBodyExcludedCLI(t *testing.T) {
	root, p := routeTableCLIFixture(t)
	source := strings.Replace(routeTableCLISource, "Method, Path, kind string", "Method, Path, kind string; Handler http.HandlerFunc", 1)
	source = strings.ReplaceAll(source, `kind: "one"},`, `kind: "one", Handler: rowLeaf},`)
	source = strings.ReplaceAll(source, `kind: "item"},`, `kind: "item", Handler: rowLeaf},`)
	source = strings.Replace(source, "w.WriteHeader(204)", "rowLeaf(w,r)", 1) + "\nfunc rowLeaf(w http.ResponseWriter,r *http.Request){w.WriteHeader(204)}\n"
	mustWrite(t, p, source)
	before := pinFixtureDispatcher(t, root, "router.Routes", "")
	if strings.Contains(strings.Join(before.Functions, "\n"), "rowLeaf") {
		t.Fatal("row handler target included in closure")
	}
	assertFreezeCLI(t, root, true)
	mustWrite(t, p, strings.Replace(source, "w.WriteHeader(204)", "w.WriteHeader(205)", 1))
	assertFreezeCLI(t, root, true)
	inv, err := scanGo(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(inv.Dispatchers) != 1 || inv.Dispatchers[0].SHA256 != before.SHA256 {
		t.Fatal("row handler body changed dispatcher fingerprint")
	}
}
