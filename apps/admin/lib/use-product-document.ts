// Purpose: staged product document/media save with exact receipt replay and canonical publication checks.
// Depends on: catalog document commands, role media uploads, session storage and authenticated Go readback.
// Used by: ProductDocumentForm; UNKNOWN keeps the original payload/key and blocks fresh creation.
"use client";
// One staged user action. Committed stages never repeat; an UNKNOWN stage retains exact bytes/key.
import { useEffect, useRef, useState } from "react";
import {
  command,
  readProduct,
  send,
  type Command,
  type Outcome,
} from "./catalog-v2-client";
import { parseCreated, type ProductDetail } from "./catalog-v2-model";
import { listImages, validImageList } from "./images-client";
import {
  createDocument,
  editDocument,
  parseBulk,
  type ProductDraft,
} from "./product-document";
import { uploadDocumentImage } from "./product-media-client";
import type { ProductEditorCopy } from "./product-editor-copy";
import type { DraftPhoto } from "../components/ProductDocumentMedia";
import { effectiveMediaAxis, optionImageMatches } from "./product-media-model";
type Workflow = {
  edit?: boolean;
  expectedStatus?: string;
  create: Command;
  photos: DraftPhoto[];
  publish: boolean;
  id?: string;
  uploaded: string[];
  orders?: Partial<Record<"main" | "detail", Command>>;
  orderedRoles?: ("main" | "detail")[];
  imageAxis?: string | null;
  axisCommand?: Command;
  axisSet?: boolean;
  publication?: Command;
  published?: boolean;
};
/** Save one staged document action; uncertain writes keep their exact command and durable recovery fence. */
export function useProductDocument(
  store: string,
  currency: string,
  boundary: string,
  c: ProductEditorCopy,
  detail?: ProductDetail | null,
  axisChanged = c.recoveryRequired,
) {
  const [savedDetail, setSavedDetail] = useState<ProductDetail | null>(null);
  const baseline = savedDetail ?? detail;
  const [busy, setBusy] = useState(false),
    [pending, setPending] = useState(false),
    [done, setDone] = useState<ProductDetail | null>(null),
    [message, setMessage] = useState("");
  const workflow = useRef<Workflow | null>(null),
    running = useRef(false);
  // A receipt fence, NOT a draft cache: no form values, media, credentials or personal data.
  // Losing the page must not turn an UNKNOWN creation into permission to create it again.
  const fenceKey = `product-document-pending:${store}${detail ? `:${detail.id}` : ""}`;
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
  function failOutcome(result: Extract<Outcome<unknown>, { ok: false }>) {
    if (result.reconcile) {
      // Auth refusal cannot resolve any earlier UNKNOWN stage. Preserve the
      // workflow receipt fence and stop automatic or user-triggered retries.
      setRecoveryBlocked(true);
      setMessage(c.recoveryRequired);
    } else failCode(result.uncertain ? "uncertain" : result.code);
  }
  async function retry() {
    const op = workflow.current;
    if (!op || running.current || recoveryBlocked) return;
    running.current = true;
    setBusy(true);
    setMessage("");
    try {
      async function checkOption(
        photo: DraftPhoto,
        imageID?: string,
      ): Promise<boolean> {
        const product = await readProduct(
          store,
          op!.id!,
          new AbortController().signal,
        );
        const media = await listImages(store, op!.id!);
        const expected = photo.optionName?.trim();
        const actual = media.list
          ? effectiveMediaAxis(product.options, media.list.image_axis)?.name
          : null;
        if (
          !media.list ||
          !expected ||
          actual !== expected ||
          (imageID &&
            !optionImageMatches(
              actual ?? null,
              media.list.option_images,
              expected,
              photo.optionValue ?? "",
              imageID,
            ))
        ) {
          setRecoveryBlocked(true);
          setMessage(axisChanged);
          return false;
        }
        return true;
      }
      if (!op.id) {
        const result = await send(store, op.create, boundary, parseCreated);
        if (!result.ok) {
          if (!result.uncertain && !result.reconcile) {
            sessionStorage.removeItem(fenceKey);
            workflow.current = null;
            setPending(false);
          }
          failOutcome(result);
          return;
        }
        op.id = result.value.id;
      }
      if (!op.edit && !op.axisSet && op.imageAxis !== undefined) {
        op.axisCommand ??= command("POST", `products/${op.id}/image-axis`, {
          axis: op.imageAxis,
        });
        const axisResult = await send(store, op.axisCommand, boundary, (v) => {
          if (!validImageList(v)) throw new Error("invalid");
          return v;
        });
        if (!axisResult.ok) {
          failOutcome(axisResult);
          return;
        }
        op.axisSet = true;
      }
      while (op.uploaded.length < op.photos.length) {
        const p = op.photos[op.uploaded.length];
        if (p.role === "sku" && !(await checkOption(p))) return;
        const result = await uploadDocumentImage(
          store,
          op.id,
          p.file!,
          p.key,
          boundary,
          p.role ?? "main",
          p.optionValue,
        );
        if (!result.ok) {
          failOutcome(result);
          return;
        }
        if (p.role === "sku" && !(await checkOption(p, result.value.id)))
          return;
        op.uploaded.push(result.value.id);
      }
      op.orders ??= {};
      op.orderedRoles ??= [];
      for (const role of ["main", "detail"] as const) {
        const ids = op.uploaded.filter(
          (_, i) => (op.photos[i].role ?? "main") === role,
        );
        if (!ids.length || op.orderedRoles.includes(role)) continue;
        op.orders[role] ??= command("POST", `products/${op.id}/images/order`, {
          role,
          ids,
        });
        const result = await send(store, op.orders[role]!, boundary, (v) => {
          if (!validImageList(v)) throw new Error("invalid");
          return v;
        });
        if (!result.ok) {
          failOutcome(result);
          return;
        }
        op.orderedRoles.push(role);
      }
      if (op.publish && !op.published) {
        for (let i = 0; i < op.photos.length; i++)
          if (
            op.photos[i].role === "sku" &&
            !(await checkOption(op.photos[i], op.uploaded[i]))
          )
            return;
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
          failOutcome(result);
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
        confirmed.status !==
          (op.expectedStatus ?? (op.publish ? "active" : "draft"))
      )
        throw new Error("unconfirmed");
      if (op.edit) {
        setSavedDetail(confirmed);
        workflow.current = null;
      } else setDone(confirmed);
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
  function save(
    draft: ProductDraft,
    photos: DraftPhoto[],
    publish: boolean,
    status?: "draft" | "active",
    imageAxis?: string | null,
  ) {
    if (
      !fenceReady ||
      recoveryBlocked ||
      busy ||
      pending ||
      done ||
      running.current
    )
      return;
    if (
      publish &&
      baseline?.status !== "active" &&
      !photos.some((p) => (p.role ?? "main") === "main")
    ) {
      setMessage(c.imageRequired);
      return;
    }
    if (draft.axes.some((a) => !a.name.trim() || !a.values.length)) {
      setMessage(c.matrixLimit);
      return;
    }
    try {
      const target = publish ? "active" : status;
      const body = baseline
        ? editDocument(draft, baseline, currency, target)
        : createDocument(draft, currency);
      if (baseline && Object.keys(body).length === 1) {
        setMessage(c.noChanges);
        return;
      }
      workflow.current = {
        create: baseline
          ? command("PUT", `products/${baseline.id}/document`, body)
          : command("POST", "products/document", body),
        edit: !!baseline,
        expectedStatus: baseline ? (target ?? baseline.status) : undefined,
        photos: baseline ? [] : [...photos],
        publish: baseline ? false : publish,
        uploaded: [],
        imageAxis: baseline ? undefined : imageAxis,
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
    savedDetail,
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
