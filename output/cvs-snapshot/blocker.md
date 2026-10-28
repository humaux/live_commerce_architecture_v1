# CVS-SNAP scope blocker — RESOLVED BY INTEGRATOR 2026-10-28

The latest human dispatch authorizes the CVS subtest in apps/storefront/tests/backend-parity.test.mjs and its necessary import. It replaces obsolete buyer name/address bounds with per-kind Go/TypeScript regex source parity and explicit positive/negative formats. All other subtests and the RETURN_PATH/migration 0093 assertion are unchanged.

The same dispatch corrects Hi-Life to ^([0-9]{4}|[A-Z][0-9]{3})$: all 1683 snapshot rows are selectable by valid code, including D-prefix stores. Go, SQL, TypeScript, copy and both golden tables now follow that rule. A full snapshot-row format loop and D010 HTTP/SQL regression cover this correction.

This resolves the prior scope STOP; it is not a GREEN receipt. Migration 0205 belongs to OC-05, and the assigned R2 pin stays 136. No change to that dependency is authorized.
