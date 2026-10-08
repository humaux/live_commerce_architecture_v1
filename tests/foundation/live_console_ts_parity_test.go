// Purpose: Pass actual LC-B7 A1 merchant-router JSON unchanged into the strict TypeScript console parser.
// Depends on: live_console_read_test lcbConsole/ltgNew, disposable real PG, production router/ConsoleSnapshot and Node TS support.
// Used by: TestLiveConsoleLCN focused gate and foundation CI; synthetic data only, no Meta reads or mutation.
package foundation_test

import (
	"bytes"
	"context"
	"net/http"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// TestLiveConsoleLCN27TypeScriptActualA1 checks the real A1 wire shape; offers_null calibrates rejection without altering the API fixture.
func TestLiveConsoleLCN27TypeScriptActualA1(t *testing.T) {
	e := ltgNew(t)
	sid, _ := e.session("A1", ltgLive, 5)
	status, _, raw := lcbConsole(t, e, e.token(), sid)
	if status != http.StatusOK {
		t.Fatalf("A1 status=%d", status)
	}
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	script := `import {readFileSync} from 'node:fs'; import {parseConsole} from './apps/admin/src/features/live/console-model.ts'; const v=JSON.parse(readFileSync(0,'utf8')); if(process.env.LC_CONSOLE_A1_FAULT==='offers_null')v.offers=null; const got=parseConsole(v,process.argv[1]); if(got.session.id!==process.argv[1] || !Array.isArray(got.offers)) throw Error('A1 parity'); console.log('LC-B7 actual A1 parser PASS');`
	command := exec.CommandContext(ctx, "node", "--experimental-strip-types", "--input-type=module", "-e", script, sid)
	command.Dir = root
	command.Stdin = bytes.NewReader(raw)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("actual A1 parser: %v: %s", err, output)
	}
}
