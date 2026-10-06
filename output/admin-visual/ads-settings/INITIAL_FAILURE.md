# Observed initial MOCK failure

Command: node --test --experimental-strip-types tests/admin/ads-model.test.ts tests/admin/ads-request.test.ts tests/admin/attribution.test.ts tests/admin/attribution-audience.test.ts tests/admin/attribution-format.test.ts

Exit code: 1. The tool output reported `Element type is invalid ... got: undefined` from SSR rendering in attribution.test.ts. Its custom VM import resolver returned an empty object for the newly consumed frozen shared components.

Fix: transpile/execute actual packages/ui/src/Presentation.tsx and AdminPageHeader.tsx in the SSR harness, supplying CSS names and fixed pathname only. No domain assertion removed or relaxed. Final repeated command: exit 0, 61 pass. Original failure was observed in tool output; a full initial stdout log was NOT_SAVED.
