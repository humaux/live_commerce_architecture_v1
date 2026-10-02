"use client";
// One staged user action. Committed stages never repeat; an UNKNOWN stage retains exact bytes/key.
import { useEffect, useRef, useState } from "react";
import { command, readProduct, send, type Command } from "./catalog-v2-client";
import { parseCreated, type ProductDetail } from "./catalog-v2-model";
import { listImages, validImageList } from "./images-client";
import {
  createDocument,
  parseBulk,
  type ProductDraft,
} from "./product-document";
import { uploadDocumentImage } from "./product-media-client";
import type { ProductEditorCopy } from "./product-editor-copy";
import type { DraftPhoto } from "../components/ProductDocumentMedia";
type Workflow = {
  create: Command;
  photos: DraftPhoto[];
  publish: boolean;
  id?: string;
  uploaded: string[];
  order?: Command;
  ordered?: boolean;
  publication?: Command;
  published?: boolean;
};
export function useProductDocument(
  store: string,
  currency: string,
  boundary: string,
  c: ProductEditorCopy,
) {
  const [busy, setBusy] = useState(false),
    [pending, setPending] = useState(false),
    [done, setDone] = useState<ProductDetail | null>(null),
    [message, setMessage] = useState("");
  const workflow = useRef<Workflow | null>(null),
    running = useRef(false);
  // A receipt fence, NOT a draft cache: no form values, media, credentials or personal data.
  // Losing the page must not turn an UNKNOWN creation into permission to create it again.
  const fenceKey = `product-document-pending:${store}`;
  const [fenceReady, setFenceReady] = useState(false),
    [recoveryBlocked, setRecoveryBlocked] = useState(false);
  useEffect(() => {
    try {
      const blocked = sessionStorage.getItem(fenceKey) !== null;
      setRecoveryBlocked(blocked);
      if (blocked) setMessage(c.recoveryRequired);
    } catch {
      setRecoveryBlocked(true);
      setMessage(c.recoveryRequired);
    }
    setFenceReady(true);
  }, [fenceKey, c.recoveryRequired]);
  const failCode = (code: string) =>
    setMessage(code in c ? c[code as keyof ProductEditorCopy] : c.failed);
  async function retry() {
    const op = workflow.current;
    if (!op || running.current) return;
    running.current = true;
    setBusy(true);
    setMessage("");
    try {
      if (!op.id) {
        const result = await send(store, op.create, boundary, parseCreated);
        if (!result.ok) {
          if (!result.uncertain) {
            sessionStorage.removeItem(fenceKey);
            workflow.current = null;
            setPending(false);
          }
          failCode(result.uncertain ? "uncertain" : result.code);
          return;
        }
        op.id = result.value.id;
      }
      while (op.uploaded.length < op.photos.length) {
        const p = op.photos[op.uploaded.length];
        const result = await uploadDocumentImage(
          store,
          op.id,
          p.file!,
          p.key,
          boundary,
        );
        if (!result.ok) {
          failCode(result.uncertain ? "uncertain" : result.code);
          return;
        }
        op.uploaded.push(result.value.id);
      }
      if (op.photos.length && !op.ordered) {
        op.order ??= command("POST", `products/${op.id}/images/order`, {
          ids: op.uploaded,
        });
        const result = await send(store, op.order, boundary, (v) => {
          if (!validImageList(v)) throw new Error("invalid");
          return v;
        });
        if (!result.ok) {
          failCode(result.uncertain ? "uncertain" : result.code);
          return;
        }
        op.ordered = true;
      }
      if (op.publish && !op.published) {
        const media = await listImages(store, op.id);
        if (!media.items?.length) {
          setMessage(c.imageRequired);
          return;
        }
        op.publication ??= command("POST", "products/bulk-status", {
          ids: [op.id],
          status: "active",
        });
        const result = await send(store, op.publication, boundary, parseBulk);
        if (!result.ok) {
          failCode(result.uncertain ? "uncertain" : result.code);
          return;
        }
        if (result.value[0]?.error) {
          failCode(result.value[0].error);
          return;
        }
        if (
          result.value.length !== 1 ||
          result.value[0].id !== op.id ||
          result.value[0].status !== "active"
        )
          throw new Error("unconfirmed");
        op.published = true;
      }
      const confirmed = await readProduct(
        store,
        op.id,
        new AbortController().signal,
      );
      if (
        confirmed.id !== op.id ||
        confirmed.status !== (op.publish ? "active" : "draft")
      )
        throw new Error("unconfirmed");
      setDone(confirmed);
      sessionStorage.removeItem(fenceKey);
      setPending(false);
      setMessage(op.publish ? c.published : c.saved);
    } catch {
      setMessage(c.uncertain);
    } finally {
      running.current = false;
      setBusy(false);
    }
  }
  function save(draft: ProductDraft, photos: DraftPhoto[], publish: boolean) {
    if (
      !fenceReady ||
      recoveryBlocked ||
      busy ||
      pending ||
      done ||
      running.current
    )
      return;
    if (publish && !photos.length) {
      setMessage(c.imageRequired);
      return;
    }
    if (draft.axes.some((a) => !a.name.trim() || !a.values.length)) {
      setMessage(c.matrixLimit);
      return;
    }
    try {
      const body = createDocument(draft, currency);
      workflow.current = {
        create: command("POST", "products/document", body),
        photos: [...photos],
        publish,
        uploaded: [],
      };
      sessionStorage.setItem(fenceKey, workflow.current.create.key);
      setPending(true);
      void retry();
    } catch (error) {
      failCode(error instanceof Error ? error.message : "failed");
    }
  }
  return {
    busy,
    pending,
    fenceReady,
    recoveryBlocked,
    done,
    message,
    setMessage,
    save,
    retry,
    reset: () => {
      workflow.current = null;
      setDone(null);
      setMessage("");
    },
  };
}
