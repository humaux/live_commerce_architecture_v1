# LC-U3 shared-form implementation handoff

- task_id: LC-U3/shared-form (parent-owned coordination; no independent claim).
- base_commit: ba2bab32523dfdb39a5a00d9478b40c317d238fe.
- branch: unit/lc-u3-form-worker.
- worktree: /Volumes/data/live_commerce_architecture_v1/.worktrees/lc-u3-form-worker.
- Role: assigned UI implementation subtask; actual runtime model / reasoning: UNKNOWN (not exposed).
- Evidence: E3 component-level MOCK + E1 typecheck/static gates, bound to source-sha256.txt. This is implementation handoff, not whole LC-U3 acceptance.

## Changes

- ManualOrder.tsx delegates item/customer/delivery/payment fields to a controlled shared fragment. Its direct hook ordering, submit/retry idempotency, result, regenerate and buyer-link DOM remain in the original controller.
- ManualOrderFormFields.tsx shares fields and emits Partial<ManualFormValues> patches. Default mo/input preserves legacy IDs, input bounds, radio grouping and existing styles. Drawer prefix/stepper isolates IDs and radio group.
- ManualOrderItemPicker.tsx reuses existing active catalog reads. Display prices are catalog formatting only; no client totals or inventory authority. Drawer plus/minus/input controls clamp at 1000 and remove at zero; legacy input still clamps to 1. Optional localized parent note is rendered per line.
- manual-order-form.ts exports fresh nested blank values and exactly the parent-frozen ManualFormLine/ManualFormValues interface.
- timezone-ui.test.mjs preserves all existing assertions and explicit import-denial behavior; adds loaded-source callbacks for fresh defaults, legacy input preservation, drawer controls in all three locales, option readiness and sole-mode selection.

## Exact shared interface

```ts
export type ManualFormLine = { sku_id: string; quantity: number; label: string; code: string; price: string; note?: string };
export type ManualFormValues = {
  lines: ManualFormLine[]; name: string; phone: string; email: string; optionKey: string;
  mode: ManualPaymentMode | ''; home: ManualDraft['home']; cvs: ManualDraft['cvs']; buyerLocale: Locale;
};
export function emptyManualForm(locale: Locale = 'zh-TW'): ManualFormValues;
// ManualOrderFormFields props:
{
  locale: Locale; store: Store; value: ManualFormValues;
  onChange: (patch: Partial<ManualFormValues>) => void;
  available: ManualOption[]; optionsReady: boolean;
  idPrefix?: string; quantityControls?: 'input' | 'stepper';
}
```

Default idPrefix = mo; quantityControls = input. Drawer uses drawer / stepper. IDs include drawer-search, drawer-lines, drawer-name, drawer-minus-${sku_id}, drawer-plus-${sku_id}, drawer-quantity-${sku_id}. Fields return a fragment; parent may use a disabled fieldset. The mt-quantity-stepper container is available for parent drawer-scoped styling.

## Actual commands and exit codes

| Command | Exit | Evidence |
| --- | --- | --- |
| pnpm install --offline --frozen-lockfile | 0 | Tool transcript; all 49 packages reused, no download, lockfile unchanged |
| node --test tests/admin/timezone-ui.test.mjs before extraction | 1 | red.log; existing 4 pass, new 3 fail because extracted source absent |
| node --test tests/admin/timezone-ui.test.mjs after extraction | 0 | green.log; 7/7 |
| TZ=UTC node --test --experimental-strip-types tests/admin/merchant-tools-model.test.ts tests/admin/timezone-ui.test.mjs | 0 | focused-utc.log; 17/17 |
| TZ=America/Los_Angeles node --test tests/admin/timezone-ui.test.mjs | 0 | focused-la.log; 7/7 |
| pnpm typecheck:admin | 0 | typecheck-final.log |
| bash scripts/dev/check-gates.sh | 0 | check-gates.log; existing unrelated >500 line warnings only |
| git diff --check | 0 | Tool transcript |
| git diff --cached --check, first run | 2 | Found trailing blank lines in typecheck evidence logs; normalized final newlines only |
| git diff --cached --check, after log newline normalization | 0 | Tool transcript |

## Unresolved / NOT_RUN

- Real --browser-manual-order, PG, click sweep, full test-node.sh, independent review and full LC-U3 integration: NOT_RUN by this worker; parent explicitly owns/serializes them.
- Loaded TSX seam invokes handlers in a MOCK hook/presentation shell and is not browser clicking evidence.
- Runtime model/reasoning: UNKNOWN; parent can fill from its routing evidence.
- No BFF/model/merchant-tools-client changes, no shared schema/dependency changes, no production actions.
- Humaux record/canvas handled by parent as assigned; no duplicate task claim.
- No process, fixture, database, container or port started; node_modules is the required offline workspace dependency tree.
