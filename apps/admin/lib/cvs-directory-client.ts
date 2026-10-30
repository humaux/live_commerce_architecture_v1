// Purpose: load and cache the public immutable CVS snapshot outside rendering components.
// Depends on: same-origin static JSON, fetch and the shared CVS directory types.
// Used by: admin CvsLookup; no buyer API, session, storage or provider request.
// Invariants: fixed brand paths, omitted credentials, validated public fields; aborts never fill the cache.
import type { CvsBrand, CvsStore } from "../../../packages/format/src/cvs-match";

const directories = new Map<CvsBrand, CvsStore[]>();
const brands: readonly CvsBrand[] = ["UNIMART", "UNIMARTFREEZE", "FAMI", "HILIFE", "OKMART"];
// One immutable filename supplies both the request URL and its displayed source date.
export function cvsDirectorySource(brand: CvsBrand): { path: string; date: string } {
  if (!brands.includes(brand)) throw new Error("invalid_directory_brand");
  const path = `/cvs/${brand.toLowerCase()}-20261028.json`;
  const date = /-(\d{4})(\d{2})(\d{2})\.json$/.exec(path)!;
  return { path, date: `${date[1]}-${date[2]}-${date[3]}` };
}
function validDirectory(value: unknown): value is CvsStore[] {
  return Array.isArray(value) && value.length > 0 && value.every((row) =>
    row !== null && typeof row === "object" && ["code", "name", "address", "county", "district"].every((key) => typeof row[key] === "string" && row[key].length > 0));
}

export async function loadCvsDirectory(brand: CvsBrand, signal: AbortSignal): Promise<CvsStore[]> {
  signal.throwIfAborted();
  const source = cvsDirectorySource(brand);
  const cached = directories.get(brand);
  if (cached) return cached;
  const response = await fetch(source.path, { signal, credentials: "omit" });
  if (!response.ok) throw new Error("directory_unavailable");
  const data: unknown = await response.json();
  if (!validDirectory(data)) throw new Error("invalid_directory");
  signal.throwIfAborted();
  directories.set(brand, data);
  return data;
}
