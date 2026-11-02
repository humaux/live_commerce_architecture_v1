# Purpose: inject the three frozen manual-discount permission faults in CI only.
# Depends on: Python stdlib, git and a disposable workflow_dispatch checkout.
# Used by: gates.yml with one closed LC_MANUAL_DISCOUNT_MUTATION enum.
# Invariants: validate all inputs/anchors before one atomic source replacement;
# retain source SHA/hashes/SQL diff and let the unchanged PG gate report RED.

import hashlib
import json
import os
from pathlib import Path
import stat
import subprocess
import tempfile


SOURCE = "migrations/0220_manual_order_discount.sql"
MUTATION = "drop-catalog-fence"
MODE = "focused:^TestMerchantToolsManualOrder$/^FEAT-MANUAL-DISCOUNT$/^runtime_database_boundary$/^authorize$"
START = b"CREATE FUNCTION fulfillment.authorize_manual_order_discount("
END = b"\nEND $$;\n"
BEFORE = b"v_auth_error:=identity.merchant_access_denied(p_hash,p_store,ARRAY['inventory:reserve','catalog:read'],s.tenant_id,s.principal_id,s.authz_revision);"
AFTER = b"v_auth_error:=identity.merchant_access_denied(p_hash,p_store,ARRAY['inventory:reserve'],s.tenant_id,s.principal_id,s.authz_revision);"


# Closed plans only: each enum has one exact focused mode and function body.
PLANS = {
    MUTATION: (MODE, START, ((BEFORE, AFTER, 2),)),
    "drop-bind-catalog-fence": (
        "focused:^TestMerchantToolsManualOrder$/^FEAT-MANUAL-DISCOUNT$/^runtime_database_boundary$/^bind_authority$",
        b"CREATE FUNCTION fulfillment.bind_manual_order_discount(",
        ((BEFORE, AFTER, 2),)),
    "drop-adjustment-orders-fence": (
        "focused:^TestMerchantToolsManualOrder$/^FEAT-MANUAL-DISCOUNT$/^runtime_database_boundary$/^read_adjustments_authority$",
        b"CREATE FUNCTION identity.read_order_adjustments(",
        ((b"SELECT * INTO s FROM identity.resolve_access(p_hash,p_store,'orders:read');",
          b"SELECT * INTO s FROM identity.resolve_access(p_hash,p_store,'store:read');", 1),
         (b"v_auth_error:=identity.merchant_access_denied(p_hash,p_store,ARRAY['orders:read'],s.tenant_id,s.principal_id,s.authz_revision);",
          b"v_auth_error:=identity.merchant_access_denied(p_hash,p_store,ARRAY['store:read'],s.tenant_id,s.principal_id,s.authz_revision);", 1))),
}


def main():
    mutation = os.environ.get("LC_MANUAL_DISCOUNT_MUTATION")
    if mutation not in PLANS:
        raise SystemExit("unknown or combined manual discount mutation input")
    mode, function_start, replacements = PLANS[mutation]
    if (os.environ.get("GITHUB_ACTIONS") != "true"
            or os.environ.get("CI") != "true"
            or os.environ.get("GITHUB_EVENT_NAME") != "workflow_dispatch"
            or os.environ.get("MODE") != mode
            or os.environ.get("BUNDLE") != "false"
            or os.environ.get("LC_PREBUILT_DIR")):
        raise SystemExit("manual discount mutation requires its exact disposable CI dispatch mode, without prebuilt")
    if (os.environ.get("EXTRA_ENV") != "LC_MANUAL_DISCOUNT_MUTATION=" + mutation
            or os.environ.get("LC_OA_MUTATION")
            or os.environ.get("LC_IMPORT_FIELD_MUTATION")):
        raise SystemExit("unknown or combined manual discount mutation input")
    root = Path(subprocess.check_output(["git", "rev-parse", "--show-toplevel"], text=True).strip()).resolve()
    workspace = os.environ.get("GITHUB_WORKSPACE")
    if not workspace or Path(workspace).resolve() != root:
        raise SystemExit("mutation checkout must be the GitHub workspace")
    os.chdir(root)
    if subprocess.check_output(["git", "status", "--porcelain", "--untracked-files=no"], text=True).strip():
        raise SystemExit("refusing to mutate a tracked-dirty checkout")
    source_sha = subprocess.check_output(["git", "rev-parse", "HEAD"], text=True).strip()
    target = root / SOURCE
    if target.is_symlink() or target.resolve() != target or not target.is_file():
        raise SystemExit("mutation source must be the regular allowlisted checkout file")
    original = target.read_bytes()
    if original != subprocess.check_output(["git", "show", "HEAD:" + SOURCE]):
        raise SystemExit("mutation source differs from HEAD")
    if original.count(function_start) != 1:
        raise SystemExit("selected function anchor must occur exactly once")
    start = original.index(function_start)
    end = original.find(END, start)
    if end < 0:
        raise SystemExit("selected function end anchor missing")
    end += len(END)
    body = original[start:end]
    # Validate every original anchor before constructing or writing a mutation.
    for before, after, matches in replacements:
        if body.count(before) != matches or after in body:
            raise SystemExit("selected permission fence anchor count differs")
    changed_body = body
    for before, after, matches in replacements:
        changed_body = changed_body.replace(before, after)
    changed = original[:start] + changed_body + original[end:]
    changed_end = start + len(changed_body)
    if changed[:start] != original[:start] or changed[changed_end:] != original[end:]:
        raise SystemExit("mutation escaped selected function")
    evidence = root / "output/ci-gates"
    receipt_path = evidence / "manual-discount-mutation.json"
    patch_path = evidence / "manual-discount-mutation.patch"
    if evidence.resolve() != evidence or any(p.exists() or p.is_symlink() for p in (receipt_path, patch_path)):
        raise SystemExit("refusing unsafe evidence path or existing mutation receipt")
    # Every guard above runs before any source or evidence write. One source
    # file is replaced atomically; no test, grant, RLS or Go guard is touched.
    temp_name = None
    try:
        with tempfile.NamedTemporaryFile(dir=target.parent, prefix=".manual-discount-", delete=False) as temp:
            temp_name = temp.name
            temp.write(changed)
            temp.flush()
            os.fsync(temp.fileno())
        os.chmod(temp_name, stat.S_IMODE(target.stat().st_mode))
        os.replace(temp_name, target)
        temp_name = None
    finally:
        if temp_name is not None:
            os.unlink(temp_name)
    patch = subprocess.check_output(["git", "diff", "--no-ext-diff", "--no-textconv", "--", SOURCE])
    receipt = {
        "mutation": mutation, "source_sha": source_sha, "mode": mode,
        "changes": [{"path": SOURCE, "matches": sum(n for _, _, n in replacements),
                     "before_sha256": hashlib.sha256(original).hexdigest(),
                     "after_sha256": hashlib.sha256(changed).hexdigest()}],
        "patch_sha256": hashlib.sha256(patch).hexdigest(),
        "function_start": function_start.decode(),
        "replacements": [{"matches": n,
                          "before_anchor_sha256": hashlib.sha256(before).hexdigest(),
                          "after_anchor_sha256": hashlib.sha256(after).hexdigest()}
                         for before, after, n in replacements],
        "prefix_unchanged": changed[:start] == original[:start],
        "suffix_unchanged": changed[changed_end:] == original[end:],
        "prefix_sha256": hashlib.sha256(original[:start]).hexdigest(),
        "suffix_sha256": hashlib.sha256(original[end:]).hexdigest(),
    }
    evidence.mkdir(parents=True, exist_ok=True)
    receipt_path.write_text(json.dumps(receipt, indent=2) + "\n")
    patch_path.write_bytes(patch)
    print(json.dumps(receipt, sort_keys=True), flush=True)
    print(patch.decode(), flush=True)


if __name__ == "__main__":
    main()
