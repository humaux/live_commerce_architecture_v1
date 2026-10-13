# Purpose: CI-only, exact-source mutations of OA1 authority and liveness rules.
# Depends on: Python standard library, a disposable GitHub checkout, real PG gates.
# Used by: workflow-dispatch LC_OA_MUTATION; ordinary PR gates never mutate source.
# Invariants: a mutation must match exactly, preserve a source/hash/diff receipt,
# and let the unchanged designated acceptance test fail; never swallow failures.

import hashlib
import json
import os
from pathlib import Path
import subprocess


def replace_exact(path, before, after):
    file = Path(path)
    original = file.read_text()
    if original.count(before) != 1:
        raise SystemExit("mutation source did not match exactly: " + path)
    changed = original.replace(before, after, 1)
    file.write_text(changed)
    return {
        "path": path,
        "before_sha256": hashlib.sha256(original.encode()).hexdigest(),
        "after_sha256": hashlib.sha256(changed.encode()).hexdigest(),
    }


def main():
    if os.environ.get("GITHUB_ACTIONS") != "true":
        raise SystemExit("OA mutations run only in disposable GitHub CI checkouts")
    name = os.environ.get("LC_OA_MUTATION", "")
    edits = {
        "m1-fee-version": [(
            "migrations/0181_order_attempt_destination.sql",
            " IF NOT FOUND OR NOT v_current_service.enabled\n",
            " IF NOT FOUND OR NOT v_current_service.enabled\n"
            "  OR v_current<>(v_service->>'version')::bigint\n",
        )],
        "m3-remove-sweep": [(
            "internal/orderattempt/worker.go",
            "\tvar result SweepResult\n",
            "\tvar result SweepResult\n\tif e != nil { return result, nil }\n",
        )],
        "m5-disclose-principal": [(
            "migrations/0176_order_attempt_admission.sql",
            "    OR a.admission_provenance->>'principal_id' IS DISTINCT FROM auth_principal::text\n",
            "",
        ), (
            "migrations/0176_order_attempt_admission.sql",
            "   IF NOT FOUND OR s.principal_id<>auth_principal THEN\n",
            "   IF NOT FOUND THEN\n",
        )],
        "m6-skip-fence": [(
            "migrations/0178_order_attempt_execution_fence.sql",
            "  FOR UPDATE;\n",
            "  ;\n",
        )],
        "pending-only-replay": [(
            "migrations/0176_order_attempt_admission.sql",
            "   WHERE x.tenant_id=auth_tenant AND x.operation_family='merchant_order' AND x.transport_key_digest=p_key;\n",
            "   WHERE x.tenant_id=auth_tenant AND x.operation_family='merchant_order' AND x.transport_key_digest=p_key AND x.state='PENDING';\n",
        )],
    }
    if name not in edits:
        raise SystemExit("unknown OA mutation")
    root = Path(subprocess.check_output(["git", "rev-parse", "--show-toplevel"], text=True).strip())
    os.chdir(root)
    if subprocess.check_output(["git", "diff", "--name-only"], text=True).strip():
        raise SystemExit("refusing to mutate an already modified checkout")
    receipt = {
        "mutation": name,
        "source_sha": subprocess.check_output(["git", "rev-parse", "HEAD"], text=True).strip(),
        "changes": [replace_exact(*edit) for edit in edits[name]],
    }
    evidence = root / "output/ci-gates"
    evidence.mkdir(parents=True, exist_ok=True)
    (evidence / "oa1-mutation.json").write_text(json.dumps(receipt, indent=2) + "\n")
    (evidence / "oa1-mutation.patch").write_bytes(subprocess.check_output(["git", "diff", "--no-ext-diff"]))
    print("Applied OA mutation " + name + " at " + receipt["source_sha"], flush=True)


if __name__ == "__main__":
    main()
