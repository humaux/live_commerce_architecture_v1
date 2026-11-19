"""Create ephemeral source variants and RF11 probes from the exact pinned Git objects."""

import argparse
import hashlib
import json
from pathlib import Path
import subprocess

PIN = "24084b4678023fa7a4bb52694c965b13751c2dfa"
TREE = "8862e2810767dbdc8cb85b80e1b42c3fd22d9aa8"
COMPONENT = "apps/admin/components/OrderRefunds.tsx"
SPEC = "tests/admin/refund.spec.ts"
GO_TEST = "tests/foundation/browser_refund_fulfilment_test.go"
SOURCE_PINS = (
    COMPONENT, "apps/admin/components/MerchantOrders.tsx",
    "apps/admin/components/OrderDetailPanel.tsx", "apps/admin/lib/orders-model.ts",
    SPEC, GO_TEST, "tests/storefront/refund-buyer.mjs",
    "scripts/dev/test-local-runtime.sh", "scripts/dev/ci-playwright-install.sh",
    "package.json", "apps/admin/package.json", "apps/storefront/package.json",
    "pnpm-lock.yaml", "go.mod", "go.sum",
)
SCHEDULE = [
    ("normal", "on"), ("normal", "off"),
    ("duplicate", "off"), ("duplicate", "on"),
    ("duplicate", "on"), ("duplicate", "off"),
]


def pinned(path):
    return subprocess.check_output(["git", "show", f"{PIN}:{path}"])


def sha(source):
    return hashlib.sha256(source).hexdigest()


def replace(source, before, after):
    if source.count(before) != 1:
        raise SystemExit(f"wrong probe anchor: {before.splitlines()[0]}")
    return source.replace(before, after)


def component_sources(source):
    before = """        if (!explicit && (repeated || (value.refunded_minor === detail.refunded_minor && value.pending_minor === detail.refund_pending_minor))) return;"""
    with_probe = """        const sameTotals = value.refunded_minor === detail.refunded_minor && value.pending_minor === detail.refund_pending_minor;
        const observedDuplicate = detail.updated_at.endsWith(".987654Z") && !explicit && repeated && !sameTotals;
        if (!explicit && (repeated || sameTotals)) {
          if (observedDuplicate) console.info("G15_RF11_DUPLICATE suppressed");
          return;
        }"""
    with_probe = replace(source, before, with_probe)
    with_probe = replace(with_probe, "          await changed.current();", """          const changedDetail = await changed.current();
          if (observedDuplicate && changedDetail) console.info("G15_RF11_DUPLICATE drained");""")
    # The observer and every authorization, cancellation, convergence and POST fence
    # are identical. Disable just repeated list snapshot suppression in the other arm.
    without_dedup = replace(with_probe, "(repeated || sameTotals)) {", "sameTotals) {")
    return {"on": with_probe, "off": without_dedup}


def instrument_browser(source):
    start = '''    await signedLogin(page);
    const posts = watchRefundPosts(page);
    const detailPath = "/api/stores/" + store + "/orders/" + order;'''
    original_path = '    const detailPath = \x60/api/stores/\x24{store}/orders/\x24{order}\x60;'
    source = replace(source, original_path, start.splitlines()[-1])
    source = replace(source, start, """    const posts = watchRefundPosts(page);
    const detailPath = "/api/stores/" + store + "/orders/" + order;
    const profile = process.env.G15_RF11_PROFILE;
    expect(["normal", "duplicate"]).toContain(profile);
    const stress = profile === "duplicate";
    let stage = "login";
    let postTerminalReads = 0;
    let readsAtReplay: number | null = null;
    let extraAfterReplay: number | null = null;
    let duplicateDrain: "suppressed" | "drained" | null = null;
    let terminalListServed = false;
    let oldDetailServed = false;
    let manualReload = false;
    let recoveryGets = 0;
    let paymentState: string | null = null;
    let refreshedPaymentState: string | null = null;
    let refreshActions: number | null = null;
    let postAmount: number | null = null;
    let terminal: { captured: number; refunded: number; pending: number; refundable: number; currency: string } | null = null;
    page.on("console", (message) => {
      if (readsAtReplay === null || duplicateDrain !== null) return;
      if (message.text() === "G15_RF11_DUPLICATE suppressed") duplicateDrain = "suppressed";
      if (message.text() === "G15_RF11_DUPLICATE drained") duplicateDrain = "drained";
    });
    page.on("request", (request) => {
      if (manualReload && request.method() === "GET" && new URL(request.url()).pathname === detailPath) recoveryGets++;
    });
    try {
    await signedLogin(page);
    stage = "read-partial";""")
    source = replace(source, """    let oldDetailServed = false;
    let terminalListServed = false;
    let postTerminalReads = 0;
""", "")
    source = replace(source, """        postTerminalReads++;
        await route.continue();""", """        postTerminalReads++;
        if (stress && readsAtReplay === null) {
          readsAtReplay = postTerminalReads;
          // Read fault only: replay the exact admitted old financial fields with
          // a new valid projection version, guaranteeing one same-state list read.
          const stale = JSON.parse(olderBody);
          const prior = new Date(stale.updated_at);
          stale.updated_at = new Date(prior.getTime() + 1000).toISOString().slice(0, 19) + ".987654Z";
          await route.fulfill({ status: older.status(), headers: olderHeaders, json: stale });
        } else {
          await route.continue();
        }""")
    source = replace(source, """        terminalListServed = true;
      }
      await route.fulfill({ response });""", """        terminalListServed = true;
        terminal = {
          captured: body.captured_minor, refunded: body.refunded_minor, pending: body.pending_minor,
          refundable: body.refundable_minor, currency: body.currency,
        };
      }
      await route.fulfill({ response });""")
    source = replace(source, """    await refundThroughDialog(page, detail, captured - partial, captured - partial);
    const body = JSON.parse(posts[posts.length - 1].body ?? "{}") as Record<string, unknown>;""", """    stage = "submit-remainder";
    await refundThroughDialog(page, detail, captured - partial, captured - partial);
    const body = JSON.parse(posts[posts.length - 1].body ?? "{}") as Record<string, unknown>;
    postAmount = Number(body.amount_minor);""")
    source = replace(source, """    await expect(async () => {
      const refresh = detail.getByRole("button", { name: ui.refresh });
      if (await refresh.count()) await refresh.first().click();
      await expect(detail.locator('[data-state="REFUNDED"]').first()).toBeVisible({ timeout: 3_000 });
    }).toPass({ timeout: 60_000, intervals: [1_000, 3_000, 5_000] });
    expect(oldDetailServed).toBe(true);
    expect(terminalListServed).toBe(true);
    expect(postTerminalReads).toBeGreaterThan(0);
    expect(postTerminalReads).toBeLessThanOrEqual(2); // bounded convergence, including a concurrent explicit refresh""", """    stage = stress ? "observe-duplicate" : "verify-automatic-convergence";
    await expect(async () => {
      const refresh = detail.getByRole("button", { name: ui.refresh });
      if (await refresh.count()) await refresh.first().click();
      if (stress) expect(duplicateDrain).not.toBeNull();
      else await expect(detail.locator('[data-state="REFUNDED"]').first()).toBeVisible({ timeout: 3_000 });
    }).toPass({ timeout: 60_000, intervals: [1_000, 3_000, 5_000] });
    expect(oldDetailServed).toBe(true);
    expect(terminalListServed).toBe(true);
    expect(postTerminalReads).toBeGreaterThan(0);
    if (stress) {
      expect(readsAtReplay).toBe(1);
      extraAfterReplay = postTerminalReads - readsAtReplay!;
      const preReloadState = duplicateDrain === "suppressed" ? "PARTIALLY_REFUNDED" : "REFUNDED";
      await expect(detail.locator('[data-state="' + preReloadState + '"]').first()).toBeVisible();
      paymentState = preReloadState;
      // End the identical fault window, then verify authoritative convergence by
      // a real merchant document refresh, without any more response overrides.
      await page.unroute("**" + detailPath);
      await page.unroute("**" + detailPath + "/refunds");
      stage = "verify-shared-refresh";
      manualReload = true;
      await page.reload();
      // The standalone merchant route renders the order facts in collapsed
      // native <details> after each navigation. Reopen via the original RF11 click.
      await revealSavedFacts(page);
      await expect(detail.locator('[data-state="REFUNDED"]').first()).toBeVisible();
      expect(recoveryGets).toBeGreaterThan(0);
    } else {
      paymentState = "REFUNDED";
      expect(postTerminalReads).toBeLessThanOrEqual(2);
    }
    refreshedPaymentState = "REFUNDED";
    expect(terminal).toEqual({ captured, refunded: captured, pending: 0, refundable: 0, currency: "TWD" });""")
    source = replace(source, """    await expect(detail.locator('[data-state="MANUAL_UNASSIGNED"]').first()).toBeVisible();
  });

  test("RF11 a member""", """    await expect(detail.locator('[data-state="MANUAL_UNASSIGNED"]').first()).toBeVisible();
    await expect(detail.getByRole("button", { name: ui.refresh })).toHaveCount(0);
    refreshActions = await detail.getByRole("button", { name: ui.refresh }).count();
    stage = "pass";
    } finally {
      await writeFile(path.join(evidence, "g15-rf11.json"), JSON.stringify({
        profile, stage, postTerminalReads, extraAfterReplay, duplicateDrain, recoveryGets,
        paymentState, refreshedPaymentState, refreshActions, captured, partial, postAmount,
        refundPosts: posts.length, terminal, oldDetailServed, terminalListServed,
      }) + "\\n", { mode: 0o600 });
    }
  });

  test("RF11 a member""")
    return source


def instrument_go(source):
    return replace(source, "\t\te.assertPreSendReads(t)\n\t\tbrfShots(t, evidence, 12)", """\t\te.assertPreSendReads(t)
\t\tt.Logf("G15_RF11_PG succeeded=%d refunded_minor=%d captured_minor=%d fake_count=%d", succeeded, refunded, order.captured, len(e.fake.RefundIDs(order.pi)))
\t\tbrfShots(t, evidence, 12)""")


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("output", type=Path)
    args = parser.parse_args()
    out = args.output
    out.mkdir(parents=True, exist_ok=True)
    subprocess.run(["git", "merge-base", "--is-ancestor", PIN, "HEAD"], check=True)
    changed = subprocess.check_output(["git", "diff", "--name-only", PIN, "HEAD"], text=True).splitlines()
    if not changed or any(
        path != ".github/workflows/rf11-ablation-diagnostic.yml" and not path.startswith("experiments/rf11-ablation/")
        for path in changed
    ):
        raise SystemExit("diagnostic commit touches files outside the diagnostic suite")
    if subprocess.check_output(["git", "show", "-s", "--format=%T", PIN], text=True).strip() != TREE:
        raise SystemExit("pinned candidate tree differs")
    pins = {path: sha(pinned(path)) for path in SOURCE_PINS}
    for path, digest in pins.items():
        if sha(Path(path).read_bytes()) != digest:
            raise SystemExit(f"candidate workspace file differs: {path}")

    components = component_sources(pinned(COMPONENT).decode())
    for arm, contents in components.items():
        (out / f"component-{arm}.tsx").write_text(contents)
    Path(SPEC).write_text(instrument_browser(pinned(SPEC).decode()))
    Path(GO_TEST).write_text(instrument_go(pinned(GO_TEST).decode()))
    manifest = {
        "commit": PIN, "tree": TREE, "diagnostic_sha": subprocess.check_output(["git", "rev-parse", "HEAD"], text=True).strip(),
        "sources_sha256": pins, "experiment_sha256": {path: sha(Path(path).read_bytes()) for path in (SPEC, GO_TEST)},
        "component_sha256": {arm: sha(src.encode()) for arm, src in components.items()},
        "schedule": SCHEDULE, "evidence_class": "NATIVE_BROWSER / REAL_PG / MOCK_STRIPE",
        "single_factor": "repeat suppression of the last admitted refund amount pair",
        "fault": "controlled duplicate only: one re-versioned stale detail replay, then an identical native document refresh",
    }
    (out / "manifest.json").write_text(json.dumps(manifest, indent=2) + "\n")


if __name__ == "__main__":
    main()
