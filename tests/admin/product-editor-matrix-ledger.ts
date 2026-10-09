// Purpose: Record canonical mobile-matrix expectations only after the real action and assertions pass.
// Depends on: shared product_editor_matrix_cases.json and rendered productEditorCopy locale bindings.
// Used by: product-editor.acceptance.ts and the DB-free action-coverage gate; Go embeds the same case table.
// actual=PASS means the callback's real action and canonical postcondition assertions passed.
// expected is the stable declaration; observed retains the original detailed values, SKU identities and screenshots.
import cases from "../foundation/testdata/product_editor_matrix_cases.json" with { type: "json" };
import { productEditorCopy } from "../../apps/admin/lib/product-editor-copy";

type Identity = { page: string; row?: string; control: string; action: string };
type Observation = { observed: unknown; [detail: string]: unknown };
const key = (entry: Identity) => JSON.stringify([entry.page, entry.row ?? "", entry.control, entry.action]);

/** Creates one locale's recorder; callback failures never emit PASS or consume a declaration. */
export function createProductEditorMatrixLedger(ledger: unknown[], locale: keyof typeof productEditorCopy) {
  const copy = productEditorCopy[locale];
  const bindings: Record<string, string> = {
    "@price": copy.price, "@compare": copy.compare, "@quantity": copy.quantity,
    "@max": copy.max, "@targetQty": copy.targetQty, "@code": copy.code,
    "@keyword": copy.keyword, "@untracked": copy.untracked,
  };
  const expand = (value: string) => value.replace(/@[A-Za-z]+/g, (token) => {
    if (!(token in bindings)) throw new Error(`Unknown matrix locale token ${token}`);
    return bindings[token];
  });
  const declared = new Map(cases.map((entry) => {
    for (const field of ["page", "row", "control", "action", "expected"] as const)
      if (typeof entry[field] !== "string" || (field !== "row" && !entry[field]))
        throw new Error(`Invalid matrix declaration ${field}`);
    const expanded = { ...entry, control: expand(entry.control), expected: expand(entry.expected) };
    return [key(expanded), expanded] as const;
  }));
  if (declared.size !== cases.length) throw new Error("Duplicate matrix declaration");
  const seen = new Set<string>();
  const pending = new Set<string>();
  return {
    /** Runs a real UI action plus DOM assertions, then records PASS for its declared canonical expectation. */
    async step(identity: Identity, perform: () => Promise<Observation>) {
      const id = key(identity), entry = declared.get(id);
      if (!entry) throw new Error(`Undeclared matrix action ${id}`);
      if (seen.has(id) || pending.has(id)) throw new Error(`Duplicate matrix action ${id}`);
      // Mark in-flight too: concurrent native-dialog recording must not admit a duplicate.
      pending.add(id);
      try {
        const details = await perform();
        if (!details || !("observed" in details)) throw new Error(`Missing matrix observation ${id}`);
        ledger.push({ ...details, ...entry, locale, width: 390, actual: "PASS" });
        seen.add(id);
      } finally {
        pending.delete(id);
      }
    },
    /** Fails the locale flow when any declared control/readback has not passed. */
    assertComplete() {
      const missing = [...declared.keys()].filter((id) => !seen.has(id));
      if (missing.length) throw new Error(`Missing matrix actions: ${missing.join(", ")}`);
    },
  };
}
