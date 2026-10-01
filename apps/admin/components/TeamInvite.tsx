"use client";

// Invitation page body (/{locale}/invite/{token}): signed out -> how to sign in or sign up with the invited address, then reopen
// the emailed link; signed in -> one Accept button. BFF POST /api/team/accept {token} -> Go /v1/identity/staff/accept
// (internal/identity/staff.go; migration 0089). The token comes from the URL of this very page and goes only to that call: it is
// never stored, logged or rendered. Every refusal shows the same generic text (invite_invalid), so the page is no oracle for whether
// a token, an invitation or an account exists. Standalone layout (the invitee has no workspace yet), no WorkspaceFrame.
import { useState } from "react";
import type { Locale } from "@live-commerce/i18n";
import { sessionBoundary } from "@/lib/settings-client";
import { acceptInvite } from "@/lib/team-client";
import { teamCopy } from "@/lib/team-copy";
import "./orders.css";
import "./customers.css";

export function TeamInvite({ locale, token, signedIn, passwordLogin }: { locale: Locale; token: string; signedIn: boolean; passwordLogin: boolean }) {
  const c = teamCopy[locale];
  const [phase, setPhase] = useState<"idle" | "busy" | "done">("idle");
  const [problem, setProblem] = useState("");

  async function accept() {
    if (phase !== "idle") return;
    setPhase("busy");
    setProblem("");
    let boundary = "";
    try {
      boundary = await sessionBoundary();
    } catch {
      setPhase("idle");
      setProblem(c.inviteErrors.unauthorized);
      return;
    }
    const result = await acceptInvite(token, boundary);
    if (result.ok) {
      setPhase("done");
      return;
    }
    setPhase("idle");
    setProblem(c.inviteErrors[result.code] ?? c.inviteErrors.default);
  }
  return (
    <main className="orders-page customers-page" data-testid="invite-page" style={{ maxWidth: 560, margin: "48px auto", padding: "0 16px" }}>
      <section className="customers-section">
        <h1>{c.title}</h1>
        <p>{c.inviteIntro}</p>
        {phase === "done" ? (
          <>
            <p role="status" data-testid="invite-done">{c.inviteDone}</p>
            <div className="customers-actions">
              <a className="orders-export" href={`/${locale}/`} data-testid="invite-open">{c.inviteOpen}</a>
            </div>
          </>
        ) : signedIn ? (
          <>
            <p className="orders-hint">{c.inviteSameEmail}</p>
            <div className="customers-actions">
              <button type="button" className="primary" data-testid="invite-accept" disabled={phase === "busy"} onClick={() => void accept()}>
                {phase === "busy" ? c.inviteAccepting : c.inviteAccept}
              </button>
            </div>
            {problem && <p className="orders-bad" role="alert" data-testid="invite-problem">{problem}</p>}
          </>
        ) : (
          <>
            <p data-testid="invite-need-login">{c.inviteNeedLogin}</p>
            <div className="customers-actions">
              <a className="orders-export" href={`/${locale}/`} data-testid="invite-signin">{c.signIn}</a>
              {passwordLogin && <a className="orders-export" href={`/${locale}/signup`} data-testid="invite-signup">{c.signUp}</a>}
            </div>
          </>
        )}
      </section>
    </main>
  );
}
