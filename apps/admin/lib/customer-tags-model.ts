// Purpose: Strict decoders and input validators for the W6-01B tag/note DTOs of internal/customers (TagRecord, TagSet,
//   DeletedTag, Note, DeletedNote, pagination.Page[Note]) plus the Go halves of ValidTagName/ValidNoteBody. Unknown key,
//   wrong type or broken bound = thrown "unavailable"; the server stays the authority, these only refuse malformed reads.
// Depends on: ./orders-model.ts (canonicalUUID, canonicalCursor), ./customers-model.ts (object, isInstant, count, tagColors, Tag/Note types).
// Used by: apps/admin/lib/customer-tags-request.ts, apps/admin/lib/customer-tags-client.ts, apps/admin/components/CustomerTags.tsx, apps/admin/components/CustomerDetail.tsx, tests/admin/customer-tags-bff.test.ts
import { canonicalCursor, canonicalUUID } from "./orders-model.ts";
import { count, isInstant, object, tagColors, type Note, type Tag } from "./customers-model.ts";

/** W6-01B TagRecord contract; validates data without writes or storage. */
export type TagRecord = Tag & { created_at: string; customers_count: number };
/** W6-01B TagCatalog contract; validates data without writes or storage. */
export type TagCatalog = { items: TagRecord[] };
/** W6-01B TagSet contract; validates data without writes or storage. */
export type TagSet = { tags: Tag[]; tags_revision: string };
/** W6-01B DeletedTag contract; validates data without writes or storage. */
export type DeletedTag = { tag_id: string; detached: number };
/** W6-01B DeletedNote contract; validates data without writes or storage. */
export type DeletedNote = { note_id: string };
/** W6-01B NotePage contract; validates data without writes or storage. */
export type NotePage = { items: Note[]; next_cursor: string };

const revisionPattern = /^[0-9a-f]{64}$/;
// Store catalogue (W6-01B): at most 100 tags; one customer carries at most 20; ListNotes pages at most 100.
const maxCatalog = 100;
const maxOwnerTags = 20;
const maxPageItems = 100;

// Go customers.ValidTagName: NFC first, then 1..20 runes, trimmed, no control/format/surrogate characters. The database
// re-checks NFC; this is the client and BFF fence so a refused name never becomes a round trip.
/** W6-01B validTagName contract; validates data without writes or storage. */
export function validTagName(name: unknown): name is string {
  if (typeof name !== "string") return false;
  const composed = name.normalize("NFC");
  return composed !== "" && Array.from(composed).length <= 20 && composed === composed.trim() &&
    !/[\p{Cc}\p{Cf}\p{Cs}]/u.test(composed);
}
// Go customers.ValidNoteBody: 1..1000 runes, not blank, no control characters other than line breaks and tabs
// (lone surrogates would be invalid UTF-8 and are refused too). Cf characters are allowed in bodies, unlike names.
/** W6-01B validNoteBody contract; validates data without writes or storage. */
export function validNoteBody(body: unknown): body is string {
  if (typeof body !== "string") return false;
  const runes = Array.from(body).length;
  return runes >= 1 && runes <= 1000 && body.trim() !== "" && !/[\p{Cc}\p{Cs}]/u.test(body.replace(/[\n\r\t]/g, ""));
}

function tagChecks(v: Record<string, unknown>) {
  if (typeof v.id !== "string" || !canonicalUUID.test(v.id) || typeof v.name !== "string" || !validTagName(v.name) || v.name !== v.name.normalize("NFC") || !(tagColors as readonly unknown[]).includes(v.color))
    throw new Error("unavailable");
}
function parseTag(value: unknown): Tag {
  const v = object(value, ["id", "name", "color"]);
  tagChecks(v);
  return v as Tag;
}
function unique(ids: string[]) {
  if (new Set(ids).size !== ids.length) throw new Error("unavailable");
}

/** Decode one server catalogue tag without changing server facts or writing storage. */
export function parseTagRecord(value: unknown): TagRecord {
  const r = object(value, ["id", "name", "color", "created_at", "customers_count"]);
  tagChecks(r);
  if (!isInstant(r.created_at) || !count(r.customers_count)) throw new Error("unavailable");
  return r as TagRecord;
}

/** W6-01B parseTagCatalog contract; validates data without writes or storage. */
export function parseTagCatalog(value: unknown): TagCatalog {
  const v = object(value, ["items"]);
  if (!Array.isArray(v.items) || v.items.length > maxCatalog) throw new Error("unavailable");
  const items = v.items.map(parseTagRecord);
  unique(items.map((tag) => tag.id));
  return { items };
}

/** W6-01B parseTagSet contract; validates data without writes or storage. */
export function parseTagSet(value: unknown): TagSet {
  const v = object(value, ["tags", "tags_revision"]);
  if (!Array.isArray(v.tags) || v.tags.length > maxOwnerTags || typeof v.tags_revision !== "string" || !revisionPattern.test(v.tags_revision))
    throw new Error("unavailable");
  const tags = v.tags.map(parseTag);
  unique(tags.map((tag) => tag.id));
  return { tags, tags_revision: v.tags_revision };
}

/** W6-01B parseDeletedTag contract; validates data without writes or storage. */
export function parseDeletedTag(value: unknown): DeletedTag {
  const v = object(value, ["tag_id", "detached"]);
  if (typeof v.tag_id !== "string" || !canonicalUUID.test(v.tag_id) || !count(v.detached)) throw new Error("unavailable");
  return v as DeletedTag;
}

/** W6-01B parseDeletedNote contract; validates data without writes or storage. */
export function parseDeletedNote(value: unknown): DeletedNote {
  const v = object(value, ["note_id"]);
  if (typeof v.note_id !== "string" || !canonicalUUID.test(v.note_id)) throw new Error("unavailable");
  return v as DeletedNote;
}

/** W6-01B parseNoteRecord contract; validates data without writes or storage. */
export function parseNoteRecord(value: unknown): Note {
  const v = object(value, ["id", "body", "author_id", "created_at", "edited_at", "version"]);
  if (typeof v.id !== "string" || !canonicalUUID.test(v.id) || typeof v.author_id !== "string" || !canonicalUUID.test(v.author_id) ||
    !validNoteBody(v.body) || !isInstant(v.created_at) || !(v.edited_at === null || isInstant(v.edited_at)) ||
    !Number.isSafeInteger(v.version) || (v.version as number) < 1) throw new Error("unavailable");
  return v as Note;
}

/** W6-01B parseNotePage contract; validates data without writes or storage. */
export function parseNotePage(value: unknown): NotePage {
  const v = object(value, ["items", "next_cursor"]);
  if (!Array.isArray(v.items) || v.items.length > maxPageItems || typeof v.next_cursor !== "string" ||
    (v.next_cursor !== "" && !canonicalCursor.test(v.next_cursor))) throw new Error("unavailable");
  const items = v.items.map(parseNoteRecord);
  unique(items.map((note) => note.id));
  return { items, next_cursor: v.next_cursor };
}
