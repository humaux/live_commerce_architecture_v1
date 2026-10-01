"use client";

// Team page (/{locale}/team): members with role select + remove (confirm), pending invitations (send again / withdraw), and the
// invite form. BFF POST /api/team/{list,invite,revoke-invite,set-role,remove} -> Go /v1/identity/staff/* (internal/identityhttp/
// staff.go; migration 0089). Only an owner receives the lists: a member with another role gets the "owner only" notice.
// The server is the authority on every action (owner role, last-owner floor, 72 h expiry); each write is followed by a re-read
// instead of trusting the response, and a revoked member stops authorizing on their next request (no session handling here).
import { useState } from "react";
import type { Locale } from "@live-commerce/i18n";
import type { Store } from "@/lib/model";
import { useGuardedRead, type ReadCode } from "@/lib/customers-client";
import { displayTime } from "@/lib/orders-model";
import { inviteMember, readTeam, removeMember, revokeInvite, setMemberRole, type Outcome } from "@/lib/team-client";
import { roles, isRole, type Role, type Team as TeamData } from "@/lib/team-model";
import { teamCopy, type TeamCopy } from "@/lib/team-copy";
import { WorkspaceFrame } from "./WorkspaceFrame";
import "./orders.css";
import "./order-actions.css";
import "./customers.css";

const errorText = (c: TeamCopy, code: string) => c.errors[code] ?? c.errors.default;

export function Team({
  locale, stores, store, initialError, renderKey,
}: {
  locale: Locale; stores: Store[]; store: Store | null; initialError: ReadCode | null; renderKey: string;
}) {
  const c = teamCopy[locale];
  const read = useGuardedRead(`${renderKey}|${locale}|${store?.id ?? ""}`, store ? (signal) => readTeam(store.id, signal) : null, initialError);
  const failure =
    read.status === "signed-out" ? c.signedOut
    : read.status === "forbidden" ? c.notOwner
    : read.status === "not-found" ? (store ? c.notFound : c.noStore)
    : read.status === "unavailable" ? c.unavailable
    : "";
  return (
    <WorkspaceFrame locale={locale} storeName={store?.name ?? c.noStore} active="team">
      <div className="orders-page customers-page" data-testid="team-page">
        <header className="orders-heading">
          <h1>{c.title}</h1>
          <p>{c.subtitle}</p>
        </header>
        <div className="orders-controls">
          {stores.length > 1 && (
            <label>
              {c.store}
              <select data-testid="store-selector" value={store?.id ?? ""} onChange={(event) => window.location.assign(`/${locale}/team?store=${event.target.value}`)}>
                {stores.map((item) => <option key={item.id} value={item.id}>{item.name}</option>)}
              </select>
            </label>
          )}
        </div>
        {(read.status === "loading" || read.status === "hidden") && <p className="orders-message" role="status">{c.loading}</p>}
        {failure && (
          <div className="orders-message" role="status">
            <p>{failure}</p>
            <button type="button" onClick={read.reload}>{c.retry}</button>
          </div>
        )}
        {read.status === "ready" && read.data && store && read.data.my_role !== "owner" && (
          <p className="orders-message" role="status" data-testid="team-not-owner">{c.notOwner}</p>
        )}
        {read.status === "ready" && read.data && store && read.data.my_role === "owner" && (
          <Sections team={read.data} store={store.id} boundary={read.boundary} refresh={read.refresh} locale={locale} c={c} />
        )}
      </div>
    </WorkspaceFrame>
  );
}

function Sections({
  team, store, boundary, refresh, locale, c,
}: {
  team: TeamData; store: string; boundary: string; refresh: () => Promise<boolean>; locale: Locale; c: TeamCopy;
}) {
  const [busy, setBusy] = useState("");
  const [problem, setProblem] = useState("");
  const [notice, setNotice] = useState("");
  const [confirming, setConfirming] = useState("");
  const [email, setEmail] = useState("");
  const [role, setRole] = useState<Role>("viewer");

  // One write at a time: the result code is shown, then the server state is re-read (never trusted from the answer).
  async function run<T>(key: string, action: () => Promise<Outcome<T>>, after?: (value: T) => string) {
    if (busy) return;
    setBusy(key);
    setProblem("");
    setNotice("");
    const result = await action();
    setBusy("");
    if (result.ok) setNotice(after ? after(result.value) : c.saved);
    else setProblem(errorText(c, result.code));
    setConfirming("");
    await refresh();
  }
  const invite = (to: string, as: Role) =>
    run(`invite:${to}`, () => inviteMember(store, to, as, locale, boundary), (v) =>
      v.mail_state === "SENT" ? c.inviteSent : v.mail_state === "FAILED" ? c.inviteMailFailed : c.inviteMailUnknown);
  const time = (value: string) => displayTime(locale, value);
  return (
    <>
      <section className="customers-section" aria-label={c.inviteTitle}>
        <h2>{c.inviteTitle}</h2>
        <p className="orders-hint">{c.inviteHint}</p>
        <form
          className="orders-controls"
          onSubmit={(event) => {
            event.preventDefault();
            if (email.trim() === "") return;
            void invite(email.trim(), role).then(() => setEmail(""));
          }}
        >
          <label>
            {c.inviteEmail}
            <input data-testid="team-invite-email" type="email" required autoComplete="off" maxLength={254} value={email} onChange={(event) => setEmail(event.target.value)} />
          </label>
          <label>
            {c.inviteRole}
            <select data-testid="team-invite-role" value={role} onChange={(event) => isRole(event.target.value) && setRole(event.target.value)}>
              {roles.map((item) => <option key={item} value={item}>{c.roleNames[item]}</option>)}
            </select>
          </label>
          <button type="submit" className="primary" data-testid="team-invite-send" disabled={busy !== ""}>
            {busy.startsWith("invite:") ? c.inviteSending : c.inviteSend}
          </button>
        </form>
        <p className="orders-hint" data-testid="team-role-help">{c.roleHelp[role]}</p>
        {notice && <p className="orders-hint" role="status" data-testid="team-notice">{notice}</p>}
        {problem && <p className="orders-bad" role="alert" data-testid="team-problem">{problem}</p>}
      </section>

      <section className="customers-section" aria-label={c.invitesTitle}>
        <h2>{c.invitesTitle}</h2>
        {team.invitations.length === 0 ? (
          <p className="orders-empty" data-testid="team-invites-none">{c.invitesNone}</p>
        ) : (
          <div className="orders-actions-scroll">
            <table className="orders-actions-table" data-testid="team-invites">
              <thead><tr><th>{c.colEmail}</th><th>{c.colRole}</th><th>{c.colStatus}</th><th>{c.colActions}</th></tr></thead>
              <tbody>
                {team.invitations.map((item) => (
                  <tr key={item.id} data-testid={`invite-${item.id}`}>
                    <td>{item.email}</td>
                    <td>{c.roleNames[item.role]}</td>
                    <td>
                      <span className={`orders-badge ${item.expired ? "orders-tone-warning" : "orders-tone-neutral"}`}>
                        {item.expired ? c.expired : c.mailState[item.mail_state]}
                      </span>
                      <small>{c.expires} {time(item.expires_at)}</small>
                    </td>
                    <td>
                      <button type="button" data-testid={`invite-resend-${item.id}`} disabled={busy !== ""} onClick={() => void invite(item.email, item.role)}>{c.resend}</button>{" "}
                      <button type="button" data-testid={`invite-revoke-${item.id}`} disabled={busy !== ""}
                        onClick={() => void run(`revoke:${item.id}`, () => revokeInvite(store, item.id, boundary))}>{c.revokeInvite}</button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </section>

      <section className="customers-section" aria-label={c.membersTitle}>
        <h2>{c.membersTitle}</h2>
        <div className="orders-actions-scroll">
          <table className="orders-actions-table" data-testid="team-members">
            <thead><tr><th>{c.colEmail}</th><th>{c.colRole}</th><th>{c.colJoined}</th><th>{c.colActions}</th></tr></thead>
            <tbody>
              {team.members.map((member) => (
                <tr key={member.principal_id} data-testid={`member-${member.principal_id}`}>
                  <td>
                    {member.email ?? c.noEmail}
                    {member.is_me && <span className="orders-badge orders-tone-neutral">{c.you}</span>}
                  </td>
                  <td>
                    <select aria-label={c.changeRole} data-testid={`member-role-${member.principal_id}`} value={member.role} disabled={busy !== ""}
                      onChange={(event) => {
                        const next = event.target.value;
                        if (isRole(next) && next !== member.role) void run(`role:${member.principal_id}`, () => setMemberRole(store, member.principal_id, next, boundary));
                      }}>
                      {roles.map((item) => <option key={item} value={item}>{c.roleNames[item]}</option>)}
                    </select>
                  </td>
                  <td>{time(member.joined_at)}</td>
                  <td>
                    {confirming === member.principal_id ? (
                      <>
                        <p className="orders-hint">{member.is_me ? c.removeConfirmSelf : c.removeConfirm}</p>
                        <button type="button" className="danger primary" data-testid={`member-remove-yes-${member.principal_id}`} disabled={busy !== ""}
                          onClick={() => void run(`remove:${member.principal_id}`, () => removeMember(store, member.principal_id, boundary))}>{c.removeYes}</button>{" "}
                        <button type="button" onClick={() => setConfirming("")}>{c.cancel}</button>
                      </>
                    ) : (
                      <button type="button" className="danger" data-testid={`member-remove-${member.principal_id}`} disabled={busy !== ""} onClick={() => setConfirming(member.principal_id)}>{c.removeMember}</button>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </section>
    </>
  );
}
