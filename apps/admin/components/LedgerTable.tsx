"use client";

// Pure inventory table display. Ledger owns catalog-ledger BFF data, selection, journals and Go inventory/adjustments commands.
// No fetch, local state or optimistic inventory calculations belong in this component.
import type { Locale } from "@live-commerce/i18n";
import { Badge, TableFrame } from "@live-commerce/ui";
import type { Copy } from "@/lib/copy";
import type { LedgerRow } from "@/lib/model";
import { money } from "@/lib/client";
import { imageURL } from "@/lib/images-client";
import { presentationCopy } from "@/lib/presentation-copy";
import { ProductPhoto } from "./ProductPhoto";
import { Icon } from "./Icon";

export type LedgerTableProps = {
  locale: Locale;
  c: Copy;
  rows: LedgerRow[];
  storeID: string;
  fixture: boolean;
  readError: boolean;
  selectedID: string;
  locked: boolean;
  onSelect: (skuID: string) => void;
  onRadioSelect: (skuID: string) => void;
};

export function LedgerTable({
  locale,
  c,
  rows,
  storeID,
  fixture,
  readError,
  selectedID,
  locked,
  onSelect,
  onRadioSelect,
}: LedgerTableProps) {
  return (
    <TableFrame
      label={c.inventory}
      scrollHint={presentationCopy[locale].scroll}
      scrollClassName="table-scroll"
    >
      <table>
        <thead>
          <tr>
            <th className="selection-col">
              <span className="sr-only">{c.edit}</span>
            </th>
            <th>{c.product}</th>
            <th className="sku-col">SKU</th>
            <th className="numeric">{c.price}</th>
            <th className="numeric stock-col">{c.onHand}</th>
            <th className="numeric stock-col">{c.reserved}</th>
            <th className="numeric">{c.available}</th>
            <th className="status-col">{c.status}</th>
            <th className="action-col">{c.actions}</th>
          </tr>
        </thead>
        <tbody>
          {rows.map((row) => (
            <tr
              key={row.sku_id}
              className={row.sku_id === selectedID ? "selected" : ""}
            >
              <td className="selection-col">
                <label className="sku-select">
                  <input
                    type="radio"
                    name="sku"
                    aria-label={`${c.edit} ${row.code}`}
                    checked={row.sku_id === selectedID}
                    disabled={locked}
                    onChange={() => onRadioSelect(row.sku_id)}
                  />
                </label>
              </td>
              <th scope="row">
                <div className="product-cell">
                  <ProductPhoto
                    code={row.code}
                    name={row.product_name}
                    demo={fixture}
                    imageSrc={
                      row.cover_image_id
                        ? imageURL(storeID, row.product_id, row.cover_image_id)
                        : undefined
                    }
                  />
                  <div>
                    <button
                      className="product-name"
                      disabled={locked}
                      onClick={() => onSelect(row.sku_id)}
                    >
                      {row.product_name}
                    </button>
                    {fixture && <small>{c.demo}</small>}
                    <small className="mobile-sku">
                      {row.code}
                      {/* status-col is display:none ≤680px; this badge is
                          the only mobile-visible active/archived signal */}
                      <Badge
                        className="mobile-status"
                        tone={row.status === "active" ? "success" : "neutral"}
                      >
                        {row.status === "active" ? c.active : c.archived}
                      </Badge>
                    </small>
                  </div>
                </div>
              </th>
              <td className="sku-col">
                <code>{row.code}</code>
              </td>
              <td className="numeric">
                {money(locale, row.currency, row.price_minor)}
              </td>
              <td className="numeric stock-col">{row.on_hand}</td>
              <td className="numeric stock-col">{row.reserved}</td>
              <td className="numeric available-value">{row.available}</td>
              <td className="status-col">
                <Badge
                  className={`status ${row.status}`}
                  tone={row.status === "active" ? "success" : "neutral"}
                >
                  {row.status === "active" ? c.active : c.archived}
                </Badge>
              </td>
              <td className="action-col">
                <button
                  className="text-button"
                  disabled={locked}
                  onClick={() => onSelect(row.sku_id)}
                >
                  {c.edit}
                </button>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
      {!rows.length && (
        <div className="empty-state">
          <Icon name="product" size={30} />
          <h2>{readError ? c.noSession : c.empty}</h2>
          <p>{readError ? c.noSessionHint : c.emptyHint}</p>
        </div>
      )}
    </TableFrame>
  );
}
