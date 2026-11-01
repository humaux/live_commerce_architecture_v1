// Figma local links: https://www.figma.com/design/iJ9lDdFIXgTlKkt5Tvrc0X?node-id=673-2847 and https://www.figma.com/design/iJ9lDdFIXgTlKkt5Tvrc0X?node-id=673-2916
// Purpose: signed initial form 673:2847/2916 with local preview/format checks only.
// Depends on: authenticated page context, existing merchantTemplate grammar and React.
// Used by: app/[locale]/messages/templates/new/page.tsx.
// Invariants: live:manage; no POST/publication/sending or persistence; drafts clear on hide/logout; server validates public safety separately.
"use client";
import { useEffect, useRef, useState, type FormEvent } from "react";
import { useRouter } from "next/navigation";
import type { Locale } from "@live-commerce/i18n";
import type { Store } from "@/lib/model";
import type { ReadCode } from "@/lib/customers-client";
import { merchantTemplate } from "@/lib/live-settings-model";
import { permitted } from "@/src/features/messages/privacy";
import { AdminForbidden } from "../AdminForbidden";
import { AdminStateBlock, adminStateCopy } from "../AdminStateBlock";
import { messageTemplateCopy } from "./MessageTemplateList.copy";
import s from "./MessageTemplateList.module.css";
/** Local-only form: a successful check is never represented as a published version. */
export function MessageTemplateForm({
  locale,
  store,
  initialError,
}: {
  locale: Locale;
  store: Store | null;
  initialError: ReadCode | null;
}) {
  const c = messageTemplateCopy[locale],
    router = useRouter();
  const [name, setName] = useState(c.exampleName),
    [identifier, setIdentifier] = useState("merchant/help-note"),
    [body, setBody] = useState(c.exampleBody),
    [kinds, setKinds] = useState(["private_reply"]),
    [safe, setSafe] = useState(false),
    [result, setResult] = useState<boolean | null>(null);
  const bodyInput = useRef<HTMLTextAreaElement>(null);
  useEffect(() => {
    const clear = () => {
      if (bodyInput.current) bodyInput.current.value = "";
      setName("");
      setIdentifier("");
      setBody("");
      setKinds([]);
      setSafe(false);
      setResult(null);
    };
    const hide = () => {
      if (document.visibilityState === "hidden") clear();
    };
    const storage = (e: StorageEvent) => {
      if (e.key === "commerce-session-logout") clear();
    };
    document.addEventListener("visibilitychange", hide);
    window.addEventListener("pagehide", clear);
    window.addEventListener("commerce-session-logout", clear);
    window.addEventListener("storage", storage);
    return () => {
      document.removeEventListener("visibilitychange", hide);
      window.removeEventListener("pagehide", clear);
      window.removeEventListener("commerce-session-logout", clear);
      window.removeEventListener("storage", storage);
    };
  }, []);
  if (initialError)
    return initialError === "forbidden" ? (
      <AdminForbidden locale={locale} />
    ) : (
      <AdminStateBlock
        variant="error"
        title={adminStateCopy[locale].errorTitle}
      />
    );
  if (!permitted(store, "live:manage"))
    return <AdminForbidden locale={locale} />;
  const back = () =>
    router.push(
      `/${locale}/messages/templates?${new URLSearchParams({ store: store!.id })}#message-template-add`,
    );
  function check(event: FormEvent) {
    event.preventDefault();
    const size = Array.from(body.normalize("NFC")).length;
    setResult(
      name.trim().length > 0 &&
        Array.from(name).length <= 120 &&
        merchantTemplate(identifier) &&
        body.trim().length > 0 &&
        size <= (safe ? 300 : 2000) &&
        kinds.length > 0 &&
        (!kinds.some((k) => k === "public_reply" || k === "recommend") || safe),
    );
  }
  return (
    <section
      className={`${s.page} ${s.formPage}`}
      data-figma-node="673:2847 673:2916"
    >
      <h2 className={s.mobile}>{c.formMobile}</h2>
      <button type="button" className={`${s.back} ${s.desktop}`} onClick={back}>
        {c.back}
      </button>
      <h1>{c.formTitle}</h1>
      <p>{c.intro}</p>
      <form onSubmit={check} noValidate className={s.formGrid}>
        <div className={s.fields}>
          <label htmlFor="template-name">{c.nameLabel}</label>
          <input
            id="template-name"
            value={name}
            onChange={(e) => {
              setName(e.target.value);
              setResult(null);
            }}
            aria-describedby="template-name-hint"
          />
          <p id="template-name-hint" className={s.hint}>
            {c.nameHint}
          </p>
          <label htmlFor="template-id">{c.idLabel}</label>
          <input
            id="template-id"
            value={identifier}
            onChange={(e) => {
              setIdentifier(e.target.value);
              setResult(null);
            }}
            aria-describedby="template-id-hint"
          />
          <p id="template-id-hint" className={s.hint}>
            {c.idHint}
          </p>
          <label htmlFor="template-body">{c.bodyLabel}</label>
          <textarea
            id="template-body"
            ref={bodyInput}
            defaultValue={c.exampleBody}
            onChange={(e) => {
              setBody(e.target.value);
              setResult(null);
            }}
            aria-describedby="template-body-hint"
          />
          <p id="template-body-hint" className={s.hint}>
            {c.bodyHint}
          </p>
          <fieldset>
            <legend>{c.kindsLabel}</legend>
            <p className={s.hint}>{c.kindsHint}</p>
            {(
              [
                ["dm", c.private],
                ["private_reply", c.reply],
                ["public_reply", c.public],
                ["recommend", c.recommend],
              ] as const
            ).map(([value, label]) => (
              <label className={s.choice} key={value}>
                <input
                  type="checkbox"
                  checked={kinds.includes(value)}
                  onChange={(e) => {
                    setKinds((old) =>
                      e.target.checked
                        ? [...old, value]
                        : old.filter((k) => k !== value),
                    );
                    setResult(null);
                  }}
                />
                {label}
              </label>
            ))}
          </fieldset>
          <label className={s.choice}>
            <input
              type="checkbox"
              checked={safe}
              onChange={(e) => {
                setSafe(e.target.checked);
                setResult(null);
              }}
            />
            {c.safeLabel}
          </label>
          <p className={s.hint}>{c.safeHint}</p>
        </div>
        <aside className={s.preview}>
          <h2>{c.preview}</h2>
          <p className={s.note}>{body}</p>
          <h2>{c.review}</h2>
          <p className={s.rules}>{c.rules}</p>
          <p>{c.qualification}</p>
          {result !== null && (
            <p role={result ? "status" : "alert"}>
              {result ? c.localOnly : c.invalid}
            </p>
          )}
        </aside>
        <div className={s.formDock}>
          <button type="button" className={s.secondary} onClick={back}>
            {c.cancel}
          </button>
          <button type="submit" className={s.primary}>
            {c.check}
          </button>
        </div>
      </form>
    </section>
  );
}
