// Purpose: load and cache the public immutable CVS snapshot outside rendering components.
// Depends on: same-origin static JSON, fetch and the shared CVS directory types.
// Used by: CvsStorePicker; no buyer API, session, storage or provider request.
// Invariants: fixed brand paths, omitted credentials, validated public fields; aborts never fill the cache.
import type { CvsBrand, CvsStore } from "../../../packages/format/src/cvs-match";

const directories = new Map<CvsBrand, CvsStore[]>();
const brands: readonly CvsBrand[] = ["UNIMART", "UNIMARTFREEZE", "FAMI", "HILIFE", "OKMART"];
function validDirectory(value: unknown): value is CvsStore[] {
  return Array.isArray(value) && value.length > 0 && value.every((row) =>
    row !== null && typeof row === "object" && ["code", "name", "address", "county", "district"].every((key) => typeof row[key] === "string" && row[key].length > 0));
}

export async function loadCvsDirectory(brand: CvsBrand, signal: AbortSignal): Promise<CvsStore[]> {
  signal.throwIfAborted();
  if (!brands.includes(brand)) throw new Error("invalid_directory_brand");
  const cached = directories.get(brand);
  if (cached) return cached;
  const response = await fetch(`/cvs/${brand.toLowerCase()}-20261028.json`, { signal, credentials: "omit" });
  if (!response.ok) throw new Error("directory_unavailable");
  const data: unknown = await response.json();
  if (!validDirectory(data)) throw new Error("invalid_directory");
  signal.throwIfAborted();
  directories.set(brand, data);
  return data;
}
