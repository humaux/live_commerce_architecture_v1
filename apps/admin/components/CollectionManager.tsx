"use client";

// Merchant collections page body (/{locale}/collections): a list of the store's collections beside one editor (create,
// edit, image, ordered products, delete). Unit catalog-core, contracts/storefront-v2.md A. BFF /api/stores/{store}/ ->
// Go internal/httpapi/collections.go (catalog:read / catalog:write): GET collections, GET collections/{id}, POST collections,
// PATCH collections/{id}, PUT collections/{id}/products (the complete ordered list: add, remove and reorder in one command),
// POST collections/{id}/delete, POST|GET collections/{id}/image and POST collections/{id}/image/delete; the product search
// uses GET catalog-products. Owns: the editor drafts and one idempotency key per user action (lib/catalog-v2-write.ts).
// Never decides validity or order: the server answers and the screen re-reads (version bump after every write).
import {
  useCallback,
  useEffect,
  useRef,
  useState,
  type FormEvent,
} from "react";
import type { Locale } from "@live-commerce/i18n";
import { FilePicker } from "@live-commerce/ui";
import type { Store } from "@/lib/model";
import { useGuardedRead, type ReadCode } from "@/lib/customers-client";
import {
  collectionImageURL,
  command,
  deleteCollectionImage,
  readCollection,
  readCollections,
  readProducts,
  uploadCollectionImage,
} from "@/lib/catalog-v2-client";
import {
  limits,
  parseCreated,
  slugPattern,
  type CollectionDetail,
  type CollectionMember,
  type CollectionSort,
  type ProductSummary,
} from "@/lib/catalog-v2-model";
import {
  catalogCopy,
  catalogPresentationCopy,
  errorText,
} from "@/lib/catalog-v2-copy";
import { useWrite } from "@/lib/catalog-v2-write";
import { WorkspaceFrame } from "./WorkspaceFrame";
import { AdminPageHeader } from "./AdminPageHeader";
import "./orders.css";
import "./ProductAdmin.css";

const NEW = "new";

export function CollectionManager({
  locale,
  stores,
  store,
  initialError,
  renderKey,
}: {
  locale: Locale;
  stores: Store[];
  store: Store | null;
  initialError: ReadCode | null;
  renderKey: string;
}) {
  const c = catalogCopy[locale].collections;
  const [selected, setSelected] = useState("");
  const list = useGuardedRead(
    `${renderKey}|${locale}|${store?.id ?? ""}`,
    store ? (signal) => readCollections(store.id, signal) : null,
    initialError,
  );
  const failure =
    list.status === "signed-out"
      ? c.signedOut
      : list.status === "forbidden"
        ? c.forbidden
        : list.status === "not-found"
          ? c.noStore
          : list.status === "unavailable"
            ? c.unavailable
            : "";
  return (
    <WorkspaceFrame
      locale={locale}
      storeName={store?.name ?? c.noStore}
      active="collections"
    >
      <div
        className="orders-page product-admin collections-page"
        data-testid="collections-page"
      >
        <AdminPageHeader
          locale={locale}
          description={
            c.subtitle + (stores.length > 1 && store ? ` — ${store.name}` : "")
          }
          actions={
            store && (
              <button
                type="button"
                className="product-primary"
                data-testid="collection-new"
                onClick={() => setSelected(NEW)}
              >
                {c.newCollection}
              </button>
            )
          }
        />
        {(list.status === "loading" || list.status === "hidden") && (
          <p className="orders-message" role="status">
            {c.loading}
          </p>
        )}
        {failure && (
          <div className="orders-message" role="status">
            <p>{failure}</p>
            <button type="button" onClick={list.reload}>
              {c.retry}
            </button>
          </div>
        )}
        {list.status === "ready" && list.data && store && (
          <div className="collections-layout">
            <nav aria-label={c.title}>
              {list.data.items.length === 0 && (
                <p className="orders-message">{c.empty}</p>
              )}
              <ul className="collections-list" data-testid="collections-list">
                {list.data.items.map((item) => (
                  <li key={item.id}>
                    <button
                      type="button"
                      className={selected === item.id ? "active" : ""}
                      aria-current={selected === item.id ? "true" : undefined}
                      data-testid={`collection-item-${item.id}`}
                      onClick={() => setSelected(item.id)}
                    >
                      <strong>{item.title}</strong>
                      <small>
                        /{item.slug} · {c.count(item.product_count)}
                        {item.status === "hidden" ? ` · ${c.hidden}` : ""}
                      </small>
                    </button>
                  </li>
                ))}
              </ul>
            </nav>
            {!selected && (
              <section
                className="collections-empty"
                aria-label={catalogPresentationCopy[locale].chooseCollection}
              >
                <h2>{catalogPresentationCopy[locale].chooseCollection}</h2>
                <p>{catalogPresentationCopy[locale].collectionHelp}</p>
              </section>
            )}
            {selected && (
              <CollectionEditor
                key={selected}
                locale={locale}
                store={store}
                id={selected}
                boundary={list.boundary}
                onChanged={() => void list.refresh()}
                onCreated={(id) => {
                  void list.refresh();
                  setSelected(id);
                }}
                onDeleted={() => {
                  void list.refresh();
                  setSelected("");
                }}
              />
            )}
          </div>
        )}
      </div>
    </WorkspaceFrame>
  );
}

function CollectionEditor({
  locale,
  store,
  id,
  boundary,
  onChanged,
  onCreated,
  onDeleted,
}: {
  locale: Locale;
  store: Store;
  id: string;
  boundary: string;
  onChanged: () => void;
  onCreated: (id: string) => void;
  onDeleted: () => void;
}) {
  const cc = catalogCopy[locale];
  const c = cc.collections;
  const creating = id === NEW;
  const read = useGuardedRead<CollectionDetail | null>(
    `${store.id}|${id}`,
    creating
      ? async () => null
      : (signal) => readCollection(store.id, id, signal),
    null,
  );
  if (read.status !== "ready")
    return (
      <p className="orders-message" role="status">
        {read.status === "not-found"
          ? c.notFound
          : read.status === "loading"
            ? c.loading
            : c.unavailable}
      </p>
    );
  return (
    <CollectionForm
      key={id}
      locale={locale}
      store={store}
      id={id}
      detail={read.data}
      boundary={read.boundary || boundary}
      refresh={async () => {
        const ok = await read.refresh();
        onChanged();
        return ok;
      }}
      onCreated={onCreated}
      onDeleted={onDeleted}
    />
  );
}

function CollectionForm({
  locale,
  store,
  detail,
  boundary,
  refresh,
  onCreated,
  onDeleted,
}: {
  locale: Locale;
  store: Store;
  id: string;
  detail: CollectionDetail | null;
  boundary: string;
  refresh: () => Promise<boolean>;
  onCreated: (id: string) => void;
  onDeleted: () => void;
}) {
  const cc = catalogCopy[locale];
  const c = cc.collections;
  const errors = useCallback((code: string) => errorText(cc, code), [cc]);
  const write = useWrite(store.id, boundary, errors, c.uncertain);
  const [title, setTitle] = useState(detail?.title ?? "");
  const [slug, setSlug] = useState(detail?.slug ?? "");
  const [description, setDescription] = useState(detail?.description ?? "");
  const [sort, setSort] = useState<CollectionSort>(
    detail?.sort_mode ?? "manual",
  );
  const [status, setStatus] = useState(detail?.status ?? "active");
  const [members, setMembers] = useState<CollectionMember[]>(
    detail?.products ?? [],
  );
  const [confirming, setConfirming] = useState(false);

  function save(event: FormEvent) {
    event.preventDefault();
    if (!title.trim() || Array.from(title).length > limits.collectionTitle)
      return write.fail(c.invalidTitle);
    if (slug && !slugPattern.test(slug)) return write.fail(c.invalidSlug);
    if (!detail) {
      const body: Record<string, string> = {
        title: title.trim(),
        description,
        sort_mode: sort,
        status,
      };
      if (slug) body.slug = slug;
      void write.run(
        command("POST", "collections", body),
        parseCreated,
        (created) => onCreated(created.id),
        c.saved,
      );
      return;
    }
    const body: Record<string, unknown> = { expected_version: detail.version };
    if (title.trim() !== detail.title) body.title = title.trim();
    if (slug && slug !== detail.slug) body.slug = slug;
    if (description !== detail.description) body.description = description;
    if (sort !== detail.sort_mode) body.sort_mode = sort;
    if (status !== detail.status) body.status = status;
    if (Object.keys(body).length === 1) return; // nothing changed
    void write.run(
      command("PATCH", `collections/${detail.id}`, body),
      parseCreated,
      () => void refresh(),
      c.saved,
    );
  }
  const msg = write.message;
  return (
    <div className="collections-editor" data-testid="collection-editor">
      <form className="product-section product-form" onSubmit={save}>
        <label>
          {c.name}
          <input
            data-testid="collection-title"
            value={title}
            maxLength={limits.collectionTitle}
            onChange={(e) => setTitle(e.target.value)}
            required
          />
        </label>
        <label>
          {c.slug}
          <input
            data-testid="collection-slug"
            value={slug}
            maxLength={limits.slug}
            onChange={(e) => setSlug(e.target.value)}
          />
        </label>
        <label>
          {c.description}
          <textarea
            data-testid="collection-description"
            value={description}
            maxLength={limits.collectionDescription}
            rows={3}
            onChange={(e) => setDescription(e.target.value)}
          />
        </label>
        <div className="product-two">
          <label>
            {c.sort}
            <select
              data-testid="collection-sort"
              value={sort}
              onChange={(e) => setSort(e.target.value as CollectionSort)}
            >
              {(["manual", "newest", "price_asc", "price_desc"] as const).map(
                (m) => (
                  <option key={m} value={m}>
                    {c.sortModes[m]}
                  </option>
                ),
              )}
            </select>
          </label>
          <label>
            {c.visibility}
            <select
              data-testid="collection-status"
              value={status}
              onChange={(e) => setStatus(e.target.value as "active" | "hidden")}
            >
              <option value="active">{c.visible}</option>
              <option value="hidden">{c.hidden}</option>
            </select>
          </label>
        </div>
        {msg && (
          <div
            className={`message ${msg.kind === "success" ? "success" : "error"}`}
            role={msg.kind === "success" ? "status" : "alert"}
            data-testid="collection-message"
          >
            <span>{msg.text}</span>
            {msg.kind === "uncertain" && (
              <button
                type="button"
                disabled={write.busy}
                onClick={() => void write.retry()}
              >
                {cc.edit.retry}
              </button>
            )}
            {msg.kind !== "success" && (
              <button type="button" onClick={write.dismiss}>
                {c.dismiss}
              </button>
            )}
          </div>
        )}
        <div className="product-row-actions">
          <button
            className="product-primary"
            type="submit"
            data-testid="collection-save"
            disabled={write.busy}
            aria-describedby={write.busy ? "collection-save-reason" : undefined}
          >
            {detail ? c.save : c.create}
          </button>
          {write.busy && (
            <p id="collection-save-reason" className="product-disabled-reason">
              {catalogPresentationCopy[locale].busy}
            </p>
          )}
          {detail && !confirming && (
            <button
              type="button"
              className="danger"
              data-testid="collection-delete"
              onClick={() => setConfirming(true)}
            >
              {c.delete}
            </button>
          )}
        </div>
        {detail && confirming && (
          <div
            className="message error"
            role="alertdialog"
            aria-label={c.delete}
          >
            <span>{c.confirmDelete}</span>
            <button
              type="button"
              className="danger"
              data-testid="collection-delete-confirm"
              disabled={write.busy}
              onClick={() =>
                void write.run(
                  command("POST", `collections/${detail.id}/delete`, {
                    expected_version: detail.version,
                  }),
                  parseCreated,
                  onDeleted,
                  c.deleted,
                )
              }
            >
              {c.confirmDeleteButton}
            </button>
            <button type="button" onClick={() => setConfirming(false)}>
              {cc.edit.cancel}
            </button>
          </div>
        )}
      </form>
      {detail && (
        <>
          <ImageBlock
            locale={locale}
            store={store}
            detail={detail}
            boundary={boundary}
            refresh={refresh}
          />
          <Members
            locale={locale}
            store={store}
            detail={detail}
            members={members}
            setMembers={setMembers}
            boundary={boundary}
            refresh={refresh}
          />
        </>
      )}
    </div>
  );
}

function ImageBlock({
  locale,
  store,
  detail,
  boundary,
  refresh,
}: {
  locale: Locale;
  store: Store;
  detail: CollectionDetail;
  boundary: string;
  refresh: () => Promise<boolean>;
}) {
  const cc = catalogCopy[locale];
  const c = cc.collections;
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState("");
  const input = useRef<HTMLInputElement>(null);
  async function upload(file: File | undefined) {
    if (!file || busy) return;
    if (!["image/jpeg", "image/png", "image/webp"].includes(file.type))
      return setMessage(c.imageType);
    if (file.size < 1 || file.size > 2 * 1024 * 1024)
      return setMessage(c.imageTooLarge);
    setBusy(true);
    setMessage("");
    const out = await uploadCollectionImage(
      store.id,
      detail.id,
      file,
      boundary,
    );
    setBusy(false);
    if (!out.ok)
      setMessage(
        out.code === "too_large" ? c.imageTooLarge : errorText(cc, out.code),
      );
    if (input.current) input.current.value = "";
    await refresh();
  }
  async function remove() {
    setBusy(true);
    setMessage("");
    const out = await deleteCollectionImage(store.id, detail.id, boundary);
    setBusy(false);
    if (!out.ok) setMessage(errorText(cc, out.code));
    await refresh();
  }
  return (
    <section className="product-section" data-testid="collection-image">
      <h2>{c.image}</h2>
      <p className="audit-hint">{c.imageHelp}</p>
      {detail.image_id && (
        <img
          className="collection-thumb"
          alt={detail.title}
          src={collectionImageURL(store.id, detail.id, detail.image_id)}
        />
      )}
      {message && (
        <p className="message error" role="alert">
          {message}
        </p>
      )}
      <div className="product-row-actions">
        <FilePicker
          label={detail.image_id ? c.replace : c.upload}
          emptyLabel={catalogPresentationCopy[locale].noFile}
          fileName=""
          inputRef={input}
          accept="image/jpeg,image/png,image/webp"
          data-testid="collection-image-input"
          disabled={busy}
          onChange={(e) => void upload(e.target.files?.[0])}
        />
        {detail.image_id && (
          <button
            type="button"
            data-testid="collection-image-remove"
            disabled={busy}
            onClick={() => void remove()}
          >
            {c.removeImage}
          </button>
        )}
      </div>
    </section>
  );
}

function Members({
  locale,
  store,
  detail,
  members,
  setMembers,
  boundary,
  refresh,
}: {
  locale: Locale;
  store: Store;
  detail: CollectionDetail;
  members: CollectionMember[];
  setMembers: (m: CollectionMember[]) => void;
  boundary: string;
  refresh: () => Promise<boolean>;
}) {
  const cc = catalogCopy[locale];
  const c = cc.collections;
  const errors = useCallback((code: string) => errorText(cc, code), [cc]);
  const write = useWrite(store.id, boundary, errors, c.uncertain);
  const [q, setQ] = useState("");
  const [found, setFound] = useState<ProductSummary[]>([]);
  useEffect(() => {
    const controller = new AbortController();
    const timer = setTimeout(() => {
      readProducts(store.id, q.trim(), "all", "", controller.signal).then(
        (page) => setFound(page.items),
        () => setFound([]),
      );
    }, 250);
    return () => {
      clearTimeout(timer);
      controller.abort();
    };
  }, [store.id, q]);
  const ids = new Set(members.map((m) => m.product_id));
  const move = (i: number, by: -1 | 1) => {
    const next = [...members];
    [next[i], next[i + by]] = [next[i + by], next[i]];
    setMembers(next);
  };
  const dirty =
    members.map((m) => m.product_id).join() !==
    detail.products.map((m) => m.product_id).join();
  return (
    <section className="product-section" data-testid="collection-members">
      <h2>{c.members}</h2>
      <p className="audit-hint">{c.membersHint}</p>
      {members.length === 0 && <p>{c.noMembers}</p>}
      <ol className="collection-members">
        {members.map((m, i) => (
          <li key={m.product_id} data-testid={`member-${m.product_id}`}>
            <span>
              <strong>{m.name}</strong>{" "}
              <small>{c.inactiveBadge(m.status)}</small>
            </span>
            <button
              type="button"
              disabled={i === 0}
              onClick={() => move(i, -1)}
            >
              {c.moveUp}
            </button>
            <button
              type="button"
              disabled={i === members.length - 1}
              onClick={() => move(i, 1)}
            >
              {c.moveDown}
            </button>
            <button
              type="button"
              onClick={() =>
                setMembers(members.filter((x) => x.product_id !== m.product_id))
              }
            >
              {c.remove}
            </button>
          </li>
        ))}
      </ol>
      <label>
        {c.findProducts}
        <input
          type="search"
          data-testid="member-search"
          value={q}
          placeholder={c.findPlaceholder}
          maxLength={120}
          onChange={(e) => setQ(e.target.value)}
        />
      </label>
      <ul className="collection-found">
        {found
          .filter((p) => !ids.has(p.id))
          .slice(0, 10)
          .map((p) => (
            <li key={p.id}>
              <span>
                {p.name} <small>{cc.status[p.status]}</small>
              </span>
              <button
                type="button"
                data-testid={`member-add-${p.id}`}
                disabled={members.length >= limits.collectionProducts}
                onClick={() =>
                  setMembers([
                    ...members,
                    {
                      product_id: p.id,
                      name: p.name,
                      slug: p.slug,
                      status: p.status,
                      position: members.length,
                      cover_image_id: p.cover_image_id,
                    },
                  ])
                }
              >
                {c.add}
              </button>
            </li>
          ))}
      </ul>
      {members.length >= limits.collectionProducts && (
        <p className="audit-hint">{c.limit}</p>
      )}
      {write.message && (
        <div
          className={`message ${write.message.kind === "success" ? "success" : "error"}`}
          role={write.message.kind === "success" ? "status" : "alert"}
        >
          <span>{write.message.text}</span>
          {write.message.kind === "uncertain" && (
            <button
              type="button"
              disabled={write.busy}
              onClick={() => void write.retry()}
            >
              {cc.edit.retry}
            </button>
          )}
          {write.message.kind !== "success" && (
            <button type="button" onClick={write.dismiss}>
              {c.dismiss}
            </button>
          )}
        </div>
      )}
      <button
        type="button"
        className="product-primary"
        data-testid="members-save"
        disabled={!dirty || write.busy}
        aria-describedby={
          !dirty || write.busy ? "members-save-reason" : undefined
        }
        onClick={() =>
          void write.run(
            command("PUT", `collections/${detail.id}/products`, {
              expected_version: detail.version,
              product_ids: members.map((m) => m.product_id),
            }),
            parseCreated,
            () => void refresh(),
            c.saved,
          )
        }
      >
        {c.saveMembers}
      </button>
      {(!dirty || write.busy) && (
        <p id="members-save-reason" className="product-disabled-reason">
          {write.busy
            ? catalogPresentationCopy[locale].busy
            : catalogPresentationCopy[locale].noChanges}
        </p>
      )}
    </section>
  );
}
