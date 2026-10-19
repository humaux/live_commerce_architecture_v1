package foundation_test

// Purpose: lost-commit handoff test, now a refusal gate for the dropped PAYUNi hosted writer.
// Depends on: p2bHostedRetired (payment_sql_contract_helpers_test.go).
// Used by: go test ./tests/foundation (CI full suite).
// Invariants: a handoff is never re-issued; the PAYUNi writer no longer exists. PAY-RM2 phase 2b (0197, post_river/0026): see output/pay-rm2-p2b/test-contract-map.md.

import (
	"bytes"
	"encoding/binary"
	"io"
	"net"
	"sync/atomic"
	"testing"
	"time"
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
	// PAY-RM2 phase 2b (0197, post_river/0026) dropped the PAYUNi SQL this test drove; mapped (a)+(b) in
	// output/pay-rm2-p2b/test-contract-map.md: the dropped path is refused and writes nothing, and the
	// surviving invariant is asserted by the Stripe test named there.
	p2bHostedRetired(t)
}
