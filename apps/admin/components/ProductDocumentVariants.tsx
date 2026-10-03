"use client";
import { useEffect, useRef, useState } from "react";
import {
  applyBulk,
  type BulkField,
  type DraftRow,
} from "@/lib/product-document";
import type { OptionAxis } from "@/lib/catalog-v2-model";
import type { ProductEditorCopy } from "@/lib/product-editor-copy";

export function ProductDocumentVariants({
  axes,
  rows,
  setAxes,
  setRows,
  disabled,
  c,
  sign,
  inventoryDisabled = false,
}: {
  axes: OptionAxis[];
  rows: DraftRow[];
  setAxes: (a: OptionAxis[]) => void;
  setRows: (r: DraftRow[]) => void;
  disabled: boolean;
  c: ProductEditorCopy;
  sign: string;
  inventoryDisabled?: boolean;
}) {
  const [bulk, setBulk] = useState<BulkField | null>(null),
    [notice, setNotice] = useState("");
  const triggers = useRef<Partial<Record<BulkField, HTMLButtonElement | null>>>(
    {},
  );
  const changeAxis = (i: number, patch: Partial<OptionAxis>) =>
    setAxes(axes.map((a, n) => (n === i ? { ...a, ...patch } : a)));
  const changeRow = (i: number, patch: Partial<DraftRow>) =>
    setRows(rows.map((r, n) => (i === n ? { ...r, ...patch } : r)));
  function closeBulk() {
    if (bulk) triggers.current[bulk]?.focus();
    setBulk(null);
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
            <label>
              {c.axis}
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
              onChange={(values) => changeAxis(i, { values })}
            />
            <button
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
          <p className="pe-hint">{rows.length} / 100</p>
          <div className="pe-bulk-controls">
            {(["price", "compare", "quantity", "code", "keyword"] as const).map(
              (field) => (
                <div className="pe-bulk-anchor" key={field}>
                  <button
                    type="button"
                    ref={(el) => {
                      triggers.current[field] = el;
                    }}
                    data-testid={`bulk-${field}`}
                    disabled={
                      disabled || (field === "quantity" && inventoryDisabled)
                    }
                    aria-expanded={bulk === field}
                    onClick={() => setBulk(bulk === field ? null : field)}
                  >
                    {c[field]} · {c.bulk}
                  </button>
                  {bulk === field && (
                    <BulkFill
                      c={c}
                      field={field}
                      close={closeBulk}
                      apply={(value, scope, operation) => {
                        const result = applyBulk(
                          rows,
                          field,
                          value,
                          scope,
                          operation,
                        );
                        setRows(result.rows);
                        setNotice(`${c.skipped}: ${result.skipped}`);
                        closeBulk();
                      }}
                    />
                  )}
                </div>
              ),
            )}
          </div>
          {notice && <p role="status">{notice}</p>}
          <div className="pe-matrix" data-testid="variant-matrix">
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
                <div>
                  <strong>{row.values.join(" / ")}</strong>
                  <small>{row.id ? c.existing : c.newRow}</small>
                </div>
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
                  <label className="pe-check">
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
}: {
  values: string[];
  onChange: (v: string[]) => void;
  disabled: boolean;
  i: number;
  c: ProductEditorCopy;
}) {
  const [text, setText] = useState(values.join(", "));
  function commit() {
    const values = [
      ...new Set(
        text
          .split(/[,，\n]/)
          .map((s) => s.trim())
          .filter(Boolean),
      ),
    ];
    onChange(values);
  }
  return (
    <label>
      {c.values}
      <textarea
        rows={2}
        data-testid={`axis-values-${i}`}
        value={text}
        disabled={disabled}
        onChange={(e) => setText(e.target.value)}
        onBlur={commit}
        onKeyDown={(e) => {
          if (e.key === "Enter") {
            e.preventDefault();
            commit();
          }
        }}
      />
      <small>{c.valueHelp}</small>
      <span className="pe-value-chips">
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
                setText(next.join(", "));
                onChange(next);
              }
            }}
          >
            {value}
          </span>
        ))}
      </span>
    </label>
  );
}
function BulkFill({
  c,
  field,
  close,
  apply,
}: {
  c: ProductEditorCopy;
  field: BulkField;
  close: () => void;
  apply: (v: string, s: "all" | "empty", o: "set" | "add" | "subtract") => void;
}) {
  const [value, setValue] = useState(""),
    [scope, setScope] = useState<"all" | "empty">("all"),
    [operation, setOperation] = useState<"set" | "add" | "subtract">("set");
  const input = useRef<HTMLInputElement>(null);
  useEffect(() => input.current?.focus(), []);
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
        if (e.key === "Enter") {
          e.preventDefault();
          apply(value, scope, operation);
        }
      }}
    >
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
          onClick={() => apply(value, scope, operation)}
        >
          {c.apply}
        </button>
      </div>
    </div>
  );
}
