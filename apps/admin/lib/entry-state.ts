export type EntryDraft = {
  tenant_name: string;
  store_name: string;
  warehouse_name: string;
  currency: string;
};

export type EntryPending = { key: string; body: string };

export type EntryJournal = {
  version: 1;
  expiresAt: number;
  step: 1 | 2 | 3;
  draft: EntryDraft;
  pending: EntryPending | null;
};

export const ENTRY_TTL_MS = 24 * 60 * 60 * 1000;
export const ENTRY_MAX_BYTES = 4096;

export function emptyEntryDraft(currency = ""): EntryDraft {
  return { tenant_name: "", store_name: "", warehouse_name: "", currency };
}

function exactDraft(value: unknown, currencies: readonly string[]) {
  if (!value || typeof value !== "object") return null;
  const item = value as Record<string, unknown>;
  if (
    Object.keys(item).sort().join(",") !==
    "currency,store_name,tenant_name,warehouse_name"
  )
    return null;
  const draft = item as EntryDraft;
  if (
    [draft.tenant_name, draft.store_name, draft.warehouse_name].some(
      (name) => typeof name !== "string" || [...name].length > 120,
    ) ||
    typeof draft.currency !== "string" ||
    (draft.currency !== "" && !currencies.includes(draft.currency))
  )
    return null;
  return draft;
}

export function parseEntryJournal(
  raw: string | null,
  currencies: readonly string[],
  now = Date.now(),
): EntryJournal | null {
  if (!raw || raw.length > ENTRY_MAX_BYTES) return null;
  try {
    const item = JSON.parse(raw) as Partial<EntryJournal>;
    if (
      item.version !== 1 ||
      !Number.isFinite(item.expiresAt) ||
      item.expiresAt! <= now ||
      item.expiresAt! > now + ENTRY_TTL_MS ||
      ![1, 2, 3].includes(item.step ?? 0)
    )
      return null;
    const draft = exactDraft(item.draft, currencies);
    if (!draft) return null;
    let pending: EntryPending | null = null;
    if (item.pending !== null && item.pending !== undefined) {
      if (
        typeof item.pending !== "object" ||
        typeof item.pending.key !== "string" ||
        !/^[A-Za-z0-9_.:-]{8,128}$/.test(item.pending.key) ||
        typeof item.pending.body !== "string" ||
        item.pending.body.length > 1024 ||
        !exactDraft(JSON.parse(item.pending.body), currencies)
      )
        return null;
      pending = { key: item.pending.key, body: item.pending.body };
    }
    return {
      version: 1,
      expiresAt: item.expiresAt!,
      step: item.step as 1 | 2 | 3,
      draft,
      pending,
    };
  } catch {
    return null;
  }
}

export function encodeEntryJournal(journal: EntryJournal) {
  const encoded = JSON.stringify(journal);
  if (encoded.length > ENTRY_MAX_BYTES)
    throw new Error("entry journal too large");
  return encoded;
}
