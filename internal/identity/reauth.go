// Purpose: password + emailed-code recent re-authentication of a live merchant session (contracts/merchant-password-
// auth-v1.md §16): StartReauth verifies the current password and mails a one-time code bound to the calling session
// hash; CompleteReauth verifies the code and moves only that session's authenticated_at. Also the
// COMMERCE_REAUTH_WINDOW_MINUTES parser for the guarded definers' window.
// Depends on: Passwords (throttle.go buckets, sender.go sendSync, password.go Argon2/CodeHMAC, mailcopy.go codeMail);
// SQL definers identity.reauth_material, identity.record_password_failure, identity.start_reauth_challenge,
// identity.complete_reauth_challenge, identity.record_challenge_mail (0070/0201) on the commerce_identity pool.
// Used by: internal/identityhttp re-auth routes (OC-15), cmd/api wiring of the window (OC-15).
// Invariants: the session comes only from the bearer token (no email or principal in the request); a wrong password
// is ErrReauthFailed (HTTP 403, never 401) and counts toward PD12 like a login failure; no path issues a session.
package identity

import (
	"context"
	"encoding/hex"
	"errors"
	"net/netip"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"golang.org/x/text/unicode/norm"
)

// ErrReauthFailed: wrong password, disabled credential, no password credential (OIDC-only principal: use
// StartOIDCReauth), another OIDC account or a stale/missing auth_time (HTTP 403 reauth_failed; the session stays).
var ErrReauthFailed = errors.New("reauth failed")

// DefaultReauthWindow is the §16 default of COMMERCE_REAUTH_WINDOW_MINUTES.
const DefaultReauthWindow = 10 * time.Minute

// ReauthWindow parses COMMERCE_REAUTH_WINDOW_MINUTES: empty = 10 minutes, else an integer 1..60.
func ReauthWindow(raw string) (time.Duration, error) {
	if raw == "" {
		return DefaultReauthWindow, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 || n > 60 {
		return 0, ErrInvalid
	}
	return time.Duration(n) * time.Minute, nil
}

func init() {
	// The reauth purpose of the code mail (mailcopy.go holds the other purposes; same shape, no link, no code in the subject).
	codeCopy["en"]["reauth"] = copyText{"Your verification code", "Use this code to confirm it is you before a protected action:", "The code is valid for 10 minutes.", "If this was not you, change your password now."}
	codeCopy["zh-CN"]["reauth"] = copyText{"您的身份确认验证码", "请使用以下验证码确认是您本人在进行受保护的操作：", "验证码 10 分钟内有效。", "如果这不是您本人的操作，请立即修改密码。"}
	codeCopy["zh-TW"]["reauth"] = copyText{"您的身分確認驗證碼", "請使用以下驗證碼確認是您本人在進行受保護的操作：", "驗證碼 10 分鐘內有效。", "如果這不是您本人的操作，請立即變更密碼。"}
}

type reauthMaterial struct {
	principal string
	email     *string
	phc       *string
	version   *int64
	disabled  *bool
}

// material reads the calling live session's principal and credential; ErrUnauthorized when the session is not live.
func (p *Passwords) reauthMaterial(ctx context.Context, sessionHash []byte) (reauthMaterial, error) {
	var m reauthMaterial
	err := withTx(ctx, p.pool, func(ctx context.Context, tx pgx.Tx) error {
		return tx.QueryRow(ctx, `SELECT principal_id::text,email,password_hash,password_version,disabled FROM identity.reauth_material($1)`, sessionHash).
			Scan(&m.principal, &m.email, &m.phc, &m.version, &m.disabled)
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return m, ErrUnauthorized
	}
	if err != nil {
		return m, ErrUnavailable
	}
	return m, nil
}

// reauthBuckets are the §16 throttle keys: principal and session hash, with the login windows of the step.
func reauthBuckets(step string, m reauthMaterial, sessionHash []byte, wins []window) []bucket {
	return []bucket{
		{kind: "reauth-" + step + "-principal", value: m.principal, wins: wins},
		{kind: "reauth-" + step + "-session", value: hex.EncodeToString(sessionHash), wins: wins},
	}
}

// StartReauth verifies the current password of the session's principal and mails a code bound to that session.
// Returns the code expiry. Errors: ErrUnauthorized (session not live), ErrReauthFailed, ThrottleError, ErrBusy,
// ErrMailUnavailable, ErrInvalid, ErrUnavailable.
func (p *Passwords) StartReauth(ctx context.Context, ip netip.Addr, sessionToken, password, locale string) (time.Time, error) {
	if !validLocale(locale) || validClient(ip) != nil {
		return time.Time{}, ErrInvalid
	}
	ip = ip.Unmap()
	if err := p.throttle(ctx, ipBuckets(ip)...); err != nil {
		return time.Time{}, err
	}
	if !validToken(sessionToken) {
		return time.Time{}, ErrUnauthorized
	}
	sessionHash := digest(sessionToken)
	m, err := p.reauthMaterial(ctx, sessionHash)
	if err != nil {
		return time.Time{}, err
	}
	if err := p.throttle(ctx, reauthBuckets("pw", m, sessionHash, winEmailPW)...); err != nil {
		return time.Time{}, err
	}
	if m.email == nil || m.phc == nil || m.version == nil || m.disabled == nil {
		return time.Time{}, ErrReauthFailed // OIDC-only principal: no password to prove (§16: StartOIDCReauth)
	}
	pw := norm.NFC.String(password)
	if len(pw) > 1024 { // as Login: longer can never match the 128-code-point policy
		pw = pw[:1024]
	}
	ok, err := VerifyPassword(*m.phc, pw)
	if err != nil {
		if errors.Is(err, ErrBusy) {
			return time.Time{}, ErrBusy
		}
		return time.Time{}, ErrUnavailable
	}
	if !ok {
		// Same counter as a login failure (PD12 hard disable at 100, which also blocks the guarded operations).
		if err := withTx(ctx, p.pool, func(ctx context.Context, tx pgx.Tx) error {
			_, err := tx.Exec(ctx, `SELECT identity.record_password_failure($1)`, *m.email)
			return err
		}); err != nil {
			return time.Time{}, ErrUnavailable
		}
		return time.Time{}, ErrReauthFailed
	}
	if *m.disabled {
		return time.Time{}, ErrReauthFailed
	}
	if err := p.throttle(ctx, bucket{kind: "email-mail-login", value: *m.email, wins: winMailLogin}); err != nil {
		return time.Time{}, err
	}
	mat, err := p.newMaterial() // fresh id + unique binding hash; the code HMAC is re-keyed on the session hash below
	if err != nil {
		return time.Time{}, err
	}
	id := mat.id
	codeHMAC := CodeHMAC(p.policy.Pepper, sessionHash, mat.code)
	var expires time.Time
	err = withTx(ctx, p.pool, func(ctx context.Context, tx pgx.Tx) error {
		// identity.start_reauth_challenge: live session + version + enabled credential, supersedes this session's open
		// reauth challenges, inserts one bound to the session hash. PT401 when anything changed since the material read.
		return tx.QueryRow(ctx, `SELECT identity.start_reauth_challenge($1,$2,$3,$4,$5,$6,$7)`,
			id, sessionHash, *m.version, mat.bindingHash, codeHMAC, locale, p.bucketKey("ip", ipPrefix(ip).String())).Scan(&expires)
	})
	if errors.Is(translateError(err), ErrUnauthorized) {
		return time.Time{}, ErrReauthFailed
	}
	if err != nil {
		return time.Time{}, ErrUnavailable
	}
	_, _, login := mailShares(p.policy.MailDailyCap)
	if err := p.throttle(ctx, p.globalBucket("global-mail-login", login)); err != nil {
		if errors.Is(err, ErrMailUnavailable) {
			p.recordMail(ctx, id, "FAILED", "") // nothing was sent; the row must not stay PENDING
		}
		return time.Time{}, err
	}
	if p.sendSync(ctx, id, codeMail(*m.email, locale, "reauth", mat.code)) == "FAILED" {
		return time.Time{}, ErrMailUnavailable
	}
	return expires.UTC().Truncate(time.Second), nil
}

// CompleteReauth verifies the code of the calling session's open reauth challenge and returns the new
// authenticated_at. Errors: ErrInvalidCode (wrong, expired, exhausted, over budget, or a code of another session),
// ErrUnauthorized (session not live), ThrottleError, ErrUnavailable, ErrInvalid.
func (p *Passwords) CompleteReauth(ctx context.Context, ip netip.Addr, sessionToken, code string) (time.Time, error) {
	if validClient(ip) != nil {
		return time.Time{}, ErrInvalid
	}
	if !validCode(code) {
		return time.Time{}, ErrInvalidCode
	}
	ip = ip.Unmap()
	if err := p.throttle(ctx, ipBuckets(ip)...); err != nil {
		return time.Time{}, err
	}
	if !validToken(sessionToken) {
		return time.Time{}, ErrUnauthorized
	}
	sessionHash := digest(sessionToken)
	m, err := p.reauthMaterial(ctx, sessionHash)
	if err != nil {
		return time.Time{}, err
	}
	if err := p.throttle(ctx, reauthBuckets("code", m, sessionHash, winBinding)...); err != nil {
		return time.Time{}, err
	}
	var outcome string
	var at *time.Time
	err = withTx(ctx, p.pool, func(ctx context.Context, tx pgx.Tx) error {
		// identity.complete_reauth_challenge: INVALID is returned, not raised, so a wrong-code attempt counter commits.
		return tx.QueryRow(ctx, `SELECT outcome,authenticated_at FROM identity.complete_reauth_challenge($1,$2)`,
			sessionHash, CodeHMAC(p.policy.Pepper, sessionHash, code)).Scan(&outcome, &at)
	})
	if err != nil {
		return time.Time{}, ErrUnavailable
	}
	if outcome != "VERIFIED" || at == nil {
		return time.Time{}, ErrInvalidCode
	}
	return *at, nil
}
