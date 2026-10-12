// Purpose: retain native inner-scroll evidence and supplemental affected-route viewport evidence without altering the primary matrix.
// Depends on: Playwright locator/mouse/screenshot transport; Node fs/path/crypto; read-only pane geometry.
// Used by: visual-audit.mjs and visual-scroll-capture.test.mjs; synthetic data only.
import { createHash } from "node:crypto";
import { mkdir, writeFile } from "node:fs/promises";
import path from "node:path";

// Read only: clipping ancestors and the viewport determine which inner content is actually on screen.
function measurePane(pane) {
  const b = pane.getBoundingClientRect();
  const content = { x: b.x + pane.clientLeft, y: b.y + pane.clientTop, width: pane.clientWidth, height: pane.clientHeight };
  let left = Math.max(0, content.x), top = Math.max(0, content.y), right = Math.min(window.innerWidth, content.x + content.width), bottom = Math.min(window.innerHeight, content.y + content.height);
  for (let ancestor = pane.parentElement; ancestor; ancestor = ancestor.parentElement) {
    const style = getComputedStyle(ancestor), r = ancestor.getBoundingClientRect();
    if (/auto|scroll|hidden|clip/.test(style.overflowX)) { left = Math.max(left, r.left + ancestor.clientLeft); right = Math.min(right, r.left + ancestor.clientLeft + ancestor.clientWidth); }
    if (/auto|scroll|hidden|clip/.test(style.overflowY)) { top = Math.max(top, r.top + ancestor.clientTop); bottom = Math.min(bottom, r.top + ancestor.clientTop + ancestor.clientHeight); }
  }
  const visible = { x: left, y: top, width: Math.max(0, right - left), height: Math.max(0, bottom - top) };
  // The pane gutter avoids sending the wheel to a nested textarea/select scrollport.
  const wheelPoint = { x: left + Math.min(4, visible.width / 2), y: top + visible.height / 2 };
  const target = document.elementFromPoint(wheelPoint.x, wheelPoint.y);
  return { scrollTop: pane.scrollTop, scrollHeight: pane.scrollHeight, clientHeight: pane.clientHeight, clientWidth: pane.clientWidth, contentTop: content.y, windowY: window.scrollY, rect: { x: b.x, y: b.y, width: b.width, height: b.height }, visible, wheelPoint: target && (target === pane || pane.contains(target)) ? wheelPoint : null };
}

// Capture native wheel frames with contiguous visible ranges; partial/frozen/unreachable evidence fails closed.
export async function captureInnerScrollFrames(page, { selector, out, filePrefix, maxFrames = 64, maxMoves = 128, waitMs = 180 }) {
  const frames = [];
  let geometry, moves = 0;
  const finish = (complete, reason) => ({ kind: "inner-scroll frames", selector, complete, ...(reason ? { reason } : {}), scrollHeight: geometry?.scrollHeight ?? 0, frames });
  try {
    const pane = page.locator(selector), count = await pane.count();
    if (count !== 1) return finish(false, `pane-count:${count}`);
    const read = async () => {
      const g = await pane.evaluate(measurePane); // [READ/MEASURE] never sets scrollTop, dimensions, styles or DOM.
      if (![g.scrollTop, g.scrollHeight, g.clientHeight, g.contentTop, g.visible.height].every(Number.isFinite) || g.clientHeight <= 0 || g.visible.height <= 8 || !g.wheelPoint) throw new Error("pane-geometry-not-visible");
      return g;
    };
    const wheel = async (dy) => {
      if (++moves > maxMoves) throw new Error("move-limit");
      await page.mouse.move(geometry.wheelPoint.x, geometry.wheelPoint.y);
      await page.mouse.wheel(0, dy); // actual browser input; document-only movement is not accepted as pane progress.
      await page.waitForTimeout(waitMs);
      geometry = await read();
    };
    geometry = await read();
    while (geometry.scrollTop > 1) {
      const previous = geometry.scrollTop;
      await wheel(-Math.max(1, Math.floor(geometry.visible.height * 0.75)));
      if (geometry.scrollTop >= previous - 0.5) return finish(false, "no-progress:rewind");
    }
    let covered = 0;
    for (;;) {
      if (frames.length >= maxFrames) return finish(false, "frame-limit");
      // Wait only for images visible in this frame; lazy images further down are reached by later native wheels.
      await pane.evaluate(async (node) => {
        await document.fonts.ready;
        const paneRect = node.getBoundingClientRect();
        await Promise.all([...node.querySelectorAll("img")].filter((image) => {
          const r = image.getBoundingClientRect();
          return r.bottom > Math.max(0, paneRect.top) && r.top < Math.min(window.innerHeight, paneRect.bottom) && !image.complete;
        }).map((image) => new Promise((resolve) => {
          const finish = () => { clearTimeout(timer); image.removeEventListener("load", finish); image.removeEventListener("error", finish); resolve(); };
          const timer = setTimeout(finish, 2000);
          image.addEventListener("load", finish, { once: true }); image.addEventListener("error", finish, { once: true });
        })));
      }); // [READ/MEASURE] waits for fonts/images without writing page state.
      geometry = await read();
      const from = Math.max(0, geometry.scrollTop + geometry.visible.y - geometry.contentTop), to = Math.min(geometry.scrollHeight, from + geometry.visible.height);
      if (from > covered + 1) return finish(false, "coverage-gap");
      const buf = await page.screenshot({ fullPage: false, animations: "disabled", caret: "hide" });
      const file = path.join("shots", `${filePrefix}-inner-${String(frames.length + 1).padStart(2, "0")}.png`);
      await mkdir(path.dirname(path.join(out, file)), { recursive: true });
      await writeFile(path.join(out, file), buf);
      frames.push({ file, sha256: createHash("sha256").update(buf).digest("hex"), bytes: buf.length, geometry, visibleRange: { from, to } });
      covered = Math.max(covered, to);
      if (covered >= geometry.scrollHeight - 1) return finish(true);
      if (geometry.scrollTop >= geometry.scrollHeight - geometry.clientHeight - 1) return finish(false, "end-not-visible");
      const previous = geometry.scrollTop;
      await wheel(Math.max(1, Math.floor(geometry.visible.height * 0.75)));
      if (geometry.scrollTop <= previous + 0.5) return finish(false, "no-progress:forward");
    }
  } catch (error) {
    return finish(false, `capture-error:${String(error.message ?? error).split("\n")[0].slice(0, 160)}`);
  }
}

// Select an explicit supplemental viewport without altering the primary route matrix.
export function supplementalViewportFor(unit) {
  return unit.app === "admin" && unit.v.name === "desktop" && !unit.v.mobile && ["/ads", "/products/new", "/products/[product]"].includes(unit.route)
    ? { name: "supplemental-desktop", size: { width: 1440, height: 900 }, mobile: false }
    : null;
}

// Fold supplemental lint evidence into its owning primary record, preserving the primary shot identity.
export function foldSupplementalCapture(parent, extra) {
  (parent.supplemental ??= []).push({ ...extra, kind: "supplemental viewport" });
  for (const field of ["counts", "groups", "blockingCounts"]) {
    parent[field] ??= {};
    for (const [rule, count] of Object.entries(extra[field] ?? {})) parent[field][rule] = (parent[field][rule] ?? 0) + count;
  }
  parent.blocking = (parent.blocking ?? 0) + (extra.blocking ?? 0);
  (parent.violations ??= []).push(...(extra.violations ?? []).map((violation) => ({ ...violation, supplementalViewport: extra.size })));
  (parent.issues ??= []).push(...(extra.issues ?? []).map((issue) => `supplemental ${extra.size}: ${issue}`));
  if (!extra.file || !extra.ok || extra.failed || extra.stateFailed || extra.notRun) {
    parent.stateFailed = true;
    parent.issues.push(`supplemental ${extra.size}: capture or required state not reached`);
  }
  return parent;
}
