package foundation_test

// CT01-CT09 (contracts/customers-billing-v1.md Amendment W6-01B; unit w6-01b-customer-tags-notes; tier REAL_PG plus one
// HTTP_PG subtest over httpapi.NewHandler). Helper prefix `ct`. What it proves:
//   - CT01 create tag, tag customers, filter the list by tag with stable keyset paging, a cursor bound to its tag;
//   - CT02 a same-name tag in another letter case is tag_exists (create and rename), other stores are independent;
//   - CT03 101st tag / 21st customer tag / 201st note are limit_reached, a 1001-character note is invalid (Go and DB);
//   - CT04 set_owner_tags is compare-and-set on the revision (version_changed), unknown / foreign-store tag ids refused;
//   - CT05 viewer (no customers:write) is forbidden, another store's customer and another tenant are not found, only the
//     author or a customers:privacy holder edits or deletes a note, a stale note version is version_changed;
//   - CT06 erasure (merchant and buyer path, and restore replay) removes tag links and notes; audit and idempotency
//     receipts never hold a note body;
//   - CT07 the buyer self-service export carries tag names and note bodies (no author principal id);
//   - CT08 deleting a tag detaches it everywhere and leaves customers and notes alone;
//   - CT09 the HTTP routes work end to end with the coded errors and nothing logs a note body.
// Disclosed owner-pool fixtures: store grants of the merchant principals (lcPrincipal) and the final purge of the rows
// this test created in the shared stores A1/A2.

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"livecommerce/internal/command"
	"livecommerce/internal/customers"
	"livecommerce/internal/httpapi"
	"livecommerce/internal/pagination"
	"livecommerce/internal/platform"
)

const ctMarker = "ct-secret-note-marker"

type ctEnv struct {
	*cbcEnv
	admin, adminTok     string // customers:read/write/privacy on A1 and A2
	authorA, authorATok string // customers:read/write on A1 and A2 (no privacy)
	authorB, authorBTok string // the same, a second principal
	otherTok            string // tenant B principal with customers:write on store B
}

func ctSetup(t *testing.T) *ctEnv {
	t.Helper()
	c := &ctEnv{cbcEnv: cbcSetup(t)}
	f := c.f
	both := []string{f.storeA1, f.storeA2}
	c.admin, c.adminTok = lcPrincipal(t, f, f.tenantA, both, "store:read", "customers:read", "customers:write", "customers:privacy")
	c.authorA, c.authorATok = lcPrincipal(t, f, f.tenantA, both, "store:read", "customers:read", "customers:write")
	c.authorB, c.authorBTok = lcPrincipal(t, f, f.tenantA, both, "store:read", "customers:read", "customers:write")
	_, c.otherTok = lcPrincipal(t, f, f.tenantB, []string{f.storeB}, "store:read", "customers:read", "customers:write")
	t.Cleanup(func() {
		for _, q := range []string{`DELETE FROM customers.notes WHERE tenant_id=$1`, `DELETE FROM customers.owner_tags WHERE tenant_id=$1`,
			`DELETE FROM customers.tags WHERE tenant_id=$1`} {
			mustExec(t, f.owner, q, f.tenantA)
		}
	})
	return c
}

// run opens one merchant scope transaction (the route shape) and runs fn in it.
func (c *ctEnv) run(token, store string, fn func(ctx context.Context, tx pgx.Tx, s platform.Scope) error) error {
	ctx := context.Background()
	return platform.WithScope(ctx, c.f.runtime, token, store, "store:read", func(tx pgx.Tx, s platform.Scope) error { return fn(ctx, tx, s) })
}

func (c *ctEnv) newTag(token, store, name, color string) (rec customers.TagRecord, err error) {
	err = c.run(token, store, func(ctx context.Context, tx pgx.Tx, s platform.Scope) (e error) {
		rec, e = customers.CreateTag(ctx, tx, s, token, t04Key("ct-tag"), customers.TagInput{Name: name, Color: color})
		return e
	})
	return rec, err
}

func (c *ctEnv) mustTag(token, store, name, color string) customers.TagRecord {
	c.t.Helper()
	rec, err := c.newTag(token, store, name, color)
	if err != nil {
		c.t.Fatalf("create tag %q: %v", name, err)
	}
	return rec
}

func (c *ctEnv) detail(token, store, owner string) customers.Detail {
	c.t.Helper()
	var d customers.Detail
	err := c.run(token, store, func(ctx context.Context, tx pgx.Tx, s platform.Scope) (e error) {
		d, e = customers.Get(ctx, tx, s, token, owner)
		return e
	})
	if err != nil {
		c.t.Fatalf("detail %s: %v", owner, err)
	}
	return d
}

func (c *ctEnv) setTags(token, store, owner, revision string, ids ...string) (set customers.TagSet, err error) {
	err = c.run(token, store, func(ctx context.Context, tx pgx.Tx, s platform.Scope) (e error) {
		set, e = customers.SetOwnerTags(ctx, tx, s, token, t04Key("ct-set"), owner, customers.SetTagsInput{TagIDs: ids, Revision: revision})
		return e
	})
	return set, err
}

// tagOwner sets exactly ids on owner at its current revision.
func (c *ctEnv) tagOwner(owner string, ids ...string) {
	c.t.Helper()
	d := c.detail(c.adminTok, c.f.storeA1, owner)
	if _, err := c.setTags(c.adminTok, c.f.storeA1, owner, d.TagsRevision, ids...); err != nil {
		c.t.Fatalf("tag owner: %v", err)
	}
}

func (c *ctEnv) addNote(token, store, owner, body string) (n customers.Note, err error) {
	err = c.run(token, store, func(ctx context.Context, tx pgx.Tx, s platform.Scope) (e error) {
		n, e = customers.AddNote(ctx, tx, s, token, t04Key("ct-note"), owner, customers.NoteInput{Body: body})
		return e
	})
	return n, err
}

func (c *ctEnv) editNote(token, store, owner, note, body string, version int64) (n customers.Note, err error) {
	err = c.run(token, store, func(ctx context.Context, tx pgx.Tx, s platform.Scope) (e error) {
		n, e = customers.EditNote(ctx, tx, s, token, t04Key("ct-edit"), owner, note, customers.EditNoteInput{Body: body, Version: version})
		return e
	})
	return n, err
}

func (c *ctEnv) listByTag(token, store, tag string, limit int, cursor string) (p pagination.Page[customers.Customer], err error) {
	err = c.run(token, store, func(ctx context.Context, tx pgx.Tx, s platform.Scope) (e error) {
		p, e = customers.List(ctx, tx, s, token, customers.ListRequest{Page: pagination.Request{Limit: limit, Cursor: cursor}, Tag: tag})
		return e
	})
	return p, err
}

func (c *ctEnv) count(query string, args ...any) (n int) {
	c.t.Helper()
	if err := c.f.owner.QueryRow(context.Background(), query, args...).Scan(&n); err != nil {
		c.t.Fatal(err)
	}
	return n
}

// rawTagInsert inserts a tag straight through the owner pool to prove the table CHECKs on their own.
func (c *ctEnv) rawTagInsert(store, name string) error {
	_, err := c.f.owner.Exec(context.Background(), `INSERT INTO customers.tags(tenant_id,store_id,name,color) VALUES($1,$2,$3,'gray')`, c.f.tenantA, store, name)
	return err
}

func ctIDs(items []customers.Customer) map[string]bool {
	out := map[string]bool{}
	for _, i := range items {
		out[i.CustomerID] = true
	}
	return out
}

func TestCustomerTags(t *testing.T) {
	c := ctSetup(t)
	f := c.f
	a1, a2 := f.storeA1, f.storeA2
	custA, custB, custC := c.bundleCustomer().Scope.OwnerID, c.bundleCustomer().Scope.OwnerID, c.bundleCustomer().Scope.OwnerID

	vip := c.mustTag(c.adminTok, a1, "VIP", "red")
	loyal := c.mustTag(c.adminTok, a1, "熟客", "green")

	t.Run("CT01 create, tag and filter the list with stable paging", func(t *testing.T) {
		if vip.Name != "VIP" || vip.Color != "red" || vip.CustomersCount != 0 {
			t.Fatalf("created tag: %+v", vip)
		}
		c.tagOwner(custA, vip.ID, loyal.ID)
		c.tagOwner(custC, vip.ID)
		c.tagOwner(custB, loyal.ID)
		d := c.detail(c.adminTok, a1, custA)
		if len(d.Tags) != 2 || d.Tags[0].Name != "VIP" || d.Tags[1].Name != "熟客" || d.Tags[0].Color != "red" {
			t.Fatalf("detail tags (name ordered): %+v", d.Tags)
		}
		seen, cursor := map[string]bool{}, ""
		for page := 0; page < 5; page++ {
			p, err := c.listByTag(c.adminTok, a1, vip.ID, 1, cursor)
			if err != nil {
				t.Fatalf("filtered list: %v", err)
			}
			for _, it := range p.Items {
				if seen[it.CustomerID] {
					t.Fatalf("customer %s on two pages", it.CustomerID)
				}
				seen[it.CustomerID] = true
				if !func() bool {
					for _, tg := range it.Tags {
						if tg.ID == vip.ID {
							return true
						}
					}
					return false
				}() {
					t.Fatalf("filtered row without the tag: %+v", it.Tags)
				}
			}
			if p.NextCursor == "" {
				break
			}
			if page == 0 { // the cursor is bound to its tag
				if _, err := c.listByTag(c.adminTok, a1, loyal.ID, 1, p.NextCursor); !errors.Is(err, command.ErrInvalid) {
					t.Fatalf("cursor of tag A used for tag B: %v", err)
				}
			}
			cursor = p.NextCursor
		}
		if len(seen) != 2 || !seen[custA] || !seen[custC] || seen[custB] {
			t.Fatalf("filtered set: %v", seen)
		}
		if p, err := c.listByTag(c.adminTok, a1, randomUUID(), 10, ""); err != nil || len(p.Items) != 0 {
			t.Fatalf("unknown tag id must list nobody: %v %v", p.Items, err)
		}
		var cat []customers.TagRecord
		if err := c.run(c.adminTok, a1, func(ctx context.Context, tx pgx.Tx, s platform.Scope) (e error) {
			cat, e = customers.ListTags(ctx, tx, s, c.adminTok)
			return e
		}); err != nil || len(cat) < 2 {
			t.Fatalf("catalogue: %v %v", cat, err)
		}
		for _, r := range cat {
			if r.ID == vip.ID && r.CustomersCount != 2 {
				t.Fatalf("VIP usage count %d, want 2", r.CustomersCount)
			}
		}
		// idempotent replay: the same key and request returns the stored tag, a different request is a key conflict
		key := t04Key("ct-replay")
		mk := func(name string) (customers.TagRecord, error) {
			var rec customers.TagRecord
			err := c.run(c.adminTok, a1, func(ctx context.Context, tx pgx.Tx, s platform.Scope) (e error) {
				rec, e = customers.CreateTag(ctx, tx, s, c.adminTok, key, customers.TagInput{Name: name, Color: "blue"})
				return e
			})
			return rec, err
		}
		first, err := mk("replay")
		if err != nil {
			t.Fatal(err)
		}
		if again, err := mk("replay"); err != nil || again.ID != first.ID {
			t.Fatalf("replay: %+v %v", again, err)
		}
		if _, err := mk("replay-other"); !errors.Is(err, customers.ErrIdempotencyConflict) {
			t.Fatalf("same key, other request: %v", err)
		}
	})

	t.Run("CT02 same name in another case is tag_exists", func(t *testing.T) {
		for _, name := range []string{"vip", "Vip", "VIP"} {
			if _, err := c.newTag(c.adminTok, a1, name, "gray"); !errors.Is(err, customers.ErrTagExists) {
				t.Fatalf("create %q: %v", name, err)
			}
		}
		err := c.run(c.adminTok, a1, func(ctx context.Context, tx pgx.Tx, s platform.Scope) error {
			n := "vip"
			_, e := customers.RenameTag(ctx, tx, s, c.adminTok, t04Key("ct-ren"), loyal.ID, customers.TagPatch{Name: &n})
			return e
		})
		if !errors.Is(err, customers.ErrTagExists) {
			t.Fatalf("rename onto an existing name: %v", err)
		}
		if _, err := c.newTag(c.adminTok, a2, "VIP", "red"); err != nil {
			t.Fatalf("the same name in another store is fine: %v", err)
		}
		// recolour to itself and rename itself are allowed; the audit has the actions only
		err = c.run(c.adminTok, a1, func(ctx context.Context, tx pgx.Tx, s platform.Scope) error {
			n, col := "vip", "teal"
			r, e := customers.RenameTag(ctx, tx, s, c.adminTok, t04Key("ct-ren"), vip.ID, customers.TagPatch{Name: &n, Color: &col})
			if e == nil && (r.Name != "vip" || r.Color != "teal") {
				e = fmt.Errorf("renamed: %+v", r)
			}
			return e
		})
		if err != nil {
			t.Fatalf("rename own case: %v", err)
		}
		// compatibility spelling and case are the same tag: full-width letters fold onto ASCII (NFKC + lower)
		for _, dup := range []string{"ＶＩＰ", "ｖｉｐ"} {
			if _, err := c.newTag(c.adminTok, a1, dup, "gray"); !errors.Is(err, customers.ErrTagExists) {
				t.Fatalf("full-width %q: %v", dup, err)
			}
		}
		// a decomposed spelling is normalised to NFC by Go before it is sent; the database refuses non-NFC and invisible-format names
		if r, err := c.newTag(c.adminTok, a1, "e\u0301", "gray"); err != nil || r.Name != "\u00e9" {
			t.Fatalf("decomposed name: %+v %v", r, err)
		}
		for _, raw := range []string{"e\u0301x", "zero\u200bwidth", "\ufeffbom"} {
			if cbcCode(c.rawTagInsert(a1, raw)) != "23514" {
				t.Fatalf("DB accepted %q", raw)
			}
		}
		for _, bad := range []string{"", " x", "x ", strings.Repeat("a", 21), "a\tb", "a\u0000b", "zero\u200bwidth", "\ufeffbom"} {
			if _, err := c.newTag(c.adminTok, a1, bad, "red"); !errors.Is(err, command.ErrInvalid) {
				t.Fatalf("name %q: %v", bad, err)
			}
		}
		if _, err := c.newTag(c.adminTok, a1, "ok", "pink"); !errors.Is(err, command.ErrInvalid) {
			t.Fatalf("colour pink: %v", err)
		}
	})

	t.Run("CT03 limits: 100 tags, 20 per customer, 200 notes, note length", func(t *testing.T) {
		// 100 tags in store A2 (one exists from CT02)
		err := c.run(c.adminTok, a2, func(ctx context.Context, tx pgx.Tx, s platform.Scope) error {
			for i := 0; i < 99; i++ {
				if _, e := customers.CreateTag(ctx, tx, s, c.adminTok, t04Key("ct-bulk"), customers.TagInput{Name: fmt.Sprintf("bulk-%03d", i), Color: "gray"}); e != nil {
					return fmt.Errorf("tag %d: %w", i, e)
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := c.newTag(c.adminTok, a2, "one-too-many", "gray"); !errors.Is(err, customers.ErrLimitReached) {
			t.Fatalf("101st tag: %v", err)
		}
		mustExec(t, f.owner, `DELETE FROM customers.tags WHERE tenant_id=$1 AND store_id=$2 AND name LIKE 'bulk-%'`, f.tenantA, a2) // keep store A2 usable for later subtests
		// 21 tags on one customer (store A1 has far fewer than 100 tags)
		var ids []string
		for i := 0; i < 21; i++ {
			ids = append(ids, c.mustTag(c.adminTok, a1, fmt.Sprintf("set-%02d", i), "blue").ID)
		}
		d := c.detail(c.adminTok, a1, custB)
		if _, err := c.setTags(c.adminTok, a1, custB, d.TagsRevision, ids...); !errors.Is(err, customers.ErrLimitReached) {
			t.Fatalf("21 tags on one customer: %v", err)
		}
		if _, err := c.setTags(c.adminTok, a1, custB, d.TagsRevision, ids[:20]...); err != nil {
			t.Fatalf("20 tags on one customer: %v", err)
		}
		// note length: Go refuses, the definer refuses too
		long := strings.Repeat("字", 1001)
		if _, err := c.addNote(c.adminTok, a1, custC, long); !errors.Is(err, command.ErrInvalid) {
			t.Fatalf("1001-character note (Go): %v", err)
		}
		if _, err := c.addNote(c.adminTok, a1, custC, " \n "); !errors.Is(err, command.ErrInvalid) {
			t.Fatalf("blank note: %v", err)
		}
		err = c.run(c.adminTok, a1, func(ctx context.Context, tx pgx.Tx, s platform.Scope) error {
			h := sha256sum(c.adminTok)
			var raw []byte
			return tx.QueryRow(ctx, `SELECT customers.add_note($1,$2::uuid,$3::uuid,$4)`, h, a1, custC, long).Scan(&raw)
		})
		if cbcCode(err) != "PT422" {
			t.Fatalf("1001-character note (definer): %v", err)
		}
		if _, err := c.addNote(c.adminTok, a1, custC, strings.Repeat("字", 1000)); err != nil {
			t.Fatalf("1000-character note: %v", err)
		}
		// 200 notes, then the 201st
		err = c.run(c.adminTok, a1, func(ctx context.Context, tx pgx.Tx, s platform.Scope) error {
			for i := 0; i < 199; i++ {
				if _, e := customers.AddNote(ctx, tx, s, c.adminTok, t04Key("ct-bulk-note"), custC, customers.NoteInput{Body: fmt.Sprintf("n%d", i)}); e != nil {
					return fmt.Errorf("note %d: %w", i, e)
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := c.addNote(c.adminTok, a1, custC, "one too many"); !errors.Is(err, customers.ErrLimitReached) {
			t.Fatalf("201st note: %v", err)
		}
		// the detail carries the newest 50, ListNotes pages the rest without gaps or repeats
		if d := c.detail(c.adminTok, a1, custC); len(d.Notes) != 50 {
			t.Fatalf("detail notes %d, want 50", len(d.Notes))
		}
		total, cursor, seen := 0, "", map[string]bool{}
		for i := 0; i < 10; i++ {
			var p pagination.Page[customers.Note]
			err := c.run(c.adminTok, a1, func(ctx context.Context, tx pgx.Tx, s platform.Scope) (e error) {
				p, e = customers.ListNotes(ctx, tx, s, c.adminTok, custC, pagination.Request{Limit: 100, Cursor: cursor})
				return e
			})
			if err != nil {
				t.Fatal(err)
			}
			for _, n := range p.Items {
				if seen[n.ID] {
					t.Fatalf("note %s twice", n.ID)
				}
				seen[n.ID] = true
				total++
			}
			if p.NextCursor == "" {
				break
			}
			cursor = p.NextCursor
		}
		if total != 200 {
			t.Fatalf("paged %d notes, want 200", total)
		}
	})

	t.Run("CT04 set_owner_tags is compare-and-set", func(t *testing.T) {
		d := c.detail(c.adminTok, a1, custA)
		if _, err := c.setTags(c.adminTok, a1, custA, strings.Repeat("0", 64), vip.ID); !errors.Is(err, customers.ErrVersionChanged) {
			t.Fatalf("stale revision: %v", err)
		}
		set, err := c.setTags(c.adminTok, a1, custA, d.TagsRevision, vip.ID)
		if err != nil || len(set.Tags) != 1 || set.TagsRevision == d.TagsRevision {
			t.Fatalf("replace: %+v %v", set, err)
		}
		if _, err := c.setTags(c.adminTok, a1, custA, d.TagsRevision, vip.ID, loyal.ID); !errors.Is(err, customers.ErrVersionChanged) {
			t.Fatalf("the old revision after a change: %v", err)
		}
		if _, err := c.setTags(c.adminTok, a1, custA, set.TagsRevision, randomUUID()); !errors.Is(err, command.ErrInvalid) {
			t.Fatalf("unknown tag id: %v", err)
		}
		foreign := c.mustTag(c.adminTok, a2, "foreign-only", "red")
		if _, err := c.setTags(c.adminTok, a1, custA, set.TagsRevision, foreign.ID); !errors.Is(err, command.ErrInvalid) {
			t.Fatalf("another store's tag: %v", err)
		}
		if _, err := c.setTags(c.adminTok, a1, custA, set.TagsRevision, vip.ID, vip.ID); !errors.Is(err, command.ErrInvalid) {
			t.Fatalf("duplicate ids: %v", err)
		}
		if cleared, err := c.setTags(c.adminTok, a1, custA, set.TagsRevision); err != nil || len(cleared.Tags) != 0 {
			t.Fatalf("clear: %+v %v", cleared, err)
		}
		c.tagOwner(custA, vip.ID, loyal.ID) // restore for the later subtests
	})

	t.Run("CT05 permissions, tenancy and note authorship", func(t *testing.T) {
		d := c.detail(c.adminTok, a1, custA)
		viewer := c.rtoken // customers:read only on A1
		if _, err := c.newTag(viewer, a1, "viewer-tag", "red"); !errors.Is(err, platform.ErrForbidden) {
			t.Fatalf("viewer create tag: %v", err)
		}
		if _, err := c.setTags(viewer, a1, custA, d.TagsRevision, vip.ID); !errors.Is(err, platform.ErrForbidden) {
			t.Fatalf("viewer set tags: %v", err)
		}
		if _, err := c.addNote(viewer, a1, custA, "viewer note"); !errors.Is(err, platform.ErrForbidden) {
			t.Fatalf("viewer add note: %v", err)
		}
		if v := c.detail(viewer, a1, custA); len(v.Tags) == 0 { // reading tags needs customers:read only
			t.Fatal("viewer cannot read tags")
		}
		// another store of the same tenant, and another tenant: not found, no oracle
		if _, err := c.setTags(c.adminTok, a2, custA, d.TagsRevision, vip.ID); !errors.Is(err, platform.ErrScopeNotFound) && !errors.Is(err, command.ErrInvalid) {
			t.Fatalf("customer of store A1 through store A2: %v", err)
		}
		if _, err := c.addNote(c.adminTok, a2, custA, "wrong store"); !errors.Is(err, platform.ErrScopeNotFound) {
			t.Fatalf("note on another store's customer: %v", err)
		}
		if _, err := c.addNote(c.otherTok, a1, custA, "other tenant"); !errors.Is(err, platform.ErrScopeNotFound) {
			t.Fatalf("other tenant: %v", err)
		}
		if _, err := c.newTag(c.otherTok, a1, "other-tenant", "red"); !errors.Is(err, platform.ErrScopeNotFound) {
			t.Fatalf("other tenant tag: %v", err)
		}
		// authorship: A writes, B (no privacy) may not touch it, the admin may, a stale version is refused
		n, err := c.addNote(c.authorATok, a1, custA, "A: only 7-11")
		if err != nil || n.AuthorID != c.authorA || n.Version != 1 || n.EditedAt != nil {
			t.Fatalf("author A note: %+v %v", n, err)
		}
		// own: the response and the detail read say so for the author (non-privacy) and say false for another reader
		if dn := c.detail(c.authorATok, a1, custA).Notes; !n.Own || len(dn) != 1 || !dn[0].Own {
			t.Fatalf("author A does not see own=true: create %v detail %+v", n.Own, dn)
		}
		if dn := c.detail(c.adminTok, a1, custA).Notes; len(dn) != 1 || dn[0].Own {
			t.Fatalf("admin sees A's note as own: %+v", dn)
		}
		if _, err := c.editNote(c.authorBTok, a1, custA, n.ID, "B edits A", 1); !errors.Is(err, platform.ErrForbidden) {
			t.Fatalf("author B edits A's note: %v", err)
		}
		err = c.run(c.authorBTok, a1, func(ctx context.Context, tx pgx.Tx, s platform.Scope) error {
			_, e := customers.DeleteNote(ctx, tx, s, c.authorBTok, t04Key("ct-del"), custA, n.ID)
			return e
		})
		if !errors.Is(err, platform.ErrForbidden) {
			t.Fatalf("author B deletes A's note: %v", err)
		}
		e1, err := c.editNote(c.authorATok, a1, custA, n.ID, "A: only 7-11, fixed", 1)
		if err != nil || e1.Version != 2 || e1.EditedAt == nil || e1.Body != "A: only 7-11, fixed" {
			t.Fatalf("author edit: %+v %v", e1, err)
		}
		if _, err := c.editNote(c.authorATok, a1, custA, n.ID, "stale", 1); !errors.Is(err, customers.ErrVersionChanged) {
			t.Fatalf("stale note version: %v", err)
		}
		if e2, err := c.editNote(c.adminTok, a1, custA, n.ID, "admin edit", 2); err != nil || e2.Version != 3 {
			t.Fatalf("customers:privacy holder edits: %+v %v", e2, err)
		}
		if _, err := c.editNote(c.authorATok, a1, custA, randomUUID(), "x", 1); !errors.Is(err, platform.ErrScopeNotFound) {
			t.Fatalf("unknown note: %v", err)
		}
		// a note id from another customer is not found under this customer
		if _, err := c.editNote(c.adminTok, a1, custB, n.ID, "wrong customer", 3); !errors.Is(err, platform.ErrScopeNotFound) {
			t.Fatalf("note under the wrong customer: %v", err)
		}
		err = c.run(c.adminTok, a1, func(ctx context.Context, tx pgx.Tx, s platform.Scope) error {
			_, e := customers.DeleteNote(ctx, tx, s, c.adminTok, t04Key("ct-del"), custA, n.ID)
			return e
		})
		if err != nil {
			t.Fatalf("admin deletes the note: %v", err)
		}
		if got := len(c.detail(c.adminTok, a1, custA).Notes); got != 0 {
			t.Fatalf("notes after delete: %d", got)
		}
		// the stored idempotency receipt of a note write never holds the body (erasure cannot reach ops.command_results)
		if _, err := c.addNote(c.authorATok, a1, custA, ctMarker+"-receipt"); err != nil {
			t.Fatal(err)
		}
		if n := c.count(`SELECT count(*) FROM ops.command_results WHERE tenant_id=$1 AND operation LIKE 'customer.note.%' AND response::text LIKE '%'||$2||'%'`, f.tenantA, ctMarker); n != 0 {
			t.Fatalf("%d idempotency receipts hold a note body", n)
		}
	})

	t.Run("CT06 erasure removes links and notes, audit holds no body", func(t *testing.T) {
		erased := c.bundleCustomer()
		owner := erased.Scope.OwnerID
		c.tagOwner(owner, vip.ID, loyal.ID)
		for i := 0; i < 3; i++ {
			if _, err := c.addNote(c.authorATok, a1, owner, fmt.Sprintf("%s-%d", ctMarker, i)); err != nil {
				t.Fatal(err)
			}
		}
		if got := c.count(`SELECT count(*) FROM customers.notes WHERE owner_id=$1`, owner); got != 3 {
			t.Fatalf("notes before erasure: %d", got)
		}
		receipts := func(op string) int {
			return c.count(`SELECT count(*) FROM ops.command_results WHERE tenant_id=$1 AND response->>'customer_id'=$2 AND operation=$3`, f.tenantA, owner, op)
		}
		if receipts("customer.note.add") != 3 || receipts("customer.tags.set") < 1 {
			t.Fatalf("expected idempotency receipts before erasure: notes=%d tags=%d", receipts("customer.note.add"), receipts("customer.tags.set"))
		}
		err := c.run(c.adminTok, a1, func(ctx context.Context, tx pgx.Tx, s platform.Scope) error {
			_, e := customers.Erase(ctx, tx, s, c.adminTok, t04Key("ct-erase"), owner)
			return e
		})
		if err != nil {
			t.Fatalf("erase: %v", err)
		}
		if n := c.count(`SELECT count(*) FROM ops.command_results WHERE tenant_id=$1 AND response->>'customer_id'=$2`, f.tenantA, owner); n != 0 {
			t.Fatalf("%d idempotency receipts of the erased customer survived", n)
		}
		if got := c.count(`SELECT count(*) FROM customers.notes WHERE owner_id=$1`, owner) + c.count(`SELECT count(*) FROM customers.owner_tags WHERE owner_id=$1`, owner); got != 0 {
			t.Fatalf("%d tag links/notes survived the merchant erasure", got)
		}
		if got := c.count(`SELECT count(*) FROM customers.tags WHERE id=$1`, vip.ID); got != 1 {
			t.Fatal("erasure must not delete the store's tag catalogue")
		}
		// the owner is inactive: no new note or tag can be written for it
		if _, err := c.addNote(c.authorATok, a1, owner, "after erasure"); !errors.Is(err, platform.ErrScopeNotFound) {
			t.Fatalf("note after erasure: %v", err)
		}
		// restore replay (CD8): a row that reappears (e.g. from an older backup) is deleted again
		mustExec(t, f.owner, `INSERT INTO customers.notes(tenant_id,store_id,owner_id,body,author_id) VALUES($1,$2,$3,'restored',$4)`, f.tenantA, a1, owner, c.authorA)
		mustExec(t, f.owner, `SELECT customers.replay_erasures(ARRAY[$1::uuid])`, owner)
		if got := c.count(`SELECT count(*) FROM customers.notes WHERE owner_id=$1`, owner); got != 0 {
			t.Fatalf("replay_erasures left %d notes", got)
		}
		// buyer-initiated erasure
		self := c.bundleCustomer()
		c.tagOwner(self.Scope.OwnerID, vip.ID)
		if _, err := c.addNote(c.authorATok, a1, self.Scope.OwnerID, ctMarker+"-self"); err != nil {
			t.Fatal(err)
		}
		tx, err := c.h.a.runtime.Begin(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		_, err = customers.BuyerErase(context.Background(), tx, self.Token, a1, t04Key("ct-berase"))
		if err == nil {
			err = tx.Commit(context.Background())
		} else {
			_ = tx.Rollback(context.Background())
		}
		if err != nil {
			t.Fatalf("buyer erase: %v", err)
		}
		if got := c.count(`SELECT count(*) FROM customers.notes WHERE owner_id=$1`, self.Scope.OwnerID) + c.count(`SELECT count(*) FROM customers.owner_tags WHERE owner_id=$1`, self.Scope.OwnerID); got != 0 {
			t.Fatalf("%d tag links/notes survived the buyer erasure", got)
		}
		// audit rows name the action only; no table of ops carries the marker
		if got := c.count(`SELECT count(*) FROM ops.audit_events WHERE tenant_id=$1 AND action LIKE 'customers.note_%'`, f.tenantA); got < 3 {
			t.Fatalf("note audit rows: %d", got)
		}
		if got := c.count(`SELECT count(*) FROM ops.audit_events a WHERE a.tenant_id=$1 AND row_to_json(a)::text LIKE '%'||$2||'%'`, f.tenantA, ctMarker); got != 0 {
			t.Fatalf("%d audit rows carry a note body", got)
		}
		if got := c.count(`SELECT count(*) FROM ops.command_results WHERE tenant_id=$1 AND response::text LIKE '%'||$2||'%'`, f.tenantA, ctMarker); got != 0 {
			t.Fatalf("%d idempotency receipts carry a note body", got)
		}
	})

	t.Run("CT07 the buyer self-service export includes tags and notes", func(t *testing.T) {
		cp := c.bundleCustomer()
		owner := cp.Scope.OwnerID
		c.tagOwner(owner, vip.ID, loyal.ID)
		if _, err := c.addNote(c.authorATok, a1, owner, ctMarker+"-export"); err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		tx, err := c.h.a.runtime.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback(context.Background())
		doc, err := customers.BuyerExport(ctx, tx, a1, cp.Token, t04Key("ct-export"))
		if err != nil {
			t.Fatalf("buyer export: %v", err)
		}
		if err := tx.Commit(ctx); err != nil {
			t.Fatal(err)
		}
		var parsed struct {
			Tags         []customers.ExportTag  `json:"tags"`
			Notes        []customers.ExportNote `json:"notes"`
			NotesOmitted int                    `json:"notes_omitted"`
		}
		if err := json.Unmarshal(doc, &parsed); err != nil || len(parsed.Tags) != 2 || len(parsed.Notes) != 1 {
			t.Fatalf("export tags/notes: %+v %v", parsed, err)
		}
		if parsed.Notes[0].Body != ctMarker+"-export" || parsed.Tags[0].Name == "" {
			t.Fatalf("export content: %+v", parsed)
		}
		if strings.Contains(string(doc), c.authorA) || parsed.NotesOmitted != 0 {
			t.Fatalf("the export names a staff principal or omitted notes: %d", parsed.NotesOmitted)
		}
		// 150 maximal notes (150000 characters): the export stays under its cap, keeps the newest 60000 characters and says how many it left out
		big := c.bundleCustomer()
		mustExec(t, f.owner, `INSERT INTO customers.notes(tenant_id,store_id,owner_id,body,author_id,created_at)
			SELECT $1,$2,$3,repeat('x',1000),$4,clock_timestamp()-make_interval(secs=>g) FROM generate_series(1,150) g`, f.tenantA, a1, big.Scope.OwnerID, c.authorA)
		tx3, err := c.h.a.runtime.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
		if err != nil {
			t.Fatal(err)
		}
		defer tx3.Rollback(context.Background())
		doc3, err := customers.BuyerExport(ctx, tx3, a1, big.Token, t04Key("ct-export3"))
		if err != nil {
			t.Fatalf("export with 150 maximal notes: %v", err)
		}
		var p3 struct {
			Notes        []customers.ExportNote `json:"notes"`
			NotesOmitted int                    `json:"notes_omitted"`
		}
		if err := json.Unmarshal(doc3, &p3); err != nil || len(p3.Notes) != 60 || p3.NotesOmitted != 90 {
			t.Fatalf("capped notes: kept=%d omitted=%d err=%v", len(p3.Notes), p3.NotesOmitted, err)
		}
		// a customer without tags or notes exports empty lists, not null
		empty := c.bundleCustomer()
		tx2, _ := c.h.a.runtime.Begin(ctx)
		defer tx2.Rollback(context.Background())
		doc2, err := customers.BuyerExport(ctx, tx2, a1, empty.Token, t04Key("ct-export2"))
		if err != nil || !bytes.Contains(doc2, []byte(`"tags":[]`)) || !bytes.Contains(doc2, []byte(`"notes":[]`)) {
			t.Fatalf("empty export: %v %s", err, doc2)
		}
	})

	t.Run("CT08 deleting a tag detaches it and leaves customers alone", func(t *testing.T) {
		gone := c.mustTag(c.adminTok, a1, "temp", "orange")
		c.tagOwner(custA, vip.ID, gone.ID)
		c.tagOwner(custB, gone.ID)
		beforeA := c.detail(c.adminTok, a1, custA)
		var out customers.DeletedTag
		err := c.run(c.adminTok, a1, func(ctx context.Context, tx pgx.Tx, s platform.Scope) (e error) {
			out, e = customers.DeleteTag(ctx, tx, s, c.adminTok, t04Key("ct-deltag"), gone.ID)
			return e
		})
		if err != nil || out.Detached != 2 || out.TagID != gone.ID {
			t.Fatalf("delete: %+v %v", out, err)
		}
		afterA := c.detail(c.adminTok, a1, custA)
		if len(afterA.Tags) != 1 || afterA.Tags[0].ID != vip.ID || afterA.OrdersCount != beforeA.OrdersCount || afterA.ClaimsCount != beforeA.ClaimsCount || !afterA.Active {
			t.Fatalf("customer changed beyond the tag: %+v", afterA)
		}
		if p, err := c.listByTag(c.adminTok, a1, gone.ID, 10, ""); err != nil || len(p.Items) != 0 {
			t.Fatalf("list by deleted tag: %v %v", p.Items, err)
		}
		err = c.run(c.adminTok, a1, func(ctx context.Context, tx pgx.Tx, s platform.Scope) error {
			_, e := customers.DeleteTag(ctx, tx, s, c.adminTok, t04Key("ct-deltag"), gone.ID)
			return e
		})
		if !errors.Is(err, platform.ErrScopeNotFound) {
			t.Fatalf("delete twice: %v", err)
		}
		// another store's tag is not deletable through this store
		other := c.mustTag(c.adminTok, a2, "a2-only", "gray")
		err = c.run(c.adminTok, a1, func(ctx context.Context, tx pgx.Tx, s platform.Scope) error {
			_, e := customers.DeleteTag(ctx, tx, s, c.adminTok, t04Key("ct-deltag"), other.ID)
			return e
		})
		if !errors.Is(err, platform.ErrScopeNotFound) {
			t.Fatalf("foreign tag delete: %v", err)
		}
	})

	t.Run("CT09 HTTP routes end to end, nothing logs a note body", func(t *testing.T) {
		var logs bytes.Buffer
		prevWriter, prevSlog := log.Writer(), slog.Default()
		log.SetOutput(&logs)
		slog.SetDefault(slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug})))
		defer func() { log.SetOutput(prevWriter); slog.SetDefault(prevSlog) }()
		srv := httptest.NewServer(httpapi.NewHandler(f.runtime))
		defer srv.Close()
		base := srv.URL + "/v1/admin/stores/" + a1 + "/customers"
		call := func(method, path, token, body string, key bool) (int, map[string]any) {
			var rd io.Reader
			if body != "" {
				rd = strings.NewReader(body)
			}
			req, _ := http.NewRequest(method, base+path, rd)
			req.Header.Set("Authorization", "Bearer "+token)
			if body != "" {
				req.Header.Set("Content-Type", "application/json")
			}
			if key {
				req.Header.Set("Idempotency-Key", t04Key("ct-http"))
			}
			res, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer res.Body.Close()
			raw, _ := io.ReadAll(res.Body)
			var m map[string]any
			_ = json.Unmarshal(raw, &m)
			return res.StatusCode, m
		}
		cust := c.bundleCustomer().Scope.OwnerID
		status, tag := call("POST", "/tags", c.adminTok, `{"name":"http-tag","color":"purple"}`, true)
		if status != 201 || tag["name"] != "http-tag" {
			t.Fatalf("create tag: %d %v", status, tag)
		}
		if status, body := call("POST", "/tags", c.adminTok, `{"name":"HTTP-TAG","color":"purple"}`, true); status != 409 || body["code"] != "tag_exists" {
			t.Fatalf("duplicate tag: %d %v", status, body)
		}
		if status, body := call("POST", "/tags", c.rtoken, `{"name":"nope","color":"red"}`, true); status != 403 || body["code"] != "forbidden" {
			t.Fatalf("viewer create tag: %d %v", status, body)
		}
		status, det := call("GET", "/"+cust, c.adminTok, "", false)
		if status != 200 || det["tags_revision"] == nil {
			t.Fatalf("detail: %d %v", status, det)
		}
		status, set := call("PUT", "/"+cust+"/tags", c.adminTok, fmt.Sprintf(`{"tag_ids":[%q],"revision":%q}`, tag["id"], det["tags_revision"]), true)
		if status != 200 || set["tags_revision"] == nil {
			t.Fatalf("set tags: %d %v", status, set)
		}
		if status, body := call("PUT", "/"+cust+"/tags", c.adminTok, fmt.Sprintf(`{"tag_ids":[],"revision":%q}`, det["tags_revision"]), true); status != 409 || body["code"] != "version_changed" {
			t.Fatalf("stale revision: %d %v", status, body)
		}
		status, note := call("POST", "/"+cust+"/notes", c.authorATok, `{"body":"`+ctMarker+`-http"}`, true)
		if status != 201 || note["body"] != ctMarker+"-http" {
			t.Fatalf("add note: %d %v", status, note)
		}
		status, list := call("GET", "?tag="+fmt.Sprint(tag["id"]), c.adminTok, "", false)
		items, _ := list["items"].([]any)
		if status != 200 || len(items) != 1 {
			t.Fatalf("list by tag: %d %v", status, list)
		}
		status, notes := call("GET", "/"+cust+"/notes?limit=10", c.adminTok, "", false)
		if status != 200 || len(notes["items"].([]any)) != 1 {
			t.Fatalf("list notes: %d %v", status, notes)
		}
		if status, body := call("PATCH", "/"+cust+"/notes/"+fmt.Sprint(note["id"]), c.authorBTok, `{"body":"x","version":1}`, true); status != 403 || body["code"] != "forbidden" {
			t.Fatalf("other author edit: %d %v", status, body)
		}
		if status, _ := call("DELETE", "/"+cust+"/notes/"+fmt.Sprint(note["id"]), c.adminTok, "", true); status != 200 {
			t.Fatalf("delete note: %d", status)
		}
		if status, _ := call("DELETE", "/tags/"+fmt.Sprint(tag["id"]), c.adminTok, "", true); status != 200 {
			t.Fatalf("delete tag: %d", status)
		}
		if strings.Contains(logs.String(), ctMarker) {
			t.Fatal("a note body reached the logs")
		}
	})
}

func sha256sum(s string) []byte { h := sha256.Sum256([]byte(s)); return h[:] }
