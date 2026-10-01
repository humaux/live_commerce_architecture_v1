package identity

// Staff team service (contracts/storefront-v2.md §D, migration 0089): list members, invite by email, revoke an
// invitation, change a role, remove a member, and accept an invitation. It runs on the identity pool (role
// commerce_identity) and calls only the identity.staff_* definers, which verify the merchant bearer and the OWNER role in SQL;
// this file never decides authority. Callers: internal/identityhttp/staff.go (BFF route /v1/identity/staff/*), reached from
// apps/admin/app/api/team/[action]/route.ts.
//
// It never takes a tenant from the client (the store id is verified against the bearer in SQL), never stores or logs a
// token (only sha256 reaches SQL; the plaintext lives in this function's stack and the email), never retries a mail
// (SMTP has no idempotency key, I06: a lost invite is re-sent as a NEW invitation that revokes the old one) and never
// lets accept distinguish why it failed (ErrInviteInvalid covers every reason, so it is no account/token oracle).

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"livecommerce/internal/mail"
)

var (
	ErrForbidden          = errors.New("staff: owner role required")
	ErrNotFound           = errors.New("staff: not found")
	ErrInviteInvalid      = errors.New("staff: invitation is not valid")
	ErrAlreadyMember      = errors.New("staff: already a member")
	ErrLastOwner          = errors.New("staff: the last owner cannot be removed")
	ErrTooManyInvitations = errors.New("staff: too many invitations")
)

// StaffRoles is the closed role vocabulary (identity.store_staff CHECK, bundles in identity.staff_role_permissions).
var StaffRoles = map[string]bool{"owner": true, "admin": true, "live_operator": true, "fulfilment": true, "viewer": true}

var uuidPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// StaffMember / StaffInvitation / StaffTeam are the JSON shapes of identity.staff_list (token hashes never leave SQL).
type StaffMember struct {
	PrincipalID string    `json:"principal_id"`
	Email       *string   `json:"email"` // nil for an OIDC-only principal (no password credential)
	Role        string    `json:"role"`
	JoinedAt    time.Time `json:"joined_at"`
	IsMe        bool      `json:"is_me"`
}
type StaffInvitation struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
	Expired   bool      `json:"expired"`
	MailState string    `json:"mail_state"`
}
type StaffTeam struct {
	MyRole      *string           `json:"my_role"` // nil: member without a staff role (pre-0089 fixture grants)
	Members     []StaffMember     `json:"members"`
	Invitations []StaffInvitation `json:"invitations"`
}

// StaffInvite is the answer to an invitation: MailState is SENT, FAILED or UNKNOWN (never PENDING after Invite returns).
type StaffInvite struct {
	ID        string    `json:"id"`
	ExpiresAt time.Time `json:"expires_at"`
	MailState string    `json:"mail_state"`
}

// StaffJoined is the answer to a successful accept.
type StaffJoined struct {
	StoreID string `json:"store_id"`
	Role    string `json:"role"`
}

// Staff is the staff-team service. mailer is the same SMTP adapter as password auth (nil disables invitations).
type Staff struct {
	pool   *pgxpool.Pool
	mailer Mailer
	origin string // COMMERCE_PUBLIC_ORIGIN, the only host that goes into an invitation link
}

// NewStaff requires an OpenIdentityPool result and an https (or loopback test) public origin without a trailing slash.
func NewStaff(pool *pgxpool.Pool, mailer Mailer, publicOrigin string) (*Staff, error) {
	if pool == nil || mailer == nil || publicOrigin == "" {
		return nil, ErrInvalid
	}
	return &Staff{pool: pool, mailer: mailer, origin: publicOrigin}, nil
}

// newUUID returns a random RFC 4122 version-4 id (invitation handle; not a secret).
func newUUID() string {
	var b [16]byte
	_, _ = rand.Read(b[:]) // Go >=1.24: Read always fills the buffer (see randomToken)
	b[6], b[8] = b[6]&0x0f|0x40, b[8]&0x3f|0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

// staffError maps the definers' SQLSTATEs (0089) to service errors. PT404 carries two meanings told apart by message:
// 'invite_invalid' (accept) versus 'not found' (store/member/invitation). Anything else is unavailable (lock timeout etc.).
func staffError(err error) error {
	if err == nil {
		return nil
	}
	var pg *pgconn.PgError
	if !errors.As(err, &pg) {
		return ErrUnavailable
	}
	switch pg.Code {
	case "PT400":
		return ErrInvalid
	case "PT401":
		return ErrUnauthorized
	case "PT403":
		return ErrForbidden
	case "PT404":
		if pg.Message == "invite_invalid" {
			return ErrInviteInvalid
		}
		return ErrNotFound
	case "PT409":
		switch pg.Message {
		case "already_member":
			return ErrAlreadyMember
		case "last_owner":
			return ErrLastOwner
		}
		return ErrConflict
	case "PT429":
		return ErrTooManyInvitations
	}
	return ErrUnavailable
}

// List returns the caller's role and, for an owner, members and open invitations (identity.staff_list).
func (s *Staff) List(ctx context.Context, bearer, store string) (StaffTeam, error) {
	if !validToken(bearer) {
		return StaffTeam{}, ErrUnauthorized
	}
	if !uuidPattern.MatchString(store) {
		return StaffTeam{}, ErrInvalid
	}
	var raw []byte
	err := withTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		return tx.QueryRow(ctx, `SELECT identity.staff_list($1,$2::uuid)`, digest(bearer), store).Scan(&raw)
	})
	if err != nil {
		return StaffTeam{}, staffError(err)
	}
	var team StaffTeam
	if json.Unmarshal(raw, &team) != nil {
		return StaffTeam{}, ErrUnavailable
	}
	return team, nil
}

// Invite creates the invitation in SQL (owner only), then sends ONE mail after commit and records its outcome. The
// token exists only in this frame and the email. A mail failure is reported through MailState, not as an error: the
// invitation exists and the owner can see it and re-send (a new invitation).
func (s *Staff) Invite(ctx context.Context, bearer, store, email, role, locale string) (StaffInvite, error) {
	if !validToken(bearer) {
		return StaffInvite{}, ErrUnauthorized
	}
	e, err := NormalizeEmail(email)
	if err != nil {
		return StaffInvite{}, err // ErrInvalidEmail: the owner typed it, so naming the problem is fine
	}
	if !uuidPattern.MatchString(store) || !StaffRoles[role] || !validLocale(locale) {
		return StaffInvite{}, ErrInvalid
	}
	token := randomToken()
	id := newUUID()
	var expires time.Time
	err = withTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		return tx.QueryRow(ctx, `SELECT expires_at FROM identity.staff_invite($1,$2::uuid,$3::uuid,$4,$5,$6,$7)`,
			digest(bearer), store, id, e, role, locale, digest(token)).Scan(&expires)
	})
	if err != nil {
		return StaffInvite{}, staffError(err)
	}
	// I06: one attempt, never retried; the outcome is recorded once. The mail text carries the only copy of the token.
	sctx, cancel := context.WithTimeout(ctx, syncSendTimeout)
	defer cancel()
	state := "SENT"
	if _, serr := s.mailer.Send(sctx, staffInviteMail(e, locale, role, s.origin+"/"+locale+"/invite/"+token)); serr != nil {
		state = "FAILED"
		if errors.Is(serr, mail.ErrUnknown) {
			state = "UNKNOWN" // may have been delivered: the owner sees UNKNOWN and decides to re-send
		}
	}
	rctx := context.WithoutCancel(ctx) // a cancelled request must still record what was actually sent
	_ = withTx(rctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `SELECT identity.staff_record_invite_mail($1::uuid,$2)`, id, state)
		return err
	}) // a lost record leaves PENDING, which the owner sees as "not confirmed"; nothing is re-sent because of it
	return StaffInvite{ID: id, ExpiresAt: expires, MailState: state}, nil
}

// RevokeInvite withdraws an open invitation (owner only).
func (s *Staff) RevokeInvite(ctx context.Context, bearer, store, invite string) error {
	return s.write(ctx, bearer, `SELECT identity.staff_revoke_invite($1,$2::uuid,$3::uuid)`, store, invite)
}

// SetRole changes a member's role (owner only); ErrLastOwner when it would leave the store without an owner.
func (s *Staff) SetRole(ctx context.Context, bearer, store, principal, role string) error {
	if !StaffRoles[role] {
		return ErrInvalid
	}
	return s.write(ctx, bearer, `SELECT identity.staff_set_role($1,$2::uuid,$3::uuid,$4)`, store, principal, role)
}

// Remove revokes a member (owner only); effective on the member's next request.
func (s *Staff) Remove(ctx context.Context, bearer, store, principal string) error {
	return s.write(ctx, bearer, `SELECT identity.staff_remove($1,$2::uuid,$3::uuid)`, store, principal)
}

// write runs one owner-only definer taking (bearer hash, store, id, extra...).
func (s *Staff) write(ctx context.Context, bearer, query, store, id string, extra ...any) error {
	if !validToken(bearer) {
		return ErrUnauthorized
	}
	if !uuidPattern.MatchString(store) || !uuidPattern.MatchString(id) {
		return ErrInvalid
	}
	args := append([]any{digest(bearer), store, id}, extra...)
	return staffError(withTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, query, args...)
		return err
	}))
}

// Accept consumes the invitation for the signed-in principal. Every refusal that could reveal whether a token,
// invitation or account exists is ErrInviteInvalid; a malformed token is the same error, not ErrInvalid.
func (s *Staff) Accept(ctx context.Context, bearer, token string) (StaffJoined, error) {
	if !validToken(bearer) {
		return StaffJoined{}, ErrUnauthorized
	}
	if !validToken(token) {
		return StaffJoined{}, ErrInviteInvalid
	}
	var out StaffJoined
	err := withTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		return tx.QueryRow(ctx, `SELECT store_id::text, role FROM identity.staff_accept($1,$2)`, digest(bearer), digest(token)).Scan(&out.StoreID, &out.Role)
	})
	if err != nil {
		return StaffJoined{}, staffError(err)
	}
	return out, nil
}
