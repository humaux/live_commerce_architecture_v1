"use client";
// Local draft-only bulk edits. ProductDocumentForm remains the sole document writer.
import { useEffect, useRef, useState } from "react";
import type { BulkField } from "@/lib/product-document";
import type { ProductEditorCopy } from "@/lib/product-editor-copy";

export function ProductBulkFill({
  c,
  inventoryDisabled,
  selectedCount,
  close,
  apply,
}: {
  c: ProductEditorCopy;
  inventoryDisabled: boolean;
  selectedCount: number;
  close: () => void;
  apply: (
    field: BulkField,
    value: string,
    scope: "all" | "empty",
    operation: "set" | "add" | "subtract",
    target: "all" | "selected",
  ) => void;
}) {
  const [field, setField] = useState<BulkField>("price");
  const [value, setValue] = useState("");
  const [scope, setScope] = useState<"all" | "empty">("all");
  const [target, setTarget] = useState<"all" | "selected">(
    selectedCount ? "selected" : "all",
  );
  const [operation, setOperation] = useState<"set" | "add" | "subtract">("set");
  const input = useRef<HTMLInputElement>(null);
  useEffect(() => input.current?.focus(), []);
  const blocked = target === "selected" && !selectedCount;
  const submit = () => {
    if (!blocked) apply(field, value, scope, operation, target);
  };
  return (
    <div
      className="pe-popover"
      role="region"
      aria-label={`${c.bulk} ${c[field]}`}
      onKeyDown={(e) => {
        if (e.key === "Escape") {
          e.preventDefault();
          close();
        }
        if (
          e.key === "Enter" &&
          e.target instanceof HTMLInputElement &&
          !e.nativeEvent.isComposing
        ) {
          e.preventDefault();
          submit();
        }
      }}
    >
      <label>
        {c.bulkField}
        <select
          data-testid="bulk-field"
          value={field}
          onChange={(e) => {
            setField(e.target.value as BulkField);
            setValue("");
            setOperation("set");
          }}
        >
          {(["price", "compare", "quantity", "code", "keyword"] as const).map(
            (f) => (
              <option
                key={f}
                value={f}
                disabled={f === "quantity" && inventoryDisabled}
              >
                {c[f]}
              </option>
            ),
          )}
        </select>
      </label>
      <label>
        {c.bulkRows}
        <select
          data-testid="bulk-target"
          value={target}
          onChange={(e) => setTarget(e.target.value as "all" | "selected")}
        >
          <option value="all">{c.all}</option>
          <option value="selected" disabled={!selectedCount}>
            {c.selected} ({selectedCount})
          </option>
        </select>
      </label>
      <label>
        {c.bulk}
        <select
          aria-label={c.bulk}
          value={scope}
          onChange={(e) => setScope(e.target.value as "all" | "empty")}
        >
          <option value="all">{c.all}</option>
          <option value="empty">{c.empty}</option>
        </select>
      </label>
      <label>
        {c.set}
        <select
          aria-label={c.set}
          value={operation}
          onChange={(e) =>
            setOperation(e.target.value as "set" | "add" | "subtract")
          }
        >
          <option value="set">{field === "code" ? c.prefix : c.set}</option>
          {field === "quantity" && (
            <>
              <option value="add">{c.add}</option>
              <option value="subtract">{c.subtract}</option>
            </>
          )}
        </select>
      </label>
      <label>
        {c.amount}
        <input
          ref={input}
          data-testid="bulk-value"
          value={value}
          onChange={(e) => setValue(e.target.value)}
        />
      </label>
      <div className="pe-actions">
        <button type="button" onClick={close}>
          {c.cancel}
        </button>
        <button
          type="button"
          data-testid="bulk-apply"
          disabled={blocked}
          onClick={submit}
        >
          {c.apply}
        </button>
      </div>
    </div>
  );
}
