// Admin team client: browser -> BFF `/api/team/{action}` (apps/admin/app/api/team/[action]/route.ts) -> Go
// `/v1/identity/staff/*` (internal/identityhttp/staff.go). Every call, reads included, is a POST carrying the cookie CSRF
// pair and the session fence of settings-client (a changed session sends nothing). The invitation token only ever goes to
// `accept`, from the page that holds it in its URL; it is never stored, logged or put in an error.
import { csrfCookie, safeError, sessionBoundary } from "./settings-client";
import { ReadError } from "./customers-client";
import { parseInviteResult, parseJoined, parseTeam, type InviteResult, type Joined, type Role, type Team } from "./team-model";

export type Outcome<T> = { ok: true; value: T } | { ok: false; code: string };

async function call(action: string, body: unknown, boundary: string | null, signal?: AbortSignal): Promise<{ response: Response } | { ok: false; code: string }> {
  const csrf = csrfCookie();
  try {
    // Recheck directly before the network write, like customers-client `post`: a changed session sends nothing.
    if (!csrf || (boundary !== null && (await sessionBoundary(csrf)) !== boundary) || csrfCookie() !== csrf) return { ok: false, code: "unauthorized" };
  } catch {
    return { ok: false, code: "unauthorized" };
  }
  try {
    const response = await fetch(`/api/team/${action}`, {
      method: "POST", credentials: "same-origin", cache: "no-store",
      headers: { "Content-Type": "application/json", "X-CSRF-Token": csrf },
      body: JSON.stringify(body),
      signal: signal ?? AbortSignal.timeout(action === "invite" ? 16000 : 8000),
    });
    return { response };
  } catch {
    return { ok: false, code: "retry_later" };
  }
}

async function outcome<T>(sent: Awaited<ReturnType<typeof call>>, parse: ((value: unknown) => T) | null): Promise<Outcome<T>> {
  if (!("response" in sent)) return sent;
  const { response } = sent;
  if (!response.ok) return { ok: false, code: safeError(await response.json().catch(() => null)).code };
  if (parse === null) return { ok: true, value: undefined as T };
  try {
    return { ok: true, value: parse(await response.json()) };
  } catch {
    return { ok: false, code: "retry_later" };
  }
}

// POST team/list: the caller's role and, for an owner, members + open invitations. Throws ReadError for useGuardedRead.
export async function readTeam(store: string, signal: AbortSignal): Promise<Team> {
  const sent = await call("list", { store_id: store }, null, signal);
  if (!("response" in sent)) throw new ReadError(sent.code === "unauthorized" ? "signed-out" : "unavailable");
  const { response } = sent;
  if (response.status === 401) throw new ReadError("signed-out");
  if (response.status === 403) throw new ReadError("forbidden");
  if (response.status === 404) throw new ReadError("not-found");
  if (!response.ok) throw new ReadError("unavailable");
  try {
    return parseTeam(await response.json());
  } catch {
    throw new ReadError("unavailable");
  }
}
export const inviteMember = async (store: string, email: string, role: Role, locale: string, boundary: string): Promise<Outcome<InviteResult>> =>
  outcome(await call("invite", { store_id: store, email, role, locale }, boundary), parseInviteResult);
export const revokeInvite = async (store: string, invite: string, boundary: string): Promise<Outcome<void>> =>
  outcome(await call("revoke-invite", { store_id: store, invite_id: invite }, boundary), null);
export const setMemberRole = async (store: string, principal: string, role: Role, boundary: string): Promise<Outcome<void>> =>
  outcome(await call("set-role", { store_id: store, principal_id: principal, role }, boundary), null);
export const removeMember = async (store: string, principal: string, boundary: string): Promise<Outcome<void>> =>
  outcome(await call("remove", { store_id: store, principal_id: principal }, boundary), null);
// POST team/accept {token}: consumes the invitation for the signed-in account. boundary = the session fence of the page.
export const acceptInvite = async (token: string, boundary: string): Promise<Outcome<Joined>> =>
  outcome(await call("accept", { token }, boundary), parseJoined);
