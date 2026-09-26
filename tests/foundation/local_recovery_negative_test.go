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

// Canonical ACLs equate NULL with an explicitly stored default, while still
// preserving each grant, grant option and owner privilege that changes access.
func TestLocalRecoverySnapshotCanonicalACLAndDatabaseOwner(t *testing.T) {
	c, f := lrSourceFixture(t)
	tag := t04Tag()
	role := pgx.Identifier{"lr_acl_reader_" + tag}.Sanitize()
	table := pgx.Identifier{"public", "lr_acl_table_" + tag}.Sanitize()
	sequence := pgx.Identifier{"public", "lr_acl_seq_" + tag}.Sanitize()
	lrExec(t, f.owner, `CREATE ROLE `+role+` NOLOGIN`)
	lrExec(t, f.owner, `CREATE TABLE `+table+` (id bigint)`)
	lrExec(t, f.owner, `CREATE SEQUENCE `+sequence)

	for _, object := range []string{table, sequence} {
		var absent bool
		if err := lrScan(f.owner, `SELECT relacl IS NULL FROM pg_class WHERE oid=$1::regclass`, []any{&absent}, object); err != nil || !absent {
			t.Fatalf("fresh object %s did not start with NULL ACL: %v", object, err)
		}
	}
	base := lrSnapshot(t, f.owner, "")
	if base.catalog["database"] == "" {
		t.Fatal("database owner missing from raw catalog snapshot")
	}
	lrExec(t, f.owner, `GRANT SELECT ON TABLE `+table+` TO `+role)
	lrExec(t, f.owner, `REVOKE SELECT ON TABLE `+table+` FROM `+role)
	lrExec(t, f.owner, `GRANT USAGE ON SEQUENCE `+sequence+` TO `+role)
	lrExec(t, f.owner, `REVOKE USAGE ON SEQUENCE `+sequence+` FROM `+role)
	for _, object := range []string{table, sequence} {
		var absent bool
		if err := lrScan(f.owner, `SELECT relacl IS NULL FROM pg_class WHERE oid=$1::regclass`, []any{&absent}, object); err != nil || absent {
			t.Fatalf("grant/revoke did not produce explicit ACL for %s: %v", object, err)
		}
	}
	explicit := lrSnapshot(t, f.owner, "")
	if explicit.catalog["relations"] != base.catalog["relations"] {
		t.Fatal("explicit default table/sequence ACL differs from NULL default")
	}

	lrExec(t, f.owner, `GRANT USAGE ON SEQUENCE `+sequence+` TO `+role)
	seqGrant := lrSnapshot(t, f.owner, "")
	if seqGrant.catalog["relations"] == explicit.catalog["relations"] {
		t.Fatal("sequence USAGE grant disappeared from canonical ACL")
	}
	lrExec(t, f.owner, `REVOKE USAGE ON SEQUENCE `+sequence+` FROM `+role)
	if lrSnapshot(t, f.owner, "").catalog["relations"] != explicit.catalog["relations"] {
		t.Fatal("sequence USAGE revoke did not restore canonical default")
	}

	lrExec(t, f.owner, `GRANT SELECT ON TABLE `+table+` TO `+role)
	plain := lrSnapshot(t, f.owner, "")
	if plain.catalog["relations"] == explicit.catalog["relations"] {
		t.Fatal("nonowner table grant disappeared from canonical ACL")
	}
	lrExec(t, f.owner, `GRANT SELECT ON TABLE `+table+` TO `+role+` WITH GRANT OPTION`)
	grantable := lrSnapshot(t, f.owner, "")
	if grantable.catalog["relations"] == plain.catalog["relations"] {
		t.Fatal("grant option collapsed into plain SELECT")
	}
	lrExec(t, f.owner, `REVOKE SELECT ON TABLE `+table+` FROM `+role)
	lrExec(t, f.owner, `REVOKE UPDATE ON TABLE `+table+` FROM `+pgx.Identifier{c.role}.Sanitize())
	if lrSnapshot(t, f.owner, "").catalog["relations"] == explicit.catalog["relations"] {
		t.Fatal("owner privilege revoke collapsed into default ACL")
	}

	lrExec(t, f.owner, `ALTER DATABASE lc_foundation_test OWNER TO `+role)
	if lrSnapshot(t, f.owner, "").catalog["database"] == base.catalog["database"] {
		t.Fatal("database owner change disappeared from catalog")
	}
	lrExec(t, f.owner, `ALTER DATABASE lc_foundation_test OWNER TO `+pgx.Identifier{c.role}.Sanitize())
	if lrSnapshot(t, f.owner, "").catalog["database"] != base.catalog["database"] {
		t.Fatal("database owner restoration did not restore catalog")
	}
}
