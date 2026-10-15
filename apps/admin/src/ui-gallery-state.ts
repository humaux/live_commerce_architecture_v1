// Purpose: Gallery-only, tab-local synthetic persistence; never used for authorization, transactions or private search.
// Depends on: React hooks and sessionStorage; validates primitive record shape and bounds strings.
// Used by: UiGalleryTemplates. Invariant: disabled storage still leaves every demo usable; no network writes.
"use client";
import { useEffect, useState } from "react";
export function useGalleryRecord<T extends Record<string, string | boolean>>(key: string, initial: T) {
  const [value, setValue] = useState<T>(initial);
  const [ready, setReady] = useState(false);
  useEffect(() => {
    try {
      const raw: unknown = JSON.parse(sessionStorage.getItem(key) ?? "null");
      if (raw && typeof raw === "object") {
        const record = raw as Record<string, unknown>;
        if (Object.keys(initial).every(k => typeof record[k] === typeof initial[k] && (typeof record[k] !== "string" || (record[k] as string).length <= 2000))) {
          setValue(Object.fromEntries(Object.keys(initial).map(k => [k, record[k]])) as T);
        }
      }
    } catch { /* Optional browser preference, not a source of business truth. */ }
    setReady(true);
    // initial is the immutable seed for this mounted template; changes are written through the returned setter.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [key]);
  const persist = (next: T) => {
    setValue(next);
    try { sessionStorage.setItem(key, JSON.stringify(next)); } catch { /* In-memory demo still works. */ }
  };
  return [value, persist, ready] as const;
}
