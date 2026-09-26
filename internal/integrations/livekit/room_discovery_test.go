package livekit_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	livekit "livecommerce/internal/integrations/livekit"
)

// LKP07: the provider may commit Start even when its acknowledgement is lost.
func TestLKP07AcceptedStartLostReplyFindsOneRoomCandidate(t *testing.T) {
	var starts, lists, stops atomic.Int32
	var accepted atomic.Bool
	type captured struct {
		method, path, host, authorization, idempotency string
		body                                           []byte
	}
	listRequest := make(chan captured, 1)
	s := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/twirp/livekit.Egress/StartEgress":
			starts.Add(1)
			_, _ = io.Copy(io.Discard, r.Body)
			accepted.Store(true)
			conn, _, err := w.(http.Hijacker).Hijack()
			if err != nil {
				t.Error(err)
				return
			}
			_ = conn.Close() // Resource accepted, response lost on the wire.
		case "/twirp/livekit.Egress/ListEgress":
			lists.Add(1)
			if !accepted.Load() {
				t.Error("List ran before the provider accepted Start")
				http.Error(w, "not accepted", http.StatusConflict)
				return
			}
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Error(err)
			}
			listRequest <- captured{r.Method, r.URL.Path, r.Host, r.Header.Get("Authorization"), r.Header.Get("Idempotency-Key"), body}
			reply(w, `{"items":[`+info()+`]}`)
		case "/twirp/livekit.Egress/StopEgress":
			stops.Add(1)
			reply(w, info())
		default:
			t.Errorf("unexpected provider path: %s", r.URL.Path)
			http.Error(w, "unexpected", http.StatusNotFound)
		}
	}))
	defer s.Close()
	c, err := livekit.New(config(), fixtureTransport(s))
	if err != nil {
		t.Fatal(err)
	}
	start, err := c.Start(context.Background(), input())
	requireErr(t, err, livekit.ErrUnknown)
	requireZero(t, start)
	got, err := c.FindByRoom(context.Background(), room)
	if err != nil || got.EgressID != egressID || got.RoomName != room || got.Status != "EGRESS_ACTIVE" || got.StartedAtNS != 123 {
		t.Fatalf("room candidate: %+v %v", got, err)
	}
	select {
	case r := <-listRequest:
		if r.method != http.MethodPost || r.path != "/twirp/livekit.Egress/ListEgress" || r.host != "tenant.livekit.cloud" || r.idempotency != "" {
			t.Fatalf("room discovery wire: %+v", r)
		}
		requireJSON(t, r.body, `{"room_name":"`+room+`","active":false}`)
		requireJWT(t, r.authorization)
	case <-time.After(time.Second):
		t.Fatal("room-scoped List request was not captured")
	}
	if starts.Load() != 1 || lists.Load() != 1 || stops.Load() != 0 {
		t.Fatalf("recovery repeated or stopped provider work: start=%d list=%d stop=%d", starts.Load(), lists.Load(), stops.Load())
	}
}

func TestLKP07InvalidInputsBeforeIO(t *testing.T) {
	c, _, calls := fixtureClient(t, func(w http.ResponseWriter, _ *http.Request) { reply(w, `{"items":[`+info()+`]}`) })
	for _, bad := range []string{"", "lc_ABCDEF0123456789abcdef0123456789", "lc_" + strings.Repeat("0", 31), "lc_" + strings.Repeat("g", 32), "lc_" + strings.Repeat("0", 33), secretWord} {
		got, err := c.FindByRoom(context.Background(), bad)
		requireErr(t, err, livekit.ErrInvalid)
		requireZero(t, got)
	}
	got, err := c.FindByRoom(nil, room)
	requireErr(t, err, livekit.ErrInvalid)
	requireZero(t, got)
	var nilClient *livekit.Client
	got, err = nilClient.FindByRoom(context.Background(), room)
	requireErr(t, err, livekit.ErrInvalid)
	requireZero(t, got)
	var zeroClient livekit.Client
	func() {
		defer func() {
			if p := recover(); p != nil {
				t.Errorf("zero-value Client.FindByRoom panicked: %v", p)
			}
		}()
		got, err := zeroClient.FindByRoom(context.Background(), room)
		requireErr(t, err, livekit.ErrInvalid)
		requireZero(t, got)
	}()
	got, err = c.Query(context.Background(), livekit.Target{RoomName: room})
	requireErr(t, err, livekit.ErrInvalid)
	requireZero(t, got)
	if calls.Load() != 0 {
		t.Fatalf("invalid room/target caused %d provider calls", calls.Load())
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	got, err = c.FindByRoom(ctx, room)
	requireErr(t, err, livekit.ErrUnavailable)
	requireZero(t, got)
	if calls.Load() != 0 {
		t.Fatalf("pre-cancelled room read caused %d provider calls", calls.Load())
	}
}

func TestLKP07RoomRowsPaginationAndStrictJSON(t *testing.T) {
	var body string
	c, _, calls := fixtureClient(t, func(w http.ResponseWriter, _ *http.Request) { reply(w, body) })
	for name, response := range map[string]string{
		"omitted items": `{}`,
		"empty items":   `{"items":[]}`,
		"empty token":   `{"items":[],"next_page_token":{"token":""}}`,
	} {
		t.Run(name, func(t *testing.T) {
			body = response
			got, err := c.FindByRoom(context.Background(), room)
			requireErr(t, err, livekit.ErrNotObserved)
			requireZero(t, got)
		})
	}
	for name, response := range map[string]string{
		"wrong room":                      `{"items":[{"egress_id":"` + egressID + `","room_name":"lc_ffffffffffffffffffffffffffffffff"}]}`,
		"invalid ID":                      `{"items":[{"egress_id":"bad","room_name":"` + room + `"}]}`,
		"duplicate match":                 `{"items":[` + info() + `,` + info() + `]}`,
		"match and unrelated":             `{"items":[` + info() + `,{"egress_id":"EG_other","room_name":"lc_ffffffffffffffffffffffffffffffff"}]}`,
		"multiple valid same room":        `{"items":[` + info() + `,{"egress_id":"EG_other","room_name":"` + room + `"}]}`,
		"nonempty page before empty":      `{"items":[],"next_page_token":{"token":"more"}}`,
		"null page before empty":          `{"items":[],"next_page_token":null}`,
		"page aliases before empty":       `{"items":[],"next_page_token":{},"nextPageToken":{}}`,
		"page invalid token before empty": `{"items":[],"next_page_token":{"token":9}}`,
		"malformed JSON":                  `{"items":[`,
		"duplicate key":                   `{"items":[],"items":[]}`,
		"trailing JSON":                   `{"items":[]}{}`,
		"items wrong type":                `{"items":{}}`,
		"items null":                      `{"items":null}`,
		"known field alias pair":          `{"items":[{"egress_id":"` + egressID + `","egressId":"` + egressID + `","room_name":"` + room + `"}]}`,
	} {
		t.Run(name, func(t *testing.T) {
			body = response
			got, err := c.FindByRoom(context.Background(), room)
			requireErr(t, err, livekit.ErrUnavailable)
			requireZero(t, got)
		})
	}
	body = `{"items":[` + strings.TrimSuffix(info(), "}") + `,"request":{"urls":["` + secretWord + `"]},"error":"` + secretWord + `"}]}`
	got, err := c.FindByRoom(context.Background(), room)
	if err != nil || got.EgressID != egressID || strings.Contains(fmt.Sprintf("%+v", got), secretWord) {
		t.Fatalf("unknown provider fields leaked or broke discovery: %+v %v", got, err)
	}
	if calls.Load() != 3+15+1 {
		t.Fatalf("room discovery did not send exactly one request per case: %d", calls.Load())
	}
}

func TestLKP07HTTPFailureAndCancellation(t *testing.T) {
	for _, code := range []int{302, 404, 503} {
		t.Run(fmt.Sprint(code), func(t *testing.T) {
			secondCalls := new(atomic.Int32)
			second := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				secondCalls.Add(1)
				reply(w, `{"items":[`+info()+`]}`)
			}))
			defer second.Close()
			firstCalls := new(atomic.Int32)
			first := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				firstCalls.Add(1)
				if code == 302 {
					w.Header().Set("Location", second.URL+"/stolen")
				}
				w.WriteHeader(code)
				_, _ = io.WriteString(w, secretWord)
			}))
			defer first.Close()
			c, err := livekit.New(config(), fixtureTransport(first, second))
			if err != nil {
				t.Fatal(err)
			}
			got, err := c.FindByRoom(context.Background(), room)
			requireErr(t, err, livekit.ErrUnavailable)
			requireZero(t, got)
			if firstCalls.Load() != 1 || secondCalls.Load() != 0 {
				t.Fatalf("room read retried/followed redirect: first=%d second=%d", firstCalls.Load(), secondCalls.Load())
			}
		})
	}
	entered := make(chan struct{})
	release := make(chan struct{})
	defer close(release)
	c, _, calls := fixtureClient(t, func(_ http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		close(entered)
		select {
		case <-r.Context().Done():
		case <-release:
		}
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	type outcome struct {
		got livekit.Observation
		err error
	}
	finished := make(chan outcome, 1)
	go func() {
		got, err := c.FindByRoom(ctx, room)
		finished <- outcome{got, err}
	}()
	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("room read never reached local TLS fixture")
	}
	cancel()
	select {
	case result := <-finished:
		requireErr(t, result.err, livekit.ErrUnavailable)
		requireZero(t, result.got)
	case <-time.After(2 * time.Second):
		t.Fatal("canceled room read did not return")
	}
	if calls.Load() != 1 {
		t.Fatalf("canceled room read retried: %d", calls.Load())
	}
}

func TestLKP07ExistingQueryStillCorrelatesExactID(t *testing.T) {
	var body string
	c, _, calls := fixtureClient(t, func(w http.ResponseWriter, _ *http.Request) { reply(w, body) })
	body = `{"items":[{"egress_id":"EG_other","room_name":"` + room + `"}]}`
	got, err := c.Query(context.Background(), target())
	requireErr(t, err, livekit.ErrUnavailable)
	requireZero(t, got)
	body = `{"items":[` + info() + `]}`
	got, err = c.Query(context.Background(), target())
	if err != nil || got.EgressID != egressID {
		t.Fatalf("existing exact Query broke: %+v %v", got, err)
	}
	if calls.Load() != 2 {
		t.Fatalf("exact Query request count: %d", calls.Load())
	}
}

func TestLKP07MalformedUTF8AndOversize(t *testing.T) {
	var body []byte
	c, _, _ := fixtureClient(t, func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(body) })
	valid := []byte(`{"items":[` + info() + `]}`)
	for name, data := range map[string][]byte{
		"UTF8":     bytes.Replace(valid, []byte(egressID), []byte{'E', 'G', '_', 0xff}, 1),
		"oversize": []byte(strings.TrimSuffix(string(valid), "}") + `,"extra":"` + strings.Repeat("x", 65536) + `"}`),
	} {
		t.Run(name, func(t *testing.T) {
			body = data
			got, err := c.FindByRoom(context.Background(), room)
			requireErr(t, err, livekit.ErrUnavailable)
			requireZero(t, got)
		})
	}
}
