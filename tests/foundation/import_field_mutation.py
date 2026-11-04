# Purpose: inject the three migration-import-v1 §6.1 source faults in CI only.
# Depends on: Python stdlib, git and a disposable workflow_dispatch checkout.
# Used by: gates.yml with an explicit LC_IMPORT_FIELD_MUTATION closed name.
# Invariants: validate every anchor before writing; preserve exact HEAD/hashes/
# patch; never change test oracles or convert an unchanged gate's RED to PASS.

import hashlib
import json
import os
from pathlib import Path
import subprocess


# Fixed recipes, not a patch/script executor. Product sources are only changed
# in the disposable CI snapshot; the submitted tree retains their exact bytes.
EDITS = {
    "unmarked-metadata": [(
        "internal/httpapi/imports.go",
        'if !rowErrors || row.Outcome != "failed"',
        'if row.Outcome != "failed"',
    )],
    "drop-commit-marker": [(
        "apps/admin/lib/import-client.ts",
        'if(rowErrors!==undefined)query.set("row_errors",rowErrors);',
        'if(rowErrors!==undefined && action==="preview")query.set("row_errors",rowErrors);',
    )],
    "capability-identity": [(
        "internal/migrationimport/customers.go",
        "func CustomersCommit(ctx context.Context, tx pgx.Tx, scope platform.Scope, token string, data []byte, mapping map[string]string, expectedApplyRows int64) (CustomerCommitResult, error) {",
        "func CustomersCommit(ctx context.Context, tx pgx.Tx, scope platform.Scope, token string, data []byte, mapping map[string]string, expectedApplyRows int64, rowErrors ...bool) (CustomerCommitResult, error) {",
    ), (
        "internal/httpapi/imports.go",
        "return migrationimport.CustomersCommit(ctx, tx, s, bearerToken(r), data, mapping, expected)",
        'return migrationimport.CustomersCommit(ctx, tx, s, bearerToken(r), data, mapping, expected, q.Get("row_errors") == "field-v1")',
    ), (
        "internal/migrationimport/customers.go",
        '\t\tExpectedApplyRows int64             `json:"expected_apply_rows"`\n\t}{digest, mapping, expectedApplyRows}, &res, func() error {',
        '\t\tExpectedApplyRows int64             `json:"expected_apply_rows"`\n\t\tRowErrors         []bool            `json:"row_errors"`\n\t}{digest, mapping, expectedApplyRows, rowErrors}, &res, func() error {',
    )],
}


def main():
    if (os.environ.get("GITHUB_ACTIONS") != "true"
            or os.environ.get("GITHUB_EVENT_NAME") != "workflow_dispatch"):
        raise SystemExit("import field mutations require disposable GitHub workflow_dispatch CI")
    if os.environ.get("LC_OA_MUTATION"):
        raise SystemExit("refusing simultaneous OA and import field mutations")
    name = os.environ.get("LC_IMPORT_FIELD_MUTATION", "")
    if name not in EDITS:
        raise SystemExit("unknown import field mutation")
    root = Path(subprocess.check_output(["git", "rev-parse", "--show-toplevel"], text=True).strip())
    os.chdir(root)
    # Includes staged and unstaged tracked changes; untracked evidence is kept.
    if subprocess.check_output(["git", "status", "--porcelain", "--untracked-files=no"], text=True).strip():
        raise SystemExit("refusing to mutate a tracked-dirty checkout")
    source_sha = subprocess.check_output(["git", "rev-parse", "HEAD"], text=True).strip()
    originals, planned = {}, {}
    for path, before, after in EDITS[name]:
        file = root / path
        if file.is_symlink() or file.resolve() != file or not file.is_file():
            raise SystemExit("mutation source is not a regular checkout file: " + path)
        if path not in originals:
            originals[path] = file.read_bytes()
            planned[path] = originals[path]
        before, after = before.encode(), after.encode()
        if originals[path].count(before) != 1 or after in originals[path]:
            raise SystemExit("mutation source did not match exactly: " + path)
        if planned[path].count(before) != 1:
            raise SystemExit("mutation replacements overlap: " + path)
        planned[path] = planned[path].replace(before, after, 1)
        if planned[path].count(after) != 1:
            raise SystemExit("mutation replacement did not match exactly: " + path)
    # No source or evidence writes happen until ALL recipe anchors are valid.
    changes = []
    for path, changed in planned.items():
        (root / path).write_bytes(changed)
        changes.append({
            "path": path,
            "before_sha256": hashlib.sha256(originals[path]).hexdigest(),
            "after_sha256": hashlib.sha256(changed).hexdigest(),
        })
    receipt = {"mutation": name, "source_sha": source_sha, "changes": changes}
    evidence = root / "output/ci-gates"
    evidence.mkdir(parents=True, exist_ok=True)
    (evidence / "import-field-mutation.json").write_text(json.dumps(receipt, indent=2) + "\n")
    (evidence / "import-field-mutation.patch").write_bytes(subprocess.check_output(
        ["git", "diff", "--no-ext-diff", "--no-textconv", "--", *planned]))
    print("Applied import field mutation " + name + " at " + source_sha, flush=True)
    for change in changes:
        print("Patched " + change["path"] + " before=" + change["before_sha256"]
              + " after=" + change["after_sha256"], flush=True)


if __name__ == "__main__":
    main()
