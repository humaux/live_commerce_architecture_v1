// Purpose: rank public stores for one selected CVS brand without choosing or changing input.
// Depends on: Unicode normalization only; callers supply the frozen per-brand directory.
// Used by: storefront CvsStorePicker and CVS real-case/unit acceptance.
// Invariants: codes remain strings; pasted non-store lines never influence results; no I/O.
export type CvsBrand = "UNIMART" | "UNIMARTFREEZE" | "FAMI" | "HILIFE" | "OKMART";
export type CvsStore = { code: string; name: string; address: string; county: string; district: string };
export type CvsMatches = { query: string; code: string | null; codeMiss: string | null; brandMismatch: boolean; candidates: CvsStore[] };

const width = (value: string) => value.normalize("NFKC").toUpperCase();
const numbers: Record<string, string> = { 一: "1", 二: "2", 三: "3", 四: "4", 五: "5", 六: "6", 七: "7", 八: "8", 九: "9", 十: "10" };
const addressCharacters: Record<string, string> = { 臺: "台", 号: "號", 区: "區", 县: "縣", 乡: "鄉", 镇: "鎮", 楼: "樓" };
export function normalizeCvsText(value: string): string {
  return width(value).replace(/[臺号区县乡镇楼]/g, (character) => addressCharacters[character])
    .replace(/[一二三四五六七八九十]{1,3}(?=段|巷|弄|號|樓)/g, (digits) => {
      if (!digits.includes("十")) return [...digits].map((digit) => numbers[digit]).join("");
      const [tens, units] = digits.split("十");
      return String(Number(tens ? numbers[tens] : 1) * 10 + Number(units ? numbers[units] : 0));
    })
    .replace(/之|[－—–-]/g, "-").replace(/[\s,，、.。:：/()（）]/g, "");
}
const nameKey = (value: string) => normalizeCvsText(value)
  .replace(/^(?:全家|7-?ELEVEN|7-11|統一超商|统一超商|萊爾富|莱尔富|OK)/, "").replace(/(?:門市|门市)$/, "");
const compare = (a: string, b: string) => a < b ? -1 : a > b ? 1 : 0;

export function matchCvsStores(brand: string, query: string, stores: readonly CvsStore[]): CvsMatches {
  const result: CvsMatches = { query, code: null, codeMiss: null, brandMismatch: false, candidates: [] };
  const input = width(query).trim();
  if (!input) return result;
  // Only explicitly labeled STORE lines are extracted from a paste. Never rank by other lines.
  const labels = [...input.matchAll(/(?:收件)?[門门]市(?:代[號号]|[編编][號号]|名称|名稱|地址)[ \t]*[:：]?[ \t]*/g)];
  let codeText = "", name = "", address = "", labeled = false;
  for (let index = 0; index < labels.length; index++) {
    const item = labels[index];
    const value = input.slice(item.index! + item[0].length, labels[index + 1]?.index).split(/[\n\r]/)[0].trim();
    labeled = true;
    if (/[代編编][號号]/.test(item[0])) codeText = value;
    else if (/名[稱称]/.test(item[0])) name = value;
    else address = value;
  }
  const rawCode = labeled ? codeText.match(/^(?:F?\d{6}|\d{4}|[A-Z]\d{3})(?![A-Z0-9])/)?.[0] : /^(?:F?\d{6}|\d{4}|[A-Z]\d{3})$/.exec(input)?.[0];
  if (rawCode) {
    if (/^F\d{6}$/.test(rawCode) && brand !== "FAMI") {
      result.brandMismatch = true;
      return result;
    }
    const code = brand === "FAMI" ? rawCode.replace(/^F(?=\d{6}$)/, "") : rawCode;
    result.code = code;
    const valid = brand === "HILIFE" ? /^(?:\d{4}|[A-Z]\d{3})$/.test(code) : /^\d{6}$/.test(code);
    const exact = valid ? stores.find((store) => store.code === code) : undefined;
    if (exact) { result.candidates = [exact]; return result; }
    result.codeMiss = code;
    if (!labeled || (!name && !address)) return result;
  }
  const text = labeled ? "" : input;
  const wantedName = nameKey(name || text), wantedAddress = normalizeCvsText(address || text);
  const tokens = normalizeCvsText(address || text).split(/(?<=[縣市區鄉鎮路街段巷弄號])|[,，、\s]+/).filter(Boolean);
  const ranked = stores.map((store) => {
    const storeName = nameKey(store.name), storeAddress = normalizeCvsText(store.address);
    const nameQuery = !!name || !/[縣市區乡鄉鎮镇路街巷弄段號]/.test(wantedName);
    const byName = nameQuery && wantedName && (storeName.includes(wantedName) || wantedName.includes(storeName));
    const byAddress = wantedAddress && storeAddress.includes(wantedAddress);
    const hits = tokens.filter((token) => token.length >= 2 && storeAddress.includes(token)).length;
    const score = (byName ? 1000 : 0) + (byAddress ? 500 : 0) + (hits && hits === tokens.length ? 100 : 0) + hits;
    return { store, score };
  }).filter((item) => item.score > 0);
  ranked.sort((a, b) => b.score - a.score || compare(a.store.county, b.store.county) || compare(a.store.district, b.store.district) || compare(a.store.name, b.store.name) || compare(a.store.code, b.store.code));
  result.candidates = ranked.slice(0, 8).map((item) => item.store);
  return result;
}
