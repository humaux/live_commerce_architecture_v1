#!/usr/bin/env python3
# Binds the strict parent to three terminal child jobs in the same Actions attempt.
# Uses GitHub's read-only APIs and Sigstore through gh. The parent never reads Stripe credentials.
import hashlib
import io
import json
import os
import re
import subprocess
import sys
import urllib.error
import urllib.parse
import urllib.request
import zipfile
from pathlib import Path

REPO = "humaux/live_commerce_architecture_v1"
REF = "refs/heads/r3/integration"
GATES = ".github/workflows/gates.yml"
STRIPE = ".github/workflows/stripe-sandbox.yml"
KINDS = {
    "g07": ("Seal canonical G07", GATES, "Bind canonical G07 native evidence"),
    "api": ("Stripe API / sandbox", STRIPE, "API sandbox gate (G07z)"),
    "browser": ("Stripe browser / sandbox", STRIPE, "SANDBOX gate"),
}
PROBES = ["TestStripeSP16Sandbox", "TestStripeRF10Sandbox",
          "TestStripeSP21Registrar/sandbox_probe_against_real_stripe"]
API_SKIPS = {
    "TestStripeSP16Sandbox/expires_at_29min_rejected",
    "TestStripeRF10Sandbox/rotated_key:_same-key_replay_on_the_same_account_returns_the_same_refund_(RD13)",
}
BROWSER_STEPS = {"node-env": 0, "sp18": 6, "su07": 1, "su09": 2}
MAX_ARCHIVE = 32 * 1024 * 1024


def require(condition, message):
    if not condition:
        raise ValueError(message)


def command(*args):
    return subprocess.check_output(args, text=True, timeout=90)


def identity(env=os.environ):
    run, attempt, sha = (env[k] for k in ("GITHUB_RUN_ID", "GITHUB_RUN_ATTEMPT", "GITHUB_SHA"))
    require(all(re.fullmatch(r"[1-9][0-9]*", x) for x in (run, attempt)), "invalid run or attempt")
    require(re.fullmatch(r"[0-9a-f]{40}", sha), "invalid SHA")
    require(env["GITHUB_REPOSITORY"] == REPO and env["GITHUB_REF"] == REF
            and env["GITHUB_EVENT_NAME"] == "workflow_dispatch"
            and env["GITHUB_WORKFLOW_REF"] == f"{REPO}/{GATES}@{REF}"
            and env["GITHUB_WORKFLOW_SHA"] == sha, "untrusted release workflow context")
    require(command("git", "rev-parse", "HEAD").strip() == sha, "checkout differs from subject")
    status = command("git", "status", "--porcelain", "--untracked-files=normal")
    require(all(line.startswith("?? output/") for line in status.splitlines()), "checkout dirty")
    return {"run_id": int(run), "attempt": int(attempt), "sha": sha, "ref": REF,
            "tree": command("git", "rev-parse", "HEAD^{tree}").strip()}


def api_projection(out):
    rows = [line.split("\t") for line in (out / "results.tsv").read_text().splitlines()]
    passed = [r for r in rows if r[:3] == ["G07z", "SANDBOX", "PASS"] and r[4] == "0"]
    skip_rows = [r for r in rows if r[:3] == ["G07z+", "SANDBOX", "NOT_RUN"]]
    require(len(passed) == 1 and len(skip_rows) <= 1 and len(rows) == 1 + len(skip_rows),
            "API native strict result incomplete")
    skipped = (out / "G07z.skipped").read_text().splitlines()
    require(len(skipped) == len(set(skipped)) and set(skipped) <= API_SKIPS
            and bool(skipped) == bool(skip_rows), "API skip list has unauthorised or duplicate tests")
    return {"passed": PROBES, "skipped": sorted(skipped)}


def browser_projection(log):
    rows = re.findall(r"^(node-env|sp18|su07|su09): pass=[1-9]\d* fail=0 skip=0 "
                      r"leaf_pass=(\d+) leaf_fail=0 leaf_skip=0 exit=0 verdict=0(?:\s|$)", log, re.M)
    marks = re.findall(r"^(node-env|sp18|su07|su09):", log, re.M)
    passed = {step: int(count) for step, count in rows}
    require(len(rows) == len(marks) == len(BROWSER_STEPS) and passed.keys() == BROWSER_STEPS.keys()
            and all(passed[s] >= n for s, n in BROWSER_STEPS.items())
            and len(re.findall(r"^PASS: stripe-browser steps ", log, re.M)) == 1
            and not re.search(r"^(?:FAIL|--- FAIL)(?::|\s|$)", log, re.M), "browser native terminal/coverage")
    reasons = [line for line in log.splitlines() if "NOT_RUN" in line]
    require(reasons and set(reasons) == {"SANDBOX checkout.stripe.com test mode; no live charge; SP17 webhook NOT_RUN"},
            "browser changed SP17 or emitted another skip")
    return {"passed": passed, "skipped": ["SP17"]}


def seal(kind, out, artifact_id, digest, source):
    context = identity()
    if kind == "api":
        result = json.loads(source.read_text())
    elif kind == "browser":
        result = browser_projection(source.read_text())
    else:
        result = {}
    require(re.fullmatch(r"[1-9][0-9]*", artifact_id)
            and re.fullmatch(r"sha256:[0-9a-f]{64}", digest), "missing original artifact ID/digest")
    out.mkdir(parents=True)
    (out / "receipt.json").write_text(json.dumps({
        **context, "kind": kind, "artifact_id": int(artifact_id), "digest": digest, "result": result,
    }, sort_keys=True) + "\n")


def api(route, key=None):
    flags = ("--paginate", "--slurp") if key else ()
    data = json.loads(command("gh", "api", *flags, f"repos/{REPO}/{route}"))
    return [item for page in data for item in page[key]] if key else data


def attempt_binding(run, jobs, specs, context):
    require(run["path"] == GATES and run["event"] == "workflow_dispatch"
            and run["head_sha"] == context["sha"] and run["head_branch"] == REF.removeprefix("refs/heads/")
            and run["id"] == context["run_id"] and run["run_attempt"] == context["attempt"],
            "GitHub run provenance mismatch")
    bound = {}
    expected = {k: (name, [step, "Sign strict child receipt", "Upload strict child receipt"])
                for k, (name, _, step) in specs.items()}
    expected["canonical"] = ("receipts", ["Strict canonical G07 intake", "Canonical finalization deadline"])
    for kind, (name, required_steps) in expected.items():
        matching = [job for job in jobs if job["name"] == name]
        require(len(matching) == 1, f"{kind}: missing or duplicate expected job")
        job = matching[0]
        require(job["status"] == "completed" and job["conclusion"] == "success"
                and job["head_sha"] == context["sha"] and job["run_id"] == context["run_id"]
                and job.get("run_attempt", context["attempt"]) == context["attempt"],
                f"{kind}: child job lacks same-attempt success")
        steps = {s["name"]: s["conclusion"] for s in job["steps"]}
        require(all(steps.get(s) == "success" for s in required_steps),
                f"{kind}: incomplete native, signature or upload step")
        bound[kind] = job["id"]
    return bound


def artifact(artifacts, name, ctx):
    matches = [item for item in artifacts if item["name"] == name]
    require(len(matches) == 1, f"{name}: missing or duplicate artifact")
    item = matches[0]
    run = item["workflow_run"]
    require(not item["expired"] and 0 < item["size_in_bytes"] <= MAX_ARCHIVE
            and run["id"] == ctx["run_id"] and run["head_sha"] == ctx["sha"]
            and run["head_branch"] == REF.removeprefix("refs/heads/")
            and re.fullmatch(r"sha256:[0-9a-f]{64}", item["digest"]), f"{name}: invalid artifact metadata")
    return item


class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, *_args):
        return None


def download(item):
    url = f"https://api.github.com/repos/{REPO}/actions/artifacts/{item['id']}/zip"
    headers = {"Authorization": "Bearer " + os.environ["GH_TOKEN"], "Accept": "application/vnd.github+json"}
    opener = urllib.request.build_opener(NoRedirect)
    try:
        response = opener.open(urllib.request.Request(url, headers=headers), timeout=30)
    except urllib.error.HTTPError as error:
        target = urllib.parse.urlsplit(error.headers.get("Location", ""))
        require(error.code in (301, 302, 303, 307, 308) and target.scheme == "https"
                and target.hostname and target.hostname.endswith((".blob.core.windows.net", ".actions.githubusercontent.com")),
                "unexpected artifact download redirect")
        response = opener.open(target.geturl(), timeout=30)
    with response:
        data = response.read(MAX_ARCHIVE + 1)
    require(len(data) <= MAX_ARCHIVE and "sha256:" + hashlib.sha256(data).hexdigest() == item["digest"],
            "artifact ZIP digest mismatch")
    archive = zipfile.ZipFile(io.BytesIO(data))
    files = archive.infolist()
    require(len(files) == len(set(archive.namelist())) and sum(f.file_size for f in files) <= 128 * 1024 * 1024,
            "duplicate ZIP members or excessive unpacked size")
    return archive


def verified_receipt(path, bundle, kind, ctx):
    signer = f"{REPO}/{KINDS[kind][1]}"
    verified = json.loads(command("gh", "attestation", "verify", str(path), "--bundle", str(bundle),
                                  "--repo", REPO, "--signer-workflow", signer,
                                  "--signer-digest", ctx["sha"], "--source-ref", REF, "--source-digest", ctx["sha"],
                                  "--deny-self-hosted-runners", "--format", "json"))
    invocation = f"https://github.com/{REPO}/actions/runs/{ctx['run_id']}/attempts/{ctx['attempt']}"
    require(len(verified) == 1 and verified[0]["verificationResult"]["statement"]["predicate"]
            ["runDetails"]["metadata"]["invocationId"] == invocation, f"{kind}: attestation invocation mismatch")
    receipt = json.loads(path.read_text())
    require(all(receipt.get(k) == v for k, v in ctx.items()) and receipt.get("kind") == kind,
            f"{kind}: signed receipt identity mismatch")
    return receipt


def native_g07(archive, out):
    raw = archive.read("g07-verdict/G07.jsonl")
    conditional = archive.read("g07-verdict/G07.conditional")
    result = archive.read("g07-verdict/results.tsv").decode().splitlines()
    require(sum(line.startswith("G07\tREAL_PG\tPASS\t") for line in result) == 1
            and all(line.split("\t")[2] in ("PASS", "NOT_RUN") and "UNACCEPTED" not in line
                    for line in result), "canonical strict G07 result")
    events = [json.loads(line) for line in raw.splitlines()]
    required = {
        ("livecommerce/bench/l01/neutral/go", "TestHTTPPostgres"),
        ("livecommerce/tests/foundation", "TestLiveBrowserInputProcessNativeQueues"),
        ("livecommerce/tests/foundation", "TestLiveBrowserInputProcessNativeMixedRecovery"),
    }
    for key in required:
        # Foundation has multiple native shard package terminals, all already verified by canonical G07.
        found = [e["Action"] for e in events if (e.get("Package"), e.get("Test")) == key
                 and e["Action"] in ("pass", "skip", "fail")]
        require(found == ["pass"], f"native G07 missing {key[1]}")
    w1 = "".join(e["Output"] for e in events if e.get("Test") == "TestHTTPPostgres" and e["Action"] == "output"
                 and e["Package"] == "livecommerce/bench/l01/neutral/go")
    require(re.search(r"REAL_PG_READBACK pairs=[1-9]\d* quarantine=[1-9]\d* rejected_mutations=[1-9]\d*", w1),
            "native W1 fixture/readback evidence missing")
    (out / "G07.jsonl").write_bytes(raw)
    (out / "G07.conditional").write_bytes(conditional)


def intake(out):
    ctx = identity()
    prefix = f"actions/runs/{ctx['run_id']}"
    attempt = f"{prefix}/attempts/{ctx['attempt']}"
    jobs = attempt_binding(api(attempt), api(attempt + "/jobs", "jobs"), KINDS, ctx)
    artifacts = api(prefix + "/artifacts", "artifacts")
    out.mkdir()
    for kind in KINDS:
        stem = f"strict-{kind}-a{ctx['attempt']}"
        signed = artifact(artifacts, stem, ctx)
        folder = out / kind
        folder.mkdir()
        with download(signed) as archive:
            require(set(archive.namelist()) == {"receipt.json", "attestation.json"}, f"{kind}: unexpected signed members")
            for filename in archive.namelist():
                (folder / filename).write_bytes(archive.read(filename))
        receipt = verified_receipt(folder / "receipt.json", folder / "attestation.json", kind, ctx)
        original_name = f"g07-verdict-a{ctx['attempt']}" if kind == "g07" else stem + "-native"
        original = artifact(artifacts, original_name, ctx)
        require((original["id"], original["digest"]) == (receipt["artifact_id"], receipt["digest"]),
                f"{kind}: signed native artifact mismatch")
        with download(original) as archive:
            if kind == "g07":
                native_g07(archive, out)
            elif kind == "api":
                expected = f"G07z SANDBOX PASS SP16 RF10 SP21/sandbox_probe_against_real_stripe {ctx['sha']}\n"
                require(set(archive.namelist()) == {"result.txt", "probes.json"} and archive.read("result.txt").decode() == expected
                        and json.loads(archive.read("probes.json")) == receipt["result"]
                        and receipt["result"]["passed"] == PROBES
                        and set(receipt["result"]["skipped"]) <= API_SKIPS, "invalid API native result")
            else:
                logs = [n for n in archive.namelist() if n.endswith("/ci-gates/stripe-browser.log")]
                require(len(logs) == 1 and receipt["result"] == browser_projection(archive.read(logs[0]).decode()),
                        "browser native projection mismatch")
        (folder / "artifacts.json").write_text(json.dumps({
            "native_id": original["id"], "native_digest": original["digest"],
            "receipt_id": signed["id"], "receipt_digest": signed["digest"], "seal_job_id": jobs[kind],
            "native_job_id": jobs["canonical"] if kind == "g07" else jobs[kind],
        }) + "\n")
    (out / "G07z.skipped").write_text("\n".join(json.loads((out / "api/receipt.json").read_text())["result"]["skipped"]))


if __name__ == "__main__":
    try:
        if sys.argv[1] == "seal":
            _, _, kind, output, aid, digest, source = sys.argv
            seal(kind, Path(output), aid, digest, Path(source))
        elif sys.argv[1] == "project-api":
            Path(sys.argv[3]).write_text(json.dumps(api_projection(Path(sys.argv[2]))) + "\n")
        elif sys.argv[1] == "intake":
            intake(Path(sys.argv[2]))
        else:
            raise ValueError("unknown evidence operation")
    except (OSError, ValueError, KeyError, subprocess.SubprocessError, zipfile.BadZipFile) as error:
        sys.exit(f"strict release evidence: {error}")
