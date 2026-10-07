// Purpose: read only the first RFC4180 header record from an in-memory UTF-8 file.
// Depends on: Blob slices/TextDecoder and import-model byte limit; never parses uploaded data rows.
// Used by: ImportWizard via import-client, header-only Node tests.
import { maxImportBytes } from "./import-model.ts";

/** Bounded header refusal; message contains no uploaded text. */
export class ImportHeaderError extends Error {
  readonly code: string;
  constructor(code: string) { super(code); this.code = code; }
}

/** Read only header bytes in memory; original Blob remains untouched for upload. */
export async function readImportHeader(file: Blob): Promise<string[]> {
  if (!file.size) throw new ImportHeaderError("required");
  if (file.size > maxImportBytes) throw new ImportHeaderError("file_too_large");
  const bytes: number[] = [];
  let quoted = false, done = false;
  for (let offset = 0; offset < file.size && !done; offset += 1024) {
    const chunk = new Uint8Array(await file.slice(offset, offset + 1024).arrayBuffer());
    for (const byte of chunk) {
      if (byte === 34) quoted = !quoted;
      if (byte === 10 && !quoted) {
        // Go's csv.Reader skips blank physical lines before a header.
        const bomBlank=bytes[0]===239&&bytes[1]===187&&bytes[2]===191&&(bytes.length===3||(bytes.length===4&&bytes[3]===13));
        if (bytes.length === 0 || (bytes.length === 1 && bytes[0] === 13) || bomBlank) { bytes.length = 0; continue; }
        done = true; break;
      }
      bytes.push(byte);
    }
  }
  let text: string;
  try { text = new TextDecoder("utf-8", { fatal:true }).decode(new Uint8Array(bytes)).replace(/^\uFEFF/, "").replace(/\r$/, "").replaceAll("\r\n", "\n"); }
  catch { throw new ImportHeaderError("encoding_not_utf8"); }
  if (!text) throw new ImportHeaderError("required");
  const fields: string[] = [];
  let value = "", state: "start" | "bare" | "quoted" | "closed" = "start";
  for (let i = 0; i < text.length; i++) {
    const ch = text[i];
    if (state === "quoted") {
      if (ch === '"') { if (text[i+1] === '"') { value += '"'; i++; } else state = "closed"; }
      else value += ch;
    } else if (ch === ",") { fields.push(value.trim()); value = ""; state = "start"; }
    else if (state === "closed" || (ch === '"' && state !== "start")) throw new ImportHeaderError("invalid_request");
    else if (ch === '"') state = "quoted";
    else { value += ch; state = "bare"; }
  }
  if (state === "quoted") throw new ImportHeaderError("invalid_request");
  fields.push(value.trim());
  return fields;
}
