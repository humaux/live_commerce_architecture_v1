# trunk-green-2 delivery
- Branch: unit/trunk-green-2  Base: 0c1dce66  Model: Claude Sonnet 5.5
- Summary: see REPORT.md. Edited 0125 in place (unreleased): DELETE grant, notify USAGE, lease_token. New package internal/integrations/metabridge/client.go; metareply/bridge.go re-exports. No new migration number.
- Contract/interface changes: none (metareply exported names preserved via aliases).
- Tests: red.log (HEAD: 5 of 6 selected fail, T06 passes); green: green-4.log `test-focused '^(TestR2Integration|TestMetaHealth|TestMetaConnect|TestT06WorkerAuthority)'` exit 0 (34 pass); green-3.log shows TestMeta* all PASS (run SIGTERMed externally at 609s in an unrelated TestMetaRuntime test); green-lcn.log TestLiveConsoleLCN + internal/live exit 0; meta-health-gate.log `test-local.sh --meta-health` exit 0.
- Gates: check-gates.sh exit 1 ONLY on node segment (typescript-api alias missing from node_modules, env, pre-existing); with the 2 node lines removed: exit 0 (header ratchet OK, depmap --check OK). go build/vet/gofmt clean; go test of internal/{metaconnect,httpapi,live,integrations/...} and cmd/... ok.
- Evidence class: MOCK / REAL_PG.
- Risks: SnapshotReader output order is now alphabetical (binding, capability); API-side compat aliases in metareply remain.
- NOT_RUN: full foundation suite; node UI gates.
- Integrator to-do: 0125 edited in place (never deployed); regenerate nothing else.
