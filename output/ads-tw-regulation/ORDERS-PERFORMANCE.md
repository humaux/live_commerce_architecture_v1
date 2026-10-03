# Observed order-search failures — not an ads code fix

Two real runs failed the unchanged strict `<1s` assertion in
`tests/foundation/merchant_orders_v2_perf_test.go:160`:

| Source / run | Query | Observed | Evidence |
|---|---|---:|---|
| 5845431c, first complete G07 | 7654 | 1.233441625s | release-gate/G07.log:9231 |
| 769816d0, fresh focused PG | 7654 | 1.107228416s | a3-focused-final.log:4 |
| b7afdf1d, final complete G07 PASS | 7654 | n10; p50=99.802125ms; max=104.989709ms | release-gate-a5/G07.log:9230 |
| b7afdf1d, final complete G07 PASS | SYNTHETIC-TRACK-7654 | n10; p50=76.05975ms; max=77.281667ms | release-gate-a5/G07.log:9231 |

The failing loop repetition is not printed. Do not describe it as the first sample.
No EXPLAIN, resource trace or controlled timing experiment establishes the cause.
Do not dismiss it as an environment flake, round down, enlarge the timeout, or remove the test.

Independent source-impact review found the test, merchant order Go path and related migrations
unchanged from base b0f835c3. Migration0112 only adds the ads refusal projection; this unit makes
no order-query/index change. Humaux review:6814e5c4-27b8-4d00-bd0d-5f84fefcfc5b.

Targeted reproduction (isolated fixture, observe the existing machine-wide lock):

```sh
export LC_TEST_LOCK_WAIT=14400
bash scripts/dev/test-focused.sh '^TestMerchantOrdersV2TenThousandScopedSearch$'
```

The test seeds 10,000 cancelled orders, analyzes the order table, and times tenant/store-scoped
searches including a telephone suffix. API → merchantorders.ListV2 → identity.read_merchant_orders_v2.
It requires every measured repetition to remain below1s. The full/focused red evidence must remain
in the integration record even if a later run passes. A separate orders-owner investigation can
collect plans and timing distributions without changing the existing privacy/tenant or latency gates.

Final complete G07 exits0 (6517 tests/subtests PASS,0FAIL;13 accepted skips).
The final order test passes in17.65s without source/threshold changes. This is not a causal diagnosis
or a claim that the earlier timing variability was fixed. Both real red runs remain in the record.
