#!/usr/bin/env python3
# Correlates native Go events for release-gate.sh. Only a successful non-Stripe G07
# package can cover its own G06 skips. Stripe tests retain their separate G07z ruling.
import json
import re
import sys
from collections import defaultdict
from pathlib import Path

STRIPE = ("livecommerce/internal/integrations/psp/stripe", "TestStripeSP16Sandbox")


def events(file):
    for line in file.read_text().splitlines():
        if line.startswith("{"):
            yield json.loads(line)


def native_passes(file):
    packages, tests = defaultdict(list), defaultdict(list)
    for event in events(file):
        action = event["Action"]
        package = event.get("Package")
        if not package or action not in ("pass", "fail", "skip", "build-fail"):
            continue
        test = event.get("Test")
        if test:
            tests[package, test].append(action)
        else:
            packages[package].append(action)
    return {key for key, end in tests.items()
            if end == ["pass"] and packages[key[0]] == ["pass"]}


def verbose_passes(file):
    passed, pending = set(), []
    for line in file.read_text(errors="replace").splitlines():
        test = re.match(r"\s*--- PASS: (\S+) \(", line)
        if test:
            pending.append(test[1])
        package = re.match(r"(ok|FAIL)\s+(\S+)", line)
        if package:
            if package[1] == "ok":
                passed.update((package[2], name) for name in pending)
            pending.clear()
    return passed


def covered_skips(out, mode):
    skipped = [(e["Package"], e["Test"]) for e in events(out / "G06.jsonl")
               if e["Action"] == "skip" and e.get("Test")]
    if mode == "native":
        passed = native_passes(out / "G07.jsonl")
    elif mode == "verbose":
        passed = verbose_passes(out / "G07.log")
    else:
        passed = set()

    def describe(key):
        return ".".join(key)

    def covered(key):
        package, test = key
        stripe = package == STRIPE[0] and (test == STRIPE[1] or test.startswith(STRIPE[1] + "/"))
        return key in passed and not stripe

    (out / "G06s.covered").write_text("\n".join(describe(key) for key in skipped if covered(key)))
    (out / "G06s.skipped").write_text("\n".join(describe(key) for key in skipped if not covered(key)))


if __name__ == "__main__":
    covered_skips(Path(sys.argv[1]), sys.argv[2])
