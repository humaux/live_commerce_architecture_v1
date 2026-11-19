"""Retain scalar observations, native test actions, and the matching browser leaf."""
import argparse
import hashlib
import json
from pathlib import Path
import re

parser = argparse.ArgumentParser()
parser.add_argument("out", type=Path)
parser.add_argument("--sample", type=int)
parser.add_argument("--profile")
parser.add_argument("--arm")
parser.add_argument("--exit-code", type=int)
parser.add_argument("--browser", type=Path)
parser.add_argument("--go", type=Path)
args = parser.parse_args()


def save(path, value):
    path.write_text(json.dumps(value, indent=2) + "\n")


def read_sample():
    matches = sorted(args.browser.glob("refund/*"))
    directory = matches[0] if len(matches) == 1 else None
    result_path = directory / "g15-rf11.json" if directory else None
    raw_browser = directory / "playwright-full.log" if directory else None
    observations = json.loads(result_path.read_text()) if result_path and result_path.is_file() else None
    browser = []
    screenshot_manifest = None
    visibility_failure = None
    if raw_browser and raw_browser.is_file():
        browser = [
            "PASS" if line.lstrip().startswith("✓") else "FAIL"
            for line in raw_browser.read_text().splitlines()
            if "RF11 full refund of the remainder:" in line and line.lstrip().startswith(("✓", "✘", "×"))
        ]
        lines = raw_browser.read_text().splitlines()
        for line in lines:
            verdict = re.fullmatch(r"\s*Received:\s+(hidden|visible|<element\(s\) not found>)\s*", line)
            if verdict:
                visibility_failure = verdict[1]
                break
        screenshots = directory / "screenshots.json"
        if screenshots.is_file():
            screenshot_manifest = {
                "entries": len(json.loads(screenshots.read_text())),
                "sha256": hashlib.sha256(screenshots.read_bytes()).hexdigest(),
            }
    go_actions = []
    pg_observations = []
    stage = None
    if args.go.is_file():
        for line in args.go.read_text().splitlines():
            try:
                event = json.loads(line)
            except ValueError:
                continue
            name = event.get("Test", "")
            action = event.get("Action")
            message = event.get("Output", "")
            if action == "start" or (name.startswith("TestBrowserRefund") and action in {"run", "pass", "skip", "fail"}):
                go_actions.append({key: event[key] for key in ("Time", "Action", "Test", "Elapsed") if key in event})
            pg = re.search(r"G15_RF11_PG succeeded=(\d+) refunded_minor=(\d+) captured_minor=(\d+) fake_count=(\d+)", message)
            if pg:
                pg_observations.append(dict(zip(("succeeded", "refunded", "captured", "fake_count"), map(int, pg.groups()))))
            if "admin browser gate failed:" in message:
                stage = "admin-browser"
            elif "storefront browser gate" in message and "failed" in message:
                stage = "storefront-browser"
            elif "panic: test timed out" in message:
                stage = "go-timeout"
    result = {
        "sample": args.sample, "profile": args.profile, "arm": args.arm, "exit_code": args.exit_code,
        "browser": observations, "go_actions": go_actions, "leaf_verdict": browser,
        "pg": pg_observations, "screenshot_manifest": screenshot_manifest,
        "first_process_failure": stage, "visibility_failure": visibility_failure,
    }
    save(args.out / f"sample-{args.sample}.json", result)
    print(json.dumps(result, ensure_ascii=False, separators=(",", ":")))


def problems(sample):
    b = sample.get("browser") or {}
    pg = sample.get("pg") or []
    captured = b.get("captured")
    partial = b.get("partial")
    if sample["exit_code"] != 0 or not b or b.get("stage") != "pass":
        return ["native browser or Go test did not pass"]
    checks = {
        "named RF11 full leaf did not pass exactly once": sample["leaf_verdict"] == ["PASS"],
        "wrong observed profile": sample["profile"] == b.get("profile"),
        "missing fixture race": b.get("oldDetailServed") and b.get("terminalListServed"),
        "wrong immutable fixture amounts": captured == 2500 and partial == 1000 and b.get("postAmount") == 1500,
        "wrong POST or missing terminal UI": b.get("refundPosts") == 1 and b.get("refreshedPaymentState") == "REFUNDED" and b.get("refreshActions") == 0,
        "wrong terminal money/currency": b.get("terminal") == {"captured": captured, "refunded": captured, "pending": 0, "refundable": 0, "currency": "TWD"},
        "wrong PG or fake provider totals": pg == [{"succeeded": 2, "refunded": captured, "captured": captured, "fake_count": 2}],
        "did not complete native MOCK leaf": any(v.get("Action") == "pass" and v.get("Test", "").startswith("TestBrowserRefund/MOCK:") for v in sample["go_actions"]),
    }
    if sample["profile"] == "normal":
        checks["normal RF11 did not auto-converge within its original GET bound"] = 1 <= b.get("postTerminalReads", 0) <= 2 and b.get("paymentState") == "REFUNDED" and b.get("recoveryGets") == 0
        checks["normal RF11 entered diagnostic replay"] = b.get("extraAfterReplay") is None
    else:
        kept = sample["arm"] == "on"
        checks["common native refresh did not perform one detail GET"] = b.get("recoveryGets") == 1
        checks["wrong suppression decision or number of additional reads"] = (
            b.get("extraAfterReplay") == (0 if kept else 1)
            and b.get("duplicateDrain") == ("suppressed" if kept else "drained")
            and b.get("paymentState") == ("PARTIALLY_REFUNDED" if kept else "REFUNDED")
            and b.get("postTerminalReads") == (1 if kept else 2)
        )
    return [message for message, passed in checks.items() if not passed]


def summarize():
    manifest = json.loads((args.out / "manifest.json").read_text())
    expected = manifest["schedule"]
    samples = []
    for index, (profile, arm) in enumerate(expected, 1):
        p = args.out / f"sample-{index}.json"
        if not p.is_file():
            samples.append({"sample": index, "profile": profile, "arm": arm, "failures": ["missing sample"]})
            continue
        sample = json.loads(p.read_text())
        if (sample["profile"], sample["arm"]) != (profile, arm):
            sample["failures"] = ["fixed schedule mismatch"]
        else:
            sample["failures"] = problems(sample)
        samples.append(sample)
    valid = len(expected) == 6 and all(not sample["failures"] for sample in samples)
    result = {"status": "PASS" if valid else "INVALID", "fixed_samples": len(expected), "samples": samples}
    save(args.out / "result.json", result)
    print(json.dumps(result, ensure_ascii=False, separators=(",", ":")))
    raise SystemExit(0 if valid else 1)


if args.sample is not None:
    read_sample()
else:
    summarize()
