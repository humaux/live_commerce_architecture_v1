// Purpose: live-console §2.2 user-deletion checks, one ≤50-id batch per source per60s, atomic fail-safe eviction.
// Depends on: Console memory/epoch fences, commentGraph shared asset budget and existing Page-token custody.
// Used by: pollOne; no new listener, persistence, token transport or comment-text storage.
package metareply

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/url"
	"sort"
	"strings"
	"time"
)

const deletionInterval = 60 * time.Second
const deletionAge = 30 * time.Minute
const deletionBatchCap = 50

type deletionRef struct {
	ref              string
	seq              int64
	checked, created time.Time
}

func (c *Console) deletionDue(s *consoleSource, now time.Time) bool {
	if !s.lastDeletionAt.IsZero() && now.Before(s.lastDeletionAt.Add(deletionInterval)) {
		return false
	}
	for _, cc := range s.comments {
		if cc.CreatedAt.After(now.Add(-deletionAge)) && !cc.CreatedAt.After(now) {
			return true
		}
	}
	return false
}

// Attempts move to the tail even on uncertainty, which preserves comments without starving another batch.
// For a stable N eligible entries, rotation takes ceil(N/50) admitted cycles (≤40 at the2000 cap), plus shared-budget delays.
// This is not a promise that all2000 entries can be verified before the30min age window ends.
func (c *Console) deletionBatch(s *consoleSource, now time.Time) []deletionRef {
	if !c.deletionDue(s, now) {
		return nil
	}
	refs := make([]deletionRef, 0, len(s.comments))
	for _, cc := range s.comments {
		if cc.CreatedAt.After(now.Add(-deletionAge)) && !cc.CreatedAt.After(now) {
			refs = append(refs, deletionRef{cc.Ref, cc.seq, cc.checkedAt, cc.CreatedAt})
		}
	}
	sort.Slice(refs, func(i, j int) bool {
		if !refs[i].checked.Equal(refs[j].checked) {
			return refs[i].checked.Before(refs[j].checked)
		}
		if !refs[i].created.Equal(refs[j].created) {
			return refs[i].created.After(refs[j].created)
		}
		return refs[i].seq > refs[j].seq
	})
	if len(refs) > deletionBatchCap {
		refs = refs[:deletionBatchCap]
	}
	return refs
}
func (c *Console) checkDeletions(ctx context.Context, s *consoleSource, refs []deletionRef, tok []byte, now time.Time) error {
	ids := make([]string, len(refs))
	asked := map[string]bool{}
	for i, r := range refs {
		ids[i] = r.ref
		asked[r.ref] = true
	}
	ctx, cancel := context.WithTimeout(ctx, c.cfg.CallTimeout)
	defer cancel()
	started := time.Now()
	rep, err := c.commentGraph(ctx, s.assetID, "", url.Values{"ids": {strings.Join(ids, ",")}, "fields": {"id"}}, tok, now, false)
	if !errors.Is(err, errCommentBudget) {
		c.mu.Lock()
		s.lastDeletionAt = now
		for i := range s.comments {
			for _, r := range refs {
				if s.comments[i].Ref == r.ref && s.comments[i].seq == r.seq {
					s.comments[i].checkedAt = now
					s.byRef[r.ref] = s.comments[i]
				}
			}
		}
		c.mu.Unlock()
	}
	if err != nil || rep.Status != 200 || commentReplyCode(rep.Body) != 0 {
		return classifyGraphErr(rep, err)
	}
	present, ok := parseDeletionIDs(rep.Body, asked)
	if !ok {
		c.commentParseFailure(s.assetID, now.Add(time.Since(started)))
		return errors.New("metareply: invalid comment id batch")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.sources[s.source] != s || !s.owned {
		return errCommentBudget
	}
	missing := map[string]int64{}
	for _, r := range refs {
		if !present[r.ref] {
			missing[r.ref] = r.seq
		}
	}
	kept := s.comments[:0]
	for _, cc := range s.comments {
		if seq, remove := missing[cc.Ref]; remove && seq == cc.seq {
			delete(s.byRef, cc.Ref)
			continue
		}
		kept = append(kept, cc)
	}
	// Zero discarded backing-array slots so deleted text/name do not linger after the slice shrinks.
	clear(s.comments[len(kept):])
	s.comments = kept
	return nil
}

// Only an exact200 dictionary of requested refs with a matching sole id field proves omission.
// Duplicate keys, trailing JSON, unknown/null/error/partial rows fail the whole response before mutation.
func parseDeletionIDs(body []byte, asked map[string]bool) (map[string]bool, bool) {
	d := json.NewDecoder(bytes.NewReader(body))
	open, err := d.Token()
	if err != nil || open != json.Delim('{') {
		return nil, false
	}
	seen := map[string]bool{}
	for d.More() {
		key, err := d.Token()
		ref, ok := key.(string)
		if err != nil || !ok || !asked[ref] || seen[ref] {
			return nil, false
		}
		open, err = d.Token()
		if err != nil || open != json.Delim('{') || !d.More() {
			return nil, false
		}
		key, err = d.Token()
		if err != nil || key != "id" {
			return nil, false
		}
		value, err := d.Token()
		if err != nil || value != ref || d.More() {
			return nil, false
		}
		close, err := d.Token()
		if err != nil || close != json.Delim('}') {
			return nil, false
		}
		seen[ref] = true
	}
	close, err := d.Token()
	if err != nil || close != json.Delim('}') {
		return nil, false
	}
	var tail any
	if d.Decode(&tail) != io.EOF {
		return nil, false
	}
	return seen, true
}
