// Purpose: focused W6-01B tag/note DTO and transport grammar tests.
// Depends on: customer-tags model, request grammar and copy; frozen Go contracts.
// Used by: W6-U1 local Node verification and independent integration review.
// W6-U1 Node half (customers-billing-v1 Amendment W6-01B): the admin BFF fence and decoders for the nine tag/note
// resources plus the tag-filtered customer list must admit exactly the amendment's grammar (Idempotency-Key on every
// write including both DELETEs, no body on the DELETEs, closed JSON bodies), and decode exactly what
// internal/customers emits (TagRecord/TagSet/DeletedTag/Note/DeletedNote, pagination.Page[Note]). Only the public pure
// functions are imported; the route.ts wiring itself stays a Playwright BFF concern (customer-tags.spec.ts).
// Run: node --test --experimental-strip-types tests/admin/customer-tags-bff.test.ts
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { test } from "node:test";
import {
  customerTagsRoute,
  validCustomerTagsBody,
  validCustomerTagsRequest,
  type CustomerTagsRouteKind,
} from "../../apps/admin/lib/customer-tags-request.ts";
import {
  parseDeletedNote,
  parseDeletedTag,
  parseNotePage,
  parseNoteRecord,
  parseTagCatalog,
  parseTagSet,
  validNoteBody,
  validTagName,
} from "../../apps/admin/lib/customer-tags-model.ts";
import { customerTagsCopy } from "../../apps/admin/lib/customer-tags-copy.ts";
import { tagColors } from "../../apps/admin/lib/customers-model.ts";

const id = "abcdef11-1111-4111-8111-111111111111";
const other = "22222222-2222-4222-8222-222222222222";
const revision = "a".repeat(64);
const origin = "http://127.0.0.1:3100";

// ---------------------------------------------------------------------------------------------- grammar
const table: [string, string, CustomerTagsRouteKind][] = [
  ["GET", "customers/tags", "tag-list"],
  ["POST", "customers/tags", "tag-create"],
  ["PATCH", `customers/tags/${id}`, "tag-patch"],
  ["DELETE", `customers/tags/${id}`, "tag-delete"],
  ["PUT", `customers/${id}/tags`, "owner-tags"],
  ["GET", `customers/${id}/notes`, "note-list"],
  ["POST", `customers/${id}/notes`, "note-create"],
  ["PATCH", `customers/${id}/notes/${other}`, "note-patch"],
  ["DELETE", `customers/${id}/notes/${other}`, "note-delete"],
];

test("exactly the nine W6-01B resources are admitted, each only under its own method", () => {
  for (const [method, path, kind] of table) {
    assert.equal(customerTagsRoute(method, path), kind, `${method} ${path}`);
    for (const wrong of ["GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"].filter((m) => m !== method))
      assert.equal(customerTagsRoute(wrong, path), table.find(([m, p]) => m === wrong && p === path)?.[2] ?? null, `${wrong} ${path} must match its own declared route`);
  }
});

test("the customer list is claimed only when a tag filter is present (customers-request.ts stays the owner otherwise)", () => {
  assert.equal(customerTagsRoute("GET", "customers", `?tag=${id}`), "customer-list");
  assert.equal(customerTagsRoute("GET", "customers", `?limit=25&tag=${id}&q=%E5%BC%B5`), "customer-list");
  assert.equal(customerTagsRoute("GET", "customers", ""), null);
  assert.equal(customerTagsRoute("GET", "customers", "?limit=25"), null);
  assert.equal(customerTagsRoute("GET", "customers"), null);
  assert.equal(customerTagsRoute("POST", "customers", `?tag=${id}`), null);
});

test("near misses are refused (no generic proxying)", () => {
  const nope: [string, string][] = [
    ["GET", "customers/tags/"], ["GET", "customers/tags/x"], ["GET", `customers/tags/${id}/x`],
    ["PATCH", "customers/tags"], ["DELETE", "customers/tags"], ["PUT", `customers/${id}/tags/${id}`],
    ["GET", `customers/not-a-uuid/notes`], ["POST", `customers/${id}/note`], ["PATCH", `customers/${id}/notes/${id}/x`],
    ["DELETE", `customers/${id}/notes`], ["GET", "tags"], ["GET", `customers/${id.toUpperCase()}/notes`],
  ];
  for (const [method, path] of nope) assert.equal(customerTagsRoute(method, path), null, `${method} ${path}`);
});

const req = (method: string, path: string, init: RequestInit = {}) =>
  new Request(`${origin}/api/stores/${id}/${path}`, { method, ...init });
const json = (headers: Record<string, string> = {}) => ({ "Content-Type": "application/json", ...headers });

test("reads: no body, no key, no transfer-encoding; tag-list takes no query at all", () => {
  assert.equal(validCustomerTagsRequest("tag-list", req("GET", "customers/tags")), true);
  assert.equal(validCustomerTagsRequest("tag-list", req("GET", "customers/tags?limit=5")), false);
  assert.equal(validCustomerTagsRequest("tag-list", req("GET", "customers/tags", { headers: { "Idempotency-Key": "tag-list-12345678" } })), false);
  assert.equal(validCustomerTagsRequest("tag-list", req("GET", "customers/tags", { headers: { "transfer-encoding": "chunked" } })), false);
  assert.equal(validCustomerTagsRequest("customer-list", req("GET", `customers?tag=${id}`)), true);
  assert.equal(validCustomerTagsRequest("customer-list", req("GET", `customers?tag=${id}`, { headers: { "Idempotency-Key": "customer-list-123" } })), false);
  assert.equal(validCustomerTagsRequest("note-list", req("GET", `customers/${id}/notes?limit=50`)), true);
  assert.equal(validCustomerTagsRequest("note-list", req("GET", `customers/${id}/notes?limit=50&after=abc`)), true);
});

test("customer-list query: closed keys, tag must be a canonical uuid, list bounds identical to customers-request", () => {
  const ok = [
    `?tag=${id}`, `?limit=25&tag=${id}`, `?limit=100&after=Abc-_1&tag=${id}`, `?q=${encodeURIComponent("張小美")}&tag=${id}`,
  ];
  for (const search of ok) assert.equal(validCustomerTagsRequest("customer-list", req("GET", `customers${search}`)), true, search);
  const bad = [
    `?tag=not-a-uuid`, `?tag=${id.toUpperCase()}`, `?tag=`, `?tag=${id}&tag=${id}`, `?tag=${id}&from=2026-01-01`,
    `?tag=${id}&limit=0`, `?tag=${id}&limit=101`, `?tag=${id}&after=a=b`, `?tag=${id}&cursor=x`, `?tag=${id}&q=`,
    `?tag=${id}&q=${encodeURIComponent("x".repeat(41))}`, `?tag=${id}&q=%`, `?tag=${id}#f`,
  ];
  for (const search of bad) assert.equal(validCustomerTagsRequest("customer-list", req("GET", `customers${search}`)), false, search);
});

test("note-list query: only limit and after, with the customers-list bounds", () => {
  assert.equal(validCustomerTagsRequest("note-list", req("GET", `customers/${id}/notes`)), true);
  assert.equal(validCustomerTagsRequest("note-list", req("GET", `customers/${id}/notes?limit=100`)), true);
  for (const search of ["?limit=0", "?limit=101", "?q=x", `?tag=${id}`, "?after=a=b", "?limit=5&limit=6", "?x=1"])
    assert.equal(validCustomerTagsRequest("note-list", req("GET", `customers/${id}/notes${search}`)), false, search);
});

test("writes: exactly one well-formed Idempotency-Key; the two DELETEs carry no body but still need the key", () => {
  const key = "tag-create-0c6f1f2e-6d1c-4b5e-9a2a-2f6f0e5c9d40";
  assert.equal(validCustomerTagsRequest("tag-create", req("POST", "customers/tags", { headers: json({ "Idempotency-Key": key }), body: "{}" })), true);
  assert.equal(validCustomerTagsRequest("tag-create", req("POST", "customers/tags", { headers: json(), body: "{}" })), false);
  assert.equal(validCustomerTagsRequest("tag-create", req("POST", "customers/tags", { headers: json({ "Idempotency-Key": "short" }), body: "{}" })), false);
  assert.equal(validCustomerTagsRequest("tag-create", req("POST", "customers/tags", { headers: { "Idempotency-Key": key }, body: "{}" })), false); // no json content-type
  assert.equal(validCustomerTagsRequest("tag-patch", req("PATCH", `customers/tags/${id}`, { headers: json({ "Idempotency-Key": key }), body: "{}" })), true);
  assert.equal(validCustomerTagsRequest("owner-tags", req("PUT", `customers/${id}/tags`, { headers: json({ "Idempotency-Key": key }), body: "{}" })), true);
  assert.equal(validCustomerTagsRequest("note-create", req("POST", `customers/${id}/notes`, { headers: json({ "Idempotency-Key": key }), body: "{}" })), true);
  assert.equal(validCustomerTagsRequest("note-patch", req("PATCH", `customers/${id}/notes/${other}`, { headers: json({ "Idempotency-Key": key }), body: "{}" })), true);
  // DELETE: key required (Go customerRoute), body must be empty-declared, no query.
  assert.equal(validCustomerTagsRequest("tag-delete", req("DELETE", `customers/tags/${id}`, { headers: json({ "Idempotency-Key": key }) })), true);
  assert.equal(validCustomerTagsRequest("tag-delete", req("DELETE", `customers/tags/${id}`, { headers: json() })), false);
  assert.equal(validCustomerTagsRequest("tag-delete", req("DELETE", `customers/tags/${id}`, { headers: json({ "Idempotency-Key": key, "content-length": "2" }), body: "{}" })), false);
  assert.equal(validCustomerTagsRequest("tag-delete", req("DELETE", `customers/tags/${id}?x=1`, { headers: json({ "Idempotency-Key": key }) })), false);
  assert.equal(validCustomerTagsRequest("note-delete", req("DELETE", `customers/${id}/notes/${other}`, { headers: json({ "Idempotency-Key": key }) })), true);
  assert.equal(validCustomerTagsRequest("note-delete", req("DELETE", `customers/${id}/notes/${other}`, { headers: json({ "Idempotency-Key": key, "transfer-encoding": "chunked" }) })), false);
  // No write kind takes a query.
  const writes: [CustomerTagsRouteKind, string, string][] = [
    ["tag-create", "POST", "customers/tags"],
    ["tag-patch", "PATCH", `customers/tags/${id}`],
    ["tag-delete", "DELETE", `customers/tags/${id}`],
    ["owner-tags", "PUT", `customers/${id}/tags`],
    ["note-create", "POST", `customers/${id}/notes`],
    ["note-patch", "PATCH", `customers/${id}/notes/${other}`],
    ["note-delete", "DELETE", `customers/${id}/notes/${other}`],
  ];
  for (const [kind, method, path] of writes)
    assert.equal(validCustomerTagsRequest(kind, req(method, `${path}?x=1`, { headers: json({ "Idempotency-Key": key }), body: "{}" })), false, `${kind} must refuse a query`);
});

// ---------------------------------------------------------------------------------------------- bodies
test("validTagName mirrors Go (NFC first, 1..20, trimmed, no control/Cf)", () => {
  assert.equal(validTagName("VIP"), true);
  assert.equal(validTagName("張小美".repeat(6) + "張小"), true); // 20 runes
  assert.equal(validTagName("x".repeat(21)), false);
  assert.equal(validTagName(""), false);
  assert.equal(validTagName(" x"), false);
  assert.equal(validTagName("x "), false);
  assert.equal(validTagName("a​b"), false); // Cf: zero-width space would let two tags look equal
  assert.equal(validTagName("a\u0000b"), false);
  assert.equal(validTagName("éclair"), true); // decomposed; NFC-composed it is 6 runes
  assert.equal(validTagName("é".repeat(21)), false); // 21 even after NFC
});

test("validNoteBody mirrors Go (1..1000, not blank, only \\n \\r \\t controls)", () => {
  assert.equal(validNoteBody("退貨後請回購"), true);
  assert.equal(validNoteBody("line1\nline2\ttab"), true);
  assert.equal(validNoteBody("   "), false);
  assert.equal(validNoteBody(""), false);
  assert.equal(validNoteBody("x".repeat(1001)), false);
  assert.equal(validNoteBody("x".repeat(1000)), true);
  assert.equal(validNoteBody("a\u0000b"), false);
  assert.equal(validNoteBody("a\u001bb"), false);
});

test("tag-create body: closed keys, valid name and one of the eight colors", () => {
  assert.equal(validCustomerTagsBody("tag-create", JSON.stringify({ name: "VIP", color: "red" })), true);
  assert.equal(validCustomerTagsBody("tag-create", JSON.stringify({ name: "VIP", color: "pink" })), false);
  assert.equal(validCustomerTagsBody("tag-create", JSON.stringify({ name: "", color: "red" })), false);
  assert.equal(validCustomerTagsBody("tag-create", JSON.stringify({ name: "x".repeat(21), color: "red" })), false);
  assert.equal(validCustomerTagsBody("tag-create", JSON.stringify({ name: " VIP", color: "red" })), false);
  assert.equal(validCustomerTagsBody("tag-create", JSON.stringify({ name: "VIP" })), false);
  assert.equal(validCustomerTagsBody("tag-create", JSON.stringify({ name: "VIP", color: "red", extra: 1 })), false);
  assert.equal(validCustomerTagsBody("tag-create", "[1]"), false);
  assert.equal(validCustomerTagsBody("tag-create", "nope"), false);
  for (const color of tagColors) assert.equal(validCustomerTagsBody("tag-create", JSON.stringify({ name: "n", color })), true, color);
});

test("tag-patch body: at least one of name/color, nothing else", () => {
  assert.equal(validCustomerTagsBody("tag-patch", JSON.stringify({ name: "新名" })), true);
  assert.equal(validCustomerTagsBody("tag-patch", JSON.stringify({ color: "teal" })), true);
  assert.equal(validCustomerTagsBody("tag-patch", JSON.stringify({ name: "新名", color: "teal" })), true);
  assert.equal(validCustomerTagsBody("tag-patch", JSON.stringify({})), false);
  assert.equal(validCustomerTagsBody("tag-patch", JSON.stringify({ name: "新名", color: "teal", id })), false);
  assert.equal(validCustomerTagsBody("tag-patch", JSON.stringify({ color: "teal " })), false);
});

test("owner-tags body: tag_ids (<=100 canonical uuids) + 64-hex revision, exact keys", () => {
  assert.equal(validCustomerTagsBody("owner-tags", JSON.stringify({ tag_ids: [], revision })), true);
  assert.equal(validCustomerTagsBody("owner-tags", JSON.stringify({ tag_ids: [id, other], revision })), true);
  assert.equal(validCustomerTagsBody("owner-tags", JSON.stringify({ tag_ids: Array(100).fill(id), revision })), true);
  assert.equal(validCustomerTagsBody("owner-tags", JSON.stringify({ tag_ids: Array(101).fill(id), revision })), false);
  assert.equal(validCustomerTagsBody("owner-tags", JSON.stringify({ tag_ids: ["x"], revision })), false);
  assert.equal(validCustomerTagsBody("owner-tags", JSON.stringify({ tag_ids: [id.toUpperCase()], revision })), false);
  assert.equal(validCustomerTagsBody("owner-tags", JSON.stringify({ tag_ids: [], revision: "a".repeat(63) })), false);
  assert.equal(validCustomerTagsBody("owner-tags", JSON.stringify({ tag_ids: [], revision: "g".repeat(64) })), false);
  assert.equal(validCustomerTagsBody("owner-tags", JSON.stringify({ tag_ids: [] })), false);
  assert.equal(validCustomerTagsBody("owner-tags", JSON.stringify({ tag_ids: [], revision, extra: true })), false);
});

test("note bodies: create {body}; patch {body, version>=1}; deletes and reads carry no body at all", () => {
  assert.equal(validCustomerTagsBody("note-create", JSON.stringify({ body: "買家偏好晚上聯繫" })), true);
  assert.equal(validCustomerTagsBody("note-create", JSON.stringify({ body: "a\nb" })), true);
  assert.equal(validCustomerTagsBody("note-create", JSON.stringify({ body: " " })), false);
  assert.equal(validCustomerTagsBody("note-create", JSON.stringify({ body: "x".repeat(1001) })), false);
  assert.equal(validCustomerTagsBody("note-create", JSON.stringify({ body: "x", version: 1 })), false);
  assert.equal(validCustomerTagsBody("note-patch", JSON.stringify({ body: "改", version: 1 })), true);
  assert.equal(validCustomerTagsBody("note-patch", JSON.stringify({ body: "改", version: 0 })), false);
  assert.equal(validCustomerTagsBody("note-patch", JSON.stringify({ body: "改", version: 1.5 })), false);
  assert.equal(validCustomerTagsBody("note-patch", JSON.stringify({ body: "改" })), false);
  for (const kind of ["tag-delete", "note-delete", "tag-list", "customer-list", "note-list"] as CustomerTagsRouteKind[]) {
    assert.equal(validCustomerTagsBody(kind, ""), true, kind);
    assert.equal(validCustomerTagsBody(kind, "{}"), false, kind);
  }
});

// ---------------------------------------------------------------------------------------------- decoders
const tagRecord = { id, name: "VIP", color: "red", created_at: "2026-10-01T02:03:04Z", customers_count: 2 };
const note = { id, body: "note body", author_id: other, created_at: "2026-10-01T02:03:04.5Z", edited_at: null, version: 1, own: false };
const throws = (fn: () => unknown) => assert.throws(fn, /unavailable/);

test("parseTagCatalog decodes {items: TagRecord[]} with the store-catalogue bounds", () => {
  const parsed = parseTagCatalog({ items: [tagRecord, { ...tagRecord, id: other }] });
  assert.equal(parsed.items.length, 2);
  assert.deepEqual(parsed.items[0], tagRecord);
  assert.deepEqual(parseTagCatalog({ items: [] }), { items: [] });
  throws(() => parseTagCatalog({ items: [tagRecord], extra: 1 }));
  throws(() => parseTagCatalog({ items: [{ ...tagRecord, color: "pink" }] }));
  throws(() => parseTagCatalog({ items: [{ ...tagRecord, name: "" }] }));
  throws(() => parseTagCatalog({ items: [{ ...tagRecord, customers_count: -1 }] }));
  throws(() => parseTagCatalog({ items: [{ ...tagRecord, created_at: "2026-10-01" }] }));
  throws(() => parseTagCatalog({ items: [tagRecord, tagRecord] })); // duplicate ids
  throws(() => parseTagCatalog({ items: Array.from({ length: 101 }, (_, i) => ({ ...tagRecord, id: `${i}`.padStart(8, "0") + "-1111-4111-8111-111111111111" })) }));
  throws(() => parseTagCatalog([tagRecord]));
});

test("parseTagSet / parseDeletedTag / parseDeletedNote decode the frozen write receipts", () => {
  assert.deepEqual(parseTagSet({ tags: [{ id, name: "VIP", color: "red" }], tags_revision: revision }),
    { tags: [{ id, name: "VIP", color: "red" }], tags_revision: revision });
  assert.deepEqual(parseTagSet({ tags: [], tags_revision: revision }).tags, []);
  throws(() => parseTagSet({ tags: [], tags_revision: "a".repeat(63) }));
  throws(() => parseTagSet({ tags: [{ id, name: "VIP", color: "pink" }], tags_revision: revision }));
  throws(() => parseTagSet({ tags: Array.from({ length: 21 }, () => ({ id, name: "VIP", color: "red" })), tags_revision: revision }));
  throws(() => parseTagSet({ tags: [], tags_revision: revision, extra: 1 }));
  assert.deepEqual(parseDeletedTag({ tag_id: id, detached: 3 }), { tag_id: id, detached: 3 });
  throws(() => parseDeletedTag({ tag_id: "x", detached: 3 }));
  throws(() => parseDeletedTag({ tag_id: id, detached: -1 }));
  assert.deepEqual(parseDeletedNote({ note_id: id }), { note_id: id });
  throws(() => parseDeletedNote({ note_id: "x" }));
  throws(() => parseDeletedNote({ note_id: id, extra: 1 }));
});

test("parseNoteRecord / parseNotePage decode notes with the Go note rules", () => {
  assert.deepEqual(parseNoteRecord(note), note);
  assert.equal(parseNoteRecord({ ...note, edited_at: "2026-10-02T00:00:00.123456789Z" }).edited_at, "2026-10-02T00:00:00.123456789Z");
  throws(() => parseNoteRecord({ ...note, version: 0 }));
  throws(() => parseNoteRecord({ ...note, body: " " }));
  throws(() => parseNoteRecord({ ...note, body: "a\u0000b" }));
  throws(() => parseNoteRecord({ ...note, body: "x".repeat(1001) }));
  throws(() => parseNoteRecord({ ...note, author_id: "x" }));
  throws(() => parseNoteRecord({ ...note, edited_at: "yesterday" }));
  throws(() => parseNoteRecord({ ...note, extra: 1 }));
  throws(() => parseNoteRecord({ ...note, own: "yes" }));
  const { own: _own, ...noOwn } = note; throws(() => parseNoteRecord(noOwn)); // own is required
  assert.equal(parseNoteRecord({ ...note, own: true }).own, true);
  const page = parseNotePage({ items: [note], next_cursor: "" });
  assert.deepEqual(page.items, [note]);
  assert.equal(parseNotePage({ items: [], next_cursor: "Abc-_1" }).next_cursor, "Abc-_1");
  throws(() => parseNotePage({ items: [note], next_cursor: "a=b" }));
  throws(() => parseNotePage({ items: [note, note], next_cursor: "" }));
  throws(() => parseNotePage({ items: Array.from({ length: 101 }, (_, i) => ({ ...note, id: `${i}`.padStart(8, "0") + "-1111-4111-8111-111111111111" })), next_cursor: "" }));
  throws(() => parseNotePage({ items: [], next_cursor: "", extra: 1 }));
});

// ---------------------------------------------------------------------------------------------- copy
test("customer-tags copy: three locales, identical key sets, the integrator's ruled note-privacy text verbatim", () => {
  for (const locale of ["zh-TW", "zh-CN"] as const)
    assert.deepEqual(Object.keys(customerTagsCopy[locale]).sort(), Object.keys(customerTagsCopy.en).sort(), locale);
  // Integrator ruling (2026-10): the note input must say notes are deleted on erasure and visible in a buyer export.
  assert.equal(customerTagsCopy["zh-TW"].notePrivacy, "備註會在買家要求刪除資料時一起刪除，買家申請匯出資料時也看得到");
  for (const locale of ["zh-TW", "zh-CN", "en"] as const) {
    const c = customerTagsCopy[locale];
    assert.equal(c.notePrivacy.length > 10, true, `${locale}: notePrivacy`);
    for (const color of tagColors) assert.ok(c.colors[color], `${locale}: color ${color}`);
    for (const code of ["default", "unauthorized", "forbidden", "not_found", "tag_exists", "limit_reached", "version_changed", "idempotency_conflict", "retry_later", "rate_limited", "invalid_request"])
      assert.ok(c.errors[code], `${locale}: errors.${code}`);
  }
});

// ---------------------------------------------------------------------------------------------- source ratchet
test("components wire the ruled copy and the tag/note controls the specs click", () => {
  const tags = readFileSync("apps/admin/components/CustomerTags.tsx", "utf8") + readFileSync("apps/admin/components/CustomerTagsNotes.tsx", "utf8");
  assert.match(tags, /customer-tags-copy/);
  assert.match(tags, /data-testid="note-privacy-hint"/);
  assert.match(tags, /data-testid="customer-tags-editor"/);
  assert.match(tags, /data-testid="customer-notes"/);
  assert.match(tags, /TagBadges/);
  assert.match(tags, /data-testid="tag-manage-dialog"/);
  assert.doesNotMatch(tags, /force: true|dispatchEvent/);
});

// Strict reads must refuse a backend drift rather than silently inventing a normalized server fact.
test("tag read names must already be trimmed NFC and Unicode code-point bounded", () => {
  for (const name of [" VIP", "VIP ", "e\u0301", "\u200bVIP"])
    throws(() => parseTagCatalog({ items: [{ ...tagRecord, name }] }));
  assert.equal(parseTagCatalog({ items: [{ ...tagRecord, name: "😀".repeat(20) }] }).items[0].name.length, 40);
});
