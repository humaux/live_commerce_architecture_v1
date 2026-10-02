// Admin cash-on-delivery model (home-cod R5, migration 0107): strict parsers for the DTO of Go internal/httpapi/cod.go (BFF
// `/api/stores/{store}/cash-on-delivery-settings`) plus the request-body builder that puts exactly the frozen keys on the wire.
// It never decides eligibility, money or permission: Go/SQL stay the authority (payments.set_cash_on_delivery_settings); a parser
// only refuses a malformed read. The cap and surcharge shown are the server whole-TWD values, never client numbers.

export type CodCarrier = "black_cat" | "hsinchu";

function object(value: unknown, keys: string[]): Record<string, unknown> {
  if (!value || typeof value !== "object" || Array.isArray(value)) throw new Error("unavailable");
  const result = value as Record<string, unknown>;
  if (Object.keys(result).sort().join(",") !== [...keys].sort().join(",")) throw new Error("unavailable");
  return result;
}
const isInt = (v: unknown, min: number, max: number): v is number =>
  typeof v === "number" && Number.isSafeInteger(v) && v >= min && v <= max;

// ---- settings card (GET/PUT cash-on-delivery-settings) -----------------------------------------------------------------------
export type CodSettings = {
  version: number;
  enabled: boolean;
  max_twd: number;
  surcharge_twd: number;
  carrier: CodCarrier;
};
export function parseCodSettings(value: unknown): CodSettings {
  const v = object(value, ["version", "enabled", "max_twd", "surcharge_twd", "carrier"]);
  if (
    !isInt(v.version, 0, Number.MAX_SAFE_INTEGER) || typeof v.enabled !== "boolean" ||
    !isInt(v.max_twd, 1, 20000) || !isInt(v.surcharge_twd, 0, 1000) ||
    !(v.carrier === "black_cat" || v.carrier === "hsinchu")
  )
    throw new Error("unavailable");
  return v as CodSettings;
}
export type CodSettingsInput = { expected_version: number } & Omit<CodSettings, "version">;
// Text -> body; null when a field is outside the rules (cap 1..20000 whole TWD, surcharge 0..1000 whole TWD, carrier black_cat|hsinchu).
export function codSettingsBody(f: {
  expectedVersion: number; enabled: boolean; maxTwd: string; surchargeTwd: string; carrier: CodCarrier;
}): CodSettingsInput | null {
  const max = /^[0-9]{1,5}$/.test(f.maxTwd.trim()) ? Number(f.maxTwd.trim()) : null;
  const sur = /^[0-9]{1,4}$/.test(f.surchargeTwd.trim()) ? Number(f.surchargeTwd.trim()) : null;
  if (
    !isInt(f.expectedVersion, 0, Number.MAX_SAFE_INTEGER - 1) ||
    max === null || max < 1 || max > 20000 ||
    sur === null || sur < 0 || sur > 1000 ||
    !(f.carrier === "black_cat" || f.carrier === "hsinchu")
  )
    return null;
  return {
    expected_version: f.expectedVersion, enabled: f.enabled, max_twd: max, surcharge_twd: sur, carrier: f.carrier,
  };
}
