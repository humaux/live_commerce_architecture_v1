// Platform operator identity/config only. No BFF or Go calls, merchant branding, or credentials.
// CI/BRC facts: docs/delivery/units/platform-site.md. Keep names/address verbatim.
export const company = {
  productName: "DaWan Live",
  legalEnglish: "Hong Kong Da Wan Trading Limited",
  legalChinese: "香港大碗貿易有限公司",
  registrationNumber: "81215167",
  businessRegistrationNumber: "81215167-000-09-26-2",
  incorporatedOn: "2026-09-11",
  businessRegistrationValidity: "11/09/2026 – 10/09/2027",
  address:
    "RM 10, 23/F, New Trend Centre, 704 Prince Edward Road East, San Po Kong, Hong Kong",
} as const;
export const operatedBy = `${company.productName} is operated by ${company.legalEnglish}`;
export function brandedTitle(pageName: string): string {
  return `${company.productName} · ${pageName}`;
}
export const platformLocales = ["zh-TW", "zh-CN", "en"] as const;
export type PlatformLocale = (typeof platformLocales)[number];
export function operatorSentence(locale: PlatformLocale): string {
  if (locale === "zh-TW")
    return `${company.productName} 由 ${company.legalChinese} 營運`;
  if (locale === "zh-CN")
    return `${company.productName} 由 ${company.legalChinese} 运营`;
  return operatedBy;
}
export const platformPages = [
  "home",
  "privacy",
  "terms",
  "data-deletion",
  "contact",
] as const;
export type PlatformPage = (typeof platformPages)[number];

export function platformPath(
  locale: PlatformLocale,
  page: PlatformPage = "home",
): string {
  const prefix = locale === "zh-TW" ? "" : `/${locale}`;
  return `${prefix}${page === "home" ? "" : `/${page}`}` || "/";
}

type Env = Record<string, string | undefined>;
// Compare only the actual Host authority. DNS casing, a terminal dot and a
// numeric port do not change the hostname; URL/userinfo/forwarded syntax does.
export function requestHostname(
  value: string | null | undefined,
): string | null {
  const parsed = /^([^:]+)(?::([0-9]{1,5}))?$/.exec(value ?? "");
  if (!parsed || (parsed[2] && Number(parsed[2]) > 65535)) return null;
  try {
    return host(parsed[1].replace(/\.$/, ""), "Host");
  } catch {
    return null;
  }
}
function host(value: string | undefined, name: string): string {
  // Config, never the request or forwarded Host, supplies canonical/CTA origins.
  const parsed = value?.toLowerCase() ?? "";
  if (
    parsed.length > 253 ||
    !/^(?:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$/.test(
      parsed,
    )
  )
    throw new Error(`Invalid ${name}`);
  return parsed;
}

export function companyConfig(env: Env = process.env) {
  const email = env.LC_COMPANY_CONTACT_EMAIL ?? "";
  // This is a public address, not a mailbox credential. Never invent a default.
  if (
    email.length > 254 ||
    !/^[A-Za-z0-9.!#$%&'*+/=?^_`{|}~-]+@[A-Za-z0-9](?:[A-Za-z0-9.-]*[A-Za-z0-9])?\.[A-Za-z]{2,}$/.test(
      email,
    )
  )
    throw new Error("Invalid LC_COMPANY_CONTACT_EMAIL");
  const [local, domain] = email.split("@");
  if (
    local.length > 64 ||
    local.startsWith(".") ||
    local.endsWith(".") ||
    local.includes("..")
  )
    throw new Error("Invalid LC_COMPANY_CONTACT_EMAIL");
  host(domain, "LC_COMPANY_CONTACT_EMAIL");
  const platformHost = host(env.LC_PLATFORM_HOST, "LC_PLATFORM_HOST");
  const adminHost = host(env.LC_ADMIN_HOST, "LC_ADMIN_HOST");
  if (platformHost === adminHost || `www.${platformHost}` === adminHost)
    throw new Error("LC_ADMIN_HOST must differ from LC_PLATFORM_HOST");
  const verification = env.LC_META_DOMAIN_VERIFICATION || undefined;
  if (verification && !/^[A-Za-z0-9_-]{1,256}$/.test(verification))
    throw new Error("Invalid LC_META_DOMAIN_VERIFICATION");
  return {
    email,
    platformHost,
    adminHost,
    platformOrigin: `https://${platformHost}`,
    adminOrigin: `https://${adminHost}`,
    verification,
  };
}

export function platformRoute(
  path: string,
): { locale: PlatformLocale; page: PlatformPage } | null {
  const parts = path.split("/").slice(1);
  if (parts.at(-1) === "") parts.pop();
  let locale: PlatformLocale = "zh-TW";
  if (platformLocales.some((item) => item === parts[0]))
    locale = parts.shift() as PlatformLocale;
  if (!parts.length) return { locale, page: "home" };
  if (
    parts.length === 1 &&
    platformPages.some((item) => item !== "home" && item === parts[0])
  )
    return { locale, page: parts[0] as PlatformPage };
  return null;
}
