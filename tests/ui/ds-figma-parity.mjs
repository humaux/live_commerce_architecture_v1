// Purpose: DS-3 gate: every Figma master variant (page 4:8829) rendered by the ds components in the UI gallery has the computed
//   background, border, text colour, radius, height and weight recorded in output/ds-3/figma-expected.json (values read from Figma, node ids inline).
//   Also captures the 1586 and 390 gallery screenshots for the Figma side-by-side.
// Depends on: a Playwright Page on /<locale>/ui-gallery (LC_UI_GALLERY=1), output/ds-3/figma-expected.json, the runner's step() ledger.
// Used by: tests/admin/ui-gallery-runner.mjs (--browser-ui-gallery, which every UI-path PR runs). MOCK tier: gallery sample data only.
import assert from "node:assert/strict";
import { mkdir, readFile, writeFile } from "node:fs/promises";

const EXPECTED = "output/ds-3/figma-expected.json";

/** "#rrggbb" | "transparent" -> the string getComputedStyle reports. */
export function cssColour(value) {
  if (value === "transparent") return "rgba(0, 0, 0, 0)";
  const m = /^#([0-9a-f]{6})$/i.exec(value);
  assert.ok(m, `not a hex colour in ${EXPECTED}: ${value}`);
  const [r, g, b] = [0, 2, 4].map((i) => parseInt(m[1].slice(i, i + 2), 16));
  return `rgb(${r}, ${g}, ${b})`;
}

/** Compares one measured record against one expect block; returns the list of mismatches (empty = pass). Pure, so it is unit-checked below. */
export function compare(expect, got) {
  const bad = [];
  for (const [key, want] of Object.entries(expect)) {
    const have = got[key];
    if (key === "height" || key === "labelInset" || key === "crumbGap") {
      if (typeof have !== "number" || Math.abs(have - want) > 0.5) bad.push(`${key} ${have} != ${want}`);
    } else if (/Color$/.test(key) || key === "color") {
      if (have !== cssColour(want)) bad.push(`${key} ${have} != ${want} (${cssColour(want)})`);
    } else if (have !== want) bad.push(`${key} ${JSON.stringify(have)} != ${JSON.stringify(want)}`);
  }
  return bad;
}

// Self-check of the comparator, so a gate that cannot fail is caught before the browser run.
assert.deepEqual(compare({ color: "#142942", height: 44 }, { color: "rgb(20, 41, 66)", height: 44.2 }), []);
assert.equal(compare({ color: "#142942" }, { color: "rgb(42, 111, 176)" }).length, 1, "wrong colour must fail");
assert.equal(compare({ height: 44 }, { height: 36 }).length, 1, "wrong height must fail");
assert.equal(compare({ backgroundColor: "transparent" }, { backgroundColor: "rgb(255, 255, 255)" }).length, 1);
assert.equal(compare({ noCaret: true }, {}).length, 1, "a missing probe must fail");

/** [READ/MEASURE] computed style of one element plus the derived probes the expect blocks name. */
function measure(n) {
  const s = getComputedStyle(n);
  const box = n.getBoundingClientRect();
  const out = {
    backgroundColor: s.backgroundColor, color: s.color, borderTopColor: s.borderTopColor, borderTopWidth: s.borderTopWidth,
    borderBottomColor: s.borderBottomColor, borderBottomWidth: s.borderBottomWidth, borderLeftWidth: s.borderLeftWidth,
    borderTopLeftRadius: s.borderTopLeftRadius, fontSize: s.fontSize, fontWeight: s.fontWeight, lineHeight: s.lineHeight,
    paddingLeft: s.paddingLeft, minWidth: s.minWidth, height: box.height,
  };
  if (n.tagName === "INPUT") out.placeholderColor = getComputedStyle(n, "::placeholder").color;
  if (n.hasAttribute("data-current")) {
    const label = n.children[1];
    out.labelInset = label ? label.getBoundingClientRect().left - box.left : -1;
  }
  if (n.hasAttribute("data-menu-trigger")) out.noCaret = n.querySelectorAll("svg").length === 0;
  if (n.matches("[data-shell-topbar]")) {
    out.holdsBreadcrumb = Boolean([...n.querySelectorAll("ol")].some((o) => o.getClientRects().length));
    out.holdsLanguage = Boolean([...n.querySelectorAll('[data-figma="1397:105153"] button')].some((b) => b.getClientRects().length));
  }
  if (n.matches("li + li")) {
    const before = getComputedStyle(n, "::before");
    out.separator = (/^"([^"]*)"/.exec(before.content) || [])[1] ?? before.content;
    out.separatorColor = before.color;
    out.crumbGap = parseFloat(getComputedStyle(n.parentElement).columnGap) || 0;
  }
  if (n.matches("ol > li")) {
    const colours = [...n.parentElement.children].map((li) => getComputedStyle(li.querySelector("a,span") || li).color);
    out.allCrumbsSameColor = colours.every((c) => c === colours[0]);
    const inner = n.querySelector("a,span");
    if (inner) Object.assign(out, { color: getComputedStyle(inner).color });
  }
  return out;
}

/** Locates the element a check names. "pager:*" picks a part of the gallery list pager (#sec-table). */
function locate(page, target) {
  const pager = page.locator("#sec-table nav").filter({ has: page.locator("ul") }).last();
  switch (target) {
    case "pager:number": return pager.locator("a:not([aria-current])").filter({ hasText: /^\d+$/ }).first();
    case "pager:current": return pager.locator('a[aria-current="page"]');
    case "pager:prevDisabled": return pager.locator('[role="link"][aria-disabled="true"]').first();
    case "pager:next": return pager.locator("li").last().locator("a");
    case "pager:gap": return pager.locator('[aria-hidden="true"]').filter({ hasText: "…" }).first();
    default: return page.locator(target).first();
  }
}

/** Puts the gallery switch into the state a check needs with a real click (never a DOM write). */
async function switchTo(page, want) {
  const sw = page.locator('#sec-switch [data-side="on"]').locator("xpath=..");
  const on = (await page.locator('#sec-switch [data-side="on"]').getAttribute("data-on")) === "true";
  if ((want === "switch-on") !== on) await sw.click();
  await page.locator(`#sec-switch [data-side="on"][data-on="${want === "switch-on"}"]`).waitFor();
}

/**
 * Runs the DS-3 Figma parity gate on an open gallery page (desktop 1586x900). `step(control, action, expected, run)` is the runner ledger.
 * Every specimen control in #sec-figma is also really clicked/filled so the click ledger covers the new card.
 */
export async function figmaParity({ page, step, output, copy }) {
  const { checks } = JSON.parse(await readFile(EXPECTED, "utf8"));
  await mkdir(`${output}/ds3`, { recursive: true });
  const results = [];
  await page.mouse.move(0, 0);
  for (const check of checks) {
    await step(`DS-3 ${check.id} (Figma ${check.figma})`, "[READ] computed style", JSON.stringify(check.expect), async () => {
      if (check.state?.startsWith("switch-")) await switchTo(page, check.state);
      const node = locate(page, check.target);
      await node.waitFor({ state: "attached", timeout: 4000 });
      await node.scrollIntoViewIfNeeded();
      if (check.state === "hover") await node.hover();
      else await page.mouse.move(0, 0);
      const got = await node.evaluate(measure); // [READ]
      const bad = compare(check.expect, got);
      results.push({ id: check.id, figma: check.figma, pass: bad.length === 0, bad, got });
      assert.deepEqual(bad, [], `${check.id} (Figma ${check.figma}): ${bad.join("; ")}`);
      return "matches Figma";
    });
  }
  await switchTo(page, "switch-on");
  // Real clicks over the specimen card: enabled buttons report their label, disabled ones reveal the reason, inputs keep typed text.
  const card = page.locator("#sec-figma");
  for (const button of await card.locator("button[data-figma]").all()) {
    const id = await button.getAttribute("data-figma");
    const blocked = (await button.getAttribute("aria-disabled")) === "true";
    await step(`DS-3 specimen ${id}`, "click", blocked ? "reason shown" : "label reported in the card status", async () => {
      const label = (await button.textContent()).trim();
      if (blocked) {
        // Playwright's enabled-wait treats aria-disabled as unclickable; a user can still click it, so click its centre with the real mouse.
        await button.scrollIntoViewIfNeeded();
        const box = await button.boundingBox(); // [MEASURE] position only
        await page.mouse.click(box.x + box.width / 2, box.y + box.height / 2);
      } else await button.click();
      if (blocked) await card.getByRole("status").filter({ hasText: copy.btnDisabledReason }).first().waitFor();
      else await card.getByRole("status").filter({ hasText: label }).first().waitFor();
    });
  }
  for (const input of await card.locator("input[data-figma]").all()) {
    const id = await input.getAttribute("data-figma");
    await step(`DS-3 specimen ${id}`, "fill", "typed value kept", async () => {
      await input.fill(copy.storeName);
      assert.equal(await input.inputValue(), copy.storeName);
    });
  }
  await writeFile(`${output}/ds3/parity.json`, `${JSON.stringify(results, null, 2)}\n`);
  for (const [w, h] of [[1586, 900], [390, 844]]) {
    await page.setViewportSize({ width: w, height: h });
    await page.evaluate(async () => { await document.fonts.ready; }); // [READ] fonts only
    await page.screenshot({ path: `${output}/ds3/gallery-${w}.png`, fullPage: false, animations: "disabled" });
    await card.scrollIntoViewIfNeeded();
    await card.screenshot({ path: `${output}/ds3/specimens-${w}.png`, animations: "disabled" });
  }
  return results;
}
