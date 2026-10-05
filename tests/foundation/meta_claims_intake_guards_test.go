// T10c meta claims intake gate MCI10 (source guards, contracts/meta-claims-intake-v1.md §12 and
// live-keyword-claims-v1.md §4/KC15), written by the independent test_worker with go/parser from the
// FROZEN contract. Pure source inspection: no database, no network, no build of the code under test.
//
// Owns: who may call each ingest entry (Ingest/IngestParsed only from RecordManualClaim,
// IngestMetaIntake only from internal/claimsintake), where net/http or net may not appear (the
// consumer's claim_intake.go, consumer.go, the intake poller), who may see core.SecretClaim, what
// internal/claims may import, that the reply/intake code adds no River job kind and no direct River
// insert other than core.InsertOperationJob, which files may read which secret env names, that no new
// third-party module is introduced by the new packages, and the PROCESS.md §5 package comments.
//
// Non-goals: the behavioural halves of MCI10 (MC/MI/MIso/KC/T06 regression, full race suite, vet,
// check_packet) are commands, not tests; see docs/delivery/units/meta-intake-tests.md.
// Evidence label: REVIEW + regression.
package foundation_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

type mciSrc struct {
	path string // repo-relative, slash-separated
	dir  string // repo-relative directory
	file *ast.File
	text string
}

func mciSources(t *testing.T, roots ...string) []mciSrc {
	t.Helper()
	var out []mciSrc
	fset := token.NewFileSet()
	for _, root := range roots {
		err := filepath.WalkDir(filepath.Join("../..", root), func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			body, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			file, err := parser.ParseFile(fset, path, body, parser.ParseComments)
			if err != nil {
				return err
			}
			rel := filepath.ToSlash(strings.TrimPrefix(path, "../../"))
			out = append(out, mciSrc{path: rel, dir: filepath.ToSlash(filepath.Dir(rel)), file: file, text: string(body)})
			return nil
		})
		if err != nil {
			t.Fatalf("parse %s: %v", root, err)
		}
	}
	if len(out) == 0 {
		t.Fatal("no Go sources found")
	}
	return out
}

func mciImports(f *ast.File) map[string]string { // import path -> local name
	out := map[string]string{}
	for _, imp := range f.Imports {
		p, _ := strconv.Unquote(imp.Path.Value)
		name := filepath.Base(p)
		if imp.Name != nil {
			name = imp.Name.Name
		}
		out[p] = name
	}
	return out
}

type mciCall struct{ dir, fn, file string }

// mciCallers lists the enclosing function of every call to pkgFunc: as claims.<name> from another
// package, or as a bare <name> inside internal/claims itself.
func mciCallers(srcs []mciSrc, importPath, dir, name string) []mciCall {
	var out []mciCall
	for _, s := range srcs {
		local, imported := mciImports(s.file)[importPath]
		sameDir := s.dir == dir
		if !imported && !sameDir {
			continue
		}
		for _, decl := range s.file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				switch f := call.Fun.(type) {
				case *ast.Ident:
					if sameDir && f.Name == name {
						out = append(out, mciCall{s.dir, fn.Name.Name, s.path})
					}
				case *ast.SelectorExpr:
					if x, ok := f.X.(*ast.Ident); ok && imported && x.Name == local && f.Sel.Name == name {
						out = append(out, mciCall{s.dir, fn.Name.Name, s.path})
					}
				}
				return true
			})
		}
	}
	return out
}

func mciNames(calls []mciCall) []string {
	var out []string
	for _, c := range calls {
		out = append(out, c.dir+"."+c.fn)
	}
	sort.Strings(out)
	return out
}

const claimsPkg = "livecommerce/internal/claims"

// TestMetaClaimsMCI10IngestCallers: the frozen ingest entries have exactly the callers the contract
// names (KC15 as amended by §4.4 clause 7). A new caller of the manual path from Meta code, or a
// Meta caller outside the intake worker, is a privilege escalation of the claim writer.
func TestMetaClaimsMCI10IngestCallers(t *testing.T) {
	srcs := mciSources(t, "internal", "cmd")
	for _, name := range []string{"Ingest", "IngestParsed"} {
		for _, c := range mciCallers(srcs, claimsPkg, "internal/claims", name) {
			// Ingest itself delegates to IngestParsed; every other caller must be RecordManualClaim.
			if c.dir == "internal/claims" && (c.fn == "RecordManualClaim" || (name == "IngestParsed" && c.fn == "Ingest")) {
				continue
			}
			t.Errorf("%s is called from %s.%s (%s); only RecordManualClaim may call it (KC15)", name, c.dir, c.fn, c.file)
		}
	}
	meta := mciCallers(srcs, claimsPkg, "internal/claims", "IngestMetaIntake")
	var outside []mciCall
	for _, c := range meta {
		if c.dir == "internal/claims" && c.fn == "IngestMetaIntake" {
			continue
		}
		outside = append(outside, c)
	}
	if len(outside) != 1 || outside[0].dir != "internal/claimsintake" {
		t.Errorf("IngestMetaIntake callers = %v, want exactly one, inside internal/claimsintake (the intake worker)", mciNames(outside))
	}
	// RecordManualClaim stays the manual path's only production entry: nobody but HTTP adapters calls it.
	for _, c := range mciCallers(srcs, claimsPkg, "internal/claims", "RecordManualClaim") {
		if strings.Contains(c.dir, "claimsintake") || strings.Contains(c.dir, "metareply") || strings.Contains(c.dir, "integrations") || strings.HasPrefix(c.dir, "cmd/claims-worker") || strings.HasPrefix(c.dir, "cmd/meta-worker") {
			t.Errorf("RecordManualClaim is called from Meta/intake code %s.%s", c.dir, c.fn)
		}
	}
}

// TestMetaClaimsMCI10NoNetworkInIntakeCode: the consumer transaction and the intake poller do no
// network I/O (contract §5, MCI10): neither net nor net/http may be imported by them.
func TestMetaClaimsMCI10NoNetworkInIntakeCode(t *testing.T) {
	srcs := mciSources(t, "internal", "cmd")
	guarded := 0
	for _, s := range srcs {
		if s.path != "internal/integrations/meta/consumer.go" && s.path != "internal/integrations/meta/claim_intake.go" && s.dir != "internal/claimsintake" && s.dir != "internal/claims" {
			continue
		}
		guarded++
		for imp := range mciImports(s.file) {
			if imp == "net" || strings.HasPrefix(imp, "net/http") || imp == "crypto/tls" {
				t.Errorf("%s imports %s; the consumer and intake poller must do no network I/O", s.path, imp)
			}
		}
	}
	for _, need := range []string{"internal/integrations/meta/consumer.go", "internal/integrations/meta/claim_intake.go"} {
		found := false
		for _, s := range srcs {
			found = found || s.path == need
		}
		if !found {
			t.Errorf("%s does not exist (the guard has nothing to guard)", need)
		}
	}
	dirs := map[string]bool{}
	for _, s := range srcs {
		dirs[s.dir] = true
	}
	if !dirs["internal/claimsintake"] {
		t.Error("package internal/claimsintake does not exist")
	}
	if guarded < 4 {
		t.Errorf("only %d guarded files found", guarded)
	}
}

// TestMetaClaimsMCI10SecretClaimOnlyInLoadSecret: core.SecretClaim (operation id, generation, lease
// token) may be named only by the dispatcher that builds it and by the one LoadSecret loader that
// forwards it to the lease-fenced SQL function, plus explicitly contracted Finish hooks;
// Check, Dispatch and Reconcile never see it.
func TestMetaClaimsMCI10SecretClaimOnlyInLoadSecret(t *testing.T) {
	srcs := mciSources(t, "internal", "cmd")
	if !mciPageLoaderReferences(srcs) {
		t.Error("shared Page loader must have exactly the two contracted constant LoadSecret callsites, without aliases")
	}
	mentions := 0
	for _, s := range srcs {
		if s.dir == "internal/integrations/core" {
			continue
		}
		for _, decl := range s.file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok {
				if gen, ok := decl.(*ast.GenDecl); ok {
					ast.Inspect(gen, func(n ast.Node) bool {
						if id, ok := n.(*ast.Ident); ok && id.Name == "SecretClaim" {
							t.Errorf("%s declares or aliases SecretClaim at package level", s.path)
						}
						return true
					})
				}
				continue
			}
			uses, loads, loadsAds, cvsSQL := false, false, false, false
			ast.Inspect(fn, func(n ast.Node) bool {
				switch x := n.(type) {
				case *ast.Ident:
					if x.Name == "SecretClaim" {
						uses = true
					}
					// taiwan-cvs-logistics-v1 §7.4, ruling B17 (E1: Finish takes the SecretClaim): the ECPay route's two
					// lease-fenced statements are package constants, not literals in the function.
					if x.Name == "loadSQL" || x.Name == "finishSQL" {
						cvsSQL = true
					}
				case *ast.BasicLit:
					if x.Kind == token.STRING && strings.Contains(x.Value, "load_meta_page_token") {
						loads = true
					}
					// meta-ads-v1 AD11: the ads loader is the one other lease-fenced LoadSecret hook.
					if x.Kind == token.STRING && strings.Contains(x.Value, "integration.load_meta_ads_token") {
						loadsAds = true
					}
				}
				return true
			})
			if !uses {
				continue
			}
			mentions++
			// ads-capi C2 (meta-ads-v1 §6.4): the CAPI route's LoadSecret is the third lease-fenced loader; it calls the same
			// integration.load_meta_ads_token and reads ads.capi_user_data in that fenced transaction.
			if !mciSecretClaimAllowed(s, fn, loads, loadsAds, cvsSQL) {
				t.Errorf("%s: function %s references SecretClaim but is not the load_meta_page_token loader in metareply, the load_meta_ads_token loader in meta_ads or attribution/capiroute, or the load_cvs_create/finish_cvs_create route in ecpayroute", s.path, fn.Name.Name)
			}
		}
	}
	if mentions == 0 {
		t.Error("no LoadSecret loader referencing core.SecretClaim was found in metareply")
	}
	// Inside core the claim may appear only in the dispatcher hook and the type's own file.
	for _, s := range srcs {
		if s.dir != "internal/integrations/core" || !strings.Contains(s.text, "SecretClaim") {
			continue
		}
		if base := filepath.Base(s.path); base != "dispatcher.go" && base != "secret.go" {
			t.Errorf("%s mentions SecretClaim; only dispatcher.go and secret.go may", s.path)
		}
	}
}

// Shared by the repository scan and the adversarial fixtures, so a helper-only
// rejection cannot be bypassed by a wider branch of the composed allowlist.
func mciSecretClaimAllowed(s mciSrc, fn *ast.FuncDecl, loads, loadsAds, cvsSQL bool) bool {
	// meta-ads-v1 F24/§5: this Finish passes the fence to an ads-owned projection,
	// never a token loader. Check it before generic loader exceptions.
	if fn.Name.Name == "FinishRefusal" {
		return mciAdsRefusalFinish(s, fn)
	}
	if fn.Name.Name == "finishAudience" || fn.Name.Name == "finishLiveVideos" {
		return mciFencedProjection(s, fn)
	}
	if fn.Name.Name == "pageSecretLoader" {
		return mciPageLoaderBody(s, fn)
	}
	return (s.path == "internal/integrations/metareply/routes.go" && fn.Name.Name == "loadSecretFor" && loads) ||
		(s.path == "internal/integrations/meta_ads/routes.go" && fn.Name.Name == "LoadSecret" && loadsAds) ||
		(s.path == "internal/attribution/capiroute/route.go" && fn.Name.Name == "loadSecret" && loadsAds) ||
		// R2 CVS: only the contracted fenced loader/finisher and their dispatcher
		// hooks; the original statement and package-scope restrictions are unchanged.
		(s.dir == "internal/integrations/shipping/ecpay/ecpayroute" && (cvsSQL || fn.Name.Name == "loadSecret" || fn.Name.Name == "finish") &&
			strings.Contains(s.text, "FROM integration.load_cvs_create(") && strings.Contains(s.text, "SELECT integration.finish_cvs_create(") &&
			strings.Count(s.text, "integration.load_") == strings.Count(s.text, "integration.load_cvs_create"))
}

// The exception is an actual Exec call in one file/function, not a SQL substring
// in a comment or unused constant. Ordinary callbacks and aliases remain forbidden.
func mciAdsRefusalFinish(s mciSrc, fn *ast.FuncDecl) bool {
	return mciFencedProjection(s, fn)
}

func TestMetaClaimsMCI10AdsRefusalFinishException(t *testing.T) {
	const path = "internal/integrations/meta_ads/finish.go"
	body, err := os.ReadFile(filepath.Join("../..", path))
	if err != nil {
		t.Fatal(err)
	}
	source := string(body)
	forgedCallback := strings.Replace(strings.Replace(source, "func FinishRefusal(", "func Dispatch(", 1), "return err", "_ = `integration.load_meta_ads_token`\nreturn err", 1)
	const statement = "_, err := tx.Exec(ctx, `SELECT ads.finish_operation_refusal($1::uuid,$2::bigint,$3::bytea,$4::text,$5::text,$6::text,$7::text)`,\n\t\tclaim.OperationID, claim.Generation, claim.LeaseToken, claim.Mode, out.State, out.Code, message)"
	if !strings.Contains(source, statement) {
		t.Fatal("fixture mutation anchor absent")
	}
	for _, tc := range []struct {
		name, path, source string
		allowed            bool
	}{
		{"contracted Finish", path, source, true},
		{"moved file", "internal/integrations/meta_ads/other.go", source, false},
		{"renamed function", path, strings.Replace(source, "func FinishRefusal(", "func FinishOther(", 1), false},
		{"Check callback", path, strings.Replace(source, "func FinishRefusal(", "func Check(", 1), false},
		{"Dispatch callback", path, strings.Replace(source, "func FinishRefusal(", "func Dispatch(", 1), false},
		{"Reconcile callback", path, strings.Replace(source, "func FinishRefusal(", "func Reconcile(", 1), false},
		{"alias", path, strings.Replace(source, "claim core.SecretClaim", "claim ClaimAlias", 1), false},
		{"loader substituted", path, strings.Replace(source, "SELECT ads.finish_operation_refusal", "SELECT integration.load_meta_ads_token", 1), false},
		{"loader added", path, strings.Replace(source, "return err", "_, _ = tx.Query(ctx, `SELECT integration.load_meta_ads_token()`)\nreturn err", 1), false},
		{"comment only", path, strings.Replace(source, statement, "// SELECT ads.finish_operation_refusal()\nvar err error", 1), false},
		{"unused literal only", path, strings.Replace(source, statement, "_ = `SELECT ads.finish_operation_refusal()`\nvar err error", 1), false},
		{"wrong fence", path, strings.Replace(source, "claim.LeaseToken, claim.Mode", "claim.Mode, claim.LeaseToken", 1), false},
		{"claim escapes", path, strings.Replace(source, "return err", "out.Detail = claim\nreturn err", 1), false},
		{"renamed callback with unused loader literal", path, forgedCallback, false},
		{"moved callback with unused loader literal", "internal/attribution/capiroute/route.go", forgedCallback, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			file, err := parser.ParseFile(token.NewFileSet(), tc.path, tc.source, parser.ParseComments)
			if err != nil {
				t.Fatal(err)
			}
			var fn *ast.FuncDecl
			for _, decl := range file.Decls {
				if f, ok := decl.(*ast.FuncDecl); ok {
					fn = f
				}
			}
			if fn == nil {
				t.Fatal("fixture has no function")
			}
			loadsAds := false
			ast.Inspect(fn, func(n ast.Node) bool {
				if lit, ok := n.(*ast.BasicLit); ok && lit.Kind == token.STRING && strings.Contains(lit.Value, "integration.load_meta_ads_token") {
					loadsAds = true
				}
				return true
			})
			if got := mciSecretClaimAllowed(mciSrc{path: tc.path, dir: filepath.ToSlash(filepath.Dir(tc.path)), file: file, text: tc.source}, fn, false, loadsAds, false); got != tc.allowed {
				t.Fatalf("exception allowed=%v, want %v", got, tc.allowed)
			}
		})
	}
}

// TestMetaClaimsMCI10ClaimsImports: internal/claims keeps the import set of live-keyword-claims-v1
// §4 (command, platform, buyer, storefront, pagination, grammar, pgx, stdlib). It never imports
// inventory, checkout, integrations/meta, metareply, claimsintake or River.
func TestMetaClaimsMCI10ClaimsImports(t *testing.T) {
	allowed := map[string]bool{"livecommerce/internal/command": true, "livecommerce/internal/platform": true, "livecommerce/internal/buyer": true,
		"livecommerce/internal/storefront": true, "livecommerce/internal/pagination": true, "livecommerce/internal/claims/grammar": true}
	for _, s := range mciSources(t, "internal/claims") {
		for imp := range mciImports(s.file) {
			first := strings.SplitN(imp, "/", 2)[0]
			switch {
			case !strings.Contains(first, "."):
				// standard library
			case strings.HasPrefix(imp, "github.com/jackc/pgx/v5"):
			case allowed[imp]:
			default:
				t.Errorf("%s imports %s, outside the claims §4 allow-list", s.path, imp)
			}
		}
	}
}

// TestMetaClaimsMCI10NoRiverKindsAndOneInsertPath: claims and the reply/intake code own no River job
// kind (claims §3, §4.4 clause 2) and reach River only through core.InsertOperationJob, so the main
// river default queue only ever receives external_operation_v1 from them (§5.4).
func TestMetaClaimsMCI10NoRiverKindsAndOneInsertPath(t *testing.T) {
	srcs := mciSources(t, "internal", "cmd")
	inserts := map[string]bool{"Insert": true, "InsertTx": true, "InsertMany": true, "InsertManyTx": true, "InsertManyFast": true, "InsertManyFastTx": true}
	scoped := func(dir string) bool {
		return dir == "internal/claims" || dir == "internal/claimsintake" || dir == "internal/integrations/metareply" || dir == "cmd/claims-worker" || dir == "cmd/meta-admin"
	}
	operationInserters := 0
	for _, s := range srcs {
		consumer := s.path == "internal/integrations/meta/consumer.go" || s.path == "internal/integrations/meta/claim_intake.go"
		if !scoped(s.dir) && !consumer {
			continue
		}
		for _, decl := range s.file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}
			// meta-connection-health-v1 §4.1/§10 sanctions exactly one scoped River kind: the claims-worker's own periodic
			// health sweep (meta_health_sweep_v1, args {}) in metareply/probe.go. It is neither a claims nor an intake job,
			// is enqueued by River's periodic enqueuer on the worker pool, never by the commerce_claims_intake login the
			// §5.4 guard confines, and carries no operation.
			healthSweep := s.path == "internal/integrations/metareply/probe.go" && fn.Name.Name == "Kind" && fn.Recv != nil &&
				len(fn.Recv.List) == 1 && exprText(fn.Recv.List[0].Type) == "ProbeJobArgs"
			if fn.Name.Name == "Kind" && fn.Recv != nil && fn.Type.Params.NumFields() == 0 && !consumer && !healthSweep {
				t.Errorf("%s defines a River job kind (%s.Kind); claims and the intake worker own none", s.path, s.dir)
			}
			if fn.Body == nil {
				continue
			}
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				sel, ok := call.Fun.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				if inserts[sel.Sel.Name] && strings.Contains(strings.ToLower(exprText(sel.X)), "river") || inserts[sel.Sel.Name] && strings.Contains(strings.ToLower(exprText(sel.X)), "jobs") {
					t.Errorf("%s: direct River insert %s.%s; use core.InsertOperationJob", s.path, exprText(sel.X), sel.Sel.Name)
				}
				if sel.Sel.Name == "InsertOperationJob" {
					operationInserters++
				}
				return true
			})
		}
	}
	if operationInserters < 1 {
		t.Error("no call to core.InsertOperationJob found in the intake worker")
	}
}

func exprText(e ast.Expr) string {
	switch x := e.(type) {
	case *ast.Ident:
		return x.Name
	case *ast.SelectorExpr:
		return exprText(x.X) + "." + x.Sel.Name
	case *ast.CallExpr:
		return exprText(x.Fun) + "()"
	case *ast.StarExpr:
		return "*" + exprText(x.X)
	}
	return ""
}

// TestMetaClaimsMCI10SecretEnvOwnership: each secret is read by the process the contract names and
// no other. K_actor belongs to the meta-worker consumer, K_link and the Page-token keyring to the
// claims-worker (and the registrar CLI for the keyring; K_actor also the retention-admin operator CLI); the claims-worker never reads the Meta
// payload keyring, K_actor or Stripe keys.
func TestMetaClaimsMCI10SecretEnvOwnership(t *testing.T) {
	srcs := mciSources(t, "internal", "cmd")
	allow := map[string][]string{
		// claims-retention-purge-v1 §6 clause 4 (IR-2): plus the operator CLI cmd/retention-admin (never a service).
		"COMMERCE_CLAIMS_ACTOR_KEY":      {"cmd/meta-worker/", "internal/integrations/meta/", "cmd/retention-admin/"},
		"COMMERCE_CLAIMS_REPLY_LINK_KEY": {"cmd/claims-worker/", "internal/claims/", "internal/claimsintake/"},
		"COMMERCE_META_PAGE_TOKEN_":      {"cmd/claims-worker/", "cmd/meta-admin/", "internal/integrations/metareply/"},
		// meta-page-token-v2 (merchant connect): the HPKE PRIVATE ring belongs to claims-worker and its opener only; cmd/api holds the
		// public ring (COMMERCE_META_PAGE_HPKE_PUBLIC_KEYS_JSON, a different name) and TestMetaConnectAPIHoldsNoPagePrivateKey checks it.
		"COMMERCE_META_PAGE_HPKE_PRIVATE_KEYS": {"cmd/claims-worker/", "internal/integrations/meta/pagetoken/pageopen/"},
	}
	for _, s := range srcs {
		for env, owners := range allow {
			if !strings.Contains(s.text, env) {
				continue
			}
			ok := false
			for _, prefix := range owners {
				ok = ok || strings.HasPrefix(s.path, prefix)
			}
			if !ok {
				t.Errorf("%s references %s, which only %v may read", s.path, env, owners)
			}
		}
		if strings.HasPrefix(s.path, "cmd/claims-worker/") || strings.HasPrefix(s.path, "internal/claimsintake/") || strings.HasPrefix(s.path, "internal/integrations/metareply/") {
			for _, forbidden := range []string{"COMMERCE_CLAIMS_ACTOR_KEY", "COMMERCE_META_PAYLOAD_", "STRIPE_"} {
				if strings.Contains(s.text, forbidden) {
					t.Errorf("%s references %s; the reply/intake process must not read it", s.path, forbidden)
				}
			}
		}
	}
}

// TestMetaClaimsMCI10NoNewThirdPartyModule: the new packages may import only third-party modules the
// rest of the code already uses (contract: no new dependency; go.mod is integrator-owned).
func TestMetaClaimsMCI10NoNewThirdPartyModule(t *testing.T) {
	srcs := mciSources(t, "internal", "cmd")
	newDir := func(dir string) bool {
		return dir == "internal/claimsintake" || dir == "internal/integrations/metareply" || dir == "cmd/claims-worker" || dir == "cmd/meta-admin"
	}
	used := map[string]bool{}
	for _, s := range srcs {
		if newDir(s.dir) {
			continue
		}
		for imp := range mciImports(s.file) {
			if first := strings.SplitN(imp, "/", 2)[0]; strings.Contains(first, ".") {
				used[imp] = true
			}
		}
	}
	for _, s := range srcs {
		if !newDir(s.dir) {
			continue
		}
		for imp := range mciImports(s.file) {
			if first := strings.SplitN(imp, "/", 2)[0]; strings.Contains(first, ".") && !used[imp] {
				t.Errorf("%s imports %s, which no pre-existing package imports (a new dependency needs an integrator-owned go.mod line)", s.path, imp)
			}
		}
	}
}

// TestMetaClaimsMCI10PackageComments: PROCESS.md §5 for the new packages: a package comment with the
// single responsibility ("owns") and non-goals ("never"). Internal depends-on / used-by lists are no
// longer hand-written: PROCESS.md §5 was amended 2026-09-29 (docs/engineering/dependency-map.md is generated
// from go list), R1 ruling F10.
func TestMetaClaimsMCI10PackageComments(t *testing.T) {
	srcs := mciSources(t, "internal/claimsintake", "internal/integrations/metareply", "cmd/claims-worker", "cmd/meta-admin")
	docs := map[string]string{}
	for _, s := range srcs {
		if s.file.Doc != nil {
			docs[s.dir] += s.file.Doc.Text() + "\n"
		}
	}
	for _, dir := range []string{"internal/claimsintake", "internal/integrations/metareply", "cmd/claims-worker", "cmd/meta-admin"} {
		doc := docs[dir]
		if strings.TrimSpace(doc) == "" {
			t.Errorf("%s has no package comment (PROCESS.md §5)", dir)
			continue
		}
		if strings.HasPrefix(dir, "internal/") {
			for _, want := range []string{" owns ", "never"} {
				if !strings.Contains(doc, want) {
					t.Errorf("%s package comment lacks %q (PROCESS.md §5 items 1-2)", dir, strings.TrimSpace(want))
				}
			}
		}
	}
	// External wire constants carry their docs URL (PROCESS.md §5).
	for _, s := range srcs {
		if s.dir == "internal/integrations/metareply" && strings.Contains(s.text, "graph.facebook.com") && !strings.Contains(s.text, "developers.facebook.com") {
			t.Errorf("%s names graph.facebook.com without its developers.facebook.com docs URL and retrieval date", s.path)
		}
	}
}
