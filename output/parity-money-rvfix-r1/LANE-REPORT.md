# RVFIX-370 relay 1 lane report

| Finding / failing | Class | Change |
|---|---|---|
| e3a7cd77 CI red: foundation, gate 06/07 (new RVFIX assertions) | a (expected RED) | recorded in output/parity-money/red.log |
| P1-1 card state in Advanced | a | apps/admin/components/CardPayments.tsx:167-216 |
| P1-2 reversed range dead end | a | apps/admin/components/Finance.tsx:71 |
| P2-3 Settlements legend | a | apps/admin/components/Settlements.tsx:146 (+ Detail) |
| P2-7 generated_at optional | a | apps/admin/lib/customers-model.ts:81,268-276 |

Nothing run locally (owner rule). GREEN = NOT_RUN. WAIT_RUN: integrator pushes and waits on the head's CI.

STATUS: DONE
