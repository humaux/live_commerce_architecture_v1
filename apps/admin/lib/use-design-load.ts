// Purpose: Load the store design editor snapshot behind its authenticated session fence.
// Depends on: react, design-client reads, customers-client errors, settings-client sessionBoundary, design-model DTOs.
// Used by: components/Design.tsx.
// Invariants: read-only loading; abort on unmount and reject snapshots crossing a session boundary.
"use client";

import { useEffect, useState } from "react";
import { ReadError, type ReadCode } from "./customers-client";
import { sessionBoundary } from "./settings-client";
import { readDraft, readMedia, readVersions } from "./design-client";
import type { Draft, MediaItem, VersionList } from "./design-model";

export type Loaded = { draft: Draft; media: MediaItem[]; versions: VersionList };

async function loadAll(store: string, signal: AbortSignal): Promise<Loaded> {
  const [draft, media, versions] = await Promise.all([
    readDraft(store, signal),
    readMedia(store, signal),
    readVersions(store, signal),
  ]);
  return { draft, media, versions };
}

// One read per (store, reload). The boundary is the session fence write requests must still match.
export function useDesignLoad(
  storeID: string | null,
  initialError: ReadCode | null,
  renderKey: string,
) {
  const [tick, setTick] = useState(0);
  const key = `${renderKey}|${storeID ?? ""}|${tick}`;
  const [view, setView] = useState<{
    key: string;
    status: "loading" | "ready" | ReadCode;
    data: Loaded | null;
    boundary: string;
  }>({ key: "", status: "loading", data: null, boundary: "" });
  useEffect(() => {
    if (initialError || !storeID) {
      setView({
        key,
        status: initialError ?? "not-found",
        data: null,
        boundary: "",
      });
      return;
    }
    let live = true;
    const controller = new AbortController();
    void (async () => {
      try {
        const boundary = await sessionBoundary();
        const data = await loadAll(storeID, controller.signal);
        if (!live) return;
        if ((await sessionBoundary()) !== boundary)
          throw new ReadError("signed-out");
        setView({ key, status: "ready", data, boundary });
      } catch (error) {
        if (live)
          setView({
            key,
            status: error instanceof ReadError ? error.code : "unavailable",
            data: null,
            boundary: "",
          });
      }
    })();
    return () => {
      live = false;
      controller.abort();
    };
  }, [key, storeID, initialError]);
  const current =
    view.key === key
      ? view
      : { key, status: "loading" as const, data: null, boundary: "" };
  return { ...current, reload: () => setTick((value) => value + 1) };
}

