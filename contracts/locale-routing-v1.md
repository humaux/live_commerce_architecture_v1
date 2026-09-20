# Locale routing v1 — explicit user choice

Confirmed 2026-09-20: both admin and storefront support `zh-CN`, `zh-TW`, `en`; user may switch freely. This is not geo-forced language and not a change of market/currency/timezone/store.

Shared package `packages/i18n` has no runtime dependencies. Next apps consume it; no duplicated route files per language. Root owns package-manager lock files. Node24 built-in test runner is enough for pure routing logic.

## Frozen exports (`src/index.ts`)

- `locales` readonly tuple `['zh-CN','zh-TW','en']`; `type Locale = typeof locales[number]`.
- `localeNames: Record<Locale,string>` self-names 简体中文/繁體中文/English.
- `isLocale(value:unknown):value is Locale` strict canonical whitelist.
- `resolveLocale(input:{pathname:string;preference?:string;acceptLanguage?:string;defaultLocale:Locale}):Locale`. Explicit canonical URL segment wins, then valid preference, then quality-sorted Accept-Language (q0 excluded; matching en-*, zh-Hans/zh-CN/zh-SG→zh-CN, zh-Hant/zh-TW/zh-HK/zh-MO→zh-TW, bare zh→zh-CN), then provided default. Bound header length4096 and at most32 ranges; malformed q ignored; never IP lookup. API/static paths never redirected by this package.
- `localizedPath(locale:Locale,path:string):string` replaces only recognized first locale segment, otherwise prefixes locale. Preserve safe relative path/query/hash; reject absolute/protocol-relative URL, backslash/control chars, encoded slash/backslash in pathname and malformed %-encoding, traversal dot segments (including encoded), URL length>4096. Throw RangeError on invalid locale/path. Does not remove/add tokens or persist secrets. Root `/` becomes `/<locale>`.
- `localeFromPath(pathname:string):Locale|undefined` recognizes exact canonical first segment only, not substring; may only take path (no absolute origin).
- `messages: Record<Locale,Messages>` strict identical keys, no empty values. `type MessageKey=keyof Messages`; `translate(locale:Locale,key:MessageKey):string`. Translation keys for initial merchant catalog UI: appName, products, inventory, warehouses, language, createProduct, name, description, sku, price, currency, available, reserved, onHand, allocated, unavailable, save, cancel, loading, emptyProducts, emptyInventory, failedRequest, sessionRequired, retry, archived, active, productCreated, adjustInventory, quantity, reason, expectedVersion, conflict, permissionDenied, notConnected, localEnvironment, storefront, signIn, signOut. No translating customer-entered product names or claiming unavailable capabilities.

Next wiring acceptance later: `app/[locale]/...`, `<html lang>`, navigation switch preserves route/state and stores locale preference only. Unsupported route locale fails404 or single canonical redirect, never loops; private/admin/cart replies no-store. Storefront public canonical/hreflang include trusted store domain; no cross-store host guessing. URL explicit language must not be overridden by browser/cookie.

Tests now: all three languages, malformed/missing preference, quality ordering/zero, Chinese script-region mappings, invalid ranges, fallback, deep links/query/hash preservation, dangerous redirect/traversal rejection, exact dictionary parity, no empty translations. Browser switch/accessibility/layout tests remain separate, not satisfied by these unit tests.
