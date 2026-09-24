package foundation_test

import (
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"livecommerce/internal/integrations/accounts"
)

// hpCommitLoss removes only the response to a real successful PG COMMIT. It
// doesn't fake transaction outcome or proxy provider traffic. The fixture uses
// sslmode=disable on loopback; no credentials or wire payloads are logged.
type hpCommitLoss struct {
	armed, committed atomic.Bool
}

type hpCommitLossConn struct {
	net.Conn
	loss    *hpCommitLoss
	pending atomic.Bool
}

func (c *hpCommitLossConn) Write(p []byte) (int, error) {
	n, err := c.Conn.Write(p)
	// pgx.Tx.Commit emits an unparameterized simple-query message. Require the
	// exact frame, not a substring that could match unrelated SQL/data.
	if err == nil && n == len(p) && len(p) >= 6 && p[0] == 'Q' &&
		int(binary.BigEndian.Uint32(p[1:5])) == len(p)-1 && p[len(p)-1] == 0 &&
		bytes.EqualFold(p[5:len(p)-1], []byte("commit")) && c.loss.armed.Load() {
		c.pending.Store(true)
	}
	return n, err
}

func (c *hpCommitLossConn) Read(p []byte) (int, error) {
	if !c.pending.CompareAndSwap(true, false) {
		return c.Conn.Read(p)
	}
	// Consume framed server messages until commit has actually succeeded, then
	// close without delivering its acknowledgement to pgx. Framing also handles
	// TCP fragmentation deterministically; a mere write of COMMIT is not proof.
	_ = c.Conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	defer c.Conn.Close()
	for {
		var head [5]byte
		if _, err := io.ReadFull(c.Conn, head[:]); err != nil {
			return 0, err
		}
		length := int(binary.BigEndian.Uint32(head[1:]))
		if length < 4 || length > 1<<20 {
			return 0, io.ErrUnexpectedEOF
		}
		body := make([]byte, length-4)
		if _, err := io.ReadFull(c.Conn, body); err != nil {
			return 0, err
		}
		if head[0] == 'C' && bytes.Equal(body, []byte("COMMIT\x00")) {
			c.loss.committed.Store(true)
			return 0, io.ErrUnexpectedEOF
		}
		if head[0] == 'E' || head[0] == 'Z' {
			return 0, io.ErrUnexpectedEOF
		}
	}
}

func TestBuyerPaymentHostedLostCommitAcknowledgementDoesNotReissue(t *testing.T) {
	h := hpSetup(t)
	result, err := h.begin(t04Key("hp-commit-loss"))
	if err != nil {
		t.Fatal(err)
	}
	before := h.counts(t)
	config := h.pool.Config()
	config.MaxConns = 1
	loss := &hpCommitLoss{}
	dial := config.ConnConfig.DialFunc
	config.ConnConfig.DialFunc = func(ctx context.Context, network, address string) (net.Conn, error) {
		conn, err := dial(ctx, network, address)
		if err != nil {
			return nil, err
		}
		return &hpCommitLossConn{Conn: conn, loss: loss}, nil
	}
	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	keys, err := accounts.NewKeyring("hosted_fixture", map[string][]byte{"hosted_fixture": h.key}, randomBytes(32))
	if err != nil {
		t.Fatal(err)
	}
	api := hpStarter(t, pool, "PROVIDER_MOCK", keys, h.config)
	loss.armed.Store(true)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	out, err := api.TakeHosted(ctx, h.cap.Token, h.f.storeA1, h.hold.OrderID)
	if err == nil || out.Form != nil || out.OrderID != "" || !loss.committed.Load() {
		t.Fatal("HP03 real commit acknowledgement loss must return an error and no form")
	}
	_, _, _, handed := h.page(t, result.AttemptID)
	if handed == nil {
		t.Fatal("HP03 independently observed database has no committed handoff")
	}
	retry, err := h.take()
	if err != nil || retry.Disposition != "ALREADY_ISSUED" || retry.Form != nil {
		t.Fatal("HP03 recovery after unknown commit reissued or lost the persisted handoff")
	}
	if h.counts(t) != before {
		t.Fatal("HP03 unknown commit changed financial identity, stock, or query facts")
	}
}
