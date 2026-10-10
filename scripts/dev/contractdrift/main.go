// Package main owns the zero-dependency API contract drift gate.
// It never runs handlers, fetches dependencies or changes application contracts.
// Purpose: compare METHOD/normalized path across Go, contracts and BFF and ratchet reviewed legacy drift.
// Depends on: Go stdlib parsers/JSON/regexp/git; CONTRACT_DRIFT_BASE defaults to origin/r3/integration.
// Used by: check-gates.sh, CI and developers; -write-baseline initially seeds unchanged legacy, then only shrinks it.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func run(args []string, out, errOut io.Writer) int {
	flags := flag.NewFlagSet("contractdrift", flag.ContinueOnError)
	flags.SetOutput(errOut)
	rootFlag := flags.String("root", ".", "repository root")
	baseDefault := os.Getenv("CONTRACT_DRIFT_BASE")
	if baseDefault == "" {
		baseDefault = "origin/r3/integration"
	}
	baseFlag := flags.String("base", baseDefault, "git base ref")
	baselineFlag := flags.String("baseline", "scripts/dev/contractdrift/baseline.json", "reviewed legacy baseline")
	write := flags.Bool("write-baseline", false, "initial seed or stale-entry removal, never adds to an existing baseline")
	asJSON := flags.Bool("json", false, "emit a structured report")
	if err := flags.Parse(args); err != nil {
		return 1
	}
	if flags.NArg() != 0 {
		fmt.Fprintln(errOut, "unexpected arguments")
		return 1
	}
	fail := func(err error) int { fmt.Fprintln(errOut, "contractdrift:", err); return 1 }
	root, err := filepath.Abs(*rootFlag)
	if err != nil {
		return fail(err)
	}
	bp := *baselineFlag
	if !filepath.IsAbs(bp) {
		bp = filepath.Join(root, bp)
	}
	changes, err := gitChanges(root, *baseFlag)
	if err != nil {
		return fail(err)
	}
	goInv, err := scanGo(root)
	if err != nil {
		return fail(err)
	}
	docInv, err := scanContracts(root)
	if err != nil {
		return fail(err)
	}
	bffInv, err := scanBFF(root)
	if err != nil {
		return fail(err)
	}
	findings := compare(goInv, docInv, bffInv)
	b, exists, err := readBaseline(bp)
	if err != nil {
		return fail(err)
	}
	old, oldExists, err := baseBaseline(root, changes.Base, bp)
	if err != nil {
		return fail(err)
	}
	legacy, prior := baselineMap(b), baselineMap(old)
	// Baseline positions describe seeding, not necessarily today's base: parse the immutable base for deletion provenance.
	baseSource, err := mergeBaseInventories(root, changes.Base)
	if err != nil {
		return fail(err)
	}
	baseFindings := compare(baseSource.Go, baseSource.Contracts, baseSource.BFF)
	regressions := bffRegressions(baseSource, sourceInventories{goInv, docInv, bffInv})
	findings = append(findings, regressions...)
	baseFacts := baseline{Version: 1, Entries: baseFindings}
	baseIssues := map[string]finding{}
	for _, f := range baseFindings {
		if mismatchKinds[f.Kind] {
			baseIssues[findingKey(f)] = f
		}
	}
	if oldExists && !exists && !*write {
		return fail(fmt.Errorf("BASELINE_DELETED: keep an explicit reviewed baseline, even when empty"))
	}
	current := map[string]finding{}
	for _, f := range findings {
		if mismatchKinds[f.Kind] {
			current[findingKey(f)] = f
		}
	}
	// Compare keys with the base version, never trust a PR-edited locations array to hide a deleted/touched source.
	if oldExists {
		for key := range legacy {
			if _, ok := prior[key]; !ok {
				return fail(fmt.Errorf("BASELINE_ADDED %s: baseline may only shrink", key))
			}
		}
	}
	// First adoption is reviewed legacy too: a hand-edited file must obey the same admission as -write-baseline.
	if !oldExists {
		for key := range legacy {
			if _, ok := baseIssues[key]; !ok {
				return fail(fmt.Errorf("BASELINE_ADDED %s: not legacy at merge base", key))
			}
		}
	}
	if *write {
		if len(regressions) > 0 {
			return fail(fmt.Errorf("BFF_REMOVED cannot be grandfathered: %s", regressions[0].Detail))
		}
		seed := baseline{Version: 1, Seed: changes.Base, Entries: []finding{}}
		for _, f := range findings {
			if !mismatchKinds[f.Kind] {
				continue
			}
			if touched(f, baseFacts, changes) {
				return fail(fmt.Errorf("TOUCHED %s cannot be grandfathered", findingKey(f)))
			}
			if oldExists {
				if _, ok := prior[findingKey(f)]; !ok {
					return fail(fmt.Errorf("NEW %s cannot be re-seeded", findingKey(f)))
				}
			}
			if !oldExists {
				if _, ok := baseIssues[findingKey(f)]; !ok {
					return fail(fmt.Errorf("NEW %s was not legacy at merge base", findingKey(f)))
				}
			}
			if exists {
				if _, ok := legacy[findingKey(f)]; !ok {
					return fail(fmt.Errorf("NEW %s cannot be added to existing baseline", findingKey(f)))
				}
			}
			seed.Entries = append(seed.Entries, f)
		}
		for _, f := range findings {
			if f.Kind == "UNRESOLVED" && unresolvedTouched(f, baseFacts, changes) {
				return fail(fmt.Errorf("changed UNRESOLVED cannot be grandfathered"))
			}
		}
		if err := writeBaseline(bp, seed); err != nil {
			return fail(err)
		}
		b = seed
		legacy = baselineMap(b)
	}
	errorCount, warnings := 0, 0
	counts := map[string]int{}
	type row struct {
		Level, Reason string
		Finding       finding
	}
	rows := []row{}
	for _, f := range findings {
		counts[f.Kind]++
		level, reason := "INFO", ""
		if f.Kind == "BFF_REMOVED" {
			level, reason = "ERROR", "merge-base regression"
		} else if mismatchKinds[f.Kind] {
			if _, ok := legacy[findingKey(f)]; !ok {
				level, reason = "ERROR", "NEW"
			} else if touched(f, baseFacts, changes) || !oldExists && touched(f, b, changes) {
				level, reason = "ERROR", "TOUCHED"
			} else {
				level, reason = "WARNING", "legacy"
			}
		} else if f.Kind == "UNRESOLVED" {
			if unresolvedTouched(f, baseFacts, changes) {
				level, reason = "ERROR", "TOUCHED"
			} else {
				level, reason = "WARNING", "unresolved"
			}
		}
		if level == "ERROR" {
			errorCount++
		}
		if level == "WARNING" {
			warnings++
		}
		rows = append(rows, row{level, reason, f})
	}
	for key, f := range legacy {
		if _, ok := current[key]; !ok {
			rows = append(rows, row{"ERROR", "STALE", f})
			errorCount++
		}
	}
	sort.SliceStable(rows, func(i, j int) bool {
		a, b := findingKey(rows[i].Finding), findingKey(rows[j].Finding)
		if a != b {
			return a < b
		}
		return rows[i].Reason < rows[j].Reason
	})
	if *asJSON {
		_ = json.NewEncoder(out).Encode(map[string]any{"routes": map[string]int{"go": len(routeMap(goInv)), "contract": len(routeMap(docInv)), "bff": len(routeMap(bffInv))}, "counts": counts, "errors": errorCount, "warnings": warnings, "base": changes.Base, "findings": rows})
	} else {
		for _, r := range rows {
			f := r.Finding
			refs := []string{}
			for _, loc := range locations(f.Locations) {
				refs = append(refs, fmt.Sprintf("%s:%d", loc.File, loc.Line))
			}
			detail := f.Detail
			if f.Kind == "GO_NOT_IN_BFF" {
				for _, prefix := range internalOnlyPrefixes {
					if strings.HasPrefix(f.Path, prefix) {
						detail = "internal-only namespace"
						break
					}
				}
			}
			fmt.Fprintf(out, "%s (%s) %s %s %s %s %s\n", r.Level, r.Reason, f.Kind, f.Method, f.Path, strings.Join(refs, ", "), detail)
		}
		fmt.Fprintf(out, "SUMMARY routes go=%d contract=%d bff=%d; errors=%d warnings=%d", len(routeMap(goInv)), len(routeMap(docInv)), len(routeMap(bffInv)), errorCount, warnings)
		keys := []string{}
		for k := range counts {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			fmt.Fprintf(out, " %s=%d", k, counts[k])
		}
		fmt.Fprintln(out)
	}
	if errorCount > 0 {
		return 1
	}
	return 0
}
func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }
