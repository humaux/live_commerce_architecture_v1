#!/usr/bin/env python3
# Purpose: Stage original G90/G91 receipts for upload by the ordinary runner.
# Depends on: release-gate.sh command logs, smoke.sh evidence layout, Python, Linux sudo.
# Used by: the strict release job in .github/workflows/gates.yml.
import json
import os
import re
import subprocess
import sys
from datetime import datetime
from pathlib import Path

TIMESTAMP = r"\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z"

# The privileged subprocess only reads a same-run regular file at an allowlisted path.
READ_NATIVE = """
import os, stat, sys
from pathlib import Path
root, run_id, name = sys.argv[1:]
flags = os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW
fd = os.open('/', flags)
for part in (*Path(root).parts[1:], 'deploy', '.evidence', run_id):
    next_fd = os.open(part, flags, dir_fd=fd)
    os.close(fd)
    fd = next_fd
try:
    raw = os.open(name, os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK, dir_fd=fd)
except FileNotFoundError:
    sys.exit(2)
with os.fdopen(raw, 'rb') as src:
    st = os.fstat(src.fileno())
    if not stat.S_ISREG(st.st_mode) or st.st_nlink != 1:
        sys.exit(1)
    sys.stdout.buffer.write(src.read())
"""


def stage(gate, mode, workspace, logs, destination, commit):
    lines = (logs / f"{gate}.log").read_text(errors="replace").splitlines()
    starts = [m[1] for line in lines if (m := re.fullmatch(rf"# started: ({TIMESTAMP})", line))]
    exits = [m[1] for line in lines if (m := re.fullmatch(r"# exit: ([0-9]+)", line))]
    markers = [line for line in lines if "smoke.sh INFO: evidence:" in line]
    prefix = re.escape(str(workspace / "deploy/.evidence"))
    match = re.fullmatch(
        rf"({TIMESTAMP}) smoke\.sh INFO: evidence: {prefix}/"
        rf"(\d{{8}}T\d{{6}}Z-[0-9a-f]{{4}})/result\.json  overall=(PASS|BLOCKED|FAIL)",
        markers[0],
    ) if len(markers) == 1 else None
    if (not match or len(starts) != 1 or len(exits) != 1 or len(lines) < 3
            or not lines[1].startswith(f"# commit: {commit[:12]} dirty_files: ")):
        raise ValueError("missing, duplicate or unexpected command evidence")
    ended, run_id, overall = match.groups()
    started = datetime.strptime(run_id.split("-")[0], "%Y%m%dT%H%M%SZ").strftime("%Y-%m-%dT%H:%M:%SZ")
    if not (starts[0] <= started <= ended) or {"0": "PASS", "1": "FAIL", "3": "BLOCKED"}.get(exits[0]) != overall:
        raise ValueError("native run time or terminal disagrees with command evidence")

    output = destination / run_id
    output.mkdir(mode=0o700)
    command = (["sudo", "-n"] if mode == "full" else []) + [
        "python3", "-I", "-c", READ_NATIVE, str(workspace), run_id,
    ]

    errors = []

    def read(name, required=True):
        try:
            observed = subprocess.run(command + [name], capture_output=True, timeout=15)
        except (OSError, subprocess.SubprocessError):
            errors.append(f"{name}: native read failed")
            return None
        if observed.returncode == 0 and observed.stdout:
            return observed.stdout
        if not required and observed.returncode == 2:
            return None
        errors.append(f"{name}: native file missing, empty or unreadable (exit {observed.returncode})")
        return None

    image_required = False
    result = read("result.json")
    if result is not None:
        try:
            receipt = json.loads(result)
            image_required = mode == "full" and any(
                case["id"] == "S07" and case["status"] == "PASS" for case in receipt["results"]
            )
            identity = (receipt["run_id"], receipt["environment"]["mode"], receipt["commit"], receipt["overall"])
            receipt_times = (receipt["started_at"], receipt["ended_at"])
            in_window = (all(re.fullmatch(TIMESTAMP, value) for value in receipt_times)
                         and started <= receipt_times[0] <= receipt_times[1] <= ended)
        except (ValueError, KeyError, TypeError):
            errors.append("result.json: invalid native receipt")
        else:
            if identity != (run_id, mode, commit, overall) or not in_window:
                raise ValueError("result.json: native receipt belongs to a different execution")
            (output / "result.json").write_bytes(result)

    names = [("cases.tsv", True), ("commands.tsv", True)]
    if mode == "full":
        names.append(("images.jsonl", image_required))
    for name, required in names:
        raw = read(name, required)
        if raw is not None:
            (output / name).write_bytes(raw)
    if errors:
        raise ValueError(", ".join(errors))


def main():
    workspace, logs, destination = map(Path, sys.argv[1:4])
    if not workspace.is_absolute() or ".." in workspace.parts:
        sys.exit("Smoke evidence staging requires an absolute checkout path")
    os.umask(0o077)
    failed = False
    for gate, mode in (("G90", "static"), ("G91", "full")):
        try:
            stage(gate, mode, workspace, logs, destination, sys.argv[4])
        except (OSError, ValueError) as error:
            print(f"{gate} smoke evidence staging failed: {error}", file=sys.stderr)
            failed = True
    return int(failed)


if __name__ == "__main__":
    sys.exit(main())
