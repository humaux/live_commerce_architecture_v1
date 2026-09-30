// One engine switch for the buyer-facing Node browser drivers in this directory (no Go caller, no BFF route).
// LC_BROWSER_ENGINE=chromium (default, every existing gate unchanged) | webkit (real Safari engine, run by
// `bash scripts/dev/test-local.sh --browser-webkit`). Taiwan buyers arrive from Facebook/Instagram on iPhones, so under
// webkit a phone-sized context (viewport width <= 430) gets Playwright's iPhone 15 profile (iOS Safari UA, touch,
// isMobile, DPR 3) and a wider one gets Desktop Safari. The Go test harness must forward LC_BROWSER_ENGINE
// (browserEnvironment in tests/foundation/browser_identity_chain_test.go) because it strips every other LC_* variable.
import { chromium, webkit, devices } from "@playwright/test";

export const engine = process.env.LC_BROWSER_ENGINE || "chromium";
console.log(`browser engine: ${engine}`); // first line of every driver log: proof of which engine ran
if (engine !== "chromium" && engine !== "webkit") throw new Error(`LC_BROWSER_ENGINE must be chromium or webkit, got ${engine}`);

// Launch options are identical for both engines (headless, optional synthetic-host CONNECT proxy).
export const launch = (options = {}) => (engine === "webkit" ? webkit : chromium).launch({ headless: true, ...options });

// The phone device the drivers spread into a mobile context: Pixel 7 (Chromium, unchanged) or iPhone 15 (WebKit).
export const phone = devices[engine === "webkit" ? "iPhone 15" : "Pixel 7"];

// Chromium: returns the options untouched. WebKit: lays the matching Safari device profile under the caller's options.
export function ctxOpts(options = {}) {
  if (engine !== "webkit") return options;
  const width = options.viewport?.width ?? 1440;
  return { ...devices[width <= 430 ? "iPhone 15" : "Desktop Safari"], ...options };
}

// Engine-specific facts some assertions must state explicitly (the phone profile's platform token and display name).
export const phoneToken = engine === "webkit" ? "iPhone" : "Android";
export const phoneName = engine === "webkit" ? "iPhone 15" : "Pixel 7";
