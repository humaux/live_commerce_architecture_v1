// Purpose: the admin re-authentication dialog and the one place that handles HTTP 403 `reauth_required` (contracts/
//   merchant-password-auth-v1.md §16, card OC-04). @ds-page: built only from DS components (ConfirmDialog, Field/TextInput,
//   Button, Alert, Stack), so scripts/dev/check-ds-rules.mjs covers this file.
// Depends on: @live-commerce/ui/ds; lib/reauth-copy.ts; the BFF /api/auth/password/reauth/{method,start,complete,oidc}
//   (app/api/auth/password/reauth/[step]/route.ts) and the OIDC callback's #reauth-done / #reauth-failed return
//   (app/api/auth/callback/route.ts).
// Used by: app/[locale]/layout.tsx (mounted once per page); tests/admin/oc/reauth.spec.ts.
// Invariants: a 403 reauth_required never signs out - every same-origin /api/ call (except /api/auth/*) that gets one waits for
//   the dialog; on success the ORIGINAL request (same method, URL, headers - including its Idempotency-Key - and body bytes,
//   cloned before it was sent) is sent again exactly once and its answer is what the caller sees; on cancel the caller gets the
//   original 403. Concurrent refusals share one dialog. Password and code live only in component state and go only to the BFF.
"use client";
import { useEffect, useRef, useState } from "react";
import type { Locale } from "@live-commerce/i18n";
import { Alert, Button, ConfirmDialog, Field, Stack, TextInput } from "@live-commerce/ui/ds";
import { reauthCopy, reauthError } from "../lib/reauth-copy";

const CHANNEL = "commerce-reauth";
const POPUP = "commerce-reauth";
type Method = { password: boolean; oidc: boolean; account: "password" | "oidc" };
type Step = "password" | "code" | "oidc" | "waiting";

function csrf() {
  const values = document.cookie.split(";").map((p) => p.trim()).filter((p) => p.startsWith("__Host-commerce_csrf=")).map((p) => p.slice(21));
  return values.length === 1 ? values[0] : "";
}

/** Wraps fetch: a guarded same-origin /api/ call answered 403 reauth_required waits for `confirm`, then is replayed once. */
export function reauthFetch(native: typeof fetch, confirm: () => Promise<boolean>): typeof fetch {
  return async (input, init) => {
    const url = new URL(input instanceof Request ? input.url : String(input), window.location.href);
    if (url.origin !== window.location.origin || !url.pathname.startsWith("/api/") || url.pathname.startsWith("/api/auth/")) return native(input, init);
    const request = new Request(input, init);
    const replay = request.clone(); // before the body is consumed: the exact bytes and headers are sent again
    const response = await native(request);
    if (response.status !== 403) return response;
    const code = await response.clone().json().then((b: { code?: unknown } | null) => b?.code, () => null);
    if (code !== "reauth_required" || !(await confirm())) return response;
    // The caller's own timeout may have run out while the person re-authenticated; an explicit abort still applies.
    const timedOut = request.signal.aborted && (request.signal.reason as { name?: string } | undefined)?.name === "TimeoutError";
    return native(new Request(replay, { signal: timedOut ? AbortSignal.timeout(16000) : request.signal }));
  };
}

async function send(path: string, body: unknown): Promise<{ status: number; body: Record<string, unknown> }> {
  try {
    const response = await fetch(`/api/auth/password/reauth/${path}`, {
      method: "POST", credentials: "same-origin", cache: "no-store",
      headers: { "Content-Type": "application/json", "X-CSRF-Token": csrf() },
      body: JSON.stringify(body), signal: AbortSignal.timeout(16000),
    });
    return { status: response.status, body: await response.json().catch(() => ({})) };
  } catch {
    return { status: 503, body: { code: "retry_later" } };
  }
}

export function ReauthDialog({ locale }: { locale: Locale }) {
  const c = reauthCopy[locale];
  const [open, setOpen] = useState(false);
  const [method, setMethod] = useState<Method | null>(null);
  const [step, setStep] = useState<Step>("password");
  const [password, setPassword] = useState("");
  const [code, setCode] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const waiters = useRef<((ok: boolean) => void)[]>([]);
  const opener = useRef<HTMLElement | null>(null);
  const lastFocus = useRef<HTMLElement | null>(null); // the page control focused last (a clicked button is often disabled by the 403 time)

  const finish = (ok: boolean) => {
    for (const resolve of waiters.current.splice(0)) resolve(ok);
    setOpen(false); setPassword(""); setCode(""); setError(""); setBusy(false);
    // Return focus to the control that started the refused request. It is often disabled until the replay settles, so retry
    // briefly; stop as soon as focus is somewhere else on purpose.
    const target = opener.current;
    let tries = 0;
    const timer = window.setInterval(() => {
      const idle = document.activeElement === null || document.activeElement === document.body;
      if (++tries > 50 || !idle || !target?.isConnected) return window.clearInterval(timer);
      if (!target.matches(":disabled")) { target.focus(); window.clearInterval(timer); }
    }, 100);
  };

  useEffect(() => {
    // The OIDC re-auth window lands back on its opener's page with #reauth-done / #reauth-failed: tell the opener, then close.
    const mark = window.location.hash;
    if (mark === "#reauth-done" || mark === "#reauth-failed") {
      new BroadcastChannel(CHANNEL).postMessage(mark === "#reauth-done" ? "done" : "failed");
      history.replaceState(null, "", window.location.pathname + window.location.search);
      if (window.name === POPUP || window.opener) window.close();
    }
    const track = (event: FocusEvent) => {
      if (event.target instanceof HTMLElement && event.target !== document.body && !event.target.closest("dialog")) lastFocus.current = event.target;
    };
    document.addEventListener("focusin", track);
    const native = window.fetch;
    const confirm = () => new Promise<boolean>((resolve) => {
      if (waiters.current.length === 0) opener.current = lastFocus.current;
      waiters.current.push(resolve);
      setOpen(true);
    });
    window.fetch = reauthFetch(native, confirm);
    return () => { window.fetch = native; document.removeEventListener("focusin", track); };
  }, []);

  useEffect(() => {
    if (!open) return;
    let live = true;
    setBusy(true);
    fetch("/api/auth/password/reauth/method", { credentials: "same-origin", cache: "no-store" })
      .then((r) => (r.ok ? r.json() : Promise.reject(new Error(String(r.status)))))
      .then((m: Method) => { if (live) { setMethod(m); setStep(m.account === "oidc" ? "oidc" : "password"); setBusy(false); } })
      .catch(() => { if (live) { setError(reauthError(c, "retry_later")); setBusy(false); } });
    const channel = new BroadcastChannel(CHANNEL);
    channel.onmessage = (event) => {
      if (event.data === "done") finish(true);
      else { setStep("oidc"); setError(reauthError(c, "reauth_failed")); }
    };
    return () => { live = false; channel.close(); };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open]);

  const fail = (status: number, body: Record<string, unknown>) => {
    const reason = typeof body.code === "string" ? body.code : "retry_later";
    setError(reauthError(c, reason) + (reason === "mail_unavailable" && method?.oidc && method.account === "oidc" ? ` ${c.mailUnavailableOidc}` : ""));
    if (status === 401) setStep("password");
  };

  async function confirm() {
    if (busy) return;
    setError("");
    if (step === "oidc" || step === "waiting") {
      // Opened synchronously from the click so pop-up blockers allow it; the IdP address is filled in after the BFF answers.
      const popup = window.open("about:blank", POPUP, "popup,width=520,height=680");
      if (!popup) return setError(reauthError(c, "popup_blocked"));
      setBusy(true);
      const search = /^\?[A-Za-z0-9=&._~%-]{0,512}$/.test(window.location.search) ? window.location.search : "";
      const sent = await send("oidc", { locale, return_to: window.location.pathname + search });
      setBusy(false);
      if (sent.status !== 200 || typeof sent.body.authorization_url !== "string") { popup.close(); return fail(sent.status, sent.body); }
      popup.location.href = sent.body.authorization_url;
      return setStep("waiting");
    }
    setBusy(true);
    const sent = step === "password" ? await send("start", { password, locale }) : await send("complete", { code: code.trim() });
    setBusy(false);
    if (sent.status !== 200) return fail(sent.status, sent.body);
    if (step === "password") { setPassword(""); setStep("code"); return; }
    finish(true);
  }

  const switchTo = (next: Step) => { setError(""); setStep(next); };
  const both = !!method?.password && !!method?.oidc;
  return (
    <ConfirmDialog
      open={open}
      title={c.title}
      description={step === "oidc" || step === "waiting" ? c.introOidc : c.introPassword}
      confirmLabel={step === "password" ? c.sendCode : step === "code" ? c.verify : c.oidc}
      cancelLabel={c.cancel}
      tone="primary"
      busy={busy}
      busyLabel={c.working}
      onConfirm={() => void confirm()}
      onCancel={() => finish(false)}
    >
      <form data-testid="reauth-dialog" data-step={step} onSubmit={(event) => { event.preventDefault(); void confirm(); }}>
        <Stack>
          {step === "password" && (
            <Field label={c.password}>
              {(control) => <TextInput {...control} type="password" autoComplete="current-password" maxLength={1024} value={password} onChange={(e) => setPassword(e.target.value)} />}
            </Field>
          )}
          {step === "code" && (
            <Field label={c.code} help={c.codeSent}>
              {(control) => <TextInput {...control} autoFocus inputMode="numeric" autoComplete="one-time-code" maxLength={6} value={code} onChange={(e) => setCode(e.target.value)} />}
            </Field>
          )}
          {step === "waiting" && <Alert tone="info">{c.waitingOidc}</Alert>}
          {error && <Alert tone="danger">{error}</Alert>}
          {step === "code" && <Button variant="link" onClick={() => switchTo("password")}>{c.startOver}</Button>}
          {both && (step === "password" || step === "code") && <Button variant="link" onClick={() => switchTo("oidc")}>{c.useOidc}</Button>}
          {both && (step === "oidc" || step === "waiting") && <Button variant="link" onClick={() => switchTo("password")}>{c.usePassword}</Button>}
        </Stack>
      </form>
    </ConfirmDialog>
  );
}
