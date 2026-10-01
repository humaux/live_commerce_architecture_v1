"use client";

// Sort select + price filter of the list pages. A real GET form (works without JS, preserves q/preview via hidden inputs);
// the only script is auto-submit when the sort changes. No BFF/Go call: submitting navigates to the same server-rendered page
// with new query params (min/max in major units, converted by lib/shop-list.ts). Inputs are 16px so iOS never zooms on focus.
import type { Locale } from "@live-commerce/i18n";
import { shopCopy } from "../lib/shop-copy";
import { FilterIcon } from "./icons";

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
  return (
    <form className="sf-controls" method="get" action={action} data-testid="list-controls">
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
            <span>
              {copy.priceMin} ({currency})
            </span>
            <input name="min" type="text" inputMode="decimal" pattern="[0-9]*[.]?[0-9]*" defaultValue={values.min} autoComplete="off" />
          </label>
          <label>
            <span>
              {copy.priceMax} ({currency})
            </span>
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
        <select name="sort" defaultValue={values.sort} onChange={(event) => event.currentTarget.form?.requestSubmit()}>
          <option value="newest">{copy.sortNewest}</option>
          <option value="price_asc">{copy.sortPriceAsc}</option>
          <option value="price_desc">{copy.sortPriceDesc}</option>
          <option value="title">{copy.sortTitle}</option>
        </select>
      </label>
    </form>
  );
}
