// Presentation and major-unit helpers moved from stop-bleed, not a second implementation.
const maxMoney = 1_000_000_000_000;
export const currencySign = (currency: string) =>
  currency === "TWD" ? "NT$" : currency;
export function money(locale: string, currency: string, minor: number) {
  const digits = new Intl.NumberFormat(locale, {
    style: "currency",
    currency,
  }).resolvedOptions().maximumFractionDigits!;
  const formatter = new Intl.NumberFormat(locale, {
    style: "currency",
    currency,
    minimumFractionDigits: minor % 10 ** digits === 0 ? 0 : digits,
  });
  return formatter
    .formatToParts(minor / 10 ** digits)
    .map((part) =>
      part.type === "currency" && currency === "TWD"
        ? currencySign(currency)
        : part.value,
    )
    .join("");
}
export const STORE_TIME_ZONE = "Asia/Taipei";
export function displayTime(locale: string, value: string) {
  return new Intl.DateTimeFormat(locale, {
    timeZone: STORE_TIME_ZONE,
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    hour12: false,
  }).format(new Date(value));
}
/** Store-timezone wall clock to the second, for "updated at" feedback stamps (displayTime stops at the minute). */
export function displayClock(locale: string, value: string | number | Date) {
  return new Intl.DateTimeFormat(locale, {
    timeZone: STORE_TIME_ZONE,
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
    hourCycle: "h23",
  }).format(new Date(value));
}
export function minorDigits(currency: string) {
  return (
    new Intl.NumberFormat("en", {
      style: "currency",
      currency,
    }).resolvedOptions().maximumFractionDigits ?? 2
  );
}
export const wholeOnly = (currency: string) => currency === "TWD";
export function amountToMinor(text: string, currency: string): number | null {
  const digits = minorDigits(currency);
  const match = new RegExp(
    digits === 0
      ? "^(\\d{1,12})()$"
      : `^(\\d{1,12})(?:\\.(\\d{1,${digits}}))?$`,
  ).exec(text.trim());
  if (!match) return null;
  const minor =
    Number(match[1]) * 10 ** digits +
    Number((match[2] ?? "").padEnd(digits, "0") || 0);
  return Number.isSafeInteger(minor) && minor <= maxMoney ? minor : null;
}
export function minorToInput(minor: number, currency: string) {
  const digits = minorDigits(currency);
  const whole = Math.floor(minor / 10 ** digits);
  const fraction = String(minor % 10 ** digits).padStart(digits, "0");
  return digits === 0 || (wholeOnly(currency) && Number(fraction) === 0)
    ? String(whole)
    : `${whole}.${fraction}`;
}
export const fractionDigits = (currency: string) => {
  try {
    return (
      new Intl.NumberFormat("en", {
        style: "currency",
        currency,
      }).resolvedOptions().maximumFractionDigits ?? 2
    );
  } catch {
    return 2;
  }
};
export function toMinor(input: string, currency: string): number | null {
  const digits = fractionDigits(currency);
  const m = /^(\d{1,12})(?:\.(\d{1,6}))?$/.exec(input.trim());
  const fraction = m?.[2] ?? "";
  if (!m || fraction.length > (wholeOnly(currency) ? 0 : digits)) return null;
  const minor = Number(m[1] + fraction.padEnd(digits, "0"));
  return Number.isSafeInteger(minor) && minor <= maxMoney ? minor : null;
}
export function fromMinor(minor: number, currency: string): string {
  const digits = fractionDigits(currency);
  if (digits === 0) return String(minor);
  const text = String(minor).padStart(digits + 1, "0");
  const whole = text.slice(0, -digits),
    fraction = text.slice(-digits);
  return wholeOnly(currency) && Number(fraction) === 0
    ? whole
    : `${whole}.${fraction}`;
}
const WALL = /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}$/;
export function taipeiToInstant(wall: string): string | null | undefined {
  const text = wall.trim();
  if (text === "") return null;
  if (!WALL.test(text)) return undefined;
  const at = Date.parse(`${text}:00+08:00`);
  return Number.isFinite(at) ? `${text}:00+08:00` : undefined;
}
export function instantToTaipei(iso: string | null): string {
  if (iso === null) return "";
  const parts = new Intl.DateTimeFormat("en-CA", {
    timeZone: STORE_TIME_ZONE,
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    hourCycle: "h23",
  }).formatToParts(new Date(iso));
  const get = (type: string) => parts.find((p) => p.type === type)?.value ?? "";
  return `${get("year")}-${get("month")}-${get("day")}T${get("hour")}:${get("minute")}`;
}
