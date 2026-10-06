// Purpose: the platform-Stripe subcommands of stripe-admin (contracts/stripe-platform-account-v1.md §3.3, §7, owner amendment AD-PF2):
//   platform-designate, platform-open, platform-close, platform-allow, platform-disallow, platform-block, platform-unblock.
// Depends on: internal/payments/stripeadmin (PlatformDesignate/Open/Block/Allow); env COMMERCE_STRIPE_REGISTRAR_DATABASE_URL, and the
//   owner's LIVE flag+reference pair for platform-open --environment LIVE only.
// Used by: cmd/stripe-admin main.go dispatch; deploy/scripts/ops-admin.sh allowlist.
//
// Flags (--tenant/--store/--principal name the PLATFORM store):
//	platform-designate --connection --display-name --descriptor --terms-version [--expected-version]
//	platform-open      --environment [--fee-bps] --expected-version     (LIVE needs the owner's flag+reference pair)
//	platform-close     --environment --expected-version
//	platform-allow | platform-disallow --target-tenant --target-store --environment --operator --ticket
//	platform-block | platform-unblock  --target-tenant --target-store --environment --operator [--ticket]
// None takes a secret; block, unblock, close, allow and disallow never read the LIVE pair (kill switches must not depend on deploy
// env). Each prints one JSON line of ids/versions; failures one fixed code.

package main

import (
	"context"
	"io"
	"strings"
	"time"

	"livecommerce/internal/integrations/psp/stripe"
	"livecommerce/internal/payments/stripeadmin"
)

func runPlatform(ctx context.Context, name string, args []string, getenv func(string) string, stdout io.Writer) error {
	c := newCommand(name)
	var connection, displayName, descriptor, terms, environment, targetTenant, targetStore, operator, ticket string
	var expected int64
	feeBPS := -1
	switch name {
	case "platform-designate":
		c.fs.StringVar(&connection, "connection", "", "")
		c.fs.StringVar(&displayName, "display-name", "", "")
		c.fs.StringVar(&descriptor, "descriptor", "", "")
		c.fs.StringVar(&terms, "terms-version", "", "")
		c.fs.Int64Var(&expected, "expected-version", 0, "")
	case "platform-open":
		c.fs.IntVar(&feeBPS, "fee-bps", -1, "")
		fallthrough
	case "platform-close":
		c.fs.StringVar(&environment, "environment", "", "")
		c.fs.Int64Var(&expected, "expected-version", 0, "")
	case "platform-allow", "platform-disallow", "platform-block", "platform-unblock":
		c.fs.StringVar(&targetTenant, "target-tenant", "", "")
		c.fs.StringVar(&targetStore, "target-store", "", "")
		c.fs.StringVar(&environment, "environment", "", "")
	default:
		return errUsage
	}
	c.fs.StringVar(&operator, "operator", "", "")
	c.fs.StringVar(&ticket, "ticket", "", "")
	if err := c.parse(args); err != nil {
		return err
	}
	dsn := getenv("COMMERCE_STRIPE_REGISTRAR_DATABASE_URL")
	if strings.TrimSpace(dsn) == "" {
		return errConfig
	}
	// Only platform-open on LIVE needs the owner's pair; everything else (incl. every kill switch) reads no LIVE variable.
	var live stripe.LiveApproval
	if name == "platform-open" && environment == "LIVE" {
		var err error
		if live, err = requireLivePair(getenv); err != nil {
			return err
		}
	}
	reg, err := openRegistrar(ctx, dsn, nil, nil, live)
	if err != nil {
		return err
	}
	defer reg.Close()
	switch name {
	case "platform-designate":
		v, err := reg.PlatformDesignate(ctx, c.scope, stripeadmin.PlatformDesignateInput{ConnectionID: connection,
			DisplayName: displayName, DescriptorDisplay: descriptor, TermsVersion: terms, ExpectedVersion: expected})
		if err != nil {
			return err
		}
		return emit(stdout, map[string]any{"connection_id": connection, "platform_version": v})
	case "platform-open", "platform-close":
		v, err := reg.PlatformOpen(ctx, c.scope, environment, name == "platform-open", feeBPS, expected)
		if err != nil {
			return err
		}
		return emit(stdout, map[string]any{"environment": environment, "platform_version": v, "enrollment_open": name == "platform-open"})
	case "platform-allow", "platform-disallow":
		v, err := reg.PlatformAllow(ctx, c.scope, targetTenant, targetStore, environment, name == "platform-allow", operator, ticket)
		if err != nil {
			return err
		}
		return emit(stdout, map[string]any{"target_store": targetStore, "allowed": name == "platform-allow", "allowlist_version": v})
	default: // platform-block | platform-unblock
		at, err := reg.PlatformBlock(ctx, c.scope, targetTenant, targetStore, environment, name == "platform-block", operator, ticket)
		if err != nil {
			return err
		}
		return emit(stdout, map[string]any{"target_store": targetStore, "blocked": name == "platform-block", "at": at.UTC().Format(time.RFC3339)})
	}
}
