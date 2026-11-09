"use client";

// Purpose: GET sort/price controls shared by ListBody's products, collection and search pages.
// Depends on: typed shop copy, published ShopName context, React refs and native dialog; called only by ListBody.
// Invariants: server list is authoritative; mobile buttons submit the same GET form as desktop/no-JS select;
// q/preview/min/max survive, cancellation changes no query, and sorting never writes cart or inventory.
import { useEffect, useId, useRef, useState } from "react";
import type { Locale } from "@live-commerce/i18n";
import { shopCopy } from "../lib/shop-copy";
import { FilterIcon } from "./icons";
import { useShopName } from "./ShopName";

export default function ListControls({
  locale,
  action,
  hidden,
  values,
  currency,
  filtered,
  clearHref,
}: {
  locale: Locale;
  action: string;
  hidden: Record<string, string>;
  values: { sort: string; min: string; max: string };
  currency: string;
  filtered: boolean;
  clearHref: string;
}) {
  const copy = shopCopy[locale];
  const storeName = useShopName();
  const [enhanced, setEnhanced] = useState(false);
  const picker = useRef<HTMLDialogElement>(null);
  const trigger = useRef<HTMLButtonElement>(null);
  const select = useRef<HTMLSelectElement>(null);
  const pickerId = useId();
  useEffect(() => {
    setEnhanced(typeof picker.current?.showModal === "function");
    const desktop = window.matchMedia("(min-width: 720px)");
    const closeOnDesktop = () => { if (desktop.matches) picker.current?.close(); };
    desktop.addEventListener("change", closeOnDesktop);
    return () => desktop.removeEventListener("change", closeOnDesktop);
  }, []);
  const sortOptions = [
    { value: "newest", label: copy.sortNewest },
    { value: "price_asc", label: copy.sortPriceAsc },
    { value: "price_desc", label: copy.sortPriceDesc },
    { value: "title", label: copy.sortTitle },
  ];
  return (
    <form className={`sf-controls${enhanced ? " sf-controls--enhanced" : ""}`} method="get" action={action} data-testid="list-controls">
      {Object.entries(hidden).map(([name, value]) => (
        <input key={name} type="hidden" name={name} value={value} />
      ))}
      <details className="sf-filter" open={filtered}>
        <summary>
          <FilterIcon />
          <span>{copy.priceRange}</span>
          {filtered && <i className="sf-dot" aria-hidden="true" />}
        </summary>
        <div className="sf-filter__panel">
          <label>
            <span>{copy.priceMin}</span>
            <input name="min" type="text" inputMode="decimal" pattern="[0-9]*[.]?[0-9]*" defaultValue={values.min} autoComplete="off" />
          </label>
          <label>
            <span>{copy.priceMax}</span>
            <input name="max" type="text" inputMode="decimal" pattern="[0-9]*[.]?[0-9]*" defaultValue={values.max} autoComplete="off" />
          </label>
          <div className="sf-filter__actions">
            <button type="submit" className="sf-btn sf-btn--primary">
              {copy.apply}
            </button>
            {filtered && (
              <a className="sf-link" href={clearHref}>
                {copy.clear}
              </a>
            )}
          </div>
        </div>
      </details>
      <label className="sf-sort">
        <span>{copy.sortBy}</span>
        <select
          ref={select}
          name="sort"
          defaultValue={values.sort}
          onChange={(event) => event.currentTarget.form?.requestSubmit()}
        >
          {sortOptions.map((o) => (
            <option key={o.value} value={o.value}>
              {o.label}
            </option>
          ))}
        </select>
      </label>
      <button ref={trigger} type="button" className="sf-sort-trigger" aria-haspopup="dialog" aria-controls={pickerId} onClick={() => picker.current?.showModal()}>
        {copy.sortBy}
      </button>
      <dialog ref={picker} id={pickerId} className="sf-mobile-sort" aria-labelledby={`${pickerId}-title`} aria-describedby={`${pickerId}-note`} onClose={() => trigger.current?.focus()}>
        <header className="sf-mobile-sort__header">{storeName}</header>
        <div className="sf-mobile-sort__body">
          <h2 id={`${pickerId}-title`}>{copy.sortBy}</h2>
          <p id={`${pickerId}-note`}>{copy.sortNote}</p>
          <div className="sf-mobile-sort__options">
            {sortOptions.map((option) => (
              <button key={option.value} type="button" aria-pressed={values.sort === option.value} onClick={() => {
                // The existing select remains the sole successful `sort` input; native GET serialization
                // preserves the current filter inputs and hidden q/preview, without a second client query model.
                if (select.current) {
                  select.current.value = option.value;
                  select.current.form?.requestSubmit();
                }
              }}>
                {option.label}
              </button>
            ))}
            <button type="button" onClick={() => picker.current?.close()}>{copy.sortCancel}</button>
          </div>
        </div>
      </dialog>
    </form>
  );
}
