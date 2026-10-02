// MOCK: a contract-shaped fake of the Go private buyer API, for the storefront shell browser gate (SF*) while the producers
// (catalog-core 0086, store-design 0087) are not merged into the integration branch. Label every result that uses it MOCK.
// It serves exactly the shapes of contracts/storefront-v2.md sections A and B-acceptance plus the session/cart/catalog/
// checkout-options/quotes routes the existing buyer BFF forwards to (apps/storefront/lib/buyer-server.ts). It is NOT the
// real API: no PG, no RLS, no real stock. Order placement is not implemented (SF gate covers up to the quote + address form;
// order placement is covered against the real stack by the existing order gates and the tester's production-shape run).
// Trust shape mirrored from Go: public reads need the BFF key header + X-Commerce-Storefront-Origin equal to the one published
// origin, otherwise 404; session routes also need a Bearer token that was bootstrapped.
import http from "node:http";
import zlib from "node:zlib";
import { createHash, randomUUID } from "node:crypto";

export const PREVIEW_TOKEN = "P".repeat(43);
const CURRENCY = "TWD";
const uid = (n) => `00000000-0000-4000-8000-${String(n).padStart(12, "0")}`;
const hashId = (text, prefix) => {
  const h = createHash("sha256").update(text).digest("hex");
  return `${prefix}${h.slice(1, 8)}-${h.slice(8, 12)}-4${h.slice(13, 16)}-8${h.slice(17, 20)}-${h.slice(20, 32)}`;
};

// ---- procedural "photos": soft gradients with one lit object, deterministic per seed --------------------------------------
const PALETTES = [
  ["#e9dfd1", "#c9b79c", "#8b6f4e"], ["#dfe7e4", "#a9bdb6", "#5f7d73"], ["#efe2dc", "#d8b4a6", "#a4695a"],
  ["#e3e6ee", "#b4bdd4", "#6a7aa3"], ["#ece8d8", "#cfc79b", "#8f8648"], ["#e8dde6", "#c7a8c3", "#8a5d85"],
];
const hex = (h) => [parseInt(h.slice(1, 3), 16), parseInt(h.slice(3, 5), 16), parseInt(h.slice(5, 7), 16)];
const mix = (a, b, t) => a.map((x, i) => Math.round(x + (b[i] - x) * t));
function png(width, height, pixel) {
  const raw = Buffer.alloc((width * 3 + 1) * height);
  for (let y = 0; y < height; y++) {
    const row = y * (width * 3 + 1);
    raw[row] = 0;
    for (let x = 0; x < width; x++) {
      const [r, g, b] = pixel(x / width, y / height);
      raw[row + 1 + x * 3] = r; raw[row + 2 + x * 3] = g; raw[row + 3 + x * 3] = b;
    }
  }
  const chunk = (type, data) => {
    const body = Buffer.concat([Buffer.from(type), data]);
    const len = Buffer.alloc(4); len.writeUInt32BE(data.length);
    const crc = Buffer.alloc(4); crc.writeUInt32BE(zlib.crc32(body) >>> 0);
    return Buffer.concat([len, body, crc]);
  };
  const ihdr = Buffer.alloc(13); ihdr.writeUInt32BE(width, 0); ihdr.writeUInt32BE(height, 4); ihdr[8] = 8; ihdr[9] = 2;
  return Buffer.concat([Buffer.from([137, 80, 78, 71, 13, 10, 26, 10]), chunk("IHDR", ihdr), chunk("IDAT", zlib.deflateSync(raw, { level: 6 })), chunk("IEND", Buffer.alloc(0))]);
}
const cache = new Map();
function photo(seed, kind) {
  const key = `${seed}:${kind}`;
  if (cache.has(key)) return cache.get(key);
  const n = parseInt(createHash("sha256").update(seed).digest("hex").slice(0, 8), 16);
  const [bg, mid, fg] = PALETTES[n % PALETTES.length].map(hex);
  const shape = (n >> 3) % 3;
  const [w, h] = kind === "wide" ? [960, 540] : kind === "square" ? [640, 480] : [480, 600];
  const cx = 0.5 + (((n >> 5) % 20) - 10) / 100, cy = kind === "wide" ? 0.55 : 0.58, size = kind === "wide" ? 0.28 : 0.3;
  const out = png(w, h, (u, v) => {
    const base = mix(bg, mid, Math.min(1, Math.max(0, (v - 0.1) * 1.1)));
    const aspect = w / h;
    const dx = (u - cx) * aspect, dy = v - cy;
    let d;
    if (shape === 0) d = Math.hypot(dx, dy) - size; // round object
    else if (shape === 1) d = Math.max(Math.abs(dx) - size * 0.62, Math.abs(dy) - size * 0.95); // tall block
    else d = Math.hypot(dx, dy * 1.5) - size * 1.15; // low wide object
    const floor = v > (kind === "wide" ? 0.82 : 0.8) ? 0.12 : 0;
    const shadow = Math.max(0, 1 - Math.hypot(dx * 0.7, (v - (cy + size * 0.95)) * 5)) * 0.25;
    let c = mix(base, [0, 0, 0], floor + shadow);
    if (d < 0) { const lit = 0.35 + 0.65 * Math.min(1, Math.max(0, 0.6 - dx * 0.8 - dy * 0.5)); c = mix(fg, [255, 255, 255], lit * 0.35); }
    else if (d < 0.012) c = mix(c, fg, 0.6);
    return c;
  });
  cache.set(key, out);
  return out;
}

// ---- catalog data ------------------------------------------------------------------------------------------------------
const money = (twd) => twd * 100;
const COLLECTIONS = [
  { id: uid(40), slug: "home-fragrance", title: "居家香氛", image: uid(44) },
  { id: uid(41), slug: "knitwear", title: "針織與配件", image: null },
  { id: uid(42), slug: "tableware", title: "餐桌器皿", image: null },
];
// [title, slug, price TWD, compare-at TWD|null, axes, stock by variant index, collections, description]
const BASE = [
  ["雪松無花果香氛蠟燭", "cedar-fig-candle", 680, null, [["容量", ["180g", "300g"]]], ["in", "low"], ["home-fragrance"], "以大豆蠟手工灌注，燃燒時帶有雪松木的乾燥溫度與熟無花果的微甜。\n\n燃燒時間約 40 小時（300g）。\n請在平穩的桌面上使用，首次點燃建議燃燒至表面全融。"],
  ["手沖陶瓷濾杯組", "ceramic-dripper-set", 1280, null, [], ["in"], ["tableware"], "一體成形的陶瓷濾杯，搭配同色分享壺。釉面為窯變霧白，每件略有深淺差異。\n\n適用 1–2 人份手沖。"],
  ["羊毛混紡針織圍巾", "wool-knit-scarf", 1480, 1980, [["顏色", ["燕麥", "墨黑", "霧藍"]]], ["in", "out", "low"], ["knitwear"], "70% 羊毛、30% 蠶絲混紡，輕盈保暖，不刺膚。\n\n尺寸 180 × 32 公分，附棉質收納袋。"],
  ["亞麻餐巾 四入組", "linen-napkins-4", 560, null, [["顏色", ["原色", "灰藍"]]], ["in", "in"], ["tableware"], "預洗亞麻，越洗越柔軟。邊緣以手工捲邊處理。\n\n每片 42 × 42 公分。"],
  ["手作釉面餐盤", "glazed-plate", 780, null, [["尺寸", ["21cm", "27cm"]]], ["in", "in"], ["tableware"], "圓潤的盤緣讓盛裝更服貼。可進烤箱與洗碗機。"],
  ["擴香竹籤禮盒", "reed-diffuser-box", 890, null, [], ["out"], ["home-fragrance"], "100ml 擴香瓶搭配六支天然籐枝，附牛皮紙禮盒。"],
  ["純棉針織襪 三雙組", "cotton-socks-3", 420, null, [["顏色", ["米白", "灰", "黑"]], ["尺寸", ["S", "M", "L"]]], ["in", "in", "in", "low", "in", "out", "in", "in", "in"], ["knitwear"], "有機棉混紡，足弓加強支撐。"],
  ["玻璃水杯 雙入", "glass-tumbler-2", 640, null, [], ["in"], ["tableware"], "手工吹製，杯身有細微氣泡紋理。容量 320ml。"],
  ["橄欖木砧板", "olive-board", 1680, 2080, [], ["low"], ["tableware"], "整塊橄欖木，紋理自然。使用後請擦乾並定期上油。"],
  ["羊毛氈杯墊 四入", "felt-coasters", 360, null, [], ["in"], ["knitwear", "tableware"], "3mm 厚羊毛氈，吸水且防滑。"],
  ["迷你香氛噴霧", "mini-room-spray", 480, null, [["香調", ["雪松", "柑橘", "白茶"]]], ["in", "in", "in"], ["home-fragrance"], "50ml 隨身尺寸，可噴灑於織品與空間。"],
  ["手織束口袋", "woven-pouch", 520, null, [["顏色", ["米", "藍"]]], ["in", "low"], ["knitwear"], "以棉線手織，適合收納耳機、首飾與小物。"],
  ["琺瑯牛奶鍋", "enamel-milk-pan", 1980, null, [], ["in"], ["tableware"], "14cm 單柄牛奶鍋，附木質把手。"],
  ["泡泡紗拭手巾", "waffle-hand-towel", 260, null, [], ["in"], [], "雙層泡泡紗，快乾柔軟。"],
];
const EXTRA = ["燕麥色陶瓷湯匙", "黃銅燭台", "手工蜂蠟蠟燭", "棉麻桌旗", "針織髮帶", "石材香氛擴香石", "陶製筷架組", "羊毛短襪", "木質餐匙", "玻璃花瓶", "草編杯墊", "麻布購物袋", "山茶皂", "亞麻圍裙", "手沖細口壺", "鑄鐵小烤盤"];
export function buildCatalog() {
  const products = BASE.map(([title, slug, price, compare, axes, stocks, cols, description], i) => {
    const id = uid(100 + i);
    const combos = axes.length === 0 ? [[]] : axes.length === 1 ? axes[0][1].map((v) => [v]) : axes[0][1].flatMap((a) => axes[1][1].map((b) => [a, b]));
    const variants = combos.map((values, j) => ({
      sku_id: uid(1000 + i * 20 + j), sku_code: `${slug.slice(0, 3).toUpperCase()}-${String(j + 1).padStart(2, "0")}`,
      title: values.length ? values.join(" / ") : "預設", option_values: values,
      price_minor: money(price + (axes.length === 1 && axes[0][0] === "容量" ? j * 300 : axes[0]?.[0] === "尺寸" ? j * 500 : 0)),
      compare_at_minor: compare ? money(compare) : null, stock: stocks[j] ?? "in",
    }));
    return { id, slug, title, description, collections: cols, options: axes.map(([name, values]) => ({ name, values })), variants, image_count: i % 4 === 2 ? 3 : i % 3 === 0 ? 2 : 1, order: i };
  });
  EXTRA.forEach((title, k) => {
    const i = BASE.length + k;
    products.push({
      id: uid(100 + i), slug: `item-${i + 1}`, title, description: `${title}，生活選物系列。`, collections: [COLLECTIONS[k % 3].slug], options: [],
      variants: [{ sku_id: uid(1000 + i * 20), sku_code: `ITM-${i + 1}`, title: "預設", option_values: [], price_minor: money(220 + k * 70), compare_at_minor: null, stock: k % 7 === 3 ? "out" : "in" }],
      image_count: 1, order: i,
    });
  });
  for (const p of products) p.images = Array.from({ length: p.image_count }, (_, n) => ({ id: hashId(`${p.id}:${n}`, "a"), width: 480, height: 600 }));
  return products;
}

function design(extra = {}) {
  const hero = hashId("hero", "b"), about = hashId("about", "b"), logo = hashId("logo", "b");
  return {
    profile: {
      name: "晨光選物", tagline: "為日常挑選安靜好用的小物", logo_image_id: null, favicon_image_id: null, accent_color: "#2f6b5a",
      announcement: "全館滿 NT$1,500 免運 · 週四晚間 8 點直播開賣", ...extra.profile,
      contact: { email: "hello@morninglight.example", phone: "02-2345-6789", address: "台北市大安區和平東路一段 100 號", line_url: "https://line.me/R/ti/p/@morninglight", facebook_url: "https://www.facebook.com/morninglight.example", instagram_url: "https://www.instagram.com/morninglight.example" },
    },
    nav: {
      header: [{ label: "全部商品", kind: "all_products", target: null }, { label: "居家香氛", kind: "collection", target: "home-fragrance" }, { label: "餐桌器皿", kind: "collection", target: "tableware" }, { label: "關於我們", kind: "page", target: "about" }],
      footer: [{ label: "關於我們", kind: "page", target: "about" }, { label: "運送說明", kind: "page", target: "shipping" }, { label: "商品分類", kind: "collection", target: "knitwear" }],
    },
    home: {
      sections: [
        { type: "hero", image_id: hero, heading: extra.heading ?? "秋日餐桌，慢一點", subheading: "從手沖濾杯到亞麻餐巾，一組安靜的日常選物。", cta_label: "探索新品", cta_kind: "all_products", cta_target: null },
        { type: "featured_collection", collection_slug: "home-fragrance", heading: "居家香氛", limit: 8 },
        { type: "rich_text", heading: "我們的選物原則", body: "每一件商品都經過**至少一個月**的日常使用測試。\n\n- 材質誠實，不過度包裝\n- 可修補、可長久使用\n- 與小型工坊直接合作\n\n了解更多請見 [關於我們](https://example.com/about)。" },
        { type: "product_grid", heading: "最新上架", sort: "newest", limit: 8 },
        { type: "image_text", image_id: about, heading: "來自台灣小工坊", body: "我們走訪陶藝、織品與木作工坊，把**真正好用**的東西帶給你。", image_side: "right" },
      ],
    },
    pages: [
      { slug: "about", title: "關於我們", body: "晨光選物是一間位於台北的小型選物店。\n\n我們相信**日常的物件**值得被好好挑選。\n\n- 每週四晚間 8 點直播\n- 全台超商取貨" },
      { slug: "shipping", title: "運送說明", body: "訂單在 **1–3 個工作天**內出貨。\n\n超商取貨與宅配皆可選擇。" },
    ],
    _ids: { hero, about, logo },
  };
}

// ---- server -------------------------------------------------------------------------------------------------------------
export function createFakeApi({ port = 0, origin = "https://shop.example", bffKey }) {
  const products = buildCatalog();
  const state = { requests: [], sessions: new Map(), carts: new Map(), receipts: new Map(), quotes: new Map(), unpublished: false, down: false, hideProductSlug: null, imagelessSlug: null, hiddenCollectionSlug: null, emptyCollectionSlug: null, designAccent: null };
  const idsOf = design()._ids;
  const imageSeeds = new Map([[idsOf.hero, ["hero", "wide"]], [idsOf.about, ["about", "square"]], [idsOf.logo, ["logo", "square"]], [uid(44), ["collection", "square"]]]);
  for (const p of products) for (const img of p.images) imageSeeds.set(img.id, [img.id, "portrait"]);
  const live = () => products.filter((p) => p.slug !== state.hideProductSlug);

  const json = (res, status, value, extra = {}) => {
    res.writeHead(status, { "Content-Type": "application/json", "Cache-Control": "no-store", "X-Content-Type-Options": "nosniff", ...extra });
    res.end(JSON.stringify(value));
  };
  const error = (res, status, code) => json(res, status, { code, message: code, request_id: "0".repeat(32), retryable: false, details: {} });
  const card = (p) => {
    const buyable = p.variants.filter((v) => v.stock !== "out"), pool = buyable.length ? buyable : p.variants;
    const prices = pool.map((v) => v.price_minor), cmp = pool.filter((v) => v.compare_at_minor && v.compare_at_minor > v.price_minor).map((v) => v.compare_at_minor);
    return { id: p.id, slug: p.slug, title: p.title, price_min_minor: Math.min(...prices), price_max_minor: Math.max(...prices), compare_at_min_minor: cmp.length ? Math.min(...cmp) : null, cover_image_id: p.slug === state.imagelessSlug ? null : p.images[0]?.id ?? null, in_stock: buyable.length > 0 };
  };
  const cartOf = (token) => state.carts.get(token) ?? { id: "", currency: CURRENCY, version: 0, items: [] };
  const skuIndex = () => new Map(products.flatMap((p) => p.variants.map((v) => [v.sku_id, { p, v }])));

  const server = http.createServer(async (req, res) => {
    const url = new URL(req.url, "http://fake");
    const chunks = []; for await (const c of req) chunks.push(c);
    const bodyText = Buffer.concat(chunks).toString("utf8");
    state.requests.push(`${req.method} ${url.pathname}${url.search}`);
    if (state.down) return error(res, 503, "unavailable");
    if (req.headers["x-commerce-buyer-bff-key"] !== bffKey) return error(res, 403, "forbidden");
    if (!url.pathname.startsWith("/v1/buyer/")) return error(res, 404, "not_found");
    const path = url.pathname.slice("/v1/buyer/".length);
    const sameOrigin = req.headers["x-commerce-storefront-origin"] === origin && !state.unpublished;
    const bearer = /^Bearer (.+)$/.exec(req.headers.authorization ?? "")?.[1];

    // public reads (no bearer)
    if (path === "design/published" || path === "design/preview") {
      if (!sameOrigin) return error(res, 404, "not_found");
      if (path === "design/preview" && req.headers["x-commerce-design-preview"] !== PREVIEW_TOKEN) return error(res, 404, "not_found");
      const d = design(path === "design/preview" ? { heading: "【草稿】冬季新系列預告", profile: { announcement: "草稿預覽：新年檔期倒數中" } } : {});
      if (state.designAccent) d.profile.accent_color = state.designAccent;
      delete d._ids;
      return json(res, 200, { version: path === "design/preview" ? 3 : 2, document: d });
    }
    const media = /^media\/(p\/[^/]+\/([^/]+)|s\/([^/]+)|c\/[^/]+\/([^/]+))$/.exec(path);
    if (media) {
      if (!sameOrigin) return error(res, 404, "not_found");
      const imageID = media[2] ?? media[3] ?? media[4];
      const seed = imageSeeds.get(imageID);
      if (!seed) return error(res, 404, "not_found");
      const bytes = photo(...seed);
      res.writeHead(200, { "Content-Type": "image/png", "Content-Length": bytes.length, "Cache-Control": "public, max-age=86400, immutable" });
      return res.end(bytes);
    }
    if (path.startsWith("catalog/v2/")) {
      if (!sameOrigin) return error(res, 404, "not_found");
      const rest = path.slice("catalog/v2/".length);
      if (rest === "products") {
        const q = url.searchParams, allowed = new Set(["collection", "q", "sort", "min", "max", "after", "limit"]);
        for (const k of q.keys()) if (!allowed.has(k)) return error(res, 422, "invalid_request");
        let list = live();
        if (q.get("collection")) { if (!COLLECTIONS.some((c) => c.slug === q.get("collection"))) return error(res, 404, "not_found"); list = list.filter((p) => p.collections.includes(q.get("collection"))); }
        if (q.get("q")) { const needle = q.get("q").toLowerCase(); list = list.filter((p) => p.title.toLowerCase().includes(needle) || p.description.toLowerCase().includes(needle) || p.variants.some((v) => v.sku_code.toLowerCase().includes(needle))); }
        const cards = list.map((p) => ({ p, c: card(p) }));
        const min = q.has("min") ? Number(q.get("min")) : null, max = q.has("max") ? Number(q.get("max")) : null;
        let out = cards.filter(({ c }) => (min === null || c.price_min_minor >= min) && (max === null || c.price_min_minor <= max));
        const sort = q.get("sort") ?? "newest";
        out.sort((a, b) => sort === "price_asc" ? a.c.price_min_minor - b.c.price_min_minor : sort === "price_desc" ? b.c.price_min_minor - a.c.price_min_minor : sort === "title" ? a.p.title.localeCompare(b.p.title) : b.p.order - a.p.order);
        const limit = Number(q.get("limit") ?? 24), offset = q.get("after") ? Number(Buffer.from(q.get("after"), "base64url").toString()) : 0;
        const slice = out.slice(offset, offset + limit);
        return json(res, 200, { store: { name: "晨光選物", currency: CURRENCY }, products: slice.map(({ c }) => c), next: offset + limit < out.length ? Buffer.from(String(offset + limit)).toString("base64url") : null });
      }
      if (rest === "collections") return json(res, 200, { collections: COLLECTIONS.filter(c => c.slug !== state.hiddenCollectionSlug).map((c) => ({ id: c.id, slug: c.slug, title: c.title, image_id: c.image, product_count: c.slug === state.emptyCollectionSlug ? 0 : live().filter((p) => p.collections.includes(c.slug)).length })) });
      const col = /^collections\/([a-z0-9-]+)$/.exec(rest);
      if (col) { const c = COLLECTIONS.find((x) => x.slug === col[1]); return c ? json(res, 200, { id: c.id, slug: c.slug, title: c.title, description: `${c.title}系列，每件都經過日常使用測試。`, image_id: c.image }) : error(res, 404, "not_found"); }
      const one = /^products\/([a-z0-9-]+)$/.exec(rest);
      if (one) {
        const p = live().find((x) => x.slug === one[1] || x.id === one[1]);
        if (!p) return error(res, 404, "not_found");
        return json(res, 200, { id: p.id, slug: p.slug, title: p.title, description: p.description, seo: { title: "", description: "" }, images: p.slug === state.imagelessSlug ? [] : p.images, options: p.options, variants: p.variants.map(({ sku_id, title, option_values, price_minor, compare_at_minor, stock }) => ({ sku_id, title, option_values, price_minor, compare_at_minor, stock })), collections: p.collections.map((s) => ({ slug: s, title: COLLECTIONS.find((c) => c.slug === s).title })) });
      }
      return error(res, 404, "not_found");
    }

    // session-bound routes (the existing BFF contract)
    if (!sameOrigin && !path.startsWith("session")) return error(res, 404, "not_found");
    if (path === "session/bootstrap" && req.method === "POST") { if (bearer) state.sessions.set(bearer, true); return json(res, 200, { authenticated: true, expires_at: new Date(Date.now() + 3600e3).toISOString() }); }
    if (path === "session" && req.method === "GET") return state.sessions.has(bearer) ? json(res, 200, { authenticated: true }) : error(res, 401, "unauthorized");
    if (path === "session/retire") { state.sessions.delete(bearer); res.writeHead(204); return res.end(); }
    if (!state.sessions.has(bearer)) return error(res, 401, "unauthorized");
    const index = skuIndex();
    if (path === "catalog") {
      const after = url.searchParams.get("cursor") ? Number(Buffer.from(url.searchParams.get("cursor"), "base64url").toString()) : 0;
      const limit = Math.min(100, Number(url.searchParams.get("limit") ?? 100));
      const rows = live().flatMap((p) => p.variants.filter((v) => !(p.variants.length === 1 && false)).map((v) => ({ product_id: p.id, sku_id: v.sku_id, name: p.title, description: p.description, sku_code: v.sku_code, currency: CURRENCY, price_minor: v.price_minor, images: p.slug === state.imagelessSlug ? [] : p.images })));
      const slice = rows.slice(after, after + limit);
      return json(res, 200, { items: slice, next_cursor: after + limit < rows.length ? Buffer.from(String(after + limit)).toString("base64url") : "", store_name: "晨光選物" });
    }
    if (path === "cart" && req.method === "GET") return json(res, 200, cartOf(bearer));
    if (path === "cart" && req.method === "PUT") {
      const key = req.headers["idempotency-key"], body = JSON.parse(bodyText || "{}");
      const receiptKey = `${bearer}:${key}`;
      if (state.receipts.has(receiptKey)) return json(res, 200, state.receipts.get(receiptKey));
      const current = cartOf(bearer);
      if (body.expected_version !== current.version) return error(res, 409, "conflict");
      for (const item of body.items) if (!index.has(item.sku_id)) return error(res, 422, "invalid_request");
      const next = { id: current.id || randomUUID(), currency: CURRENCY, version: current.version + 1, items: body.items };
      state.carts.set(bearer, next); state.receipts.set(receiptKey, next);
      return json(res, 200, next);
    }
    if (path.startsWith("checkout-options")) {
      return json(res, 200, { items: [{ market_id: uid(7), country: "TW", currency: CURRENCY, method: "delivery:home_tw", delivery_kind: "home", service_version: 1, allocation_version: 1, mode: "MANUAL", name_hans: "台湾宅配", name_hant: "台灣宅配", name_en: "Taiwan home delivery", sort_order: 1, free_shipping_threshold_minor: money(1500) }], next_cursor: "" });
    }
    if (path === "quotes" && req.method === "POST") {
      const body = JSON.parse(bodyText || "{}"), cart = cartOf(bearer);
      if (body.cart_version !== cart.version || cart.items.length === 0) return error(res, 409, "conflict");
      const lines = cart.items.map((i) => { const { p, v } = index.get(i.sku_id); return { sku_id: i.sku_id, name: p.title, code: v.sku_code, quantity: i.quantity, unit_price_minor: v.price_minor }; });
      const subtotal = lines.reduce((s, l) => s + l.unit_price_minor * l.quantity, 0), shipping = subtotal >= money(1500) ? 0 : money(80);
      const quote = { id: randomUUID(), cart_id: cart.id, cart_version: cart.version, market_id: body.market_id, country: body.country, method: body.method, currency: CURRENCY, expires_at: new Date(Date.now() + 1800e3).toISOString(), lines, amount: { subtotal_minor: subtotal, discount_minor: 0, shipping_minor: shipping, shipping_tax_minor: 0, tax_minor: 0, total_minor: subtotal + shipping } };
      state.quotes.set(quote.id, quote);
      return json(res, 200, quote);
    }
    const quoteGet = /^quotes\/([0-9a-f-]{36})$/.exec(path);
    if (quoteGet) return state.quotes.has(quoteGet[1]) ? json(res, 200, state.quotes.get(quoteGet[1])) : error(res, 404, "not_found");
    if (path.startsWith("destination")) return error(res, 404, "not_found");
    return error(res, 404, "not_found");
  });
  return {
    state, products, server, design: () => design(),
    listen: () => new Promise((resolve) => server.listen(port, "127.0.0.1", () => resolve(server.address().port))),
    close: () => new Promise((resolve) => { server.closeAllConnections?.(); server.close(resolve); }),
  };
}

if (import.meta.url === `file://${process.argv[1]}`) {
  const api = createFakeApi({ port: Number(process.env.PORT ?? 4900), bffKey: process.env.COMMERCE_BUYER_BFF_KEY });
  console.log(`MOCK buyer API on :${await api.listen()}`);
}
