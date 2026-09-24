package checkout

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"

	"livecommerce/internal/buyer"
	"livecommerce/internal/command"
	"livecommerce/internal/integrations/accounts"
	"livecommerce/internal/integrations/psp/payuni"
	"livecommerce/internal/platform"
)

type HostedConfig = accounts.HostedConfig

type HostedInput struct {
	OrderID       string `json:"order_id"`
	MethodCode    string `json:"method_code"`
	MethodVersion int64  `json:"method_version"`
	Locale        string `json:"locale"`
}

type HostedForm struct {
	Action string            `json:"action"`
	Fields map[string]string `json:"fields"`
}

type HostedHandoff struct {
	OrderID     string      `json:"order_id"`
	Disposition string      `json:"disposition"`
	ExpiresAt   time.Time   `json:"expires_at"`
	Form        *HostedForm `json:"form,omitempty"`
}

type HostedPaymentStarter struct {
	starter      PaymentStarter
	keys         *accounts.Keyring
	config       HostedConfig
	configDigest [32]byte
}

func NewHostedPaymentStarter(ctx context.Context, hostedPool *pgxpool.Pool, jobs *river.Client[pgx.Tx],
	profile string, keys *accounts.Keyring, config HostedConfig) (*HostedPaymentStarter, error) {
	canonical, digest, err := config.CanonicalDigest()
	if ctx == nil || jobs == nil || keys == nil || !validPaymentProfile(profile) || err != nil {
		return nil, command.ErrInvalid
	}
	if err := platform.ValidateHostedPool(ctx, hostedPool); err != nil {
		return nil, err
	}
	return &HostedPaymentStarter{starter: PaymentStarter{pool: hostedPool, jobs: jobs, profile: profile},
		keys: keys, config: canonical, configDigest: digest}, nil
}

// BeginHosted commits the original payment start and its sealed form together.
// The repeatable result contains no form or credential material.
func (s *HostedPaymentStarter) BeginHosted(ctx context.Context, token, storeID, key string, in HostedInput) (PaymentResult, error) {
	if ctx == nil || s == nil || s.starter.pool == nil || s.starter.jobs == nil || s.keys == nil ||
		!validPaymentProfile(s.starter.profile) || !checkoutKey.MatchString(key) || !validHostedInput(in) {
		return PaymentResult{}, command.ErrInvalid
	}
	request, _ := json.Marshal(struct {
		Input        HostedInput `json:"input"`
		Profile      string      `json:"profile"`
		ConfigDigest string      `json:"config_digest"`
	}{in, s.starter.profile, hex.EncodeToString(s.configDigest[:])})
	digest := sha256.Sum256(request)
	tokenHash := sha256.Sum256([]byte(token))
	var out PaymentResult
	err := buyer.WithScope(ctx, s.starter.pool, token, storeID, func(callCtx context.Context, tx pgx.Tx, scope buyer.Scope) error {
		payment := PaymentInput{OrderID: in.OrderID, MethodCode: in.MethodCode, MethodVersion: in.MethodVersion}
		var found bool
		var err error
		out, found, err = s.starter.startPaymentTx(callCtx, tx, scope, tokenHash[:], storeID, key, payment, digest)
		if err != nil || found {
			return err
		}
		built, err := s.keys.BuildPaymentHosted(callCtx, tx, tokenHash[:], storeID, in.OrderID,
			s.starter.profile, in.Locale, s.config)
		if err != nil {
			return err
		}
		form, err := boundedHostedForm(built.Form, s.starter.profile)
		if err != nil {
			return err
		}
		body, err := json.Marshal(form)
		if err != nil {
			return command.ErrConflict
		}
		if _, err = tx.Exec(callCtx, `SELECT checkout.save_hosted_page($1::bytea,$2::uuid,$3::uuid,$4::text,$5::text,$6::bytea,$7::jsonb)`,
			tokenHash[:], storeID, in.OrderID, s.starter.profile, in.Locale, s.configDigest[:], body); err != nil {
			return err
		}
		return checkCapability(callCtx, tx, tokenHash[:], storeID, scope)
	})
	if err != nil {
		return PaymentResult{}, safeError(ctx, err)
	}
	return out, nil
}

// TakeHosted releases a form only after the database has committed the one-shot
// transition. Any uncertain response must be reconciled, never taken again.
func (s *HostedPaymentStarter) TakeHosted(ctx context.Context, token, storeID, orderID string) (HostedHandoff, error) {
	if ctx == nil || s == nil || s.starter.pool == nil || !validPaymentProfile(s.starter.profile) || !command.ValidID(orderID) {
		return HostedHandoff{}, command.ErrInvalid
	}
	tokenHash := sha256.Sum256([]byte(token))
	var out HostedHandoff
	err := buyer.WithScope(ctx, s.starter.pool, token, storeID, func(callCtx context.Context, tx pgx.Tx, scope buyer.Scope) error {
		var body []byte
		if err := tx.QueryRow(callCtx, `SELECT checkout.take_hosted_page($1::bytea,$2::uuid,$3::uuid,$4::text,$5::bytea)`,
			tokenHash[:], storeID, orderID, s.starter.profile, s.configDigest[:]).Scan(&body); err != nil {
			return err
		}
		decoder := json.NewDecoder(bytes.NewReader(body))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&out); err != nil || !validHostedHandoff(out, orderID, s.starter.profile) {
			return command.ErrConflict
		}
		return checkCapability(callCtx, tx, tokenHash[:], storeID, scope)
	})
	if err != nil {
		return HostedHandoff{}, safeError(ctx, err)
	}
	return out, nil
}

func validHostedInput(in HostedInput) bool {
	return validPaymentInput(PaymentInput{OrderID: in.OrderID, MethodCode: in.MethodCode,
		MethodVersion: in.MethodVersion}) &&
		(in.Locale == "zh-CN" || in.Locale == "zh-TW" || in.Locale == "en")
}

func boundedHostedForm(raw payuni.HostedForm, profile string) (HostedForm, error) {
	form := HostedForm{Action: raw.Action, Fields: make(map[string]string, 4)}
	if len(raw.Fields) != 4 {
		return HostedForm{}, command.ErrConflict
	}
	for key, values := range raw.Fields {
		if len(values) != 1 {
			return HostedForm{}, command.ErrConflict
		}
		form.Fields[key] = values[0]
	}
	if !validHostedForm(form, profile) {
		return HostedForm{}, command.ErrConflict
	}
	return form, nil
}

func validHostedForm(form HostedForm, profile string) bool {
	action := "https://sandbox-api.payuni.com.tw/api/upp"
	if profile == "LIVE" {
		action = "https://api.payuni.com.tw/api/upp"
	}
	if form.Action != action || len(form.Fields) != 4 || form.Fields["Version"] != "2.0" {
		return false
	}
	for _, key := range []string{"MerID", "Version", "EncryptInfo", "HashInfo"} {
		value, ok := form.Fields[key]
		if !ok || len(value) == 0 || len(value) > 8192 {
			return false
		}
	}
	return true
}

func validHostedHandoff(out HostedHandoff, orderID, profile string) bool {
	if out.OrderID != orderID || out.ExpiresAt.IsZero() {
		return false
	}
	switch out.Disposition {
	case "ISSUED":
		return out.Form != nil && validHostedForm(*out.Form, profile)
	case "ALREADY_ISSUED":
		return out.Form == nil
	default:
		return false
	}
}
