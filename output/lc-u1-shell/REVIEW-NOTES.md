<!-- Purpose: Preserve the independent LC-U1 review findings and the bounded author fixes without inventing runtime acceptance.
Depends on: Pinned source commits 6a9ed9d7, a589b4fe, a1d3d271; independent lcu1_safety_review responses and red/green logs.
Used by: LC-U1 final delivery and integrator review. -->
# LC-U1 independent review notes

Reviewer: separate read-only security_reviewer; requested `gpt-6.1-sol`, high effort. Actual runtime model/effort telemetry was not exposed. Reviewer did not implement, launch tests or call providers.

| Finding | Original evidence | Author change | Independent status |
| --- | --- | --- | --- |
| P1 delayed copy response navigated after changing the active scene | `6a9ed9d7`, LiveConsole/SessionCopy navigated inside execute before the hook's identity check; no unmount generation | `a589b4fe`: commands return receipts; completion/navigation runs after mounted generation + session/store checks; confirmed navigation invalidates pending context immediately | Static closure confirmed at `a589b4fe`; runtime proof pending |
| P2 ordinary refresh/conceal lost UNKNOWN same-key retry | `6a9ed9d7`, transient read boundary reset cleared pending request | `a589b4fe`: preserve command identity through transient reads; retain durable opaque fence | Local path statically closed; reviewer found parent path below |
| P2 parent list failure/scene-less conceal unmounted command owner | `a589b4fe`, LiveWorkspace error branch/empty selection removed LiveConsole | `90f6efcb`: retained store-scoped scene ID; transient list errors are additive, actual auth/scope changes still purge | Static closure confirmed at `a1d3d271` |
| P2 delayed-copy browser case did not distinguish SPA stale callbacks | `a1d3d271`, shell store selector performs a full document navigation | `ac536415`: dedicated source/destination scenes; actual scene-picker `router.push`; delayed ACK must not override selected scene | Browser run pending; old-unsafe browser mutation NOT_RUN |

The reviewer observed real Next/OIDC/`platform.WithScope` boundaries in the fixture and found no concrete identity bypass in this bounded read. That is not SQL/provider acceptance. The console business responses are explicitly MOCK. Root separately strengthened the narrow-permission case to start from a fresh **draft**, so an ended session cannot masquerade as the permission refusal.

## Root evidence and limits

- Pure deferred-copy completion gate: `red-copy-scope.log` exit 1 (new helper missing) → `green-copy-scope.log` exit 0, five tests. This is **not** a browser mutation red of the old unsafe callback.
- Browser case now tests UNKNOWN → refresh → parent list 503 → same-key retry; real HTTP receipts assert stable key/body and one effect. Execution pending.
- `f3b465b8` maps remaining logout cookies to names before assertion, preserving the zero-cookie check without leaking values in failure output.
- `caca0f0f` fixes the root-found reauthentication gap: durable receipt-fence keys now depend on store/scene, not the replaced session cookie. In-memory replay remains bound to the original session. `red-reauth-fence.log` exit 1 → `green-reauth-fence.log` exit 0; the real sign-out/sign-in browser assertion remains pending.
- Historical `ffe45ef6` browser RED was interrupted without a final result. Its resumed waiter exited 137 after author cancellation before acquiring PG; neither is browser RED proof. The current-source launcher exited 2 during its pre-test merge; conflicts were resolved in `d50953a8`, without restarting locally.
- Under owner rule `2f596a0c`, browser/visual/sweep acceptance is GitHub-only. There are no active unit-owned local waiters. See DELIVERY.md for exact CI modes and source pin.
- Independent screenshot review and runtime closure remain pending. No source-only report is labelled E3/E4 runtime acceptance.
