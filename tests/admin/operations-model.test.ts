// Purpose: exercise W6-U2 closed DTO, capability, request and locale boundaries.
// Depends on: node:test/assert; operations-model/request/copy and ads-copy.
// Used by: test-node.sh; no provider or buyer data.
import { test } from "node:test";
import assert from "node:assert/strict";
import {
  canOperate,
  parseOperationList,
  parseOperationDetail,
  parseOperationResult,
  operationObjectHref,
} from "../../apps/admin/lib/operations-model.ts";
import {
  operationRoute,
  validOperationsRequest,
  validOperationBody,
  operationRetryAfter,
  operationQueryWaitSeconds,
} from "../../apps/admin/lib/operations-request.ts";
import { operationsCopy } from "../../apps/admin/lib/operations-copy.ts";
import * as operationCopyModule from "../../apps/admin/lib/operations-copy.ts";
import * as operationModelModule from "../../apps/admin/lib/operations-model.ts";
import { adsCopy } from "../../apps/admin/lib/ads-copy.ts";
const id = "a1000000-0000-4000-8000-000000000001";
const cap = (available: boolean, reason = "") => ({ available, reason });
const row = {
  operation_id: id,
  provider: "facebook",
  action: "meta.live_videos",
  purpose: "marketing",
  state: "UNKNOWN",
  reason_code: "provider_unknown",
  attempts: 2,
  created_at: "2026-10-06T17:00:00Z",
  updated_at: "2026-10-06T17:01:00Z",
  object: { kind: "claim_bundle", id },
  actions: {
    query: cap(true),
    cancel: cap(false, "already_dispatched"),
    retry: cap(false, "reconcile_first"),
  },
};
test("closed projections reject secret-shaped extras, invalid enums, IDs and contradictory capabilities", () => {
  assert.equal(
    parseOperationList({ items: [row], next_cursor: "" }).items[0].actions.retry
      .available,
    false,
  );
  for (const bad of [
    { ...row, payload: {} },
    { ...row, state: "PENDING" },
    { ...row, attempts: -1 },
    { ...row, operation_id: "foreign" },
    { ...row, created_at: "tomorrow" },
    { ...row, actions: { ...row.actions, query: cap(true, "lease_active") } },
    { ...row, object: { kind: "conversation", id, token: "secret" } },
  ])
    assert.throws(() => parseOperationList({ items: [bad], next_cursor: "" }));
  assert.throws(() =>
    parseOperationList({ items: [row], next_cursor: "", request: {} }),
  );
  assert.throws(() =>
    parseOperationList({ items: [row, row], next_cursor: "" }),
  );
});
test("detail accepts only bounded events and keeps actual server action flags", () => {
  const event = {
    generation: 2,
    state: "UNKNOWN",
    reason_code: "provider_unknown",
    created_at: row.updated_at,
  };
  assert.deepEqual(parseOperationDetail({ ...row, events: [event] }).events, [
    event,
  ]);
  assert.throws(() =>
    parseOperationDetail({
      ...row,
      events: [{ ...event, provider_body: "hidden" }],
    }),
  );
  assert.throws(() =>
    parseOperationDetail({ ...row, events: Array(51).fill(event) }),
  );
  assert.deepEqual(
    parseOperationResult({ operation_id: id, state: "CANCELLED", attempts: 3 }),
    { operation_id: id, state: "CANCELLED", attempts: 3 },
  );
  assert.throws(() =>
    parseOperationResult({
      operation_id: id,
      state: "CANCELLED",
      attempts: 3,
      request: {},
    }),
  );
});
test("BFF exact routes and request grammar forbid keys/body on reads, filters on detail and arbitrary writes", () => {
  assert.equal(operationRoute("GET", "operations"), "list");
  assert.equal(operationRoute("POST", `operations/${id}/cancel`), "command");
  for (const path of [`operations/${id}/delete`, "operations/../../orders"])
    assert.equal(operationRoute("POST", path), null);
  const req = (suffix: string, init: RequestInit = {}) =>
    new Request(`https://admin.example/operations${suffix}`, init);
  assert.equal(
    validOperationsRequest("list", req("?state=FAILED&limit=10")),
    true,
  );
  for (const suffix of [
    "?",
    "?state=FAILED&state=UNKNOWN",
    "?state=SUCCEEDED",
    "?tenant_id=x",
    "?limit=1000",
  ])
    assert.equal(validOperationsRequest("list", req(suffix)), false, suffix);
  assert.equal(validOperationsRequest("detail", req("?state=UNKNOWN")), false);
  assert.equal(
    validOperationsRequest(
      "list",
      req("", { headers: { "Idempotency-Key": "x" } }),
    ),
    false,
  );
  assert.equal(
    validOperationsRequest(
      "command",
      req("", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          "Idempotency-Key": "unit-ops-1",
        },
        body: '{"expected_attempts":2}',
      }),
    ),
    true,
  );
  assert.equal(
    validOperationsRequest(
      "command",
      req("?state=READY", { method: "POST", body: "{}" }),
    ),
    false,
  );
  for (const raw of [
    "{}",
    '{"expected_attempts":2,"tenant_id":"x"}',
    '{"expected_attempts":-1}',
    '{"expected_attempts":1.5}',
    '{"expected_attempts":2,"expected_attempts":2}',
  ])
    assert.equal(validOperationBody(raw), false, raw);
  assert.equal(validOperationBody('{"expected_attempts":0}'), true);
  assert.equal(operationRetryAfter("86400"), 86400);
  for (const raw of ["86401", "0", "tomorrow", "1\r\nX-Test: a"])
    assert.equal(operationRetryAfter(raw), null);
});
test("copy parity, safe object navigation and UNKNOWN/protective reasons", () => {
  for (const locale of ["zh-TW", "zh-CN"] as const) {
    assert.deepEqual(
      Object.keys(operationsCopy[locale]).sort(),
      Object.keys(operationsCopy.en).sort(),
    );
    assert.deepEqual(
      Object.keys(operationsCopy[locale].reasons).sort(),
      Object.keys(operationsCopy.en.reasons).sort(),
    );
  }
  assert.match(
    operationsCopy["zh-TW"].reasons.reconcile_first,
    /結果未知.*先查詢/,
  );
  assert.match(
    operationsCopy["zh-TW"].reasons.protective_operation,
    /保護性.*不可取消/,
  );
  assert.equal(
    operationObjectHref("zh-TW", id, { kind: "ad_draft", id }),
    `/zh-TW/ads?store=${id}&draft=${id}`,
  );
  assert.equal(
    operationObjectHref("en", id, { kind: "payment_attempt", id }),
    null,
  );
});

test("read-only store capability never exposes a command even if the state allows one", () => {
  assert.equal(
    canOperate({ role: "staff", permissions: ["integration:read"] }),
    false,
  );
  assert.equal(
    canOperate({ role: "staff", permissions: ["integration:execute"] }),
    true,
  );
  assert.equal(canOperate(null), false);
  assert.equal(canOperate({}), false);
});

test("duplicate escaped keys cannot bypass the exact command grammar", () => {
  assert.equal(
    validOperationBody(
      String.raw`{"expected_attempts":2,"\u0065xpected_attempts":2}`,
    ),
    false,
  );
});

test("query backoff affects only its operation and expires without another action", () => {
  const wait = { id, until: 10000 };
  assert.equal(operationQueryWaitSeconds(id, wait, 9001), 1);
  assert.equal(operationQueryWaitSeconds("other", wait, 9001), 0);
  assert.equal(operationQueryWaitSeconds(id, wait, 10000), 0);
});

test("merchant reason text translates ledger events and conceals unknown or prototype codes", () => {
  assert.equal(typeof operationCopyModule.operationReasonText, "function");
  const reason = operationCopyModule.operationReasonText;
  const expected = {
    en: [
      "Status query requested",
      "Retry authorized",
      "Operation queued again",
      "Cancelled by merchant",
      "Provider reported a failure",
      "Provider result unknown",
      "Query budget exhausted",
    ],
    "zh-TW": [
      "已請求查詢狀態",
      "已授權重試",
      "操作已重新排入佇列",
      "商家已取消",
      "服務平台回報失敗",
      "服務平台結果未知",
      "查詢額度已用盡",
    ],
    "zh-CN": [
      "已请求查询状态",
      "已授权重试",
      "操作已重新排入队列",
      "商家已取消",
      "服务平台报告失败",
      "服务平台结果未知",
      "查询额度已用尽",
    ],
  };
  const codes = [
    "query_requested",
    "retry_authorized",
    "requeue_authorized",
    "cancelled_by_merchant",
    "provider_failed",
    "provider_unknown",
    "reconcile_budget_exhausted",
  ];
  const fallback = {
    en: "Additional details are not available.",
    "zh-TW": "沒有其他詳細資訊。",
    "zh-CN": "没有其他详细信息。",
  };
  const labels = {
    en: "Ledger store",
    "zh-TW": "台賬商店",
    "zh-CN": "台账店铺",
  };
  for (const locale of ["en", "zh-TW", "zh-CN"] as const) {
    for (const [index, code] of codes.entries())
      assert.equal(reason(locale, code), expected[locale][index]);
    assert.equal(reason(locale, ""), "—");
    assert.equal(operationsCopy[locale].switchStore, labels[locale]);
    assert.equal(operationsCopy[locale].reasonFallback, fallback[locale]);
    assert.equal(
      Object.hasOwn(operationsCopy[locale].reasons, "synthetic_unknown"),
      false,
    );
    for (const code of [
      "synthetic_unknown",
      "new_provider_reason",
      "constructor",
      "toString",
      "__proto__",
    ])
      assert.equal(
        reason(locale, code),
        fallback[locale],
        `${locale}: ${code}`,
      );
    assert.equal(
      reason(locale, "protective_operation"),
      operationsCopy[locale].reasons.protective_operation,
    );
    assert.equal(
      reason(locale, "reconcile_first"),
      operationsCopy[locale].reasons.reconcile_first,
    );
    assert.equal(
      adsCopy[locale].opStates.UNKNOWN,
      { en: "Result unknown", "zh-TW": "結果未知", "zh-CN": "结果未知" }[
        locale
      ],
    );
  }
});

test("ledger read navigation requires effective integration:read permission regardless of role", () => {
  assert.equal(typeof operationModelModule.canReadOperations, "function");
  const canRead = operationModelModule.canReadOperations;
  for (const store of [
    null,
    {},
    { role: "owner" },
    { role: "owner", permissions: [] },
    { role: "staff", permissions: ["integration:execute"] },
    { role: "owner", permissions: ["ads:read"] },
  ])
    assert.equal(canRead(store), false);
  for (const role of ["owner", "staff", "custom", undefined])
    assert.equal(canRead({ role, permissions: ["integration:read"] }), true);
});
