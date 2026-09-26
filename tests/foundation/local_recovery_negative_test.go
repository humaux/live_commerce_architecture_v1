package foundation_test

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/jackc/pgx/v5"
)

// A matching backup/restore digest is useful only if the snapshot observes
// privilege and sequence changes that a broken restore could silently lose.
func TestLocalRecoverySnapshotDetectsSecurityAndSequenceDrift(t *testing.T) {
	_, f := lrSourceFixture(t)
	loginURL := bcRole(t, f, "commerce_runtime")
	parsed, err := url.Parse(loginURL)
	if err != nil || parsed.User == nil || parsed.User.Username() == "" {
		t.Fatal("test login URL missing username")
	}
	role := pgx.Identifier{parsed.User.Username()}.Sanitize()
	base := lrSnapshot(t, f.owner, "")

	// pg_read_all_data is a predefined role, not an application role. A
	// snapshot that filters all pg_ memberships would miss this escalation.
	mustExec(t, f.owner, `GRANT pg_read_all_data TO `+role)
	t.Cleanup(func() { mustExec(t, f.owner, `REVOKE pg_read_all_data FROM `+role) })
	member := lrSnapshot(t, f.owner, "")
	if member.catalog["members"] == base.catalog["members"] {
		t.Fatal("snapshot missed predefined-role membership escalation")
	}
	mustExec(t, f.owner, `REVOKE pg_read_all_data FROM `+role)

	// Column ACLs live in pg_attribute.attacl, not pg_class.relacl.
	mustExec(t, f.owner, `GRANT SELECT(version) ON public.lc_schema_migrations TO `+role)
	t.Cleanup(func() { mustExec(t, f.owner, `REVOKE SELECT(version) ON public.lc_schema_migrations FROM `+role) })
	column := lrSnapshot(t, f.owner, "")
	if column.catalog["columns"] == base.catalog["columns"] {
		t.Fatal("snapshot missed column-level SELECT grant")
	}
	mustExec(t, f.owner, `REVOKE SELECT(version) ON public.lc_schema_migrations FROM `+role)

	const sequence = "river_payment.river_job_id_seq"
	var high int64
	if err := lrScan(f.owner, `SELECT setval('river_payment.river_job_id_seq',last_value+1000,false) FROM river_payment.river_job_id_seq`, []any{&high}); err != nil {
		t.Fatal(err)
	}
	uncalled := lrSnapshot(t, f.owner, "")
	if got := uncalled.sequences[sequence]; got != fmt.Sprintf("%d/false", high) || got == base.sequences[sequence] {
		t.Fatalf("snapshot missed pruned high-water or is_called=false: %s", got)
	}
	var next int64
	if err := lrScan(f.owner, `SELECT nextval('river_payment.river_job_id_seq')`, []any{&next}); err != nil {
		t.Fatal(err)
	}
	called := lrSnapshot(t, f.owner, "")
	if next != high || called.sequences[sequence] != fmt.Sprintf("%d/true", high) || called.sequences[sequence] == uncalled.sequences[sequence] {
		t.Fatalf("snapshot missed false-to-true sequence transition: next=%d high=%d seen=%s", next, high, called.sequences[sequence])
	}
}

func TestLocalRecoveryArtifactHashDetectsByteChange(t *testing.T) {
	path := filepath.Join(t.TempDir(), "this-run.dump")
	body := []byte("synthetic native archive bytes")
	if err := os.WriteFile(path, body, 0600); err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256(body)
	want := hex.EncodeToString(hash[:])
	if !lrArtifactOK(path, want) {
		t.Fatal("unchanged artifact rejected")
	}
	body[0] ^= 1
	if err := os.WriteFile(path, body, 0600); err != nil {
		t.Fatal(err)
	}
	if lrArtifactOK(path, want) {
		t.Fatal("modified artifact accepted")
	}
}
