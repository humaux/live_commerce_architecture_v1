// Purpose: PM-U media roles, canonical upload queries, effective image axis and resize geometry.
// Depends on: frozen image-list caps and product option types; no DOM/fetch.
// Used by: admin BFF, media editor/preprocessor and focused Node acceptance.
import {
  MAX_MAIN_PHOTOS,
  MAX_DETAIL_PHOTOS,
  MAX_OPTION_PHOTOS,
} from "./image-list-contract.ts";
export type MediaRole = "main" | "detail" | "sku";
export const mediaCaps = {
  main: MAX_MAIN_PHOTOS,
  detail: MAX_DETAIL_PHOTOS,
  sku: MAX_OPTION_PHOTOS,
} as const;
const uuid = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/;
const valueOK = (s: string, max: number) =>
  [...s].length > 0 && [...s].length <= max && !/[\p{Cc}\p{Cf}\p{Cs}]/u.test(s);
/** Bound camera geometry without enlarging smaller originals or changing their aspect ratio. */
export function fitPhoto(
  width: number,
  height: number,
  longest = 2000,
): { width: number; height: number } {
  if (
    !Number.isSafeInteger(width) ||
    !Number.isSafeInteger(height) ||
    width < 1 ||
    height < 1 ||
    !Number.isSafeInteger(longest) ||
    longest < 1
  )
    throw new Error("invalid_dimensions");
  const factor = Math.min(1, longest / Math.max(width, height));
  return {
    width: Math.max(1, Math.floor(width * factor)),
    height: Math.max(1, Math.floor(height * factor)),
  };
}
/** Only upload accepts a query; role sku requires exactly one current-value parameter. */
export function validMediaQuery(search: string, upload: boolean): boolean {
  if (!search) return true;
  if (
    !upload ||
    search === "?" ||
    !search.startsWith("?") ||
    search.length > 1024
  )
    return false;
  try {
    const entries = new Map<string, string>();
    for (const part of search.slice(1).split("&")) {
      const match = /^(role|option_value)=([^=]+)$/.exec(part);
      if (!match || entries.has(match[1])) return false;
      entries.set(match[1], decodeURIComponent(match[2].replaceAll("+", " ")));
    }
    const role = entries.get("role");
    if (!role || !Object.hasOwn(mediaCaps, role)) return false;
    const value = entries.get("option_value");
    return role === "sku" ? !!value && valueOK(value, 40) : value === undefined;
  } catch {
    return false;
  }
}
/** Encode one exact media upload address; store membership remains a BFF/server decision. */
export function mediaUploadPath(
  store: string,
  product: string,
  role: MediaRole = "main",
  optionValue?: string,
): string {
  if (
    !uuid.test(store) ||
    !uuid.test(product) ||
    !Object.hasOwn(mediaCaps, role) ||
    (role === "sku"
      ? !optionValue || !valueOK(optionValue, 40)
      : optionValue !== undefined)
  )
    throw new Error("invalid_request");
  const query = new URLSearchParams({ role });
  if (optionValue !== undefined) query.set("option_value", optionValue);
  return `/api/stores/${store}/products/${product}/images?${query}`;
}
/** Resolve the axis from current option definitions, mirroring the backend's stale-axis fallback. */
export function effectiveMediaAxis<
  T extends { name: string; values: string[] },
>(axes: T[], axis: string | null): T | null {
  return axes.find((a) => a.name === axis) ?? axes[0] ?? null;
}

/** Only main photos contribute to cover/readiness recommendations; details and option images do not. */
export function mainPhotoCount(photos: readonly { role?: string }[]): number {
  return photos.filter((photo) => (photo.role ?? "main") === "main").length;
}

/** Keep UNKNOWN recovery across re-authentication; session authority remains a separate transport fence. */
export function mediaRecoveryKey(store: string, product: string): string {
  if (!uuid.test(store) || !uuid.test(product))
    throw new Error("invalid_request");
  return `product-media-pending:${store}:${product}`;
}

/** A canonical read must bind an option receipt to the axis/value captured before normalization. */
export function optionImageMatches(
  actualAxis: string | null,
  links: readonly {
    option_name: string;
    option_value: string;
    image_id: string;
  }[],
  expectedAxis: string,
  value: string,
  imageID: string,
): boolean {
  return (
    actualAxis === expectedAxis &&
    links.some(
      (link) =>
        link.option_name === expectedAxis &&
        link.option_value === value &&
        link.image_id === imageID,
    )
  );
}

/** Persist or clear the recovery marker; callers must refuse dispatch when the browser cannot retain it. */
export function persistMediaPending(
  storage: Pick<Storage, "setItem" | "removeItem">,
  journal: string,
  key: string | null,
): boolean {
  try {
    if (key) storage.setItem(journal, key);
    else storage.removeItem(journal);
    return true;
  } catch {
    return false;
  }
}
