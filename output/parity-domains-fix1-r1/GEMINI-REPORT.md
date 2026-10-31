# PARITY-DOMAINS-FIX1 Relay 1 Report

| Failing job / test | Class | What changed | File:line |
|---|---|---|---|
| 124551071912: build (Next builds: type check failure on `DesignDomains.tsx`) | (b) | Re-exported `DomainRequestResult`, `MerchantDomainState`, `StorefrontDomainRow`, `StorefrontDomains` from `storefront-client.ts`, imported types from `storefront-model.ts`, added explicit `StorefrontDomainRow` types to closure parameters, typed `stateMap` record, and cleanly destructured `edge_addresses` | `apps/admin/lib/storefront-client.ts:19-26`, `apps/admin/components/DesignDomains.tsx:9-18,113-119,483-539,689-700` |
| 124551295763: receipts (matrix-receipts) | Downstream | Downstream failure caused by missing build artifact from failed build job 124551071912 | N/A (resolved by build fix) |
| 124551295809: aggregate | Downstream | Downstream failure caused by missing build artifact from failed build job 124551071912 | N/A (resolved by build fix) |
| 124551389764: required | Downstream | Downstream failure caused by upstream build/receipts/aggregate | N/A (resolved by build fix) |

STATUS: DONE
