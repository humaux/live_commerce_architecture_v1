# CTUI readiness source review — E1

- task_id: `0cb19bd4-sub-ctui-readiness-review`; parent: `0cb19bd4-2062-42dc-99f9-322a1bfb828c`.
- Reviewer: independent_review, read-only role; assigned gpt-6.1-sol/high, runtime model identity UNKNOWN.
- base_commit: `5ca3b82cede63d2e691b3b3b88e0da80c945ed20`.
- Reviewed worktree: `/Volumes/data/live_commerce_architecture_v1/.worktrees/w3-u4-parcel-merge-ui`.
- Change path: `tests/admin/customer-tags.spec.ts:144`; only three explanatory comment lines and two readiness assertions added before the existing fault control call. Product/BFF unchanged.
- Reviewed spec SHA256: `100557a7df9255e42efad7b56d767bbf9172cf43fe53b309dbd067bee4b60e52` (reconfirmed before receipt).

No confirmed scoped P0/P1 or missing readiness barrier found. `CustomerTags.tsx:29–36,140–146` renders the Seed01 checkbox only after the actual catalogue read; detail badges cannot satisfy it. `CustomerTagsNotes.tsx:23–24,48–51,135` starts without a cursor, resets it before the independent read, and enables pagination only with a returned cursor and no active read/write. Prefilled detail notes alone cannot satisfy the second barrier. The existing seeded fault loop already requires pagination. Precise customer tag/notes BFF leaf routes use the customer-tags grammar; parcel catchall paths are disjoint. No product privacy fence or error assertion was relaxed.

## Actual reviewer commands and results

In the reviewed worktree, this AST-only command exited **0**:

```sh
node --input-type=module <<'NODE'
import fs from 'node:fs';import cp from 'node:child_process';import ts from 'typescript-api';const file='tests/admin/customer-tags.spec.ts',p=ts.createPrinter({removeComments:true});function a(s){const ast=ts.createSourceFile(file,s,ts.ScriptTarget.Latest,true),r=[];function w(n){if(ts.isCallExpression(n)&&n.expression.getText(ast)==='expect'){let v=n;while(v.parent&&(ts.isCallExpression(v.parent)||ts.isPropertyAccessExpression(v.parent)||ts.isAwaitExpression(v.parent)))v=v.parent;r.push(p.printNode(ts.EmitHint.Unspecified,v,ast));}ts.forEachChild(n,w);}w(ast);return r;}
const old=a(cp.execFileSync('git',['show','5ca3b82c:'+file],{encoding:'utf8'})),fresh=a(fs.readFileSync(file,'utf8'));let i=0;const added=[];for(const s of fresh){if(s===old[i])i++;else added.push(s);}const ok=i===old.length&&added.length===2;console.log(JSON.stringify({oldExpectChains:old.length,newExpectChains:fresh.length,allOriginalsAndOrderPreserved:ok,added}));if(!ok)process.exitCode=1;
NODE
```

Actual output:

```json
{"oldExpectChains":98,"newExpectChains":100,"allOriginalsAndOrderPreserved":true,"added":["await expect(page.getByTestId(\"customer-tags-editor\").getByRole(\"checkbox\", { name: \"Seed01\", exact: true })).toBeVisible()","await expect(notes.getByRole(\"button\", { name: c.notesMore, exact: true })).toBeEnabled()"]}
```

`git diff --check` exited 0. `git diff --name-only HEAD -- apps/admin` exited 0 with empty output. Final `git rev-parse HEAD` and `shasum -a 256 tests/admin/customer-tags.spec.ts` exited 0 and match the values above. These are structural/source checks, not runtime acceptance.

## Runtime evidence attribution and limits

The following are parent-provided results, not independently rerun by this reviewer:

| Evidence | Result |
| --- | --- |
| Original CI `37729726011`, `customer-tags.spec.ts:148` | RED: initial catalogue GET 15397.669 → 503; reload began 15407.958; reload catalogue 15551.698 → 200. The initial child read consumed the one-shot fault. |
| Unchanged local full customers mode | exit 0; no local RED claimed. |
| Fixed local full customers mode | exit 0, CTUI 9/9. |
| Other requested gates | Parent reports CB12, picklist, CVS Chromium/WebKit, BFF46, Node910, check-gates80modes and adminTS green. |

Evidence directory: primary `output/w3-u4-parcel-merge-ui/w6-round/`. Reviewer browser/Go/PG/full gates: **NOT_RUN**. No source edits, test execution, assertion weakening, or independent runtime PASS claim. Unresolved scoped source issues: none confirmed; final runtime acceptance remains the parent's gate responsibility.
