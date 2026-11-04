// Purpose: Owns the invitation acceptance view and sign-in guidance (signed frames 1468:76880 link … 1468:77350 invitee-off).
// Depends on: react, @live-commerce/i18n, @/lib/invite-next, @/lib/settings-client, @/lib/team-client, @/lib/team-copy, @/lib/team-model, @/lib/company, @/src/shell/api, ./OperatorFooter, ./orders.css, ./customers.css
// Used by: apps/admin/app/[locale]/invite/[token]/page.tsx
"use client";

// Invitation page body (/{locale}/invite/{token}): signed out -> sign in or sign up with the invited address, then return here
// (invite-next fragment); signed in -> one Accept button. BFF POST /api/team/accept {token} -> Go /v1/identity/staff/accept
// (internal/identity/staff.go; migration 0089). The token comes from the URL of this very page and goes only to that call: it is
// never stored, logged or rendered. Every refusal of the invitation itself shows the same generic text (invite_invalid), so the page
// is no oracle for whether a token, an invitation or an account exists. Answer -> signed state: 200 joined, 404 invite_invalid /
// 401 unusable, 409 already_member member, 404 not_found (BFF: email sign-in off) off, anything else checking (never re-accepted).
// Standalone layout (the invitee has no workspace yet), no WorkspaceFrame.
import { useState, type MouseEvent } from "react";
import type { Locale } from "@live-commerce/i18n";
import { inviteNextHash } from "@/lib/invite-next";
import { sessionBoundary } from "@/lib/settings-client";
import { acceptInvite } from "@/lib/team-client";
import { teamParityCopy } from "@/lib/team-copy";
import type { Role } from "@/lib/team-model";
import { company } from "@/lib/company";
import { logoutWorkspace } from "@/src/shell/api";
import { OperatorFooter } from "./OperatorFooter";
import "./orders.css";
import "./customers.css";

type Phase = "idle" | "busy" | "checking" | "joined" | "invalid" | "member" | "off";

/** Owns the invitation acceptance view and sign-in guidance. User acceptance submits the token through team-client. */
export function TeamInvite({ locale, token, signedIn, passwordLogin }: { locale: Locale; token: string; signedIn: boolean; passwordLogin: boolean }) {
  const c = teamParityCopy[locale];
  const [phase, setPhase] = useState<Phase>("idle");
  const [role, setRole] = useState<Role | null>(null);
  const next = inviteNextHash(`/${locale}/invite/${token}`);

  async function accept() {
    if (phase !== "idle") return;
    setPhase("busy");
    let boundary = "";
    try {
      boundary = await sessionBoundary();
    } catch {
      return setPhase("invalid");
    }
    const result = await acceptInvite(token, boundary);
    if (result.ok) { setRole(result.value.role); return setPhase("joined"); }
    // A lost or retry-later answer may already have consumed the single-use link: show checking, never accept again.
    setPhase(result.code === "invite_invalid" || result.code === "unauthorized" ? "invalid" : result.code === "already_member" ? "member" : result.code === "not_found" ? "off" : "checking");
  }
  // "Sign in again" switches account: end this session first, then sign in and come back to this link (invite-next fragment).
  async function signInAgain(event: MouseEvent<HTMLAnchorElement>) {
    event.preventDefault();
    const target = event.currentTarget.href;
    await logoutWorkspace().catch(() => {}); // ponytail: a failed sign-out lands on the existing session's dashboard.
    window.location.assign(target);
  }
  const signInLink = <a className="orders-export" href={`/${locale}/${next}`} onClick={event => void signInAgain(event)}>{c.signIn}</a>;
  const openStore = <a className="orders-export" href={`/${locale}/`} data-testid="invite-open">{c.openStore}</a>;
  const [title, help, actions] = phase === "joined" ? [c.inviteeJoined, c.joinedHelp.replace("{role}", role ? c.roleNames[role] : ""), openStore]
    : phase === "checking" ? [c.checking, c.acceptChecking, null]
    : phase === "invalid" ? [c.inviteInvalid, c.inviteInvalidHelp, signInLink]
    : phase === "member" ? [c.inviteeMember, c.inviteeMemberHelp, openStore]
    : phase === "off" ? [c.inviteeOff, c.inviteeOffHelp, <button key="accept" type="button" className="primary" data-testid="invite-accept" disabled>{c.accept}</button>]
    : [c.join, c.joinHelp, null];
  return (
    <main className="orders-page customers-page" data-testid="invite-page" style={{ maxWidth: 560, margin: "48px auto", padding: "0 16px" }}>
      <section className="customers-section">
        <p className="invite-brand">{company.productName}</p>
        <h1>{title}</h1>
        <p data-testid={phase === "joined" ? "invite-done" : phase === "invalid" ? "invite-problem" : !signedIn ? "invite-need-login" : undefined}
          role={phase === "idle" || phase === "busy" ? undefined : phase === "joined" || phase === "checking" ? "status" : "alert"}>{help}</p>
        <div className="customers-actions">
          {actions ?? (!signedIn ? <>
            {/* invite-next: hand the invite path to sign-in/sign-up in the URL fragment (never sent to a server, so the token
                stays out of request URLs/Referer/logs); PasswordAuth validates it with lib/invite-next.ts (no open redirect). */}
            <a className="orders-export" href={`/${locale}/${next}`} data-testid="invite-signin">{c.signInLink}</a>
            {passwordLogin && <a className="orders-export" href={`/${locale}/signup${next}`} data-testid="invite-signup">{c.signUp}</a>}
          </> : phase !== "checking" && <>
            <button type="button" className="primary" data-testid="invite-accept" disabled={phase === "busy"} onClick={() => void accept()}>{c.accept}</button>
            {signInLink}
          </>)}
        </div>
      </section>
      <OperatorFooter locale={locale} />
    </main>
  );
}
