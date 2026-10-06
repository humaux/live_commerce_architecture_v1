// Purpose: staged PM-v2 main/detail/option photos for new product creation, with local downsizing and role ordering.
// Depends on: product-media-model/copy, photo-preprocess, ProductMediaTiles and caller's fenced document workflow.
// Used by: ProductDocumentForm create; no upload occurs before the product has been created.
"use client";
import {
  useRef,
  useState,
  useEffect,
  useId,
  type Dispatch,
  type SetStateAction,
} from "react";
import type { Locale } from "@live-commerce/i18n";
import type { OptionAxis } from "@/lib/catalog-v2-model";
import { PHOTO_ACCEPT } from "@/lib/images-client";
import {
  mediaCaps,
  effectiveMediaAxis,
  type MediaRole,
} from "@/lib/product-media-model";
import { productMediaCopy } from "@/lib/product-media-copy";
import { preparePhoto, PhotoProblem } from "@/lib/photo-preprocess";
import { ProductMediaTiles } from "./ProductMediaTiles";
import "./product-media.css";
export type DraftPhoto = {
  key: string;
  url: string;
  file?: File;
  id?: string;
  role?: MediaRole;
  optionName?: string;
  optionValue?: string;
  width?: number | null;
  height?: number | null;
};
/** Stage immutable normalized files; the parent later saves exact File/key/role with its creation recovery flow. */
export function ProductDocumentMedia({
  photos,
  setPhotos,
  disabled,
  locale,
  axes,
  imageAxis,
  setImageAxis,
  fail,
  onProcessingChange,
}: {
  photos: DraftPhoto[];
  setPhotos: Dispatch<SetStateAction<DraftPhoto[]>>;
  disabled: boolean;
  locale: Locale;
  axes: OptionAxis[];
  imageAxis: string | null;
  setImageAxis: (axis: string | null) => void;
  fail: (message: string) => void;
  onProcessingChange: (busy: boolean) => void;
}) {
  const c = productMediaCopy[locale],
    axis = effectiveMediaAxis(axes, imageAxis),
    id = useId();
  const [busy, setBusy] = useState(false),
    alive = useRef(true),
    working = useRef(false);
  useEffect(() => {
    alive.current = true;
    return () => {
      alive.current = false;
      onProcessingChange(false);
    };
  }, [onProcessingChange]);
  const group = (role: MediaRole) =>
    photos.filter((p) => (p.role ?? "main") === role);
  async function add(files: FileList | null, role: MediaRole, value?: string) {
    if (!files || disabled || working.current) return;
    const candidates = Array.from(files);
    if (!candidates.length) return;
    if (
      (role === "sku" &&
        (!axis ||
          !value ||
          candidates.length !== 1 ||
          photos.some(
            (p) =>
              p.role === "sku" &&
              p.optionName === axis.name &&
              p.optionValue === value,
          ))) ||
      group(role).length + candidates.length > mediaCaps[role]
    ) {
      fail(
        role === "main"
          ? c.mainFull
          : role === "detail"
            ? c.detailFull
            : c.optionFull,
      );
      return;
    }
    working.current = true;
    setBusy(true);
    onProcessingChange(true);
    fail("");
    try {
      const added: DraftPhoto[] = [];
      // Process sequentially so several phone-sized decoded bitmaps do not coexist.
      for (const file of candidates) {
        const ready = await preparePhoto(file, role);
        added.push({
          key: crypto.randomUUID(),
          url: "",
          file: ready.file,
          role,
          optionName: role === "sku" ? axis!.name : undefined,
          optionValue: value,
          width: ready.width,
          height: ready.height,
        });
      }
      if (!alive.current) return;
      for (const photo of added) photo.url = URL.createObjectURL(photo.file!);
      setPhotos((now) => [...now, ...added]);
    } catch (error) {
      if (alive.current)
        fail(error instanceof PhotoProblem ? c[error.reason] : c.failed);
    } finally {
      working.current = false;
      if (alive.current) {
        setBusy(false);
        onProcessingChange(false);
      }
    }
  }
  const remove = (key: string) =>
    setPhotos((now) => {
      const photo = now.find((p) => p.key === key);
      if (photo?.file) URL.revokeObjectURL(photo.url);
      return now.filter((p) => p.key !== key);
    });
  function reorder(role: MediaRole, from: number, to: number) {
    if (disabled || busy) return;
    setPhotos((now) => {
      const indexes = now
        .map((p, i) => ((p.role ?? "main") === role ? i : -1))
        .filter((i) => i >= 0);
      const members = indexes.map((i) => now[i]);
      members.splice(to, 0, members.splice(from, 1)[0]);
      const copy = [...now];
      indexes.forEach((i, n) => (copy[i] = members[n]));
      return copy;
    });
  }
  function changeRole(key: string, role: "main" | "detail") {
    if (group(role).length >= mediaCaps[role]) {
      fail(role === "main" ? c.mainFull : c.detailFull);
      return;
    }
    const p = photos.find((p) => p.key === key);
    if (role === "detail" && p?.width && p.height && p.height > 6 * p.width) {
      fail(c.ratio);
      return;
    }
    setPhotos((now) => now.map((p) => (p.key === key ? { ...p, role } : p)));
  }
  const locked = disabled || busy;
  const picker = (role: MediaRole, value?: string) => (
    <label className="pm-upload">
      <span>{role === "sku" ? c.addOption : c.add}</span>
      <input
        aria-label={
          role === "sku" ? `${c.addOption}: ${value}` : `${c.add}: ${c[role]}`
        }
        data-testid={role === "main" ? "photo-input" : `photo-input-${role}`}
        type="file"
        accept={PHOTO_ACCEPT}
        multiple={role !== "sku"}
        disabled={locked || group(role).length >= mediaCaps[role]}
        onChange={(e) => {
          void add(e.target.files, role, value);
          e.target.value = "";
        }}
      />
    </label>
  );
  const section = (role: "main" | "detail") => (
    <section key={role} aria-labelledby={`${id}-${role}`}>
      <h2 id={`${id}-${role}`}>
        {c[role]}{" "}
        <small>
          {group(role).length}/{mediaCaps[role]}
        </small>
      </h2>
      <p className="pe-hint">{role === "main" ? c.mainHelp : c.detailHelp}</p>
      <ProductMediaTiles
        role={role}
        photos={group(role)}
        disabled={locked}
        c={c}
        onMove={(a, b) => reorder(role, a, b)}
        onDelete={remove}
        onRole={changeRole}
      />
      {picker(role)}
    </section>
  );
  return (
    <section
      id="media"
      className="product-section pm-media"
      aria-label={c.title}
    >
      {busy && <p role="status">{c.processing}</p>}
      {section("main")}
      <section aria-labelledby={`${id}-sku`}>
        <h2 id={`${id}-sku`}>{c.sku}</h2>
        <p className="pe-hint">{c.skuHelp}</p>
        <div className="pm-field">
          <label htmlFor={`${id}-axis`}>{c.axis}</label>
          <select
            data-testid="media-axis-picker"
            id={`${id}-axis`}
            value={imageAxis ?? ""}
            disabled={locked || !axes.length}
            onChange={(e) => {
              if (photos.some((p) => p.role === "sku")) {
                fail(c.changedAxis);
                return;
              }
              setImageAxis(e.target.value || null);
            }}
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
              const chosen = photos.filter(
                (p) =>
                  p.role === "sku" &&
                  p.optionName === axis.name &&
                  p.optionValue === value,
              );
              return (
                <div
                  key={value}
                  className="pm-option"
                  data-option-value={value}
                >
                  <h3>{value}</h3>
                  {chosen.length ? (
                    <ProductMediaTiles
                      role="sku"
                      photos={chosen}
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
        {photos.some(
          (p) =>
            p.role === "sku" &&
            (!axis ||
              p.optionName !== axis.name ||
              !axis.values.includes(p.optionValue ?? "")),
        ) && (
          <div>
            <p role="alert">{c.staleOption}</p>
            <ProductMediaTiles
              role="sku"
              photos={photos.filter(
                (p) =>
                  p.role === "sku" &&
                  (!axis ||
                    p.optionName !== axis.name ||
                    !axis.values.includes(p.optionValue ?? "")),
              )}
              disabled={locked}
              c={c}
              onMove={() => {}}
              onDelete={remove}
            />
          </div>
        )}
      </section>
      {section("detail")}
    </section>
  );
}
