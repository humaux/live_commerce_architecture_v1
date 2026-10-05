"use client";
// Purpose: Variant (SKU) table of the product editor: option axes, generated rows, per-row price/quantity/SKU and bulk selection.
// Depends on: lib/product-document (applyBulk, DraftRow); lib/catalog-v2-model (OptionAxis); ProductBulkFill; lib/product-editor-copy. Local state only, no API call.
// Used by: ProductDocumentForm.
import { useEffect, useRef, useState } from "react";
import { applyBulk, type DraftRow } from "@/lib/product-document";
import type { OptionAxis } from "@/lib/catalog-v2-model";
import type { ProductEditorCopy } from "@/lib/product-editor-copy";
import { ProductBulkFill } from "./ProductBulkFill";

export function ProductDocumentVariants({
  axes,
  rows,
  setAxes,
  setRows,
  disabled,
  c,
  sign,
  inventoryDisabled = false,
  onInvalidValues,
}: {
  axes: OptionAxis[];
  rows: DraftRow[];
  setAxes: (a: OptionAxis[]) => void;
  setRows: (r: DraftRow[]) => void;
  disabled: boolean;
  c: ProductEditorCopy;
  sign: string;
  inventoryDisabled?: boolean;
  onInvalidValues: () => void;
}) {
  const [bulk, setBulk] = useState(false),
    [notice, setNotice] = useState("");
  const [selected, setSelected] = useState<string[]>([]);
  const trigger = useRef<HTMLButtonElement>(null);
  const rowKey = (row: DraftRow) => JSON.stringify(row.values);
  const changeAxis = (i: number, patch: Partial<OptionAxis>) =>
    setAxes(axes.map((a, n) => (n === i ? { ...a, ...patch } : a)));
  const changeRow = (i: number, patch: Partial<DraftRow>) =>
    setRows(rows.map((r, n) => (i === n ? { ...r, ...patch } : r)));
  function closeBulk() {
    trigger.current?.focus();
    setBulk(false);
  }
  return (
    <section
      id="variants"
      className="product-section pe-variants"
      aria-labelledby="variants-title"
    >
      <h2 id="variants-title">{c.variants}</h2>
      <div className="pe-axes">
        {axes.map((axis, i) => (
          <div className="product-axis" key={i}>
            <label className="pe-axis-name">
              <span>{c.axis}</span>
              <input
                data-testid={`axis-name-${i}`}
                value={axis.name}
                maxLength={30}
                onChange={(e) => changeAxis(i, { name: e.target.value })}
                disabled={disabled}
                list="axis-suggestions"
              />
            </label>
            <AxisValues
              key={i}
              c={c}
              i={i}
              values={axis.values}
              disabled={disabled}
              maxValues={Math.min(
                50,
                Math.floor(
                  100 /
                    axes.reduce(
                      (count, a, n) =>
                        n === i ? count : count * Math.max(1, a.values.length),
                      1,
                    ),
                ),
              )}
              onInvalid={onInvalidValues}
              onChange={(values) => changeAxis(i, { values })}
            />
            <button
              className="pe-axis-remove"
              type="button"
              disabled={disabled}
              aria-label={`${c.remove} ${c.axis} ${i + 1}`}
              onClick={() => setAxes(axes.filter((_, n) => n !== i))}
            >
              {c.remove}
            </button>
          </div>
        ))}
      </div>
      <datalist id="axis-suggestions">
        {[c.color, c.size, c.style, c.volume].map((s) => (
          <option key={s} value={s} />
        ))}
      </datalist>
      <button
        type="button"
        data-testid="axis-add"
        disabled={disabled || axes.length >= 3}
        onClick={() => setAxes([...axes, { name: "", values: [] }])}
      >
        {c.addAxis}
      </button>
      {(axes.length > 0 || rows.length > 1) && (
        <>
          <div className="pe-bulk-controls">
            <div className="pe-bulk-anchor">
              <button
                type="button"
                ref={trigger}
                data-testid="bulk-open"
                disabled={disabled}
                aria-expanded={bulk}
                onClick={() => setBulk(!bulk)}
              >
                {c.bulk}
              </button>
              {bulk && (
                <ProductBulkFill
                  c={c}
                  inventoryDisabled={inventoryDisabled}
                  selectedCount={
                    rows.filter((r) => selected.includes(rowKey(r))).length
                  }
                  close={closeBulk}
                  apply={(field, value, scope, operation, target) => {
                    const targets = rows.filter(
                      (r) => target === "all" || selected.includes(rowKey(r)),
                    );
                    const result = applyBulk(
                      targets,
                      field,
                      value,
                      scope,
                      operation,
                    );
                    // Code suffixes retain the original matrix ordinal even
                    // when only a subset is selected (SKU2 must not become SKU1).
                    const patchRows =
                      field === "code"
                        ? applyBulk(
                            rows,
                            field,
                            value,
                            scope,
                            operation,
                          ).rows.filter((r) =>
                            targets.some((t) => rowKey(t) === rowKey(r)),
                          )
                        : result.rows;
                    const patches = new Map(
                      patchRows.map((r) => [rowKey(r), r]),
                    );
                    setRows(rows.map((r) => patches.get(rowKey(r)) ?? r));
                    setNotice(`${c.skipped}: ${result.skipped}`);
                    closeBulk();
                  }}
                />
              )}
            </div>
            <p className="pe-hint">
              {rows.length} / 100 · {c.selected}:{" "}
              {rows.filter((r) => selected.includes(rowKey(r))).length}
            </p>
          </div>
          {notice && <p role="status">{notice}</p>}
          <div
            className="pe-matrix"
            data-testid="variant-matrix"
            tabIndex={0}
            role="region"
            aria-label={c.variants}
          >
            <div className="pe-matrix-head" aria-hidden="true">
              <span>{c.variants}</span>
              <span>
                {c.price} ({sign})
              </span>
              <span>{c.compare}</span>
              <span>{c.stock}</span>
              <span>{c.code}</span>
              <span>{c.keyword}</span>
              <span>{c.active}</span>
            </div>
            {rows.map((row, i) => (
              <div
                className="pe-matrix-row"
                data-testid={`matrix-row-${i}`}
                data-sku-id={row.id}
                key={JSON.stringify(row.values)}
              >
                <label
                  className="pe-variant-name"
                  title={row.values.join(" / ") || c.defaultVariant}
                >
                  <input
                    type="checkbox"
                    data-testid={`matrix-select-${i}`}
                    aria-label={`${c.selected} ${row.values.join(" / ") || c.defaultVariant}`}
                    disabled={disabled}
                    checked={selected.includes(rowKey(row))}
                    onChange={(e) =>
                      setSelected((now) =>
                        e.target.checked
                          ? [...now, rowKey(row)]
                          : now.filter((key) => key !== rowKey(row)),
                      )
                    }
                  />
                  <strong>{row.values.join(" / ") || c.defaultVariant}</strong>
                </label>
                <label>
                  <span>{c.price}</span>
                  <input
                    aria-label={`${c.price} ${row.values.join(" / ")}`}
                    data-testid={`new-price-${i}`}
                    inputMode="decimal"
                    value={row.price}
                    disabled={disabled}
                    onChange={(e) => changeRow(i, { price: e.target.value })}
                  />
                </label>
                <label>
                  <span>{c.compare}</span>
                  <input
                    aria-label={`${c.compare} ${i + 1}`}
                    data-testid={`matrix-compare-${i}`}
                    inputMode="decimal"
                    value={row.compare}
                    disabled={disabled}
                    onChange={(e) => changeRow(i, { compare: e.target.value })}
                  />
                </label>
                <div className="pe-row-stock">
                  <label className="pe-check" title={c.untracked}>
                    <input
                      type="checkbox"
                      checked={!row.tracked}
                      disabled={disabled}
                      onChange={(e) =>
                        changeRow(i, { tracked: !e.target.checked })
                      }
                    />
                    {c.untracked}
                  </label>
                  <label>
                    <span>
                      {row.tracked
                        ? row.id
                          ? c.targetQty
                          : c.quantity
                        : c.max}
                    </span>
                    <input
                      aria-label={`${row.tracked ? (row.id ? c.targetQty : c.quantity) : c.max} ${i + 1}`}
                      data-testid={`matrix-quantity-${i}`}
                      inputMode="numeric"
                      value={row.tracked ? row.quantity : row.max}
                      disabled={disabled || (row.tracked && inventoryDisabled)}
                      onChange={(e) =>
                        changeRow(
                          i,
                          row.tracked
                            ? { quantity: e.target.value }
                            : { max: e.target.value },
                        )
                      }
                    />
                  </label>
                </div>
                <label>
                  <span>{c.code}</span>
                  <input
                    aria-label={`${c.code} ${i + 1}`}
                    data-testid={`matrix-code-${i}`}
                    value={row.code}
                    disabled={disabled || !!row.id}
                    maxLength={64}
                    placeholder={c.generated}
                    onChange={(e) => changeRow(i, { code: e.target.value })}
                  />
                </label>
                <label>
                  <span>{c.keyword}</span>
                  <input
                    aria-label={`${c.keyword} ${i + 1}`}
                    value={row.keyword}
                    maxLength={16}
                    disabled={disabled}
                    onChange={(e) =>
                      changeRow(i, { keyword: e.target.value.toUpperCase() })
                    }
                  />
                </label>
                <label className="pe-check">
                  <input
                    aria-label={`${c.active} ${i + 1}`}
                    data-testid={`matrix-active-${i}`}
                    type="checkbox"
                    checked={row.active}
                    disabled={disabled}
                    onChange={(e) => changeRow(i, { active: e.target.checked })}
                  />
                  <span>{c.active}</span>
                </label>
              </div>
            ))}
          </div>
        </>
      )}
    </section>
  );
}
function AxisValues({
  values,
  onChange,
  disabled,
  i,
  c,
  maxValues,
  onInvalid,
}: {
  values: string[];
  onChange: (v: string[]) => void;
  disabled: boolean;
  i: number;
  c: ProductEditorCopy;
  maxValues: number;
  onInvalid: () => void;
}) {
  const [text, setText] = useState("");
  const inputRef = useRef<HTMLInputElement>(null);
  // Clear only after the parent accepts the axis; a rejected 100-SKU matrix
  // must leave the user's pending values editable.
  useEffect(() => setText(""), [values]);
  function commit() {
    if (!text.trim()) return;
    const next = [
      ...new Set([
        ...values,
        ...text
          .split(/[,，\n]/)
          .map((s) => s.trim())
          .filter(Boolean),
      ]),
    ];
    if (next.length > maxValues) {
      onInvalid();
      return;
    }
    onChange(next);
  }
  return (
    <div className="pe-axis-values">
      <label htmlFor={`axis-values-${i}`}>{c.values}</label>
      <input
        ref={inputRef}
        id={`axis-values-${i}`}
        data-testid={`axis-values-${i}`}
        value={text}
        disabled={disabled}
        onChange={(e) => setText(e.target.value)}
        onPaste={(e) => {
          const pasted = e.clipboardData.getData("text/plain");
          if (!/[\r\n]/.test(pasted)) return;
          // A single-line input otherwise strips line breaks before parsing.
          e.preventDefault();
          const input = e.currentTarget;
          setText(
            text.slice(0, input.selectionStart ?? 0) +
              pasted.replace(/\r?\n/g, ",") +
              text.slice(input.selectionEnd ?? text.length),
          );
        }}
        onBlur={commit}
        onKeyDown={(e) => {
          if (e.key === "Enter" && !e.nativeEvent.isComposing) {
            e.preventDefault();
            commit();
          }
        }}
      />
      <small className="pe-hint">{c.valueHelp}</small>
      <div className="pe-value-chips">
        {values.map((value, n) => (
          <span
            draggable={!disabled}
            key={value}
            onDragStart={(e) => e.dataTransfer.setData("text/plain", String(n))}
            onDragOver={(e) => e.preventDefault()}
            onDrop={(e) => {
              e.preventDefault();
              const from = Number(e.dataTransfer.getData("text/plain"));
              if (Number.isInteger(from) && from >= 0 && from < values.length) {
                const next = [...values];
                next.splice(n, 0, next.splice(from, 1)[0]);
                onChange(next);
              }
            }}
          >
            {value}
            <button
              type="button"
              disabled={disabled}
              aria-label={`${c.remove} ${value}`}
              onClick={(e) => {
                onChange(values.filter((v) => v !== value));
                if (e.detail === 0) inputRef.current?.focus();
              }}
            >
              ×
            </button>
          </span>
        ))}
      </div>
    </div>
  );
}
