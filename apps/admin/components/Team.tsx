// Purpose: Owns the signed staff lists, role review, removal and invitation-withdrawal flows.
// Depends on: react, @live-commerce/i18n, @/lib/model, @/lib/customers-client, @/lib/team-client, @/lib/team-model, @/lib/team-copy, ./WorkspaceFrame, ./team.css
// Used by: apps/admin/app/[locale]/team/page.tsx
// Invariants: server scopes every read/write; invites remain paused; admin promotion is unavailable; uncertain writes never replay.
"use client";

import { useRef, useState } from "react";
import type { Locale } from "@live-commerce/i18n";
import type { Store } from "@/lib/model";
import { useGuardedRead, type ReadCode } from "@/lib/customers-client";
import { readTeam, removeMember, revokeInvite, setMemberRole } from "@/lib/team-client";
import { roles, type Role, type Member, type Invitation, type Team as TeamData } from "@/lib/team-model";
import { teamCopy, teamParityCopy, type TeamParityCopy } from "@/lib/team-copy";
import { WorkspaceFrame } from "./WorkspaceFrame";
import "./team.css";

const format = (text: string, values: Record<string, string>) => text.replace(/\{(\w+)\}/g, (match, key: string) => values[key] ?? match);
// Signed DTO rule: a label may be derived from the email local part; never imply a stored profile name.
const identity = (member: Member, c: TeamParityCopy) => {
  if (!member.email) return member.role === "owner" ? c.noEmail : c.roleNames[member.role];
  if (member.role === "owner" && member.is_me) return member.email;
  const local = member.email.split("@")[0];
  return `${local.charAt(0).toLocaleUpperCase()}${local.slice(1)} · ${member.email}`;
};
const date = (value: string, time = false) => {
  const d = new Date(value);
  const two = (n: number) => String(n).padStart(2, "0");
  // The signed example is Taipei time; format the server instant, never a fixture date or browser timezone.
  const parts = new Intl.DateTimeFormat("en-GB", { timeZone: "Asia/Taipei", month: "2-digit", day: "2-digit", hour: "2-digit", minute: "2-digit", hourCycle: "h23" }).formatToParts(d);
  const part = (key: string) => parts.find(p => p.type === key)?.value ?? two(0);
  return `${part("month")}/${part("day")}${time ? ` ${part("hour")}:${part("minute")}` : ""}`;
};

/** Renders only the current server/session-scoped staff state; all detail state unmounts when the read fence closes. */
export function Team({ locale, stores, store, initialError, renderKey }: {
  locale: Locale; stores: Store[]; store: Store | null; initialError: ReadCode | null; renderKey: string;
}) {
  const c = teamParityCopy[locale];
  const legacy = teamCopy[locale];
  const read = useGuardedRead(`${renderKey}|${locale}|${store?.id ?? ""}`, store ? signal => readTeam(store.id, signal) : null, initialError);
  const loading = read.status === "loading" || read.status === "hidden";
  const unavailable = read.status === "not-found" && store;
  return <WorkspaceFrame locale={locale} storeName={store?.name ?? legacy.noStore} active="team">
    <div className="team-page" data-testid="team-page">
      {stores.length > 1 && <label className="team-store">{legacy.store}
        <select data-testid="store-selector" value={store?.id ?? ""} onChange={e => window.location.assign(`/${locale}/team?store=${e.target.value}`)}>
          {stores.map(item => <option key={item.id} value={item.id}>{item.name}</option>)}
        </select>
      </label>}
      {loading ? <p role="status">{legacy.loading}</p> : read.status === "ready" && read.data && store ? (
        read.data.my_role === "owner" ? <Sections key={`${store.id}|${read.boundary}|${locale}`} team={read.data} store={store.id} boundary={read.boundary} refresh={read.refresh} locale={locale} c={c} /> : <>
          <h1>{c.title}</h1><section className="team-card" data-testid="team-not-owner"><p>{c.notOwner}</p>
            {read.data.my_role && <p className="team-help">{format(c.currentRole, { role: c.roleNames[read.data.my_role] })}</p>}
          </section>
        </>
      ) : <>
        <h1>{unavailable ? c.passwordOff : read.status === "signed-out" ? c.reauth : read.status === "forbidden" ? c.title : c.loadingError}</h1>
        <section className="team-card" role="status"><p>{unavailable ? c.passwordOffHelp : read.status === "signed-out" ? c.reauthHelp : read.status === "forbidden" ? c.notOwner : c.loadingHelp}</p></section>
        {unavailable ? <button disabled>{c.invite}</button> : read.status === "signed-out" ? <a href={`/${locale}/`}>{c.signIn}</a> : <button onClick={read.reload}>{c.latest}</button>}
      </>}
    </div>
  </WorkspaceFrame>;
}

type Action = "role" | "remove" | "revoke";
type Screen = "list" | "pending" | "invite" | "paused" | "edit" | "review" | "done" | "checking" | "unresolved" | "error";

/** Keeps one local review snapshot; completion and reconciliation depend on existing server responses. */
function Sections({ team, store, boundary, refresh, locale, c }: {
  team: TeamData; store: string; boundary: string; refresh: () => Promise<boolean>; locale: Locale; c: TeamParityCopy;
}) {
  const [screen, setScreen] = useState<Screen>("list");
  const [member, setMember] = useState<Member | null>(null);
  const [invitation, setInvitation] = useState<Invitation | null>(null);
  const [role, setRole] = useState<Role>("viewer");
  const [email, setEmail] = useState("");
  const [action, setAction] = useState<Action>("role");
  const [problem, setProblem] = useState("");
  const [busy, setBusy] = useState(false);
  const sending = useRef(false);
  const back = () => { setScreen("list"); setMember(null); setInvitation(null); setProblem(""); };
  const person = action === "revoke" ? invitation?.email : member ? identity(member, c) : "";
  const inviteEntry = () => { setEmail(""); setRole("viewer"); setScreen("invite"); };
  const roleChoices = <fieldset className="team-roles" aria-label={teamCopy[locale].colRole}>
    {roles.map(item => <label key={item}>
      <input type="radio" name="team-role" value={item} checked={role === item} onChange={() => setRole(item)} />
      <span>{c.roleLines[item]}</span>
    </label>)}
  </fieldset>;
  async function submit() {
    if (sending.current || (action === "role" && (!member || role === "admin" || role === member.role))) return;
    if (action === "revoke" ? !invitation : !member || member.role === "owner") return;
    sending.current = true; setBusy(true);
    const result = action === "role" ? await setMemberRole(store, member!.principal_id, role, boundary)
      : action === "remove" ? await removeMember(store, member!.principal_id, boundary)
      : await revokeInvite(store, invitation!.id, boundary);
    if (result.ok) setScreen(await refresh() ? "done" : "checking");
    else if (["retry_later", "unavailable"].includes(result.code)) setScreen("checking");
    else { setProblem(result.code); setScreen("error"); }
    sending.current = false; setBusy(false);
  }
  async function reconcile() {
    if (sending.current) return;
    sending.current = true; setBusy(true);
    // A latest-list read can confirm a change, but never resends an uncertain mutation.
    try {
      const latest = await readTeam(store, AbortSignal.timeout(8000));
      const confirmed = latest.my_role === "owner" && (action === "role"
        ? latest.members.some(item => item.principal_id === member?.principal_id && item.role === role)
        : action === "remove" ? !latest.members.some(item => item.principal_id === member?.principal_id)
        : !latest.invitations.some(item => item.id === invitation?.id));
      setScreen(await refresh() && confirmed ? "done" : "unresolved");
    } catch { await refresh(); setScreen("unresolved"); }
    sending.current = false; setBusy(false);
  }
  const button = (text: string, onClick: () => void, primary = false, disabled = false) => <button type="button" className={primary ? "primary" : ""} onClick={onClick} disabled={busy || disabled}>{text}</button>;
  const invitationTime = (item: Invitation) => format(c.sentExpiry, { sent: date(item.created_at, true), expiry: date(item.expires_at, true) });
  const title = screen === "invite" ? c.invite : screen === "paused" ? c.mailFailed : screen === "edit" || screen === "review" && action === "role" ? c.edit
    : screen === "review" ? action === "remove" ? c.removeTitle : c.withdrawTitle
    : screen === "done" ? action === "role" ? c.roleDone : action === "remove" ? c.removeDone : c.withdrawDone
    : screen === "checking" || screen === "unresolved" ? c.checking
    : screen === "error" ? problem === "last_owner" ? c.lastOwner : problem === "owner_role_disabled" ? c.ownerDisabled : ["unauthorized", "reauth_required", "reauth_failed"].includes(problem) ? c.reauth : c.loadingError
    : screen === "pending" && team.invitations.length === 0 ? c.pending : c.title;
  return <>
    <h1>{title}</h1>
    {(screen === "list" || screen === "pending") && <>
      {(team.members.length === 1 && team.invitations.length === 0 && screen === "list") || (screen === "pending" && team.invitations.length === 0) ? <section className="team-card"><p>{screen === "pending" ? c.pendingEmpty : c.empty}</p></section> : <>
        <div className="team-tabs" aria-label={c.title}>
          {button(c.members, () => setScreen("list"), screen === "list")}
          {button(c.pending, () => setScreen("pending"), screen === "pending")}
        </div>
        {screen === "list" ? <div className="team-list" data-testid="team-members">
          {team.members.map(item => <section className="team-card" key={item.principal_id} data-testid={`member-${item.principal_id}`}>
            <p className="team-person">{identity(item, c)}</p>
            <p className="team-help">{format(item.role === "owner" && item.is_me ? c.ownerJoined : c.joined, { role: c.roleNames[item.role], date: date(item.joined_at) })}</p>
            {item.role === "owner" ? !item.is_me && <p className="team-help">{c.protected}</p> : <div className="team-member-actions">
              {button(c.edit, () => { setMember(item); setRole(item.role); setAction("role"); setScreen("edit"); })}
              {button(c.remove, () => { setMember(item); setAction("remove"); setScreen("review"); })}
            </div>}
          </section>)}
        </div> : <div className="team-list" data-testid="team-invites">
          {team.invitations.length === 0 ? <section className="team-card"><p>{c.pendingEmpty}</p></section> : team.invitations.map(item => <section className="team-card" key={item.id}>
            <p className="team-person">{item.email}</p><p>{c.roleNames[item.role]}</p><p className="team-help">{invitationTime(item)}</p>
            {button(c.withdraw, () => { setInvitation(item); setAction("revoke"); setScreen("review"); })}
          </section>)}
        </div>}
      </>}
      <div className="team-actions">{button(c.invite, inviteEntry, true)}{screen === "pending" && team.invitations.length === 0 && button(c.back, back)}</div>
    </>}
    {screen === "invite" && <form onSubmit={event => { event.preventDefault(); setScreen("paused"); }}>
      <section className="team-card">
        <label className="team-email" htmlFor="team-invite-email">{c.email}
          <input id="team-invite-email" data-testid="team-invite-email" type="email" required maxLength={254} autoComplete="off" value={email} onChange={event => setEmail(event.target.value)} />
        </label>{roleChoices}
      </section>
      <div className="team-actions">{button(c.cancel, back)}<button type="submit" className="primary" data-testid="team-invite-send">{c.send}</button></div>
    </form>}
    {screen === "paused" && <><section className="team-card"><p className="team-person">{email}</p><p>{c.mailFailedHelp}</p></section><div className="team-actions">{button(c.invite, inviteEntry, true, true)}{button(c.back, back)}</div></>}
    {screen === "edit" && member && <>
      <section className="team-card"><p className="team-person">{identity(member, c)}</p>
        {role === "admin" && member.role === "admin" && <p className="team-help">{c.unchangedAdmin}</p>}{roleChoices}
      </section>
      <div className="team-actions">{button(c.cancel, back)}{button(c.next, () => setScreen("review"), true, role === "admin" || role === member.role)}</div>
    </>}
    {screen === "review" && <>
      <section className="team-card"><p className="team-person">{person}</p>
        <p>{action === "role" ? format(role === "viewer" ? c.viewerReview : c.roleReview, { from: c.roleNames[member!.role], to: c.roleNames[role] }) : action === "remove" ? c.removeWarning : c.withdrawWarning}</p>
        {action === "revoke" && invitation && <p>{c.roleNames[invitation.role]}</p>}
      </section>
      <div className="team-actions">{button(c.cancel, () => action === "revoke" ? setScreen("pending") : back())}{button(action === "role" ? c.confirmRole : action === "remove" ? c.confirmRemove : c.confirmWithdraw, () => void submit(), true)}</div>
    </>}
    {screen === "done" && <>
      <section className="team-card" role="status"><p className="team-person">{person}</p><p>{action === "role" ? c.roleNames[role] : action === "remove" ? c.removedHelp : c.withdrawnHelp}</p></section>
      <div className="team-actions">{button(c.back, () => { back(); if (action === "revoke") setScreen("pending"); }, true)}</div>
    </>}
    {(screen === "checking" || screen === "unresolved") && <>
      <section className="team-card" role="status"><p className="team-person">{person}</p>
        <p>{format(c[`${action}${screen === "checking" ? "Checking" : "Unresolved"}`], { role: c.roleNames[action === "revoke" ? invitation!.role : role] })}</p>
      </section>
      {screen === "checking" && <div className="team-actions">{button(c.latest, () => void reconcile(), true)}</div>}
    </>}
    {screen === "error" && <>
      <section className="team-card" role="alert"><p>{problem === "last_owner" ? c.protected : problem === "owner_role_disabled" ? c.ownerDisabledHelp : ["unauthorized", "reauth_required", "reauth_failed"].includes(problem) ? c.reauthHelp : c.loadingHelp}</p></section>
      <div className="team-actions">{["unauthorized", "reauth_required", "reauth_failed"].includes(problem) ? <a href={`/${locale}/`}>{c.signIn}</a> : button(c.back, back, true)}</div>
    </>}
  </>;
}
