// Purpose: own one private, immutable create/adjust intent and its accessible form/confirmation lifecycle.
// Depends on: frozen members-client commands, read-session boundary, DS Button and promoted member tokens/copy.
// Used by: MemberList/MemberDetail; UNKNOWN never replays, ordinary cancel never changes authentication.
"use client";
import { useEffect, useRef, useState, type FormEvent } from "react";
import { Button } from "@live-commerce/ui/ds";
import type { Locale } from "@live-commerce/i18n";
import type { Store } from "@/lib/model";
import type { ReadStatus } from "@/lib/customers-client";
import { sessionBoundary } from "@/lib/settings-client";
import {
  createMember,
  adjustPoints,
  MemberWriteError,
  type Member,
  type CreateMemberInput,
  type AdjustPointsInput,
} from "@/lib/members-client";
import { memberFormCopy } from "./copy";
import s from "./members.module.css";

type Step = "edit" | "confirm" | "pending" | "unknown" | "refused";
type Intent = Readonly<{
  key: string;
  scope: string;
  boundary: string;
  generation: number;
  store: string;
  member: string | null;
  controller: AbortController;
  body: Readonly<CreateMemberInput> | Readonly<AdjustPointsInput>;
}>;
type Props = {
  locale: Locale;
  store: Store | null;
  member?: Member | null;
  memberId?: string;
  scope: string;
  boundary: string;
  status: ReadStatus;
  onSaved: () => void;
  onRefresh: () => void;
};

/** Caller keeps this owner mounted across authoritative reloads; only scope/session changes discard private state. */
export function MemberForms({
  locale,
  store,
  member,
  memberId,
  scope,
  boundary,
  status,
  onSaved,
  onRefresh,
}: Props) {
  const c = memberFormCopy[locale],
    adjusting = memberId !== undefined;
  const [open, setOpen] = useState(false),
    [step, setStep] = useState<Step>("edit");
  const [name, setName] = useState(""),
    [contact, setContact] = useState(""),
    [notes, setNotes] = useState("");
  const [amount, setAmount] = useState(""),
    [sign, setSign] = useState(1),
    [note, setNote] = useState("");
  const [error, setError] = useState(""),
    [invalid, setInvalid] = useState("");
  const [snapshot, setSnapshot] = useState<{
    name: string;
    balance: number;
  } | null>(null);
  const intent = useRef<Intent | null>(null),
    generation = useRef(0),
    activeScope = useRef(scope);
  const dialog = useRef<HTMLDialogElement>(null),
    opener = useRef<HTMLElement | null>(null);
  const privateBoundary = useRef("");
  activeScope.current = scope;
  const permitted = !!store?.permissions?.includes("customers:points");
  const concealed =
    !permitted ||
    status === "hidden" ||
    status === "signed-out" ||
    status === "forbidden" ||
    status === "not-found";
  const busy = step === "pending",
    unresolved = step === "unknown";
  const reason = !store
    ? c.noStore
    : member?.erased_at
      ? c.erased
      : status !== "ready"
        ? c.loading
        : undefined;

  function clear() {
    generation.current++;
    intent.current?.controller.abort();
    intent.current = null;
    privateBoundary.current = "";
    setOpen(false);
    setStep("edit");
    setName("");
    setContact("");
    setNotes("");
    setAmount("");
    setNote("");
    setSign(1);
    setError("");
    setInvalid("");
    setSnapshot(null);
  }
  useEffect(() => {
    clear();
    const logout = () => clear();
    window.addEventListener("commerce-session-logout", logout);
    return () => {
      generation.current++;
      intent.current?.controller.abort();
      window.removeEventListener("commerce-session-logout", logout);
    };
    // Scope is the server render/store/member generation, not a mutable read revision.
  }, [scope]);
  useEffect(() => {
    if (status === "signed-out" || !permitted) clear();
  }, [status, permitted]);
  useEffect(() => {
    const el = dialog.current;
    if (!el) return;
    if (open && !concealed && !el.open) {
      el.showModal();
      (
        el.querySelector<HTMLElement>("input,textarea,select,button") ?? el
      ).focus();
    } else if ((!open || concealed) && el.open) {
      el.close();
      if (!concealed && opener.current?.isConnected) opener.current.focus();
    }
  }, [open, concealed]);
  useEffect(() => {
    if (!open || concealed) return;
    if (invalid)
      dialog.current
        ?.querySelector<HTMLElement>(`[name="${invalid}"]`)
        ?.focus();
    else if (step !== "edit")
      dialog.current?.querySelector<HTMLElement>("h2")?.focus();
  }, [invalid, step, open, concealed]);

  function close() {
    if (intent.current || busy) return;
    clear(); // Local draft only; deliberately no logout or read-session invalidation.
  }
  function start(event: { currentTarget: HTMLElement }) {
    if (!permitted || reason) return;
    opener.current = event.currentTarget;
    if (!intent.current) {
      privateBoundary.current = boundary;
      setSnapshot(
        member ? { name: member.display_name, balance: member.balance } : null,
      );
    }
    setOpen(true);
  }
  function validate(): CreateMemberInput | AdjustPointsInput | null {
    const fail = (field: string, text: string) => {
      setInvalid(field);
      setError(text);
      // Repeated failures can batch back to identical state, so the effect may not rerun.
      if (open && !concealed)
        dialog.current
          ?.querySelector<HTMLElement>(`[name="${field}"]`)
          ?.focus();
      return null;
    };
    setInvalid("");
    setError("");
    if (!adjusting) {
      const display = name.trim().normalize("NFC");
      if (
        !display ||
        Array.from(display).length > 60 ||
        /[\p{Cc}\p{Cf}\p{Cs}]/u.test(display)
      )
        return fail("name", c.invalidName);
      if (Array.from(contact).length > 120 || /\p{Cs}/u.test(contact))
        return fail("contact", c.invalidContact);
      if (Array.from(notes).length > 1000 || /\p{Cs}/u.test(notes))
        return fail("notes", c.invalidNotes);
      return {
        display_name: display,
        contact_note: contact || null,
        notes: notes || null,
      };
    }
    const count = Number(amount);
    if (
      !/^\d+$/.test(amount) ||
      !Number.isSafeInteger(count) ||
      count < 1 ||
      count > 1e6
    )
      return fail("amount", c.invalidAmount);
    if (
      !note.trim() ||
      Array.from(note).length > 200 ||
      /[\p{Cc}\p{Cf}\p{Cs}]/u.test(note)
    )
      return fail("note", c.invalidNote);
    if (sign < 0 && count > (snapshot?.balance ?? 0))
      return fail("amount", c.insufficient);
    return { delta: sign * count, note, order_id: null };
  }
  function review(event: FormEvent) {
    event.preventDefault();
    if (intent.current || busy || !permitted) return;
    if (validate()) {
      if (adjusting) setStep("confirm");
      else void submit();
    }
  }
  async function submit() {
    if (intent.current || !store || !permitted || concealed) return;
    const body = validate();
    if (!body) return;
    const command: Intent = Object.freeze({
      key: crypto.randomUUID(),
      scope,
      boundary: privateBoundary.current,
      generation: generation.current,
      store: store.id,
      member: memberId ?? null,
      controller: new AbortController(),
      body: Object.freeze(body),
    });
    intent.current = command; // Synchronous exclusion before any await or React render.
    setStep("pending");
    const live = () =>
      intent.current === command &&
      activeScope.current === command.scope &&
      generation.current === command.generation;
    try {
      let currentBoundary: string;
      try {
        currentBoundary = await sessionBoundary();
      } catch {
        if (live()) clear();
        return;
      }
      if (!command.boundary || currentBoundary !== command.boundary) {
        if (live()) clear();
        return;
      }
      if (!live()) return;
      if (command.member)
        await adjustPoints(
          command.store,
          command.member,
          command.body as AdjustPointsInput,
          command.key,
          command.controller.signal,
        );
      else
        await createMember(
          command.store,
          command.body as CreateMemberInput,
          command.key,
          command.controller.signal,
        );
      if (!live()) return;
      clear();
      onSaved();
    } catch (e) {
      if (!live()) return;
      if (e instanceof MemberWriteError && e.code === "signed-out") {
        clear();
        return;
      }
      if (
        !(e instanceof MemberWriteError) ||
        e.outcome === "unknown" ||
        e.code === "idempotency_conflict"
      ) {
        setStep("unknown");
        setError(c.unknown);
        return; // Retain the same body/key/target: reading never clears or replays this intent.
      }
      intent.current = null;
      setStep("refused");
      setError(
        e.code === "points_insufficient"
          ? c.insufficient
          : e.code === "forbidden"
            ? c.forbidden
            : e.code === "member_erased"
              ? c.erased
              : e.code === "not-found"
                ? c.unavailable
                : e.code === "limit_reached"
                  ? c.limit
                  : c.refused,
      );
    }
  }
  const title = unresolved
    ? adjusting
      ? c.unknownTitle
      : c.createUnknownTitle
    : step === "confirm"
      ? sign > 0
        ? c.confirmAdd
        : c.confirmDeduct
      : step === "refused"
        ? c.refusedTitle
        : adjusting
          ? c.adjust
          : c.create;
  const input = (
    field: string,
    label: string,
    value: string,
    change: (v: string) => void,
    max: number,
    multiline = false,
  ) => (
    <div className={s.formField} data-invalid={invalid === field || undefined}>
      <label htmlFor={`member-form-${field}`}>{label}</label>
      {multiline ? (
        <textarea
          rows={1}
          id={`member-form-${field}`}
          name={field}
          value={value}
          maxLength={max * 2}
          disabled={busy}
          aria-invalid={invalid === field}
          aria-describedby={invalid === field ? "member-form-error" : undefined}
          onChange={(e) => change(e.target.value)}
        />
      ) : (
        <input
          id={`member-form-${field}`}
          name={field}
          value={value}
          autoComplete="off"
          disabled={busy}
          inputMode={field === "amount" ? "numeric" : undefined}
          maxLength={max * 2}
          aria-invalid={invalid === field}
          aria-describedby={invalid === field ? "member-form-error" : undefined}
          onChange={(e) => change(e.target.value)}
        />
      )}
    </div>
  );
  return (
    <>
      {permitted && (
        <Button
          size="lg"
          data-testid={adjusting ? "member-adjust-open" : "member-create-open"}
          disabledReason={reason}
          onClick={start}
        >
          {adjusting ? c.adjust : c.create}
        </Button>
      )}
      <dialog
        ref={dialog}
        className={s.formDialog}
        aria-labelledby="member-form-title"
        aria-describedby="member-form-guidance"
        data-testid="member-form-dialog"
        data-kind={adjusting ? "adjust" : "create"}
        data-step={step}
        data-validation-invalid={!!invalid || undefined}
        onCancel={(e) => {
          e.preventDefault();
          close();
        }}
        onKeyDown={(e) => {
          if (e.key !== "Tab") return;
          const items = Array.from(
            e.currentTarget.querySelectorAll<HTMLElement>(
              'input,textarea,select,button,[tabindex="0"]',
            ),
          ).filter(
            (n) =>
              !n.matches(":disabled,[aria-disabled=true]") &&
              n.getClientRects().length > 0,
          );
          const first = items[0],
            last = items.at(-1);
          if (e.shiftKey && document.activeElement === first) {
            e.preventDefault();
            last?.focus();
          } else if (!e.shiftKey && document.activeElement === last) {
            e.preventDefault();
            first?.focus();
          } else if (!items.length) e.preventDefault();
        }}
      >
        {open && !concealed && (
          <>
            <h2 id="member-form-title" tabIndex={-1}>
              {title}
            </h2>
            <p id="member-form-guidance">
              {unresolved
                ? c.readOnlyRefresh
                : adjusting
                  ? c.irreversible
                  : c.searchFirst}
            </p>
            {(step === "edit" || busy) && (
              <form data-testid="member-form" onSubmit={review} noValidate>
                <fieldset disabled={busy}>
                  {adjusting ? (
                    <>
                      <p>{snapshot?.name}</p>
                      <label htmlFor="member-form-sign">{c.direction}</label>
                      <select
                        id="member-form-sign"
                        name="sign"
                        value={sign}
                        onChange={(e) => setSign(Number(e.target.value))}
                      >
                        <option value={1}>{c.add}</option>
                        <option value={-1}>{c.deduct}</option>
                      </select>
                      {input("amount", c.amount, amount, setAmount, 7)}
                      {input("note", c.note, note, setNote, 200, true)}
                    </>
                  ) : (
                    <>
                      {input("name", c.name, name, setName, 60)}
                      {input("contact", c.contact, contact, setContact, 120)}
                      <p className={s.formHint}>{c.contactHint}</p>
                      {input("notes", c.notes, notes, setNotes, 1000, true)}
                    </>
                  )}
                </fieldset>
                {error && (
                  <p
                    id="member-form-error"
                    role="alert"
                    className={s.formError}
                  >
                    {error}
                  </p>
                )}
                {busy && <p role="status">{c.pending}</p>}
                <div className={s.formActions}>
                  <Button
                    size="lg"
                    data-testid="member-form-cancel"
                    disabledReason={busy ? c.pending : undefined}
                    onClick={close}
                  >
                    {c.cancel}
                  </Button>
                  <Button
                    size="lg"
                    type="submit"
                    variant="primary"
                    data-testid="member-form-submit"
                    disabledReason={busy ? c.pending : undefined}
                  >
                    {busy ? c.pending : adjusting ? c.review : c.create}
                  </Button>
                </div>
              </form>
            )}
            {step === "confirm" && (
              <>
                <p>{snapshot?.name}</p>
                <p>
                  {c.amount}: {sign > 0 ? "+" : "−"}
                  {amount}
                </p>
                <p>
                  {c.note}: {note}
                </p>
                <p>
                  {`${c.balance}: ${snapshot?.balance} → ${(snapshot?.balance ?? 0) + sign * Number(amount)}`}
                </p>
                <div className={s.formActions}>
                  <Button
                    size="lg"
                    data-testid="member-form-back"
                    onClick={() => setStep("edit")}
                  >
                    {c.back}
                  </Button>
                  <Button
                    size="lg"
                    variant="primary"
                    data-testid="member-form-confirm"
                    onClick={() => void submit()}
                  >
                    {sign > 0 ? c.confirmAdd : c.confirmDeduct}
                  </Button>
                </div>
              </>
            )}
            {step === "refused" && (
              <>
                <p role="alert" className={s.formError}>
                  {error}
                </p>
                <div className={s.formActions}>
                  <Button
                    size="lg"
                    data-testid="member-form-close"
                    onClick={close}
                  >
                    {c.close}
                  </Button>
                  <Button
                    size="lg"
                    data-testid="member-form-back"
                    onClick={() => {
                      setError("");
                      setStep("edit");
                    }}
                  >
                    {c.back}
                  </Button>
                </div>
              </>
            )}
            {unresolved && (
              <>
                <p role="status">{error}</p>
                <Button
                  size="lg"
                  variant="primary"
                  data-testid="member-form-refresh"
                  onClick={onRefresh}
                >
                  {c.refresh}
                </Button>
              </>
            )}
          </>
        )}
      </dialog>
    </>
  );
}
