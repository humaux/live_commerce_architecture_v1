import { chromium, expect, type Browser } from "@playwright/test";
import { mkdtemp, readFile, rm } from "node:fs/promises";
import { spawn } from "node:child_process";

// The default context and noDefaults are essential: Playwright's normal CDP
// attachment captures focus and cannot observe a real tab becoming hidden.
export async function nativePage(evidence: string, profilePrefix: string) {
  const profile = await mkdtemp(`${evidence}/${profilePrefix}`);
  const child = spawn(chromium.executablePath(), [
    `--user-data-dir=${profile}`, "--remote-debugging-address=127.0.0.1",
    "--remote-debugging-port=0", "--no-first-run", "--no-default-browser-check", "about:blank",
  ], { stdio: "ignore" });
  let browser: Browser | undefined;
  let launchError: Error | undefined;
  child.once("error", (error) => { launchError = error; });
  const exited = () => child.exitCode !== null || child.signalCode !== null;
  const delay = (ms: number) => new Promise<void>((resolve) => setTimeout(resolve, ms));
  const waitExit = (ms: number) => new Promise<boolean>((resolve) => {
    if (exited()) return resolve(true);
    const onExit = () => { clearTimeout(timer); resolve(true); };
    const timer = setTimeout(() => { child.off("exit", onExit); resolve(exited()); }, ms);
    child.once("exit", onExit);
  });
  const close = async () => {
    if (browser?.isConnected()) {
      await Promise.race([browser.newBrowserCDPSession().then((session) =>
        session.send("Browser.close")).catch(() => {}), delay(1_500)]);
      await Promise.race([browser.close().catch(() => {}), delay(1_500)]);
    }
    if (!(await waitExit(1_500))) child.kill("SIGTERM");
    if (!(await waitExit(1_500))) child.kill("SIGKILL");
    if (!(await waitExit(3_000))) throw new Error(`owned Chromium pid ${child.pid} did not exit`);
    await rm(profile, { recursive: true, force: true });
  };
  try {
    let port = 0;
    const deadline = performance.now() + 10_000;
    while (performance.now() < deadline && !exited()) {
      if (launchError) throw launchError;
      try { port = Number((await readFile(`${profile}/DevToolsActivePort`, "utf8")).split("\n")[0]); } catch { /* Chromium has not opened CDP yet. */ }
      if (Number.isInteger(port) && port > 0 && port < 65_536) break;
      await delay(100);
    }
    if (!port) throw new Error("owned Chromium did not expose loopback CDP");
    browser = await chromium.connectOverCDP(`http://127.0.0.1:${port}`, { noDefaults: true });
    if (browser.contexts().length !== 1) throw new Error("native device needs the existing default context");
    const context = browser.contexts()[0];
    // Chromium publishes DevToolsActivePort before its startup window exists, so the first connect can see no page yet (observed in the R4 final gate:
    // MOU03's trace shows "Create page"). Creating one here opens a second window that the startup window then overtakes as the last-active one; a
    // later cover tab lands there, this page is never hidden and the visibility assertions time out. Wait for the startup tab instead.
    await expect.poll(() => context.pages().length, { timeout: 10_000 }).toBeGreaterThan(0);
    const page = context.pages()[0];
    await page.bringToFront();
    await expect.poll(() => page.evaluate(() => document.visibilityState)).toBe("visible");
    return { page, close };
  } catch (error) {
    await close();
    throw error;
  }
}
