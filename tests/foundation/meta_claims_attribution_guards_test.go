package foundation_test

// 0113 / meta-ads-v1 §4.4 D9 extends the existing MCI10 fence contract, not
// an exemption for an integration package. Every SQL call and callsite is exact.
import (
	"bytes"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func mciExpr(n ast.Node) string {
	var b bytes.Buffer
	if format.Node(&b, token.NewFileSet(), n) != nil {
		return ""
	}
	return b.String()
}

func mciParents(root ast.Node) map[ast.Node]ast.Node {
	parents := map[ast.Node]ast.Node{}
	var stack []ast.Node
	ast.Inspect(root, func(n ast.Node) bool {
		if n == nil {
			stack = stack[:len(stack)-1]
			return true
		}
		if len(stack) != 0 {
			parents[n] = stack[len(stack)-1]
		}
		stack = append(stack, n)
		return true
	})
	return parents
}

func mciLiteral(e ast.Expr) string {
	lit, ok := e.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return ""
	}
	s, err := strconv.Unquote(lit.Value)
	if err != nil {
		return ""
	}
	return strings.Join(strings.Fields(s), " ")
}

func mciDirectAssignment(parents map[ast.Node]ast.Node, call *ast.CallExpr) bool {
	a, ok := parents[call].(*ast.AssignStmt)
	return ok && len(a.Rhs) == 1 && a.Rhs[0] == call
}

func mciFencedProjection(s mciSrc, fn *ast.FuncDecl) bool {
	if fn.Recv != nil || fn.Body == nil || mciExpr(fn.Type) != "func(ctx context.Context, tx pgx.Tx, claim core.SecretClaim, out core.Outcome) error" {
		return false
	}
	sqls := map[string][]string{}
	pure := map[string]bool{}
	switch {
	case s.path == "internal/integrations/meta_ads/finish.go" && fn.Name.Name == "FinishRefusal":
		sqls["SELECT ads.finish_operation_refusal($1::uuid,$2::bigint,$3::bytea,$4::text,$5::text,$6::text,$7::text)"] = []string{"out.State", "out.Code", "message"}
		sqls["SELECT ads.finish_insights_breakdowns($1::uuid,$2::bigint,$3::bytea,$4::text,$5::jsonb)"] = []string{"raw"}
		pure = adsSet("graphCodePattern.MatchString(out.Code)", "ParseInsights(out.ProviderReference)", "json.Marshal(breakdowns)")
	case s.path == "internal/integrations/metareply/audience.go" && fn.Name.Name == "finishAudience":
		sqls["SELECT integration.finish_meta_audience($1::uuid,$2::bigint,$3::bytea,$4::text,$5::jsonb)"] = []string{"raw"}
		pure = adsSet("json.Marshal(snapshot)")
	case s.path == "internal/integrations/metareply/live_videos.go" && fn.Name.Name == "finishLiveVideos":
		// 0118 (A5-3): the live-videos Finish hook, same fenced-projection shape as finishAudience (one lease-fenced
		// definer call in the completion transaction); its other calls are pure JSON building.
		sqls["SELECT integration.finish_meta_live_videos($1::uuid,$2::bigint,$3::bytea,$4::text,$5::jsonb)"] = []string{"raw"}
		pure = adsSet("json.Marshal(items)", "json.Marshal(result)", `json.RawMessage("[]")`)
	default:
		return false
	}
	parents, seen, refs, valid := mciParents(fn), map[string]int{}, map[string]int{}, true
	ast.Inspect(fn, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.FuncLit, *ast.GoStmt, *ast.DeferStmt:
			valid = false // completion is synchronous inside the dispatcher transaction
		case *ast.Ident:
			refs[x.Name]++
		case *ast.CallExpr:
			if pure[mciExpr(x)] {
				break
			}
			if mciExpr(x.Fun) != "tx.Exec" || len(x.Args) < 2 || !mciDirectAssignment(parents, x) || x.Ellipsis.IsValid() {
				valid = false
				break
			}
			sql := mciLiteral(x.Args[1])
			tail, ok := sqls[sql]
			if !ok || len(x.Args) != 6+len(tail) {
				valid = false
				break
			}
			seen[sql]++
			args := append([]string{"claim.OperationID", "claim.Generation", "claim.LeaseToken", "claim.Mode"}, tail...)
			if mciExpr(x.Args[0]) != "ctx" {
				valid = false
			}
			for i, arg := range args {
				if mciExpr(x.Args[i+2]) != arg {
					valid = false
				}
			}
		}
		return true
	})
	for sql := range sqls {
		valid = valid && seen[sql] == 1
	}
	// Parameter plus the exact validated call arguments: rebinding/shadow/escape
	// introduces an extra identifier occurrence and is rejected, not just renamed.
	return valid && refs["claim"] == 1+4*len(sqls) && refs["tx"] == 1+len(sqls) && refs["ctx"] == 1+len(sqls)
}

func mciPageLoaderBody(s mciSrc, fn *ast.FuncDecl) bool {
	if s.path != "internal/integrations/metareply/routes.go" || fn.Name.Name != "pageSecretLoader" || fn.Recv != nil || fn.Body == nil || len(fn.Body.List) != 1 ||
		mciExpr(fn.Type) != "func(keys *PageTokenKeyring, v2 *pageopen.Keyring, provider string, scopes []string, query string) func(context.Context, pgx.Tx, core.SecretClaim) (core.Secret, error)" {
		return false
	}
	ret, ok := fn.Body.List[0].(*ast.ReturnStmt)
	if !ok || len(ret.Results) != 1 {
		return false
	}
	closure, ok := ret.Results[0].(*ast.FuncLit)
	if !ok || mciExpr(closure.Type) != "func(ctx context.Context, tx pgx.Tx, claim core.SecretClaim) (core.Secret, error)" {
		return false
	}
	parents, refs, queries, valid := mciParents(fn), map[string]int{}, 0, true
	ast.Inspect(fn, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.GoStmt, *ast.DeferStmt:
			valid = false
		case *ast.FuncLit:
			valid = valid && x == closure
		case *ast.Ident:
			refs[x.Name]++
		case *ast.CallExpr:
			if mciExpr(x.Fun) == "tx.QueryRow" {
				queries++
				valid = valid && mciExpr(x) == "tx.QueryRow(ctx, query, claim.OperationID, claim.Generation, claim.LeaseToken)"
				sel, ok := parents[x].(*ast.SelectorExpr)
				if !ok || sel.Sel.Name != "Scan" {
					valid = false
					break
				}
				scan, ok := parents[sel].(*ast.CallExpr)
				valid = valid && ok && mciDirectAssignment(parents, scan)
			} else if sel, ok := x.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "Scan" {
				q, ok := sel.X.(*ast.CallExpr)
				valid = valid && ok && mciExpr(q.Fun) == "tx.QueryRow"
			} else {
				allowed := adsSet("errors.Is", "errors.New", "fmt.Errorf", "hasScopes", "openPageToken")
				valid = valid && allowed[mciExpr(x.Fun)]
			}
		}
		return true
	})
	return valid && queries == 1 && refs["claim"] == 4 && refs["query"] == 2 && refs["tx"] == 2 && refs["ctx"] == 2
}

func mciPageLoaderReferences(srcs []mciSrc) bool {
	decls, calls, valid := 0, 0, true
	owners := map[string]int{}
	for _, s := range srcs {
		parents := mciParents(s.file)
		ast.Inspect(s.file, func(n ast.Node) bool {
			id, ok := n.(*ast.Ident)
			if !ok || id.Name != "pageSecretLoader" {
				return true
			}
			if fn, ok := parents[id].(*ast.FuncDecl); ok && fn.Name == id {
				decls++
				valid = valid && mciPageLoaderBody(s, fn)
				return true
			}
			call, ok := parents[id].(*ast.CallExpr)
			if !ok || call.Fun != id || call.Ellipsis.IsValid() || len(call.Args) != 5 {
				valid = false // aliases, stored callbacks, other fields and arbitrary queries
				return true
			}
			calls++
			var owner *ast.FuncDecl
			for p := parents[call]; p != nil; p = parents[p] {
				switch p.(type) {
				case *ast.FuncLit, *ast.GoStmt, *ast.DeferStmt:
					valid = false
				}
				if f, ok := p.(*ast.FuncDecl); ok {
					owner = f
					break
				}
			}
			if owner == nil {
				valid = false
				return true
			}
			owners[s.path+":"+owner.Name.Name]++
			var args []string
			var loader string
			switch {
			case s.path == "internal/integrations/metareply/routes.go" && owner.Name.Name == "loadSecretFor":
				r, ok := parents[call].(*ast.ReturnStmt)
				valid = valid && ok && len(r.Results) == 1 && r.Results[0] == call && len(owner.Body.List) == 1 && parents[r] == owner.Body
				args, loader = []string{"a.keys", "a.v2", "provider", "requiredScopes[provider]"}, "integration.load_meta_page_token"
			case s.path == "internal/integrations/metareply/send_dm.go" && owner.Name.Name == "loadSecretFor":
				// LC-B4 (live-console-v1 §4.3): the manual-send routes' constant LoadSecret callsite is the same lease-fenced Page-token loader as
				// routes.go; the dispatch-copy attachment (inbox.load_send_secret) wraps it OUTSIDE this function (withDispatchCopy).
				r, ok := parents[call].(*ast.ReturnStmt)
				valid = valid && ok && len(r.Results) == 1 && r.Results[0] == call && len(owner.Body.List) == 1 && parents[r] == owner.Body
				args, loader = []string{"a.keys", "a.v2", "provider", "scopes"}, "integration.load_meta_page_token"
			case (s.path == "internal/integrations/metareply/audience.go" && owner.Name.Name == "newAudienceRoute") ||
				(s.path == "internal/integrations/metareply/live_videos.go" && owner.Name.Name == "newLiveVideoRoute"):
				// 0118 (A5-3, live-console-v1 §6.2) adds the live-videos route as the third constant LoadSecret callsite, the same
				// return-DispatchRoute shape as the audience route; only its loader, scope and result differ.
				kv, ok := parents[call].(*ast.KeyValueExpr)
				if !ok || mciExpr(kv.Key) != "LoadSecret" || kv.Value != call {
					valid = false
					break
				}
				lit, ok := parents[kv].(*ast.CompositeLit)
				if !ok || mciExpr(lit.Type) != "core.DispatchRoute" {
					valid = false
					break
				}
				r, ok := parents[lit].(*ast.ReturnStmt)
				valid = valid && ok && len(r.Results) == 2 && r.Results[0] == lit && parents[r] == owner.Body
				if owner.Name.Name == "newLiveVideoRoute" {
					args, loader = []string{"keys", "v2", `"facebook"`, `[]string{"pages_read_engagement"}`}, "integration.load_meta_live_videos_token"
				} else {
					args, loader = []string{"keys", "v2", `"facebook"`, `[]string{"read_insights", "pages_read_engagement"}`}, "integration.load_meta_audience_token"
				}
			default:
				valid = false
			}
			for i, arg := range args {
				valid = valid && mciExpr(call.Args[i]) == arg
			}
			want := "SELECT tenant_id::text,store_id::text,binding_id::text,provider,asset_id,version,key_id,nonce,ciphertext,scopes_attested FROM " + loader + "($1::uuid,$2::bigint,$3::bytea)"
			valid = valid && loader != "" && mciLiteral(call.Args[4]) == want
			return true
		})
	}
	return valid && decls == 1 && calls == 4 &&
		owners["internal/integrations/metareply/routes.go:loadSecretFor"] == 1 &&
		owners["internal/integrations/metareply/send_dm.go:loadSecretFor"] == 1 &&
		owners["internal/integrations/metareply/audience.go:newAudienceRoute"] == 1 &&
		owners["internal/integrations/metareply/live_videos.go:newLiveVideoRoute"] == 1
}

func mciMutatedSource(t *testing.T, path, source string) mciSrc {
	t.Helper()
	f, err := parser.ParseFile(token.NewFileSet(), path, source, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}
	return mciSrc{path: path, dir: filepath.ToSlash(filepath.Dir(path)), file: f, text: source}
}

func mciReplace(t *testing.T, source, from, to string) string {
	t.Helper()
	if !strings.Contains(source, from) {
		t.Fatalf("mutation anchor absent: %s", from)
	}
	return strings.Replace(source, from, to, 1)
}

func TestMetaClaimsMCI10AttributionFinishNegatives(t *testing.T) {
	const path = "internal/integrations/metareply/audience.go"
	raw, err := os.ReadFile(filepath.Join("../..", path))
	if err != nil {
		t.Fatal(err)
	}
	s := mciMutatedSource(t, path, string(raw))
	var source string
	for _, d := range s.file.Decls {
		if fn, ok := d.(*ast.FuncDecl); ok && fn.Name.Name == "finishAudience" {
			source = "package metareply\n" + mciExpr(fn)
		}
	}
	if source == "" {
		t.Fatal("missing contracted Finish")
	}
	for _, tc := range []struct{ name, from, to string }{
		{"contracted", "", ""},
		{"arbitrary query", "integration.finish_meta_audience", "integration.load_meta_page_token"},
		{"wrong fence", "claim.Generation, claim.LeaseToken", "claim.LeaseToken, claim.Generation"},
		{"claim escape", "return err", "out.Detail = claim; return err"},
		{"tx rebind", "raw, err :=", "tx = other; raw, err :="},
		{"tx shadow", "raw, err :=", "tx := tx; raw, err :="},
		{"ctx rebind", "raw, err :=", "ctx = other; raw, err :="},
		{"extra SQL", "return err", "_, _ = tx.Exec(ctx, `SELECT 1`); return err"},
		{"async SQL", "_, err = tx.Exec", "go tx.Exec"},
		{"deferred SQL", "_, err = tx.Exec", "defer tx.Exec"},
		{"extra closure", "return err", "func() { _ = claim }(); return err"},
		{"claim marshalled", "json.Marshal(snapshot)", "json.Marshal(claim)"},
		{"renamed callback", "func finishAudience(", "func Dispatch("},
	} {
		t.Run(tc.name, func(t *testing.T) {
			text := source
			if tc.from != "" {
				text = mciReplace(t, text, tc.from, tc.to)
			}
			s := mciMutatedSource(t, path, text)
			fn := s.file.Decls[0].(*ast.FuncDecl)
			// loads=true adversarially: a stray loader literal cannot widen a callback.
			if got := mciSecretClaimAllowed(s, fn, true, true, false); got != (tc.name == "contracted") {
				t.Fatalf("allowed=%v for %s", got, tc.name)
			}
		})
	}
}

func TestMetaClaimsMCI10SharedPageLoaderNegatives(t *testing.T) {
	const routes = "internal/integrations/metareply/routes.go"
	const audience = "internal/integrations/metareply/audience.go"
	// 0118 (A5-3): the live-videos route is the third contracted constant callsite, so it joins the parsed source set.
	const liveVideos = "internal/integrations/metareply/live_videos.go"
	// LC-B4: the manual-send routes' constant callsite (send_dm.go loadSecretFor) is the fourth.
	const sendRoutes = "internal/integrations/metareply/send_dm.go"
	texts := map[string]string{}
	for _, path := range []string{routes, audience, liveVideos, sendRoutes} {
		raw, err := os.ReadFile(filepath.Join("../..", path))
		if err != nil {
			t.Fatal(err)
		}
		texts[path] = string(raw)
	}
	for _, tc := range []struct{ name, path, from, to string }{
		{"contracted", routes, "", ""},
		{"arbitrary query", routes, "tx.QueryRow(ctx, query,", "tx.QueryRow(ctx, `SELECT 1`,"},
		{"query rebind", routes, "var row struct", "query = `SELECT 1`; var row struct"},
		{"tx rebind", routes, "var row struct", "tx = other; var row struct"},
		{"tx shadow", routes, "var row struct", "tx := tx; var row struct"},
		{"claim escapes", routes, "var row struct", "sink(claim); var row struct"},
		{"wrong fence", routes, "claim.OperationID, claim.Generation, claim.LeaseToken", "claim.OperationID, claim.LeaseToken, claim.Generation"},
		{"extra SQL", routes, "var row struct", "_, _ = tx.Exec(ctx, `SELECT 1`); var row struct"},
		{"async SQL", routes, "var row struct", "go tx.Exec(ctx, `SELECT 1`); var row struct"},
		{"extra closure", routes, "var row struct", "func() { sink(claim) }(); var row struct"},
		{"aliased loader", routes, "// pageSecretLoader shares", "var escaped = pageSecretLoader\n// pageSecretLoader shares"},
		{"extra call", routes, "// pageSecretLoader shares", "func bad() { pageSecretLoader(nil,nil,`facebook`,nil,`SELECT 1`) }\n// pageSecretLoader shares"},
		{"package level call", routes, "// pageSecretLoader shares", "var bad = pageSecretLoader(nil,nil,`facebook`,nil,`SELECT 1`)\n// pageSecretLoader shares"},
		{"wrong audience SQL", audience, "FROM integration.load_meta_audience_token(", "FROM integration.load_meta_page_token("},
		{"wrong live-videos SQL", liveVideos, "FROM integration.load_meta_live_videos_token(", "FROM integration.load_meta_page_token("},
		{"wrong live-videos scopes", liveVideos, `[]string{"pages_read_engagement"}`, `[]string{"pages_messaging"}`},
		{"wrong route field", audience, "LoadSecret: pageSecretLoader", "Dispatch: pageSecretLoader"},
		{"wrong scopes", audience, `[]string{"read_insights", "pages_read_engagement"}`, `[]string{"pages_messaging"}`},
		{"unused audience call", audience, "return core.DispatchRoute{\n\t\tProvider", "_ = pageSecretLoader; return core.DispatchRoute{\n\t\tProvider"},
		{"two audience calls replace private reply", audience, "", ""},
		{"nested returning closure", audience, "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var srcs []mciSrc
			for _, path := range []string{routes, audience, liveVideos, sendRoutes} {
				text := texts[path]
				if path == tc.path && tc.from != "" {
					text = mciReplace(t, text, tc.from, tc.to)
				}
				src := mciMutatedSource(t, path, text)
				if tc.name == "two audience calls replace private reply" || tc.name == "nested returning closure" {
					for _, decl := range src.file.Decls {
						fn, ok := decl.(*ast.FuncDecl)
						if !ok {
							continue
						}
						if tc.name == "two audience calls replace private reply" && fn.Name.Name == "loadSecretFor" {
							fn.Body.List = []ast.Stmt{&ast.ReturnStmt{Results: []ast.Expr{ast.NewIdent("nil")}}}
						}
						if fn.Name.Name != "newAudienceRoute" {
							continue
						}
						last := len(fn.Body.List) - 1
						ret := fn.Body.List[last].(*ast.ReturnStmt)
						if tc.name == "two audience calls replace private reply" {
							// Parse a distinct copy, so the AST parent map cannot hide one occurrence.
							copy := mciMutatedSource(t, path, "package metareply\nfunc copyRoute() {"+mciExpr(ret)+"}")
							duplicate := copy.file.Decls[0].(*ast.FuncDecl).Body.List[0]
							fn.Body.List = append(fn.Body.List[:last], &ast.IfStmt{Cond: ast.NewIdent("true"), Body: &ast.BlockStmt{List: []ast.Stmt{duplicate}}}, ret)
						} else {
							fn.Body.List[last] = &ast.ReturnStmt{Results: []ast.Expr{&ast.CallExpr{Fun: &ast.FuncLit{Type: &ast.FuncType{Params: &ast.FieldList{}, Results: fn.Type.Results}, Body: &ast.BlockStmt{List: []ast.Stmt{ret}}}}}}
						}
					}
				}
				srcs = append(srcs, src)
			}
			if got := mciPageLoaderReferences(srcs); got != (tc.name == "contracted") {
				t.Fatalf("allowed=%v for %s", got, tc.name)
			}
		})
	}
}
