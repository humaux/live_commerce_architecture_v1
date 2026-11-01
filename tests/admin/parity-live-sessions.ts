// Purpose: Independent signed-frame copy, real-click ledger and screenshot oracle for C4.
// Depends on: Playwright, node:fs/promises and the owning spec's isolated evidence directory.
// Used by: studio-ui, claims-ui, live-console, attribution, meta-connect, meta-connect-gate, meta-health-ui, customers-billing and e2e specs.
// Invariants: real routes only; screenshots supplement clicks; no product state or DOM mutation.
import { expect, test, type Page, type Locator } from "@playwright/test";
import { mkdir, writeFile } from "node:fs/promises";

export const parityWords = {
  "zh-TW": {
    sessions: "直播場次", create: "建立場次", copyLast: "複製上一場", all: "全部", preparing: "準備中", live: "直播中", ended: "已結束",
    prepare: "直播準備", preflight: "開播前準備", check: "檢查準備", source: "1 · 確認直播來源", device: "2 · 在直播工具檢查設備", productsCheck: "3 · 核對商品與回覆",
    console: "開啟直播控台", connections: "管理主頁連接", settings: "查看本場設定", templates: "查看訊息範本", back: "返回場次",
    after: "直播後", copy: "複製成下一場", copyName: "新場次名稱", copyReview: "預覽下一場草稿", copied: "下一場草稿", openPrep: "開啟準備清單",
    products: "本場商品", stock: "調整直播庫存", plus: "+1 立即生效", minus: "−1 立即生效", available: "目前可售", adjusted: "調整後可售",
    end: "結束直播", continue: "繼續直播", confirmEnd: "確認結束", keyword: "編輯預設留言口令", keywordInput: "新的預設口令", preview: "預覽變更", remove: "預覽移除",
    review: "直播復盤", funnel: "從留言到收款", timeline: "同一時間軸", timelineTitle: "直播同一時間軸", causal: "時間相近不等於因果", audience: "觀眾彙總", audienceTitle: "直播觀眾彙總", audienceRead: "讀取觀眾彙總",
    meta: "Facebook 與 Instagram", add: "新增主頁", detail: "查看連接與權限", detailTitle: "這個主頁的連接", intro: "在 Meta 完成授權", official: "前往 Meta 授權", bridge: "官方授權，返回後再確認", pick: "選擇這次要連接的主頁", pickReview: "核對這次選擇", confirm: "確認這次連接", confirmConnect: "確認連接", edit: "返回修改", result: "操作結果", returnList: "返回連接列表", disconnect: "中斷此主頁連接", disconnectTitle: "中斷這個主頁？", keep: "保留連接", confirmDisconnect: "確認中斷", uncertain: "先核對這次操作", readResult: "查詢這次操作結果", health: "查看這個主頁的功能檢查", healthTitle: "功能逐項看", recheck: "安排重新檢查",
  },
  "zh-CN": {
    sessions: "直播场次", create: "创建场次", copyLast: "复制上一场", all: "全部", preparing: "准备中", live: "直播中", ended: "已结束",
    prepare: "直播准备", preflight: "开播前准备", check: "检查准备", source: "1 · 确认直播来源", device: "2 · 在直播工具检查设备", productsCheck: "3 · 核对商品与回复",
    console: "打开直播控制台", connections: "管理主页连接", settings: "查看本场设置", templates: "查看消息模板", back: "返回场次",
    after: "直播后", copy: "复制为下一场", copyName: "新场次名称", copyReview: "预览下一场草稿", copied: "下一场草稿", openPrep: "打开准备清单",
    products: "本场商品", stock: "调整直播库存", plus: "+1 立即生效", minus: "−1 立即生效", available: "当前可售", adjusted: "调整后可售",
    end: "结束直播", continue: "继续直播", confirmEnd: "确认结束", keyword: "编辑默认评论口令", keywordInput: "新的默认口令", preview: "预览更改", remove: "预览移除",
    review: "直播复盘", funnel: "从评论到收款", timeline: "同一时间轴", timelineTitle: "直播同一时间轴", causal: "时间相近不代表因果", audience: "观众汇总", audienceTitle: "直播观众汇总", audienceRead: "读取观众汇总",
    meta: "Facebook 与 Instagram", add: "添加主页", detail: "查看连接与权限", detailTitle: "这个主页的连接", intro: "在 Meta 完成授权", official: "前往 Meta 授权", bridge: "官方授权，返回后再确认", pick: "选择本次要连接的主页", pickReview: "核对本次选择", confirm: "确认本次连接", confirmConnect: "确认连接", edit: "返回修改", result: "操作结果", returnList: "返回连接列表", disconnect: "断开此主页连接", disconnectTitle: "断开这个主页？", keep: "保留连接", confirmDisconnect: "确认断开", uncertain: "先核对本次操作", readResult: "查询本次操作结果", health: "查看这个主页的功能检查", healthTitle: "逐项查看功能", recheck: "安排重新检查",
  },
  en: {
    sessions: "Live sessions", create: "Create session", copyLast: "Copy last session", all: "All", preparing: "Preparing", live: "Live", ended: "Ended",
    prepare: "Live preparation", preflight: "Before going live", check: "Check preparation", source: "1 · Confirm the live source", device: "2 · Check devices in your streaming tool", productsCheck: "3 · Check products and replies",
    console: "Open live console", connections: "Manage Page connections", settings: "View session settings", templates: "View message templates", back: "Back to sessions",
    after: "After the live", copy: "Copy to next session", copyName: "New session name", copyReview: "Preview next draft", copied: "Next session draft", openPrep: "Open preparation checklist",
    products: "Session products", stock: "Adjust live stock", plus: "+1 applies immediately", minus: "−1 applies immediately", available: "Available now", adjusted: "Available after adjustment",
    end: "End session", continue: "Continue live", confirmEnd: "Confirm end", keyword: "Edit default comment keyword", keywordInput: "New default keyword", preview: "Preview changes", remove: "Preview removal",
    review: "Live review", funnel: "From comments to payments", timeline: "Shared timeline", timelineTitle: "Live timeline", causal: "Similar timing does not prove causation", audience: "Audience summary", audienceTitle: "Live audience summary", audienceRead: "Read audience summary",
    meta: "Facebook and Instagram", add: "Add Page", detail: "View connection and permissions", detailTitle: "This Page’s connection", intro: "Authorize in Meta", official: "Continue to Meta", bridge: "Official authorization; confirm after returning", pick: "Choose the Page to connect", pickReview: "Review this selection", confirm: "Confirm this connection", confirmConnect: "Confirm connection", edit: "Back to edit", result: "Operation result", returnList: "Back to connections", disconnect: "Disconnect this Page", disconnectTitle: "Disconnect this Page?", keep: "Keep connection", confirmDisconnect: "Confirm disconnect", uncertain: "Check this operation first", readResult: "Check this operation’s result", health: "View this Page’s feature checks", healthTitle: "Check each feature", recheck: "Schedule another check",
  },
} as const;

export type ParityLocale = keyof typeof parityWords;
export const parityViewports = [{ width: 1440, height: 900 }, { width: 390, height: 844 }] as const;

/** Record actual control interactions, including failed expectations, in the browser artifact. */
export async function parityClick(page: Page, name: string, verify: () => Promise<unknown>, within: Page | Locator = page) {
  const row = { route: new URL(page.url()).pathname, control: name, operation: "click", expected: "visible signed-frame result", actual: "NOT_RUN", result: "FAIL" };
  try {
    const button = within.getByRole("button", { name, exact: true });
    await expect(button).toBeVisible();
    await button.click();
    await verify(); row.actual = "expected visible result observed"; row.result = "PASS";
  } finally {
    await test.info().attach("C4-click-ledger", { body: JSON.stringify(row), contentType: "application/json" });
  }
}

/** One real-route viewport PNG per signed frame; only zh-TW is the Figma copy reference. */
export async function parityShot(page: Page, evidence: string, locale: ParityLocale, ...frames: string[]) {
  expect(new URL(page.url()).pathname).not.toContain("ui-gallery");
  // READ/MEASURE: geometry only; no DOM or business-state mutation.
  expect(await page.evaluate(() => document.documentElement.scrollWidth - innerWidth)).toBeLessThanOrEqual(1);
  // READ/MEASURE: visible text blocks and character totals only; no content, DOM or application state is changed.
  const density = await page.evaluate(() => {
    const root = document.querySelector('[data-testid="metaconnect-card"], [data-testid="merchant-studio"], [data-testid="merchant-claims"]');
    const blocks = new Map<Element, number>();
    if (root) {
      const walker = document.createTreeWalker(root, NodeFilter.SHOW_TEXT);
      while (walker.nextNode()) {
        const node = walker.currentNode, text = node.textContent?.trim() ?? "", parent = node.parentElement;
        if (!text || !parent || getComputedStyle(parent).visibility === "hidden") continue;
        const range = document.createRange(); range.selectNodeContents(node);
        const rect = range.getBoundingClientRect();
        if (!rect.width || !rect.height || rect.bottom <= 0 || rect.top >= innerHeight || rect.right <= 0 || rect.left >= innerWidth) continue;
        const block = parent.closest("h1,h2,h3,h4,p,dt,dd,label,button,a,summary,td,th,output,strong,small") ?? parent;
        blocks.set(block, (blocks.get(block) ?? 0) + Array.from(text).length);
      }
    }
    return { visible_text_blocks: blocks.size, characters: [...blocks.values()].reduce((sum, count) => sum + count, 0) };
  });
  if (locale !== "zh-TW") return;
  await mkdir(`${evidence}/parity`, { recursive: true });
  for (const frame of frames) {
    // GitHub upload-artifact rejects colons; the JSON retains the exact signed frame ID.
    const artifactId = frame.replaceAll(":", "-");
    await page.screenshot({ path: `${evidence}/parity/${artifactId}.png`, fullPage: false, animations: "disabled" });
    await writeFile(`${evidence}/parity/${artifactId}.json`, JSON.stringify({ frame, route: new URL(page.url()).pathname, viewport: page.viewportSize(), density, evidence: "MOCK browser; screenshot supplements real clicks" }));
  }
}

/** Enter the signed advanced claim tools through the real disclosure, once per visit. */
export async function openClaimTools(page: Page) {
  const disclosure = page.locator("details").filter({ has: page.getByText(/^(Advanced|進階|进阶)$/, { exact: true }) });
  await expect(disclosure).toBeVisible();
  if (await disclosure.getAttribute("open") === null)
    await disclosure.locator("summary").click();
  await expect(page.locator("#claims-mode")).toBeVisible();
}

/** Shared real-click setup; server-created drafts keep their original authority and receipts. */
export async function createClaimSession(page: Page, origin: string, store: string, locale: keyof typeof parityWords, title: string) {
  await page.goto(`${origin}/${locale}/studio?store=${store}`);
  await expect(page.getByTestId("merchant-studio")).toBeVisible();
  await page.getByRole("button", { name: parityWords[locale].create, exact: true }).click();
  await page.getByLabel({ en: "Scene name", "zh-TW": "場次名稱", "zh-CN": "场次名称" }[locale], { exact: true }).fill(title);
  await page.getByRole("button", { name: locale === "en" ? "Create draft" : "建立草稿", exact: true }).click();
  await page.waitForURL(/scene=[0-9a-f-]{36}/);
  await expect(page.locator(".studio-surface.studio-planning-only")).toBeVisible();
  await expect(page.getByRole("button", { name: /MOCK rehearsal|Request stop/ })).toHaveCount(0);
  await page.getByTestId("studio-open-claims").click();
  await expect(page.getByTestId("merchant-claims")).toBeVisible();
  await openClaimTools(page);
  return new URL(page.url()).searchParams.get("scene")!;
}
