// Purpose: the merchant message-template domain (W2-05B, contracts/live-console-v1.md §3.4/§3.5/§7.3 and the §11
// /message-templates row). Service is deliberately config-free: publish/list/resolve all run inside the single
// scoped transaction platform.WithScope opens, and every SECURITY DEFINER call re-checks the server-resolved
// tenant/store/principal, so the Service itself never holds a pool, a key or an authority.
// Depends on: migration 0121 (msgtemplates.publish/list/resolve definers), internal/command (idempotent Run),
// internal/platform (Scope), golang.org/x/text (the §3.5 NFKC/NFD/fold normalization).
// Used by: internal/httpapi/templates.go (POST/GET /message-templates) and LC-B4's send path via Resolve.

package msgtemplates

// Service is the template publish/list/resolve side. It holds no pool and no config; each method runs inside the
// caller's scoped transaction.
type Service struct{}

// NewService builds the config-free Service.
func NewService() *Service { return &Service{} }
