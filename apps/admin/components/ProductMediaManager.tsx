// Purpose: PM-v2 committed media editing: per-role order/move, option-axis cells and normalized uploads.
// Depends on: fenced catalog commands, /api/stores/{store}/products/{product}/images and PM-v2 action routes, photo-preprocess.
// Used by: ProductDocumentForm edit; Go /v1/admin/stores/{store}/products/{product}/images remains authority.
"use client";
import { useEffect, useRef, useState, useId } from "react";
import type { Locale } from "@live-commerce/i18n";
import type { OptionAxis } from "@/lib/catalog-v2-model";
import type { ProductImageList } from "@/lib/model";
import {
  command,
  send,
  type Command,
  type Outcome,
} from "@/lib/catalog-v2-client";
import {
  listImages,
  imageURL,
  PHOTO_ACCEPT,
  validImageList,
} from "@/lib/images-client";
import { uploadDocumentImage } from "@/lib/product-media-client";
import { preparePhoto, PhotoProblem } from "@/lib/photo-preprocess";
import { productMediaCopy } from "@/lib/product-media-copy";
import {
  mediaCaps,
  effectiveMediaAxis,
  mediaRecoveryKey,
  optionImageMatches,
  persistMediaPending,
  type MediaRole,
} from "@/lib/product-media-model";
import { ProductMediaTiles } from "./ProductMediaTiles";
import "./product-media.css";
type Pending =
  | {
      kind: "upload";
      file: File;
      key: string;
      role: MediaRole;
      value?: string;
      axis?: string;
    }
  | { kind: "command"; request: Command };
/** Apply one explicit media mutation. UNKNOWN keeps the original request in memory for an identical retry. */
export function ProductMediaManager({
  locale,
  store,
  productID,
  boundary,
  axes,
  disabled,
  onChanged,
  onLocked,
}: {
  locale: Locale;
  store: string;
  productID: string;
  boundary: string;
  axes: OptionAxis[];
  disabled: boolean;
  onChanged: (list: ProductImageList | null) => void;
  onLocked: (locked: boolean) => void;
}) {
  const c = productMediaCopy[locale],
    scope = `${store}:${productID}:${boundary}`,
    journal = mediaRecoveryKey(store, productID),
    id = useId();
  const [list, setList] = useState<ProductImageList | null>(null),
    [busy, setBusy] = useState(false),
    [pending, setPending] = useState<Pending | null>(null),
    [lost, setLost] = useState(false),
    [ready, setReady] = useState(false),
    [message, setMessage] = useState("");
  const current = useRef(scope),
    alive = useRef(true),
    running = useRef(false),
    readEpoch = useRef(0);
  current.current = scope;
  const axis = effectiveMediaAxis(axes, list?.image_axis ?? null);
  const group = (role: MediaRole) =>
    list?.items.filter((p) => p.role === role) ?? [];
  const parser = (value: unknown) => {
    if (
      !validImageList(value) ||
      value.items.some((p) => p.product_id !== productID)
    )
      throw new Error("invalid_response");
    return value;
  };
  async function read(): Promise<ProductImageList | null> {
    const before = current.current;
    const epoch = ++readEpoch.current;
    const result = await listImages(store, productID);
    if (
      before !== current.current ||
      !alive.current ||
      epoch !== readEpoch.current
    )
      return null;
    if (result.list) {
      setList(result.list);
      onChanged(result.list);
      return result.list;
    }
    setList(null);
    onChanged(null);
    setMessage(c.failed);
    return null;
  }
  useEffect(() => {
    alive.current = true;
    setList(null);
    onChanged(null);
    try {
      setLost(!!sessionStorage.getItem(journal));
      setReady(true);
    } catch {
      setLost(true);
      setReady(false);
      setMessage(c.storageFailed);
    }
    const controller = new AbortController();
    const epoch = ++readEpoch.current;
    void listImages(store, productID, controller.signal).then((result) => {
      if (
        !controller.signal.aborted &&
        current.current === scope &&
        epoch === readEpoch.current
      ) {
        if (result.list) {
          setList(result.list);
          onChanged(result.list);
        } else {
          onChanged(null);
          setMessage(c.failed);
        }
      }
    });
    return () => {
      alive.current = false;
      controller.abort();
      onLocked(false);
    };
  }, [scope, store, productID, journal, onLocked, onChanged, c.failed]);
  useEffect(() => {
    onLocked(busy || !!pending || lost || !ready || !list);
  }, [busy, pending, lost, ready, list, onLocked]);
  const remember = (value: string | null) => {
    try {
      const saved = persistMediaPending(sessionStorage, journal, value);
      setReady(saved);
      if (!saved) {
        setLost(true);
        setMessage(c.storageFailed);
      }
      return saved;
    } catch {
      setReady(false);
      setLost(true);
      setMessage(c.storageFailed);
      return false;
    }
  };
  async function execute(op: Pending): Promise<boolean> {
    const before = current.current;
    ++readEpoch.current; // A delayed pre-mutation read cannot replace the new canonical head.
    setPending(op);
    if (!remember(op.kind === "upload" ? op.key : op.request.key)) return false;
    if (op.kind === "upload" && op.role === "sku") {
      const canonical = await read();
      if (!canonical) return false;
      if (
        !op.axis ||
        effectiveMediaAxis(axes, canonical.image_axis)?.name !== op.axis
      ) {
        setPending(null);
        setLost(true);
        setMessage(c.axisChanged);
        return false;
      }
    }
    const answer: Outcome<unknown> =
      op.kind === "upload"
        ? await uploadDocumentImage(
            store,
            productID,
            op.file,
            op.key,
            boundary,
            op.role,
            op.value,
          )
        : await send(store, op.request, boundary, parser);
    if (before !== current.current || !alive.current) return false;
    if (!answer.ok) {
      setMessage(
        answer.reconcile
          ? c.unauthorized
          : answer.uncertain
            ? c.unknown
            : c.failed,
      );
      if (!answer.uncertain && !answer.reconcile) {
        setPending(null);
        remember(null);
      }
      if (answer.reconcile) {
        setList(null);
        onChanged(null);
        return false;
      }
      const canonical = await read();
      if (canonical && answer.code === "conflict" && op.kind === "upload") {
        if (
          canonical.items.filter((p) => p.role === op.role).length >=
          mediaCaps[op.role]
        )
          setMessage(
            op.role === "main"
              ? c.mainFull
              : op.role === "detail"
                ? c.detailFull
                : c.optionCap,
          );
        else if (
          op.role === "sku" &&
          canonical.option_images.some((link) => link.option_value === op.value)
        )
          setMessage(c.optionFull);
      }
      return false;
    }
    setPending(null);
    if (!remember(null)) {
      setLost(true);
      return false;
    }
    setLost(false);
    const canonical = await read();
    if (!canonical) return false;
    if (
      op.kind === "upload" &&
      op.role === "sku" &&
      (!op.axis ||
        !optionImageMatches(
          effectiveMediaAxis(axes, canonical.image_axis)?.name ?? null,
          canonical.option_images,
          op.axis,
          op.value ?? "",
          (answer.value as { id: string }).id,
        ))
    ) {
      setMessage(c.axisChanged);
      return false;
    }
    setMessage(c.done);
    return true;
  }
  async function run(op: Pending) {
    if (running.current) return;
    running.current = true;
    setBusy(true);
    setMessage("");
    try {
      await execute(op);
    } finally {
      running.current = false;
      if (alive.current) setBusy(false);
    }
  }
  function action(resource: string, body: unknown) {
    if (disabled || running.current || pending || lost) return;
    void run({
      kind: "command",
      request: command("POST", `products/${productID}/${resource}`, body),
    });
  }
  async function upload(
    files: FileList | null,
    role: MediaRole,
    value?: string,
  ) {
    if (!files || disabled || running.current || pending || lost || !list)
      return;
    const selected = Array.from(files);
    const capturedAxis = axis?.name;
    if (!selected.length) return;
    if (group(role).length + selected.length > mediaCaps[role]) {
      setMessage(
        role === "main"
          ? c.mainFull
          : role === "detail"
            ? c.detailFull
            : c.optionCap,
      );
      return;
    }
    running.current = true;
    setBusy(true);
    setMessage(c.processing);
    try {
      for (const file of selected) {
        const ready = await preparePhoto(file, role);
        if (!alive.current) return;
        if (
          !(await execute({
            kind: "upload",
            file: ready.file,
            key: crypto.randomUUID(),
            role,
            value,
            axis: capturedAxis,
          }))
        )
          break;
      }
    } catch (error) {
      if (alive.current)
        setMessage(error instanceof PhotoProblem ? c[error.reason] : c.failed);
    } finally {
      running.current = false;
      if (alive.current) setBusy(false);
    }
  }
  const locked = disabled || busy || !!pending || lost || !list || !ready;
  const tile = (photo: ProductImageList["items"][number]) => ({
    key: photo.id,
    url: imageURL(store, productID, photo.id),
    width: photo.width,
    height: photo.height,
  });
  function order(role: MediaRole, from: number, to: number) {
    const ids = group(role).map((p) => p.id);
    ids.splice(to, 0, ids.splice(from, 1)[0]);
    action("images/order", { role, ids });
  }
  function remove(key: string) {
    if (window.confirm(c.deleteConfirm)) action(`images/${key}/delete`, {});
  }
  function moveRole(key: string, target: "main" | "detail") {
    if (group(target).length >= mediaCaps[target]) {
      setMessage(target === "main" ? c.mainFull : c.detailFull);
      return;
    }
    action(`images/${key}/move`, { role: target });
  }
  const picker = (role: MediaRole, value?: string) => (
    <label className="pm-upload">
      <span>{role === "sku" ? c.addOption : c.add}</span>
      <input
        type="file"
        aria-label={
          role === "sku" ? `${c.addOption}: ${value}` : `${c.add}: ${c[role]}`
        }
        data-testid={role === "main" ? "photo-input" : `photo-input-${role}`}
        accept={PHOTO_ACCEPT}
        multiple={role !== "sku"}
        disabled={locked || group(role).length >= mediaCaps[role]}
        onChange={(e) => {
          void upload(e.target.files, role, value);
          e.target.value = "";
        }}
      />
    </label>
  );
  const section = (role: "main" | "detail") => (
    <section aria-labelledby={`${id}-${role}`}>
      <h2 id={`${id}-${role}`}>
        {c[role]}{" "}
        <small>
          {group(role).length}/{mediaCaps[role]}
        </small>
      </h2>
      <p className="pe-hint">{role === "main" ? c.mainHelp : c.detailHelp}</p>
      <ProductMediaTiles
        role={role}
        photos={group(role).map(tile)}
        disabled={locked}
        c={c}
        onMove={(a, b) => order(role, a, b)}
        onDelete={remove}
        onRole={moveRole}
      />
      {picker(role)}
    </section>
  );
  return (
    <div className="pm-media" data-testid="photo-manager">
      {!list && <p role="status">{c.loading}</p>}
      {!list && !busy && (
        <button type="button" onClick={() => void read()}>
          {c.refresh}
        </button>
      )}
      {busy && <p role="status">{c.saving}</p>}
      {message && <p role="status">{message}</p>}
      {pending && (
        <div className="pm-recovery">
          <p>{c.unknown}</p>
          <button
            type="button"
            disabled={busy}
            onClick={() => void run(pending)}
          >
            {c.retry}
          </button>
          <button type="button" disabled={busy} onClick={() => void read()}>
            {c.refresh}
          </button>
        </div>
      )}
      {lost && (
        <div className="pm-recovery">
          <p>{c.lost}</p>
          <button type="button" disabled={busy} onClick={() => void read()}>
            {c.refresh}
          </button>
          <button
            type="button"
            disabled={!list || busy}
            onClick={() => {
              if (remember(null)) setLost(false);
            }}
          >
            {c.ack}
          </button>
        </div>
      )}
      {section("main")}
      <section aria-labelledby={`${id}-sku`}>
        <h2 id={`${id}-sku`}>{c.sku}</h2>
        <p className="pe-hint">{c.skuHelp}</p>
        <div className="pm-field">
          <label htmlFor={`${id}-axis`}>{c.axis}</label>
          <select
            data-testid="media-axis-picker"
            id={`${id}-axis`}
            value={list?.image_axis ?? ""}
            disabled={locked || !axes.length}
            onChange={(e) =>
              action("image-axis", { axis: e.target.value || null })
            }
          >
            <option value="">{c.defaultAxis}</option>
            {axes.map((a) => (
              <option key={a.name} value={a.name}>
                {a.name}
              </option>
            ))}
          </select>
        </div>
        {!axis ? (
          <p>{c.noAxis}</p>
        ) : (
          <div className="pm-options">
            {axis.values.map((value) => {
              const link = list?.option_images.find(
                  (p) =>
                    p.option_name === axis.name && p.option_value === value,
                ),
                photo =
                  link &&
                  list?.items.find(
                    (p) => p.id === link.image_id && p.role === "sku",
                  );
              return (
                <div
                  className="pm-option"
                  key={value}
                  data-option-value={value}
                >
                  <h3>{value}</h3>
                  {photo ? (
                    <ProductMediaTiles
                      role="sku"
                      photos={[tile(photo)]}
                      disabled={locked}
                      c={c}
                      onMove={() => {}}
                      onDelete={remove}
                    />
                  ) : (
                    <>
                      <p>{c.useCover}</p>
                      {picker("sku", value)}
                    </>
                  )}
                </div>
              );
            })}
          </div>
        )}
        {group("sku").some(
          (photo) =>
            !list?.option_images.some((link) => link.image_id === photo.id),
        ) && (
          <details>
            <summary>{c.unused}</summary>
            <ProductMediaTiles
              role="sku"
              photos={group("sku")
                .filter(
                  (photo) =>
                    !list?.option_images.some(
                      (link) => link.image_id === photo.id,
                    ),
                )
                .map(tile)}
              disabled={locked}
              c={c}
              onMove={() => {}}
              onDelete={remove}
            />
          </details>
        )}
      </section>
      {section("detail")}
    </div>
  );
}
