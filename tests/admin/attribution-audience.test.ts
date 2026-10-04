// MOCK transport/journal counterexamples. Not a worker, PG or provider permission acceptance claim.
import assert from "node:assert/strict";
import { test } from "node:test";
import {
  parseAudienceAck,
  parseAudienceJournal,
  audienceStorageKey,
  audienceAckPhase,
} from "../../apps/admin/lib/attribution-audience.ts";
import { requestAudienceRead } from "../../apps/admin/lib/attribution-client.ts";
import { sessionBoundary } from "../../apps/admin/lib/settings-client.ts";
import { draftID, sessionID } from "./attribution.fixture.ts";
import { attributionCopy } from "../../apps/admin/lib/attribution-copy.ts";
import { readFileSync } from "node:fs";
import { createRequire } from "node:module";
import { runInNewContext } from "node:vm";
import ts from "typescript-api";
const operation = "44444444-4444-4444-8444-444444444444",
  key = `audience-read-${operation}`;

test("R11 SQL's exact receipt states are accepted; missing/forged operation/state remains unknown", () => {
  assert.deepEqual(
    parseAudienceAck({
      operation_id: operation,
      state: "READY",
      extra: "ignored",
    }),
    { operation_id: operation, state: "READY" },
  );
  for (const state of [
    "DISPATCHING",
    "ACKNOWLEDGED",
    "SUCCEEDED",
    "UNKNOWN",
    "FAILED_FINAL",
    "CANCELLED",
    "BLOCKED_POLICY",
    "STALE_BINDING",
  ]) {
    assert.deepEqual(parseAudienceAck({ operation_id: operation, state }), {
      operation_id: operation,
      state,
    });
  }
  for (const v of [
    {},
    null,
    { operation_id: operation, state: "SENT" },
    { operation_id: "invalid", state: "READY" },
    { operation_id: operation },
  ])
    assert.throws(() => parseAudienceAck(v));
});

test("R11 durable receipt journal validates phase against exact operation state", () => {
  const phases = {
    READY: "queued",
    DISPATCHING: "inflight",
    ACKNOWLEDGED: "inflight",
    SUCCEEDED: "completed",
    UNKNOWN: "unconfirmed",
    FAILED_FINAL: "failed",
    CANCELLED: "failed",
    BLOCKED_POLICY: "failed",
    STALE_BINDING: "failed",
  };
  for (const [state, phase] of Object.entries(phases)) {
    const journal = {
      key,
      boundary: "a".repeat(64),
      phase,
      operation_id: operation,
      state,
    };
    assert.deepEqual(parseAudienceJournal(JSON.stringify(journal)), journal);
    assert.throws(() =>
      parseAudienceJournal(JSON.stringify({ ...journal, phase: "unknown" })),
    );
    assert.throws(() =>
      parseAudienceJournal(JSON.stringify({ ...journal, state: "SENT" })),
    );
  }
  assert.throws(() =>
    parseAudienceJournal(
      JSON.stringify({
        key,
        boundary: "a".repeat(64),
        phase: "queued",
        operation_id: operation,
        state: "SUCCEEDED",
      }),
    ),
  );
});

// MOCK SSR executes the actual read control; effects/transport are not a browser acceptance claim.
const requireApp = createRequire(
  new URL("../../apps/admin/package.json", import.meta.url),
);
const React = requireApp("react"),
  { renderToStaticMarkup } = requireApp("react-dom/server");
let stateCalls = 0,
  receipt: any = null;
const component: Record<string, any> = {};
runInNewContext(
  ts.transpileModule(
    readFileSync(
      new URL(
        "../../apps/admin/components/AttributionAudienceRead.tsx",
        import.meta.url,
      ),
      "utf8",
    ),
    {
      compilerOptions: {
        module: ts.ModuleKind.CommonJS,
        jsx: ts.JsxEmit.ReactJSX,
      },
    },
  ).outputText,
  {
    exports: component,
    require: (name: string) => {
      if (name === "react")
        return {
          ...React,
          useState: (initial: any) => {
            stateCalls++;
            return React.useState(
              stateCalls === 1 ? receipt : stateCalls === 2 ? true : initial,
            );
          },
        };
      if (name === "react/jsx-runtime") return requireApp(name);
      if (name === "@/lib/attribution-audience") return { audienceStorageKey };
      return {};
    },
  },
);
const receiptLabels = {
  en: {
    completed:
      "Reusing a completed audience read; no new read was queued. Read the report again to view the saved insights.",
    inflight:
      "The existing audience read is in progress. Read the report again later.",
    unconfirmed:
      "The existing audience read outcome is unknown. You can request again; the server reuses this read until its 10-minute cooldown ends.",
  },
  "zh-TW": {
    completed:
      "沿用已完成的觀眾讀取，未新增讀取請求。請重新讀取報表查看已儲存的洞察。",
    inflight: "既有觀眾讀取正在處理，請稍後重新讀取報表。",
    unconfirmed:
      "既有觀眾讀取結果未知，可再次要求讀取；10 分鐘冷卻期結束前，伺服器會沿用這次讀取。",
  },
  "zh-CN": {
    completed:
      "沿用已完成的观众读取，未新增读取请求。请重新读取报表查看已保存的洞察。",
    inflight: "现有观众读取正在处理，请稍后重新读取报表。",
    unconfirmed:
      "现有观众读取结果未知，可再次请求读取；10 分钟冷却期结束前，服务器会沿用这次读取。",
  },
};
for (const locale of ["en", "zh-TW", "zh-CN"] as const) {
  test(`R11 actual read control distinguishes replay/inflight/unknown/failure in ${locale}`, () => {
    const c = attributionCopy[locale];
    for (const [state, phase] of [
      ["SUCCEEDED", "completed"],
      ["DISPATCHING", "inflight"],
      ["ACKNOWLEDGED", "inflight"],
      ["UNKNOWN", "unconfirmed"],
      ["FAILED_FINAL", "failed"],
      ["CANCELLED", "failed"],
      ["BLOCKED_POLICY", "failed"],
      ["STALE_BINDING", "failed"],
    ]) {
      receipt = {
        key,
        boundary: "a".repeat(64),
        phase,
        operation_id: operation,
        state,
      };
      stateCalls = 0;
      const html = renderToStaticMarkup(
        React.createElement(component.AttributionAudienceRead, {
          store: draftID,
          session: sessionID,
          c,
        }),
      );
      assert.ok(
        html.includes(`data-testid="attribution-audience-${phase}"`),
        `${state} should render ${phase}`,
      );
      assert.ok(!html.includes('data-testid="attribution-audience-queued"'));
      assert.ok(!html.includes('data-testid="attribution-audience-unknown"'));
      if (phase !== "failed")
        assert.ok(
          html.includes(
            receiptLabels[locale][phase as keyof typeof receiptLabels.en],
          ),
        );
      if (phase === "unconfirmed") {
        // R12: authoritative UNKNOWN is a GET receipt, not a permanent write fence.
        assert.doesNotMatch(
          html,
          /data-testid="attribution-audience-refresh" disabled=""/,
        );
        assert.ok(!html.includes('data-testid="attribution-audience-retry"'));
        assert.equal(Object.hasOwn(c, "audienceCheck"), false);
      }
    }
  });
}
test("unknown journal retains exact key/boundary and is scoped to store+session; malformed state fails closed", () => {
  const journal = {
    key,
    boundary: "a".repeat(64),
    phase: "unknown",
    operation_id: null,
  };
  assert.deepEqual(parseAudienceJournal(JSON.stringify(journal)), journal);
  assert.notEqual(
    audienceStorageKey(draftID, sessionID),
    audienceStorageKey(sessionID, draftID),
  );
  for (const changed of [
    { ...journal, key: "fresh-key" },
    { ...journal, boundary: "raw-cookie" },
    { ...journal, phase: "completed" },
    { ...journal, phase: "queued" },
    { ...journal, operation_id: operation },
  ])
    assert.throws(() => parseAudienceJournal(JSON.stringify(changed)));
});

test("R12 read hook keeps network UNKNOWN on one key and explicitly refreshes acknowledged UNKNOWN", async () => {
  const stored = new Map<string, string>(),
    requests: string[] = [];
  let generated = 0,
    result: any = { kind: "unknown" };
  const storageKey = audienceStorageKey(draftID, sessionID);
  const source = ts.transpileModule(
    readFileSync(
      new URL(
        "../../apps/admin/components/AttributionAudienceRead.tsx",
        import.meta.url,
      ),
      "utf8",
    ),
    {
      compilerOptions: {
        module: ts.ModuleKind.CommonJS,
        jsx: ts.JsxEmit.ReactJSX,
      },
    },
  ).outputText;
  const harness = async () => {
    const state: any[] = [],
      refs: any[] = [];
    let stateIndex = 0,
      refIndex = 0,
      effect: (() => void | (() => void)) | undefined;
    const out: Record<string, any> = {};
    runInNewContext(source, {
      exports: out,
      crypto: {
        randomUUID: () =>
          `44444444-4444-4444-8444-${String(++generated).padStart(12, "0")}`,
      },
      sessionStorage: {
        getItem: (name: string) => stored.get(name) ?? null,
        setItem: (name: string, value: string) => stored.set(name, value),
        removeItem: (name: string) => stored.delete(name),
      },
      require: (name: string) => {
        if (name === "react/jsx-runtime") return requireApp(name);
        if (name === "react")
          return {
            useState: (initial: any) => {
              const i = stateIndex++;
              if (!(i in state)) state[i] = initial;
              return [
                state[i],
                (value: any) => {
                  state[i] = value;
                },
              ];
            },
            useRef: (initial: any) => {
              const i = refIndex++;
              return (refs[i] ??= { current: initial });
            },
            useEffect: (callback: () => void | (() => void)) => {
              effect ??= callback;
            },
          };
        if (name === "@/lib/settings-client")
          return { sessionBoundary: async () => "a".repeat(64) };
        if (name === "@/lib/attribution-audience")
          return { audienceStorageKey, parseAudienceJournal, audienceAckPhase };
        if (name === "@/lib/attribution-client")
          return {
            requestAudienceRead: async (
              _store: string,
              _session: string,
              requestKey: string,
            ) => {
              assert.equal(
                JSON.parse(stored.get(storageKey)!).key,
                requestKey,
                "intention must be stored before transport",
              );
              requests.push(requestKey);
              return result;
            },
          };
        return {};
      },
    });
    const render = () => {
      stateIndex = refIndex = 0;
      return out.AttributionAudienceRead({
        store: draftID,
        session: sessionID,
        c: attributionCopy.en,
      });
    };
    const find = (node: any, id: string): any => {
      if (!node || typeof node !== "object") return undefined;
      if (Array.isArray(node))
        return node.map((child) => find(child, id)).find(Boolean);
      return node.props?.["data-testid"] === id
        ? node
        : find(node.props?.children, id);
    };
    render();
    const cleanup = effect?.();
    await new Promise(setImmediate);
    return {
      control: (id: string) => find(render(), id),
      cleanup,
      click: async (id: string) => {
        const button = find(render(), id);
        assert.ok(button);
        assert.equal(button.props.disabled, false);
        button.props.onClick();
        await new Promise(setImmediate);
      },
    };
  };
  const first = await harness();
  await first.click("attribution-audience-refresh");
  const initial = parseAudienceJournal(stored.get(storageKey)!);
  assert.equal(initial.phase, "unknown");
  assert.equal(
    first.control("attribution-audience-refresh").props.disabled,
    true,
  );
  first.cleanup?.();
  const reloaded = await harness();
  assert.ok(reloaded.control("attribution-audience-unknown"));
  result = {
    kind: "acknowledged",
    ack: { operation_id: operation, state: "SUCCEEDED" },
  };
  await reloaded.click("attribution-audience-retry");
  assert.deepEqual(requests, [initial.key, initial.key]);
  assert.equal(generated, 1);
  assert.equal(
    parseAudienceJournal(stored.get(storageKey)!).phase,
    "completed",
  );
  assert.ok(reloaded.control("attribution-audience-completed"));
  result = {
    kind: "acknowledged",
    ack: { operation_id: operation, state: "UNKNOWN" },
  };
  await reloaded.click("attribution-audience-refresh");
  const unconfirmed = parseAudienceJournal(stored.get(storageKey)!);
  assert.equal(unconfirmed.phase, "unconfirmed");
  assert.equal(unconfirmed.operation_id, operation);
  assert.equal(
    reloaded.control("attribution-audience-refresh").props.disabled,
    false,
  );
  assert.equal(reloaded.control("attribution-audience-retry"), undefined);
  assert.deepEqual(requests.slice(2), [unconfirmed.key]);
  assert.equal(generated, 2);
  const savedReceipt = stored.get(storageKey)!;
  reloaded.cleanup?.();
  const unconfirmedReload = await harness();
  assert.ok(unconfirmedReload.control("attribution-audience-unconfirmed"));
  assert.equal(
    unconfirmedReload.control("attribution-audience-refresh").props.disabled,
    false,
  );
  assert.equal(
    unconfirmedReload.control("attribution-audience-retry"),
    undefined,
  );
  assert.equal(stored.get(storageKey), savedReceipt);
  assert.deepEqual(parseAudienceJournal(stored.get(storageKey)!), unconfirmed);
  assert.equal(
    requests.length,
    3,
    "restoring the receipt must not send a POST",
  );
  assert.equal(generated, 2);
  await unconfirmedReload.click("attribution-audience-refresh");
  const refreshed = parseAudienceJournal(stored.get(storageKey)!);
  assert.notEqual(refreshed.key, unconfirmed.key);
  assert.equal(refreshed.operation_id, operation);
  assert.equal(refreshed.state, "UNKNOWN"); // Backend cooldown still replays the same operation.
  assert.equal(requests.length, 4);
  assert.equal(generated, 3);
  unconfirmedReload.cleanup?.();
});
test("actual POST transport uses CSRF and session fence; 403 independent of response text", async (t) => {
  const originalDocument = Object.getOwnPropertyDescriptor(
      globalThis,
      "document",
    ),
    originalFetch = globalThis.fetch;
  const fakeDocument = { cookie: `__Host-commerce_csrf=${"A".repeat(43)}` };
  Object.defineProperty(globalThis, "document", {
    value: fakeDocument,
    configurable: true,
  });
  t.after(() => {
    globalThis.fetch = originalFetch;
    if (originalDocument)
      Object.defineProperty(globalThis, "document", originalDocument);
    else Reflect.deleteProperty(globalThis, "document");
  });
  const boundary = await sessionBoundary(),
    sent: {
      url: string;
      key: string | null;
      body: unknown;
      method: string | undefined;
    }[] = [];
  let status = 202,
    value: unknown = { operation_id: operation, state: "READY" };
  globalThis.fetch = async (input, init) => {
    const headers = new Headers(init?.headers);
    sent.push({
      url: String(input),
      key: headers.get("Idempotency-Key"),
      body: init?.body,
      method: init?.method,
    });
    assert.equal(headers.get("X-CSRF-Token"), "A".repeat(43));
    return new Response(JSON.stringify(value), {
      status,
      headers: { "Content-Type": "application/json" },
    });
  };
  assert.equal(
    (await requestAudienceRead(draftID, sessionID, key, boundary)).kind,
    "acknowledged",
  );
  for (const state of ["SUCCEEDED", "DISPATCHING", "UNKNOWN", "FAILED_FINAL"]) {
    value = { operation_id: operation, state };
    assert.deepEqual(
      await requestAudienceRead(draftID, sessionID, key, boundary),
      { kind: "acknowledged", ack: value },
    );
  }
  sent.splice(1); // Keep the existing network/CSRF counterexample indices independent of the added receipt cases.
  assert.deepEqual(sent[0], {
    url: `/api/stores/${draftID}/ads/sessions/${sessionID}/audience-read`,
    key,
    body: "{}",
    method: "POST",
  });
  status = 403;
  value = { message: "untrusted external prose" };
  assert.equal(
    (await requestAudienceRead(draftID, sessionID, key, boundary)).kind,
    "forbidden",
  );
  status = 503;
  assert.equal(
    (await requestAudienceRead(draftID, sessionID, key, boundary)).kind,
    "unknown",
  );
  assert.equal(
    (await requestAudienceRead(draftID, sessionID, key, boundary)).kind,
    "unknown",
  );
  assert.equal(sent[2].key, sent[3].key);
  status = 202;
  value = { operation_id: operation, state: "COMPLETED" };
  assert.equal(
    (await requestAudienceRead(draftID, sessionID, key, boundary)).kind,
    "unknown",
  );
  const before = sent.length;
  fakeDocument.cookie = `__Host-commerce_csrf=${"B".repeat(43)}`;
  assert.equal(
    (await requestAudienceRead(draftID, sessionID, key, boundary)).kind,
    "signed-out",
  );
  assert.equal(sent.length, before);
});
