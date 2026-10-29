#!/usr/bin/env node
// Purpose: lint current Merchant Figma frames for copy, render overflow and obsolete descendants via read-only REST.
// Depends on: Node >=20 stdlib/global fetch, Figma file nodes API and FIGMA_TOKEN from the environment only.
// Used by: figma-design-lint.yml, tests/design/figma-lint.test.mjs and the integrator's copy-lint adapter after merge.
// Invariants: no Figma writes; <=50 IDs/request; <=3 attempts; credentials never enter URLs, logs or reports.
// Usage: node scripts/design/figma-lint.mjs [--file-key KEY] [--page-id ID] [--report PATH]
import { mkdir, writeFile } from "node:fs/promises";
import { dirname, resolve } from "node:path";
import { pathToFileURL } from "node:url";
import { setTimeout as delay } from "node:timers/promises";

// Verbatim from figma-copy-lint.ref.mjs; SKU is merchant data, not a banned term.
export const BANNED = [
  /\bMOCK\b/, /\bUNKNOWN\b/i, /\bqueued\b/i, /\bJSON\b/, /\bMiB\b/, /\bERASE\b/,
  /設計原型|設計示範|示範原型|原型示範|原型只|此畫框|回應示範|演練[:：]|合成資料|未連接服務/,
  /伺服器/, /工作階段/, /快照/, /≠/, /輸入框為有限示範/,
];
const DEFAULT_FILE = "iJ9lDdFIXgTlKkt5Tvrc0X";
const DEFAULT_PAGE = "4:8829";
const PREFIX = "DESIGN v2 · Merchant";
const SUPERSEDED = "已被取代 · ";
const keptFrame = (node) => node?.type === "FRAME" && node.name?.startsWith(PREFIX) && !node.name.startsWith(SUPERSEDED);

export function selectFrames(page) {
  return page?.visible === false ? [] : (page?.children ?? []).filter(keptFrame);
}

export function checkFrame(frame) {
  if (!keptFrame(frame) || frame.visible === false) return [];
  const hits = [];
  const hit = (node, rule, text) => hits.push({ frameId: frame.id, nodeId: node.id, rule, text });
  const box = frame.absoluteBoundingBox?.width;
  const render = frame.absoluteRenderBounds?.width;
  if (frame.clipsContent === false && Number.isFinite(box) && Number.isFinite(render) && Math.abs(render - box) > 1) {
    hit(frame, "WIDTH", `render=${render}px bounding=${box}px`);
  }
  const visit = (node) => {
    if (node.visible === false) return;
    if (node !== frame && (node.name?.startsWith(SUPERSEDED) || node.name?.startsWith("fix-"))) {
      hit(node, "STALE", node.name);
    }
    if (node.type === "TEXT" && typeof node.characters === "string" && node.characters !== "示範資料") {
      if (BANNED.some((pattern) => pattern.test(node.characters))) hit(node, "COPY", node.characters);
    }
    for (const child of node.children ?? []) visit(child);
  };
  visit(frame);
  return hits;
}

// Errors use fixed local messages: never surface fetch errors or Figma response bodies containing secrets.
class LintError extends Error {}

export async function lintFile({ fileKey = DEFAULT_FILE, pageId = DEFAULT_PAGE, fetchImpl = globalThis.fetch, sleep = delay } = {}) {
  const token = process.env.FIGMA_TOKEN;
  if (!token?.trim()) throw new LintError("FIGMA_TOKEN is required");
  if (!/^[A-Za-z0-9_-]+$/.test(fileKey) || !/^\d+:\d+$/.test(pageId)) throw new LintError("Invalid file key or page ID");
  const deadline = Date.now() + 8 * 60_000;
  const readNodes = async (ids, depth, version) => {
    const url = new URL(`https://api.figma.com/v1/files/${encodeURIComponent(fileKey)}/nodes`);
    url.searchParams.set("ids", ids.join(","));
    if (depth) url.searchParams.set("depth", String(depth));
    if (version) url.searchParams.set("version", version);
    if (url.href.includes(token)) throw new LintError("Credential must not appear in a URL");
    for (let attempt = 1; attempt <= 3; attempt++) {
      const remaining = deadline - Date.now();
      if (remaining <= 0) throw new LintError("Figma read deadline exceeded");
      let response;
      try {
        response = await fetchImpl(url.href, {
          method: "GET", headers: { "X-Figma-Token": token }, redirect: "error",
          signal: AbortSignal.timeout(Math.min(30_000, remaining)),
        });
      } catch { throw new LintError("Figma request failed or timed out"); }
      if (response.ok) {
        let result;
        try { result = await response.json(); }
        catch { throw new LintError("Invalid Figma JSON response"); }
        if (!result?.nodes || result.err) throw new LintError("Invalid Figma nodes response");
        return result;
      }
      const status = response.status;
      const retryAfter = response.headers.get("Retry-After");
      await response.body?.cancel();
      if (attempt === 3 || (status !== 429 && !(status >= 500 && status <= 599))) {
        throw new LintError(`Figma API HTTP ${status}`);
      }
      const seconds = retryAfter === null ? NaN : Number(retryAfter);
      const retryMs = Number.isFinite(seconds) ? seconds * 1000 : Date.parse(retryAfter) - Date.now();
      const waitMs = Math.max(1000 * 2 ** (attempt - 1), Number.isFinite(retryMs) ? retryMs : 0);
      // A longer server delay cannot fit this job: fail rather than retry before Retry-After.
      if (waitMs >= deadline - Date.now()) throw new LintError(`Figma API HTTP ${status}: Retry-After exceeds read deadline`);
      await sleep(waitMs);
    }
  };
  const discovery = await readNodes([pageId], 1);
  const page = discovery.nodes[pageId]?.document;
  if (page?.id !== pageId || page.type !== "CANVAS") throw new LintError("Requested Figma page is missing");
  const frames = selectFrames(page);
  if (!frames.length) throw new LintError("No matching Merchant frames found");
  const hits = [];
  for (let start = 0; start < frames.length; start += 50) {
    const batch = frames.slice(start, start + 50);
    // Pin subsequent reads to discovery's version so page selection and subtrees describe one snapshot.
    const result = await readNodes(batch.map((frame) => frame.id), undefined, discovery.version);
    for (const frame of batch) {
      const document = result.nodes[frame.id]?.document;
      if (document?.id !== frame.id || !keptFrame(document)) throw new LintError("Requested Merchant frame is missing or changed");
      hits.push(...checkFrame(document));
    }
  }
  return { status: hits.length ? "hits" : "clean", fileKey, pageId, version: discovery.version ?? null, frames: frames.length, hits };
}

export async function main(args = process.argv.slice(2), dependencies = {}) {
  let reportPath = "output/FIGMA-DESIGN-LINT/report.json";
  let report;
  try {
    const options = {};
    for (let i = 0; i < args.length; i += 2) {
      const key = { "--file-key": "fileKey", "--page-id": "pageId", "--report": "reportPath" }[args[i]];
      if (!key || !args[i + 1] || args[i + 1].startsWith("--")) throw new LintError("Usage: figma-lint.mjs [--file-key KEY] [--page-id ID] [--report PATH]");
      if (key === "reportPath") reportPath = args[i + 1];
      else options[key] = args[i + 1];
    }
    report = await lintFile({ ...dependencies, ...options });
  } catch (error) {
    report = { status: "error", frames: 0, hits: [], error: error instanceof LintError ? error.message : "Figma lint failed" };
  }
  const token = process.env.FIGMA_TOKEN;
  const redact = (text) => token ? text.split(token).join("[REDACTED]") : text;
  try {
    await mkdir(dirname(resolve(reportPath)), { recursive: true });
    await writeFile(reportPath, redact(JSON.stringify(report, null, 2)) + "\n", { mode: 0o600 });
  } catch {
    console.error("figma-lint: unable to write JSON report");
    return 2;
  }
  for (const hit of report.hits) console.log(redact(`HIT ${hit.frameId} ${hit.nodeId} ${hit.rule} :: ${hit.text.replace(/\s+/g, " ")}`));
  console.log(`figma-lint: frames=${report.frames} hits=${report.hits.length} status=${report.status}`);
  if (report.status === "error") { console.error(redact(report.error)); return 2; }
  return report.hits.length ? 1 : 0;
}

if (process.argv[1] && import.meta.url === pathToFileURL(resolve(process.argv[1])).href) process.exitCode = await main();
