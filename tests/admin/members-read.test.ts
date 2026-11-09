// Purpose: actual-source MOCK member transport, lifecycle and component counterexamples.
// Depends on: existing inbox hook host, native Node assertions; DS presentation/native navigation are adapters.
// Used by: test-node; does not replace REAL_PG/browser clicks or independent acceptance.
import test from "node:test";
import assert from "node:assert/strict";
import { registerHooks } from "node:module";
import { readFileSync } from "node:fs";
import { environment, nodes, node, textOf } from "./inbox-review-host.test.ts";
const mock = (s: string) => `data:text/javascript,${encodeURIComponent(s)}`;
registerHooks({
  resolve(specifier, context, next) {
    if (specifier === "@live-commerce/format")
      return {
        shortCircuit: true,
        url: new URL("../../packages/format/src/index.ts", import.meta.url)
          .href,
      };
    if (specifier === "@live-commerce/ui/ds")
      return {
        shortCircuit: true,
        url: mock(`import {jsx} from 'react/jsx-runtime';
    export const Button=p=>jsx('button',{...p,disabled:!!p.disabledReason});
    export const DataTable=p=>jsx('table',{children:p.rows.length?p.rows.map(row=>jsx('tr',{children:p.columns.map(c=>jsx('td',{children:c.cell(row)}))})):p.empty});
    export const Card=p=>jsx('section',p), Badge=p=>jsx('span',p), EmptyState=p=>jsx('div',{children:p.title}), PageHeader=p=>jsx('h1',{children:p.title}), PermissionState=p=>jsx('div',{children:p.title});
    export const ErrorState=p=>jsx('div',{children:[p.title,p.onRetry&&jsx('button',{onClick:p.onRetry,children:p.retryLabel}),p.actions]});`),
      };
    if (specifier === "next/link")
      return {
        shortCircuit: true,
        url: mock(
          "import {jsx} from 'react/jsx-runtime'; export default p=>jsx('a',p);",
        ),
      };
    if (/WorkspaceFrame$/.test(specifier))
      return {
        shortCircuit: true,
        url: mock("export const WorkspaceFrame=({children})=>children;"),
      };
    if (/AdminLoading$/.test(specifier))
      return {
        shortCircuit: true,
        url: mock("export default ()=> 'loading';"),
      };
    return next(specifier, context);
  },
});
const {
  searchMembers,
  readMember,
  parseMembers,
  parseMember,
  memberQuery,
  useMemberPages,
  appendMembers,
  appendEntries,
  MemberReadError,
} = await import("../../apps/admin/lib/members-client.ts");
const { MemberList } =
  await import("../../apps/admin/components/members/MemberList.tsx");
const { MemberDetail } =
  await import("../../apps/admin/components/members/MemberDetail.tsx");
const { membersCopy } =
  await import("../../apps/admin/components/members/copy.ts");
const sid = "11111111-1111-4111-8111-111111111111",
  mid = "22222222-2222-4222-8222-222222222222",
  eid = "33333333-3333-4333-8333-333333333333";
const stamp = "2026-11-09T00:00:00Z";
const memberRow = (id = mid) => ({
  id,
  display_name: "SYNTHETIC_MEMBER",
  contact_note: null,
  balance: 80,
  updated_at: stamp,
  erased: false,
});
const list = (after: string | null = null, id = mid) => ({
  members: [memberRow(id)],
  next_cursor: after,
});
const detail = (after: string | null = null) => ({
  member: {
    ...memberRow(),
    notes: null,
    version: 1,
    created_at: stamp,
    erased_at: null,
  },
  entries: [
    {
      id: eid,
      delta: 2,
      reason: "manual_adjust",
      order_id: null,
      note: null,
      actor: { principal_id: sid, display_name: null },
      created_at: stamp,
    },
  ],
  next_ledger_cursor: after,
});
const response = (data: unknown, status = 200, headers = {}) =>
  new Response(JSON.stringify(data), {
    status,
    headers: {
      "Content-Type": "application/json",
      "Cache-Control": "private, no-store",
      ...headers,
    },
  });
const store = {
  id: sid,
  name: "Synthetic store",
  permissions: ["customers:read"],
  role: "staff",
} as any;
const props = {
  locale: "en" as const,
  stores: [store],
  store,
  renderKey: "synthetic-render",
  initialError: null,
};
const defer = () => {
  let resolve!: (v: Response) => void;
  return {
    promise: new Promise<Response>((r) => {
      resolve = r;
    }),
    resolve,
  };
};

test("exact POST JSON null/limit/CSRF with no PII or cursor URL and no idempotency key", async (t) => {
  environment(t);
  let request: any;
  globalThis.fetch = async (url, options) => {
    request = { url, options };
    return response(list());
  };
  await searchMembers(sid, "  ", null, new AbortController().signal);
  assert.equal(request.url, `/api/stores/${sid}/members/search`);
  assert.deepEqual(JSON.parse(request.options.body), {
    q: null,
    include_erased: false,
    cursor: null,
    limit: 20,
  });
  assert.equal(request.options.headers["X-CSRF-Token"], "a".repeat(43));
  assert.equal(request.options.headers["Idempotency-Key"], undefined);
  assert.equal(request.options.cache, "no-store");
  assert.equal(request.options.credentials, "same-origin");
  await searchMembers(
    sid,
    " PRIVATE_QUERY ",
    "a+/=",
    new AbortController().signal,
  );
  assert.equal(request.url.includes("PRIVATE_QUERY"), false);
  assert.equal(request.url.includes("a+/="), false);
  assert.deepEqual(JSON.parse(request.options.body), {
    q: "PRIVATE_QUERY",
    include_erased: false,
    cursor: "a+/=",
    limit: 20,
  });
});
test("opaque detail cursor is encoded without URL-safe customer regexp; exact detail DTO", async (t) => {
  environment(t);
  let url = "";
  globalThis.fetch = async (input) => {
    url = String(input);
    return response(detail());
  };
  const result = await readMember(
    sid,
    mid,
    "a+/=",
    new AbortController().signal,
  );
  assert.equal(
    url,
    `/api/stores/${sid}/members/${mid}?ledger_cursor=a%2B%2F%3D`,
  );
  assert.equal(result.member.balance, 80);
  assert.equal(result.entries[0].actor.display_name, null);
  assert.equal(result.entries[0].delta, 2);
  assert.equal(result.member.contact_note, null);
});
test("missing CSRF sends nothing and changed cookie discards late success", async (t) => {
  const env = environment(t);
  let calls = 0;
  const pending = defer();
  env.document.cookie = "";
  globalThis.fetch = async () => {
    calls++;
    return pending.promise;
  };
  await assert.rejects(
    searchMembers(sid, "", null, new AbortController().signal),
    /signed-out/,
  );
  assert.equal(calls, 0);
  env.document.cookie = `__Host-commerce_csrf=${"a".repeat(43)}`;
  const read = searchMembers(sid, "", null, new AbortController().signal);
  await new Promise((r) => setTimeout(r, 15));
  assert.equal(calls, 1);
  env.document.cookie = `__Host-commerce_csrf=${"b".repeat(43)}`;
  pending.resolve(response(list()));
  await assert.rejects(read, /signed-out/);
});
test("safe classifications and strict malformed/null/bounds/identity/oversize checks", async (t) => {
  environment(t);
  for (const [status, code] of [
    [401, "signed-out"],
    [403, "forbidden"],
    [404, "not-found"],
    [422, "unavailable"],
    [503, "unavailable"],
  ] as const) {
    globalThis.fetch = async () =>
      response({ secret: "RAW_PRIVATE_ERROR" }, status);
    await assert.rejects(
      searchMembers(sid, "", null, new AbortController().signal),
      (e: any) => e.code === code && !String(e).includes("RAW_PRIVATE_ERROR"),
    );
  }
  for (const value of [
    {},
    { ...list(), next_cursor: undefined },
    { ...list(), members: [{ ...memberRow(), balance: -1 }] },
    { ...list(), members: [memberRow(), memberRow()] },
  ])
    assert.throws(() => parseMembers(value), MemberReadError);
  assert.throws(() => parseMember(detail(), sid));
  assert.throws(() => memberQuery("x".repeat(61)));
  assert.throws(() => memberQuery("x\ny"));
  globalThis.fetch = async () =>
    response(list(), 200, { "Cache-Control": "no-store" });
  await assert.rejects(
    searchMembers(sid, "", null, new AbortController().signal),
    MemberReadError,
  );
  globalThis.fetch = async () => response({ padding: "x".repeat(1048576) });
  await assert.rejects(
    searchMembers(sid, "", null, new AbortController().signal),
    MemberReadError,
  );
  globalThis.fetch = async () => {
    throw new Error("RAW_PRIVATE_ERROR");
  };
  await assert.rejects(
    searchMembers(sid, "", null, new AbortController().signal),
    (e: any) => e.kind === "network" && e.message === "unavailable",
  );
});
test("actual pagination keeps rows on failure, explicit retry and duplicate suppression", async (t) => {
  const env = environment(t);
  let fail = true,
    calls = 0;
  globalThis.fetch = async (_url, options) => {
    const body = JSON.parse(String(options?.body));
    calls++;
    if (!body.cursor) return response(list("a+/="));
    if (fail) return response({}, 503);
    return response({
      members: [memberRow(), memberRow(eid)],
      next_cursor: null,
    });
  };
  const h = env.mount(() =>
    useMemberPages(
      "scope",
      (after, signal) => searchMembers(sid, "", after, signal),
      null,
      (p) => p.next_cursor,
      appendMembers,
    ),
  );
  await h.settle();
  assert.equal(h.output.data.members.length, 1);
  await h.output.more();
  await h.settle();
  assert.equal(h.output.data.members.length, 1);
  assert.equal(h.output.pageError, "network");
  fail = false;
  await h.output.more();
  await h.settle();
  assert.equal(h.output.data.members.length, 2);
  assert.equal(h.output.data.next_cursor, null);
  await h.output.more();
  assert.equal(calls, 3);
});
test("actual search resets cursor; stale old query/store results cannot append", async (t) => {
  const env = environment(t);
  const pending = defer();
  let q = "old",
    scope = "old",
    shop = sid;
  const requests: any[] = [];
  globalThis.fetch = async (url, options) => {
    const body = JSON.parse(String(options?.body));
    requests.push({ url, ...body });
    if (body.cursor) return pending.promise;
    return response(
      list(
        body.q === "old" ? "cursor-old" : null,
        body.q === "old" ? mid : eid,
      ),
    );
  };
  const h = env.mount(() =>
    useMemberPages(
      scope,
      (after, signal) => searchMembers(shop, q, after, signal),
      null,
      (p) => p.next_cursor,
      appendMembers,
    ),
  );
  await h.settle();
  const paging = h.output.more();
  await h.settle();
  q = "new";
  scope = "new";
  shop = eid;
  h.dirty = true;
  await h.settle();
  pending.resolve(response(list(null, sid)));
  await paging;
  await h.settle();
  assert.equal(requests.at(-1).cursor, null);
  assert.equal(requests.at(-1).q, "new");
  assert.deepEqual(
    h.output.data.members.map((m: any) => m.id),
    [eid],
  );
});
test("logout clears actual hook data and a late successful page cannot resurrect it", async (t) => {
  const env = environment(t);
  const pending = defer();
  globalThis.fetch = async (_url, options) =>
    JSON.parse(String(options?.body)).cursor
      ? pending.promise
      : response(list("next"));
  const h = env.mount(() =>
    useMemberPages(
      "scope",
      (after, signal) => searchMembers(sid, "", after, signal),
      null,
      (p) => p.next_cursor,
      appendMembers,
    ),
  );
  await h.settle();
  const paging = h.output.more();
  await h.settle();
  env.window.dispatchEvent(new Event("commerce-session-logout"));
  await h.settle();
  assert.equal(h.output.data, null);
  assert.equal(h.output.status, "signed-out");
  pending.resolve(response(list(null, eid)));
  await paging;
  await h.settle();
  assert.equal(h.output.data, null);
});
test("actual list search component cursor resets after loaded pages with private URL-free query", async (t) => {
  const env = environment(t);
  const bodies: any[] = [];
  globalThis.fetch = async (_url, options) => {
    const b = JSON.parse(String(options?.body));
    bodies.push(b);
    return response(list(b.q === null && b.cursor === null ? "next" : null));
  };
  const h = env.mount(() => MemberList(props));
  await h.settle();
  node(
    h,
    (n) => n.props["data-testid"] === "members-load-more",
  ).props.onClick();
  await h.settle();
  assert.equal(bodies.at(-1).cursor, "next");
  node(
    h,
    (n) => n.props["data-testid"] === "members-search-input",
  ).props.onChange({ target: { value: "PRIVATE_SEARCH" } });
  await h.settle();
  node(h, (n) => n.props["data-testid"] === "members-search").props.onSubmit({
    preventDefault() {},
  });
  await h.settle();
  assert.equal(bodies.at(-1).q, "PRIVATE_SEARCH");
  assert.equal(bodies.at(-1).cursor, null);
  for (const n of nodes(h.output))
    if (n.props.href)
      assert.equal(/PRIVATE_SEARCH|next/.test(n.props.href), false);
});
for (const locale of ["en", "zh-TW", "zh-CN"] as const)
  test(`actual detail ${locale} authoritative balance, null actor and native links`, async (t) => {
    const env = environment(t);
    globalThis.fetch = async () => response(detail());
    const h = env.mount(() => MemberDetail({ ...props, locale, member: mid }));
    await h.settle();
    const text = textOf(h.output);
    assert.ok(text.includes("80"));
    assert.ok(text.includes(`${membersCopy[locale].staffID}: ${sid}`));
    assert.equal(
      node(h, (n) => n.props["data-testid"] === "member-back").props.href,
      `/${locale}/members?store=${sid}`,
    );
    assert.equal(/Amy|小安/.test(text), false);
    assert.equal(
      appendEntries(parseMember(detail(), mid), parseMember(detail(), mid))
        .entries.length,
      1,
    );
  });
test("formal consumed token bindings and old token values unchanged", () => {
  const css = readFileSync("packages/ui/src/tokens.css", "utf8"),
    business = readFileSync(
      "apps/admin/components/members/members.module.css",
      "utf8",
    );
  for (const [key, value] of Object.entries({
    page: "#f5f7fa",
    ink: "#142942",
    muted: "#596579",
    border: "#dfe5ec",
    surface: "var(--ds-surface)",
    link: "var(--ui-primary)",
  })) {
    assert.ok(css.includes(`--ds-members-${key}: ${value};`));
    assert.ok(business.includes(`var(--ds-members-${key})`));
  }
  assert.equal(/#[\da-f]{3,8}\b|rgba?\(/i.test(business), false);
});
test("actual initial query response arriving after query change is discarded", async (t) => {
  const env = environment(t),
    pending = defer();
  let q = "old";
  globalThis.fetch = async (_url, options) =>
    JSON.parse(String(options?.body)).q === "old"
      ? pending.promise
      : response(list(null, eid));
  const h = env.mount(() =>
    useMemberPages(
      q,
      (after, signal) => searchMembers(sid, q, after, signal),
      null,
      (p) => p.next_cursor,
      appendMembers,
    ),
  );
  await h.settle();
  q = "new";
  h.dirty = true;
  await h.settle();
  pending.resolve(response(list()));
  await h.settle();
  assert.deepEqual(
    h.output.data.members.map((m: any) => m.id),
    [eid],
  );
});
test("actual ledger failed next page retains authoritative balance and explicit retry", async (t) => {
  const env = environment(t);
  let fail = true;
  globalThis.fetch = async (url) =>
    String(url).includes("ledger_cursor")
      ? fail
        ? response({}, 503)
        : response({
            ...detail(),
            entries: [{ ...detail().entries[0], id: sid }],
          })
      : response(detail("cursor+/="));
  const h = env.mount(() => MemberDetail({ ...props, member: mid }));
  await h.settle();
  node(
    h,
    (n) => n.props["data-testid"] === "member-ledger-load-more",
  ).props.onClick();
  await h.settle();
  assert.ok(textOf(h.output).includes("80"));
  assert.ok(textOf(h.output).includes(membersCopy.en.network));
  assert.equal(nodes(h.output).filter((n) => n.type === "time").length, 1);
  fail = false;
  node(
    h,
    (n) => n.props["data-testid"] === "member-ledger-load-more",
  ).props.onClick();
  await h.settle();
  assert.equal(nodes(h.output).filter((n) => n.type === "time").length, 2);
  assert.equal(
    nodes(h.output).some(
      (n) => n.props["data-testid"] === "member-ledger-load-more",
    ),
    false,
  );
});
test("actual empty/query-empty, denial, not-found, protocol and signed-out component states", async (t) => {
  const env = environment(t);
  let status = 200;
  globalThis.fetch = async () =>
    status === 200
      ? response({ members: [], next_cursor: null })
      : status === 201
        ? response({ wrong: true })
        : response({}, status);
  const h = env.mount(() => MemberList(props));
  await h.settle();
  assert.ok(textOf(h.output).includes(membersCopy.en.empty));
  node(
    h,
    (n) => n.props["data-testid"] === "members-search-input",
  ).props.onChange({ target: { value: "nobody" } });
  await h.settle();
  node(h, (n) => n.props["data-testid"] === "members-search").props.onSubmit({
    preventDefault() {},
  });
  await h.settle();
  assert.ok(textOf(h.output).includes(membersCopy.en.queryEmpty));
  for (const [code, label] of [
    [403, "forbidden"],
    [404, "notFound"],
    [201, "protocol"],
    [401, "signedOut"],
  ] as const) {
    status = code;
    node(h, (n) => n.props["data-testid"] === "members-search").props.onSubmit({
      preventDefault() {},
    });
    await h.settle();
    assert.ok(
      textOf(h.output).includes(membersCopy.en[label]),
      `${code} visible state`,
    );
    assert.equal(textOf(h.output).includes("SYNTHETIC_MEMBER"), false);
  }
  assert.equal(
    nodes(h.output).some(
      (n) => n.props["data-testid"] === "members-search-input",
    ),
    false,
  );
});
