// Purpose: the publish command (POST /message-templates): structural validation in Go, the §3.5 public-safe check for
// flagged templates, and the idempotent command.Run wrapper around the msgtemplates.publish SECURITY DEFINER (which
// appends the next version, audits template.published, and re-checks live:manage). A republish is a new version; there
// is no edit, so a correction is a new version by design.
// Depends on: msgtemplates.publish definer (migration 0124), internal/command (Run), internal/platform (Scope).
// Used by: internal/httpapi/templates.go (the POST handler) and the foundation tests.

package msgtemplates

import (
	"context"
	"errors"
	"regexp"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"

	"livecommerce/internal/command"
	"livecommerce/internal/platform"
)

// templateIDPattern mirrors the definer's grammar: a lowercase merchant id, forward-slash/underscore/dash allowed after
// the first char (the two fixed ids, order-pay-link/v1 and offer-recommend/v1, match it and are rejected at the definer).
var templateIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9/_-]{0,63}$`)

// Publish validates and appends a template version. public_safe=true additionally requires the body to pass §3.5; a
// template usable for public_reply/recommend must be public_safe (checked here and again by the DB CHECK + definer).
func (s *Service) Publish(ctx context.Context, tx pgx.Tx, scope platform.Scope, key string, in PublishInput) (PublishOutput, error) {
	var out PublishOutput
	if err := validatePublishInput(in); err != nil {
		return out, err
	}
	if in.PublicSafe {
		if reason := ValidatePublicSafe(in.Body, ""); reason != "" {
			return out, PublicUnsafe()
		}
	}
	err := command.Run(ctx, tx, scope, "template.publish", key, publishRequest{
		TemplateID: in.TemplateID, Name: in.Name, Body: in.Body, Kinds: in.Kinds, PublicSafe: in.PublicSafe,
	}, &out, func() error {
		return tx.QueryRow(ctx, `SELECT template_id, version, public_safe, kinds FROM msgtemplates.publish($1,$2,$3,$4,$5)`,
			in.TemplateID, in.Name, in.Body, in.Kinds, in.PublicSafe).
			Scan(&out.TemplateID, &out.Version, &out.PublicSafe, &out.Kinds)
	})
	if err != nil {
		if errors.Is(err, command.ErrConflict) {
			return out, err
		}
		return out, databaseError(err)
	}
	return out, nil
}

// validatePublishInput is the Go-side structural gate (the definer re-checks the same invariants as defence-in-depth).
func validatePublishInput(in PublishInput) error {
	if !templateIDPattern.MatchString(in.TemplateID) {
		return command.ErrInvalid
	}
	if n := utf8.RuneCountInString(in.Name); n < 1 || n > 120 {
		return command.ErrInvalid
	}
	if n := utf8.RuneCountInString(in.Body); n < 1 || n > 2000 {
		return command.ErrInvalid
	}
	if len(in.Kinds) < 1 || len(in.Kinds) > 4 {
		return command.ErrInvalid
	}
	seen := make(map[string]bool, len(in.Kinds))
	publicKind := false
	for _, kind := range in.Kinds {
		if !validKind[kind] || seen[kind] {
			return command.ErrInvalid
		}
		seen[kind] = true
		if kind == KindPublicReply || kind == KindRecommend {
			publicKind = true
		}
	}
	// A template usable for public replies or the recommend comment must be flagged public_safe (§3.5).
	if publicKind && !in.PublicSafe {
		return command.ErrInvalid
	}
	return nil
}
