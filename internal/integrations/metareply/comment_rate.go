// Purpose: live-console §2.2/2.4 shared per-asset call slots and backoff for every comment Graph read.
// Depends on: Console mutex/config, bounded Graph replies and usage metadata; token transport remains Graph.Do.
// Used by: forward polling, deletion batches, older pages and comment-facts; no credential or body logging.
package metareply

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	metaoauth "livecommerce/internal/integrations/meta/oauth"
)

var errCommentBudget = errors.New("metareply: comment rate budget unavailable")

type consoleRate struct {
	next           time.Time
	busy           bool
	backoff, usage time.Duration
}

// commentGraph reserves the shared slot before I/O. Polls skip occupied slots; bridge reads may wait within their existing deadline.
func (c *Console) commentGraph(ctx context.Context, asset, path string, q url.Values, tok []byte, now time.Time, wait bool) (metaoauth.Reply, error) {
	if len(tok) == 0 {
		return metaoauth.Reply{}, errGraphReauth
	}
	started := time.Now()
	var b *consoleRate
	for {
		if ctx.Err() != nil {
			return metaoauth.Reply{}, errCommentBudget
		}
		c.mu.Lock()
		if c.rates == nil {
			c.rates = map[string]*consoleRate{}
		}
		b = c.rates[asset]
		if b == nil {
			b = &consoleRate{}
			c.rates[asset] = b
		}
		at := now
		if wait {
			at = now.Add(time.Since(started))
		}
		if !b.busy && !at.Before(b.next) {
			b.busy = true
			b.next = at.Add(max(c.cfg.PollInterval, b.usage))
			now = at
			c.mu.Unlock()
			break
		}
		delay := b.next.Sub(at)
		if delay <= 0 {
			delay = 10 * time.Millisecond
		}
		c.mu.Unlock()
		// A bridge call can run inside an existing scoped transaction. Never queue beyond3s
		// and consume its idle budget; the API can use its existing safe fallback instead.
		if !wait || time.Since(started)+delay > 3*time.Second {
			return metaoauth.Reply{}, errCommentBudget
		}
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return metaoauth.Reply{}, errCommentBudget
		case <-timer.C:
		}
	}
	// Calls Graph GET with Authorization only (live-console-v1 §2.4); root ids checks share the exact transport.
	callStarted := time.Now()
	rep, err := c.graph.Do(ctx, http.MethodGet, path, q, tok, nil)
	completed := now.Add(time.Since(callStarted))
	c.mu.Lock()
	b.busy = false
	code := commentReplyCode(rep.Body)
	// A single-comment404 is a confirmed absence, not a quota failure. It still spent this slot.
	if err != nil || (!commentFactsMissing(rep, path) && (!rep.OK() || code != 0 || !json.Valid(rep.Body))) {
		c.backoffLocked(b, completed, code == 190)
	} else {
		b.backoff = 0
		b.usage = commentUsageDelay(rep)
		b.next = maxTime(b.next, completed.Add(b.usage))
	}
	c.mu.Unlock()
	return rep, err
}
func (c *Console) backoffLocked(b *consoleRate, now time.Time, reauth bool) {
	if reauth {
		b.backoff = c.cfg.MaxBackoff
	} else if b.backoff == 0 {
		b.backoff = min(2*time.Second, c.cfg.MaxBackoff)
	} else {
		b.backoff = min(b.backoff*2, c.cfg.MaxBackoff)
	}
	b.next = now.Add(max(c.cfg.PollInterval, b.backoff, b.usage))
}
func (c *Console) commentParseFailure(asset string, now time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if b := c.rates[asset]; b != nil {
		c.backoffLocked(b, now, false)
	}
}

// Error envelopes can be per-id in a successful batch response. Only fixed codes are consumed, never messages.
func commentReplyCode(body []byte) int {
	if code := graphErrorCode(body); code != 0 {
		return code
	}
	var rows map[string]json.RawMessage
	if json.Unmarshal(body, &rows) != nil {
		return 0
	}
	code := 0
	for _, row := range rows {
		v := graphErrorCode(row)
		if v == 190 {
			return v
		}
		if v != 0 {
			code = v
		}
	}
	return code
}
func commentUsageDelay(rep metaoauth.Reply) time.Duration {
	percent := float64(0)
	var visit func(any, int)
	visit = func(v any, depth int) {
		if depth > 4 {
			return
		}
		switch x := v.(type) {
		case map[string]any:
			for k, v := range x {
				if k == "call_count" || k == "total_time" || k == "total_cputime" {
					if n, ok := v.(float64); ok {
						percent = max(percent, n)
					}
				} else {
					visit(v, depth+1)
				}
			}
		case []any:
			for _, v := range x {
				visit(v, depth+1)
			}
		}
	}
	for _, raw := range []string{rep.AppUsage, rep.BusinessUsage} {
		if raw == "" {
			continue
		}
		var v any
		if len(raw) > 16384 || json.Unmarshal([]byte(raw), &v) != nil {
			return 15 * time.Second
		}
		visit(v, 0)
	}
	if percent >= 80 {
		return 15 * time.Second
	}
	if percent >= 50 {
		return 5 * time.Second
	}
	return 0
}

func maxTime(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}

func commentFactsMissing(rep metaoauth.Reply, path string) bool {
	code := commentReplyCode(rep.Body)
	return rep.Status == 404 && !strings.Contains(path, "/") && commentRefShape.MatchString(path) && (code == 0 || code == 100)
}
