"use client";
// Purpose: Local SKU bulk task panel and legacy column popovers; no business write.
// Depends on: React, catalog currency parsers and product-editor-copy; called by ProductDocumentVariants.
// Invariants: actual store currency, explicit active target, no clamp, sole document save remains in Form.
import { useEffect, useRef, useState } from "react";
import type { Locale } from "@live-commerce/i18n";
import { fromMinor, toMinor } from "@/lib/catalog-v2-model";
import type { BulkField } from "@/lib/product-document";
import type { ProductEditorCopy } from "@/lib/product-editor-copy";

/** Applies only local draft edits, with legacy inventory/compare fields opened at their columns. */
export function ProductBulkFill({ c, currency, locale, productName, productCode, dirty,
  initialField = "price", inventoryDisabled, codeDisabled = false, selectedCount, hasSelection, close, apply,
}: {
  c: ProductEditorCopy; currency: string; locale: Locale; productName: string; productCode: string; dirty: boolean;
  initialField?: BulkField; inventoryDisabled: boolean; codeDisabled?: boolean; selectedCount: number; hasSelection: boolean;
  close: () => void;
  apply: (field: BulkField, value: string, scope: "all" | "empty", operation: "set" | "add" | "subtract", target: "all" | "selected") => void;
}) {
  const [field, setField] = useState<BulkField>(initialField);
  const [value, setValue] = useState("");
  const [scope, setScope] = useState<"all" | "empty">("all");
  const [target, setTarget] = useState<"all" | "selected">(hasSelection ? "selected" : "all");
  const [operation, setOperation] = useState<"set" | "add" | "subtract">("set");
  const input = useRef<HTMLInputElement>(null);
  const legacy = initialField === "compare" || initialField === "quantity";
  useEffect(() => input.current?.focus(), [field]);
  const blocked = (target === "selected" && !selectedCount) || (field === "code" && codeDisabled) || (field === "quantity" && inventoryDisabled);
  const submit = () => { if (!blocked) apply(field, value, scope, operation, target); };
  function chooseField(next: BulkField) { setField(next); setValue(""); setOperation("set"); }
  function step(direction: number) {
    const current = toMinor(value, currency), delta = toMinor("10", currency);
    if (current === null || delta === null) return;
    const next = current + direction * delta;
    if (!Number.isSafeInteger(next) || next < 0) return;
    const text = fromMinor(next, currency);
    if (toMinor(text, currency) === next) setValue(text);
  }
  return (
    <div className={legacy ? "pe-popover" : "pe-bulk-task"} data-testid="bulk-dialog" data-locale={locale} role="region"
      aria-label={`${c.bulk} ${c[field]}`}
      onKeyDown={(e) => {
        if (e.key === "Escape") { e.preventDefault(); close(); }
        if (e.key === "Enter" && e.target instanceof HTMLInputElement && !e.nativeEvent.isComposing) { e.preventDefault(); submit(); }
      }}>
      {!legacy && <header className="pe-bulk-task-header">
        <button type="button" data-testid="bulk-back" onClick={close}>{c.bulkBack}</button>
        <strong>{c.bulk}</strong>
        <span className="pe-bulk-locale-slot" aria-hidden="true" />
      </header>}
      <div className={legacy ? undefined : "pe-bulk-task-content"}>
        {!legacy && <><h2>{c.bulk}</h2><h3>{productName}</h3><div className="pe-bulk-identity"><span>{productCode}</span><span>{dirty ? c.dirty : c.bulkUnchanged}</span></div></>}
        <div className={legacy ? undefined : "pe-bulk-task-card"}>
          <span>{legacy ? c[field] : c.bulkField}</span>
          {!legacy && <div className="pe-bulk-three">
            <button type="button" data-testid="bulk-field-price" aria-pressed={field === "price"} onClick={() => chooseField("price")}>{c.price}</button>
            <button type="button" data-testid="bulk-field-code" disabled={codeDisabled} aria-pressed={field === "code"} aria-describedby={codeDisabled ? "bulk-code-locked" : undefined} onClick={() => chooseField("code")}>{c.bulkCode}</button>
            <button type="button" data-testid="bulk-field-keyword" aria-pressed={field === "keyword"} onClick={() => chooseField("keyword")}>{c.keyword}</button>
          </div>}
          {!legacy && codeDisabled && <small id="bulk-code-locked" className="pe-hint">{c.bulkCodeLocked}</small>}
          <span className="pe-bulk-current-field">{c[field]}</span>
          <button type="button" data-testid="bulk-scope-all" aria-pressed={scope === "all" && target === "all"}
            onClick={() => { setScope("all"); setTarget("all"); }}>{c.bulkScope}{c.bulkAll}</button>
          <button type="button" data-testid="bulk-scope-empty" aria-pressed={scope === "empty"} onClick={() => setScope("empty")}>{c.bulkScope}{c.empty}</button>
          <small data-testid="bulk-scope-summary" className="pe-hint">{scope === "empty" ? c.empty : target === "all" ? c.bulkAll : `${c.selected} (${selectedCount})`}</small>
          {hasSelection && <label>{c.bulkRows}<select data-testid="bulk-target" value={target} onChange={e => setTarget(e.target.value as "all" | "selected")}>
            <option value="all">{c.bulkAll}</option><option value="selected">{c.selected} ({selectedCount})</option>
          </select></label>}
          <label>{field === "price" ? c.bulkPrice : field === "code" ? c.prefix : c.amount}
            <input ref={input} data-testid="bulk-value" type={field === "price" ? "number" : "text"}
              step={currency === "TWD" ? "1" : "any"} inputMode={field === "price" || field === "compare" ? "decimal" : field === "quantity" ? "numeric" : "text"}
              value={value} onChange={e => setValue(e.target.value)} />
          </label>
          {field === "price" && <div className="pe-bulk-two">
            <button type="button" data-testid="bulk-step-down" onClick={() => step(-1)}>− 10</button>
            <button type="button" data-testid="bulk-step-up" onClick={() => step(1)}>+ 10</button>
          </div>}
          <div className="pe-bulk-three">
            <button type="button" data-testid="bulk-operation-set" aria-pressed={operation === "set"} onClick={() => setOperation("set")}>{field === "code" ? c.prefix : c.set}</button>
            {(field === "price" || field === "quantity") && <>
              <button type="button" data-testid="bulk-operation-add" aria-pressed={operation === "add"} onClick={() => setOperation("add")}>{c.add}</button>
              <button type="button" data-testid="bulk-operation-subtract" aria-pressed={operation === "subtract"} onClick={() => setOperation("subtract")}>{c.subtract}</button>
            </>}
          </div>
          <small data-testid="bulk-operation-summary" className="pe-hint">{field === "code" ? c.prefix : c[operation]}</small>
        </div>
      </div>
      <footer className={legacy ? "pe-actions" : "pe-bulk-task-footer"}>
        <button type="button" data-testid="bulk-cancel" onClick={close}>{c.cancel}</button>
        <button type="button" data-testid="bulk-apply" disabled={blocked} onClick={submit}>{legacy ? c.apply : c.bulkApply}</button>
      </footer>
    </div>
  );
}
