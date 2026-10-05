<!-- Purpose: Distinguish the deliberately interrupted old-source sweep from product acceptance. -->
<!-- Depends on: f0b93ca6, runner51998, Node89470 and the preserved wrapper/trace logs. -->
<!-- Used by: admin-visual rebase and final DELIVERY provenance. -->
# Old-source sweep interrupted for the new integration request

The user explicitly requested rebasing/merging the current r3/integration (including newer backend units). It was pinned to1a6a917774672f297e13ce56ef070a3084a2a924. The previously tested source wasf0b93ca6d56d01c68e8787dcf692de7318dfe0f5.

After fresh cwd/parent checks, the root paused only its runner51998 and sent SIGINT only to its actual Node sweep process89470. The Go fixture returned exit1 after cleanup; all own descendant PIDs89227/89379/89409/89469/89470 were gone before touching source. No foreign test or machine lock was modified. The parent was then resumed only to record the actual exit and finish its last loop entry.

The full sweep is **INTERRUPTED / NOT_ACCEPTED**, not PASS and not a diagnosed product failure. Earlier31 functional exits remain historical evidence forf0. No old result is automatically inherited by the new integration baseline. No assertion, detector, fixture or test threshold was changed to stop this run.
