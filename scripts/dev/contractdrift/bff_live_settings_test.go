// Purpose: verify real W3-U2 leaves and adversarial forwarding/admission edits.
// Depends on: actual repository sources and scanBFF; used by CI-DRIFT Go tests.
// Invariant: unsupported exports and sinks fail closed; fixtures use no product secrets.
package main

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func liveSettingsFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, dir := range []string{"apps", "packages"} {
		err := filepath.WalkDir("../../../"+dir, func(p string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				if d.Name() == "node_modules" || d.Name() == ".next" {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(p, ".ts") && !strings.HasSuffix(p, ".json") {
				return nil
			}
			data, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			rel, _ := filepath.Rel("../../..", p)
			bffWrite(t, root, rel, string(data))
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	return root
}
func liveSettingsKeys(inv inventory, leaf string) []string {
	var keys []string
	for _, r := range inv.Producers[leaf] {
		keys = append(keys, r.Method+" "+r.Path)
	}
	sort.Strings(keys)
	return keys
}

const liveCheckLeaf = "apps/admin/app/api/stores/[store]/live-sessions/[session]/claims/blocklist/check/route.ts"

func TestBFFLiveSettingsRealLeaves(t *testing.T) {
	inv, err := scanBFF("../../..")
	if err != nil {
		t.Fatal(err)
	}
	if len(inv.Unresolved) != 0 {
		t.Fatalf("real source unresolved: %+v", inv.Unresolved)
	}
	prefix := "apps/admin/app/api/stores/[store]/"
	for leaf, want := range map[string][]string{
		"live-settings/sold-out-reply/route.ts":                             {"GET /v1/admin/stores/{}/live-settings/sold-out-reply", "PUT /v1/admin/stores/{}/live-settings/sold-out-reply"},
		"message-templates/route.ts":                                        {"GET /v1/admin/stores/{}/message-templates", "POST /v1/admin/stores/{}/message-templates"},
		"live-sessions/[session]/reminders/route.ts":                        {"GET /v1/admin/stores/{}/live-sessions/{}/reminders", "POST /v1/admin/stores/{}/live-sessions/{}/reminders"},
		"live-sessions/[session]/claims/blocklist/route.ts":                 {"GET /v1/admin/stores/{}/live-sessions/{}/claims/blocklist", "POST /v1/admin/stores/{}/live-sessions/{}/claims/blocklist"},
		"live-sessions/[session]/claims/blocklist/check/route.ts":           {"GET /v1/admin/stores/{}/live-sessions/{}/claims/blocklist/check"},
		"live-sessions/[session]/claims/blocklist/entries/[entry]/route.ts": {"DELETE /v1/admin/stores/{}/live-sessions/{}/claims/blocklist/entries/{}"},
	} {
		// The signed-session scope check also reaches its real stores-list sink.
		want = append([]string{"GET /v1/admin/stores"}, want...)
		sort.Strings(want)
		if got := liveSettingsKeys(inv, prefix+leaf); !reflect.DeepEqual(got, want) {
			t.Errorf("%s: got %v want %v", leaf, got, want)
		}
	}
}
func TestBFFLocalExportAliases(t *testing.T) {
	root := t.TempDir()
	leaf := "apps/admin/app/api/stores/[store]/alias/route.ts"
	bffWrite(t, root, leaf, `async function send() { return fetch("https://fixture.invalid/v1/probe",{method:"POST"}); }
 function deny() { return new Response(null,{status:405}); }
 export {send as GET,send as POST,send as PUT,send as PATCH,send as DELETE,send as HEAD,deny as OPTIONS};`)
	reader := &bffReader{root: root, modules: map[string]*bffModule{}}
	m, err := reader.load(leaf)
	if err != nil {
		t.Fatal(err)
	}
	if len(bffExports(m)) != 7 {
		t.Fatalf("export aliases: %v", bffExports(m))
	}
	s := &bffScanner{reader: reader}
	for _, method := range []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD"} {
		if !s.forwardingExport(m, method, map[string]bool{}, 0) {
			t.Errorf("lost forwarding alias %s", method)
		}
	}
	if s.forwardingExport(m, "OPTIONS", map[string]bool{}, 0) {
		t.Fatal("405 alias forwards")
	}
	inv, err := scanBFF(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(inv.Unresolved) != 0 || !bffHas(inv, "POST", "/v1/probe") {
		t.Fatalf("alias forwarding: %+v", inv)
	}
}
func TestBFFLiveSettingsMutations(t *testing.T) {
	guard := `if (!route.methods.includes(request.method)) {
    const r = denied(405, "method_not_allowed");
    r.headers.set("Allow", route.methods.join(", "));
    return r;
  }`
	for _, tc := range []struct {
		name, file, old, new, want string
		unresolved                 bool
	}{
		{"nested false guard", "apps/admin/lib/live-settings-proxy.ts", guard, "if (false) {" + guard + "}", "", true},
		{"nested function guard", "apps/admin/lib/live-settings-proxy.ts", guard, "function neverCalled() {" + guard + "}", "", true},
		{"computed method override", "apps/admin/lib/live-settings-proxy.ts", "method: request.method,", "method: request.method, ['method']: 'POST',", "POST /v1/admin/stores/{}/live-sessions/{}/claims/blocklist/check", false},
		{"unknown computed key", "apps/admin/lib/live-settings-proxy.ts", "method: request.method,", "method: request.method, [chooseKey()]: 'POST',", "", true},
		{"extra sink", "apps/admin/lib/live-settings-proxy.ts", "const route = settingsResource(path);", `await fetch("https://fixture.invalid/v1/__live_extra",{method:"POST"}); const route = settingsResource(path);`, "POST /v1/__live_extra", false},
		{"changed method", "apps/admin/lib/live-settings-proxy.ts", "method: request.method,", "method: \"POST\",", "POST /v1/admin/stores/{}/live-sessions/{}/claims/blocklist/check", false},
		{"changed path", "apps/admin/lib/live-settings-proxy.ts", "/v1/admin/stores/${store}/${path}", "/v1/admin/stores/${store}/retarget/${path}", "", true},
		{"guard bypass", "apps/admin/lib/live-settings-proxy.ts", "!route.methods.includes(request.method)", "false && !route.methods.includes(request.method)", "", true},
		{"guard removed", "apps/admin/lib/live-settings-proxy.ts", "!route.methods.includes(request.method)", "false", "", true},
		{"unknown admission", "apps/admin/lib/live-settings-model.ts", "resources.find(([, re]) => re.test(path))", "chooseResource(path)", "", true},
		{"unknown leaf", liveCheckLeaf, "p.store,`live-sessions/${p.session}/claims/blocklist/check`", "p.store,choosePath(p.session)", "", true},
		{"extra passed fetch", liveCheckLeaf, "return liveSettingsProxy", "const leak=fetch; return liveSettingsProxy", "", true},
		{"405 export", liveCheckLeaf, "handle as GET", "denied as GET", "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := liveSettingsFixture(t)
			p := filepath.Join(root, tc.file)
			data, err := os.ReadFile(p)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(data), tc.old) {
				t.Fatal("mutation target missing")
			}
			bffWrite(t, root, tc.file, mustReplace(t, string(data), tc.old, tc.new))
			inv, err := scanBFF(root)
			if err != nil {
				t.Fatal(err)
			}
			if tc.unresolved {
				if len(inv.Unresolved) == 0 {
					t.Fatal("unknown source accepted")
				}
				return
			}
			keys := liveSettingsKeys(inv, liveCheckLeaf)
			found := false
			for _, k := range keys {
				if k == tc.want {
					found = true
				}
			}
			if !found {
				t.Fatalf("changed sink missing: %v; unresolved=%v", keys, inv.Unresolved)
			}
		})
	}
}
