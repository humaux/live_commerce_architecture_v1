package livekit_test

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	livekit "livecommerce/internal/integrations/livekit"
)

const publisherIdentity = "lcp_0123456789abcdef0123456789abcdef"

func inputTarget() livekit.InputTarget {
	return livekit.InputTarget{RoomName: room, Identity: publisherIdentity}
}

func jwtParts(t *testing.T, bearer string) map[string]any {
	t.Helper()
	parts := strings.Split(strings.TrimPrefix(bearer, "Bearer "), ".")
	if len(parts) != 3 || !strings.HasPrefix(bearer, "Bearer ") {
		t.Fatalf("invalid bearer shape: %d parts", len(parts))
	}
	decode := func(part string) map[string]any {
		t.Helper()
		b, err := base64.RawURLEncoding.DecodeString(part)
		if err != nil {
			t.Fatal(err)
		}
		var value map[string]any
		if err := json.Unmarshal(b, &value); err != nil {
			t.Fatal(err)
		}
		return value
	}
	if got := decode(parts[0]); !reflect.DeepEqual(got, map[string]any{"alg": "HS256", "typ": "JWT"}) {
		t.Fatalf("unexpected JWT header: %v", got)
	}
	mac := hmac.New(sha256.New, []byte(apiSecret))
	_, _ = io.WriteString(mac, parts[0]+"."+parts[1])
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || !hmac.Equal(signature, mac.Sum(nil)) {
		t.Fatal("JWT signature invalid")
	}
	return decode(parts[1])
}

func inputCall(c *livekit.Client, method string, ctx context.Context) (any, error) {
	switch method {
	case "GetParticipant":
		return c.ObserveInput(ctx, inputTarget())
	case "RemoveParticipant":
		return nil, c.RemoveInput(ctx, inputTarget(), time.Now().Unix())
	case "DeleteRoom":
		return nil, c.DeleteInputRoom(ctx, room)
	case "ListRooms":
		return c.ObserveInputRoom(ctx, room)
	default:
		panic(method)
	}
}

func inputResult(t *testing.T, got any, err, want error) {
	t.Helper()
	requireErr(t, err, want)
	switch v := got.(type) {
	case livekit.InputObservation:
		if v != (livekit.InputObservation{}) {
			t.Fatalf("failure returned partial participant: %+v", v)
		}
	case livekit.InputRoomObservation:
		if v != (livekit.InputRoomObservation{}) {
			t.Fatalf("failure returned partial room: %+v", v)
		}
	}
}

func TestLKI01PublisherSigningExactAndRedacted(t *testing.T) {
	c, _, calls := fixtureClient(t, func(w http.ResponseWriter, _ *http.Request) { reply(w, `{}`) })
	now := time.Now().Unix()
	grant := livekit.PublisherGrant{RoomName: room, Identity: publisherIdentity, IssuedAt: now - 2, ExpiresAt: now + 40}
	first, err := c.MintPublisher(grant)
	if err != nil || first.Bearer() == "" {
		t.Fatalf("mint failed: %v", err)
	}
	second, err := c.MintPublisher(grant)
	if err != nil || first.Bearer() != second.Bearer() {
		t.Fatalf("same frozen grant changed: %v", err)
	}
	claims := jwtParts(t, "Bearer "+first.Bearer())
	want := map[string]any{
		"iss": "test_key-1", "sub": publisherIdentity, "iat": float64(now - 2), "nbf": float64(now - 2), "exp": float64(now + 40),
		"video": map[string]any{"room": room, "roomJoin": true, "canPublish": true,
			"canPublishSources": []any{"camera", "microphone"}, "canSubscribe": false,
			"canPublishData": false, "canUpdateOwnMetadata": false},
	}
	if !reflect.DeepEqual(claims, want) {
		t.Fatalf("publisher claims differ: got %v", claims)
	}
	if calls.Load() != 0 {
		t.Fatalf("mint made %d network calls", calls.Load())
	}
	for _, value := range []any{first, &first} {
		for _, display := range []string{fmt.Sprint(value), fmt.Sprintf("%#v", value)} {
			if strings.Contains(display, first.Bearer()) || strings.Contains(display, apiSecret) {
				t.Fatal("publisher token display leaked secret")
			}
		}
		b, err := json.Marshal(value)
		if err != nil || bytes.Contains(b, []byte(first.Bearer())) || bytes.Contains(b, []byte(apiSecret)) {
			t.Fatalf("publisher token JSON leaked secret: %v", err)
		}
	}
	if (livekit.PublisherToken{}).Bearer() != "" {
		t.Fatal("zero publisher token has bearer")
	}
	for name, change := range map[string]func(*livekit.PublisherGrant){
		"room":          func(g *livekit.PublisherGrant) { g.RoomName = "lc_BAD" },
		"identity":      func(g *livekit.PublisherGrant) { g.Identity = "other" },
		"zero issued":   func(g *livekit.PublisherGrant) { g.IssuedAt = 0 },
		"future issued": func(g *livekit.PublisherGrant) { g.IssuedAt = now + 2 },
		"expired":       func(g *livekit.PublisherGrant) { g.ExpiresAt = now - 1 },
		"zero TTL":      func(g *livekit.PublisherGrant) { g.ExpiresAt = g.IssuedAt },
		"long TTL":      func(g *livekit.PublisherGrant) { g.ExpiresAt = g.IssuedAt + 61 },
		"overflow":      func(g *livekit.PublisherGrant) { g.ExpiresAt = int64(^uint64(0) >> 1) },
	} {
		t.Run(name, func(t *testing.T) {
			bad := grant
			change(&bad)
			token, err := c.MintPublisher(bad)
			requireErr(t, err, livekit.ErrInvalid)
			if token.Bearer() != "" || calls.Load() != 0 {
				t.Fatal("invalid grant signed or made network I/O")
			}
		})
	}
	var nilClient *livekit.Client
	if token, err := nilClient.MintPublisher(grant); err != livekit.ErrInvalid || token.Bearer() != "" {
		t.Fatalf("nil client mint: token=%v err=%v", token, err)
	}
}

func TestLKI02ExactTLSRoomServiceWireAndGrant(t *testing.T) {
	now := time.Now().Unix()
	responses := map[string]string{
		"GetParticipant":    `{"identity":"` + publisherIdentity + `","sid":"PA_pub","state":"ACTIVE","tracks":[{"sid":"TR_cam","source":"CAMERA","type":"VIDEO"},{"sid":"TR_mic","source":"MICROPHONE","type":"AUDIO","muted":true}]}`,
		"RemoveParticipant": `{}`, "DeleteRoom": `{}`, "ListRooms": `{"rooms":[{"name":"` + room + `","sid":"RM_room"}]}`,
	}
	wantBodies := map[string]string{
		"GetParticipant":    `{"room":"` + room + `","identity":"` + publisherIdentity + `"}`,
		"RemoveParticipant": fmt.Sprintf(`{"room":%q,"identity":%q,"revoke_token_ts":"%d"}`, room, publisherIdentity, now),
		"DeleteRoom":        `{"room":"` + room + `"}`,
		"ListRooms":         `{"names":["` + room + `"]}`,
	}
	wantGrants := map[string]map[string]any{
		"GetParticipant":    {"room": room, "roomAdmin": true},
		"RemoveParticipant": {"room": room, "roomAdmin": true},
		"DeleteRoom":        {"room": room, "roomCreate": true},
		"ListRooms":         {"room": room, "roomList": true},
	}
	seen := make(map[string]int)
	c, _, calls := fixtureClient(t, func(w http.ResponseWriter, r *http.Request) {
		method := strings.TrimPrefix(r.URL.Path, "/twirp/livekit.RoomService/")
		if _, ok := responses[method]; !ok || r.Method != http.MethodPost || r.Host != "tenant.livekit.cloud" || r.URL.Path != "/twirp/livekit.RoomService/"+method {
			t.Errorf("unexpected management request: %s %s host=%s", r.Method, r.URL, r.Host)
			w.WriteHeader(400)
			return
		}
		seen[method]++
		body, _ := io.ReadAll(r.Body)
		requireJSON(t, body, wantBodies[method])
		claims := jwtParts(t, r.Header.Get("Authorization"))
		if len(claims) != 5 || claims["iss"] != "test_key-1" || !reflect.DeepEqual(claims["video"], wantGrants[method]) || claims["sub"] != nil {
			t.Errorf("overbroad/wrong %s JWT: %v", method, claims)
		}
		issued, iok := claims["iat"].(float64)
		before, bok := claims["nbf"].(float64)
		expires, eok := claims["exp"].(float64)
		if !iok || !bok || !eok || issued != before || expires != issued+60 || issued < float64(now-3) || issued > float64(time.Now().Unix()+3) {
			t.Errorf("wrong %s JWT times: %v", method, claims)
		}
		reply(w, responses[method])
	})
	for _, method := range []string{"GetParticipant", "RemoveParticipant", "DeleteRoom", "ListRooms"} {
		var got any
		var err error
		if method == "RemoveParticipant" {
			err = c.RemoveInput(context.Background(), inputTarget(), now)
		} else {
			got, err = inputCall(c, method, context.Background())
		}
		if err != nil {
			t.Fatalf("%s: %v", method, err)
		}
		switch method {
		case "GetParticipant":
			want := livekit.InputObservation{RoomName: room, Identity: publisherIdentity, ParticipantID: "PA_pub", State: "ACTIVE", CameraPublished: true, MicrophonePublished: true, MicrophoneMuted: true}
			if got != want {
				t.Fatalf("participant projection: %+v", got)
			}
		case "ListRooms":
			if got != (livekit.InputRoomObservation{RoomName: room, RoomID: "RM_room"}) {
				t.Fatalf("room projection: %+v", got)
			}
		}
	}
	if calls.Load() != 4 || len(seen) != 4 {
		t.Fatalf("missing/duplicate management calls: total=%d methods=%v", calls.Load(), seen)
	}
}

func TestLKI02InvalidTargetsAndCutoffNeverCall(t *testing.T) {
	c, _, calls := fixtureClient(t, func(w http.ResponseWriter, _ *http.Request) { reply(w, `{}`) })
	bad := inputTarget()
	bad.Identity = "lcp_NOT_HEX"
	for _, tc := range []struct {
		name string
		call func() error
	}{
		{"observe target", func() error { _, err := c.ObserveInput(context.Background(), bad); return err }},
		{"remove target", func() error { return c.RemoveInput(context.Background(), bad, time.Now().Unix()) }},
		{"remove zero cutoff", func() error { return c.RemoveInput(context.Background(), inputTarget(), 0) }},
		{"remove old cutoff", func() error { return c.RemoveInput(context.Background(), inputTarget(), time.Now().Unix()-31) }},
		{"remove future cutoff", func() error { return c.RemoveInput(context.Background(), inputTarget(), time.Now().Unix()+1) }},
		{"delete room", func() error { return c.DeleteInputRoom(context.Background(), "lc_BAD") }},
		{"list room", func() error { _, err := c.ObserveInputRoom(context.Background(), "lc_BAD"); return err }},
		{"nil context", func() error { _, err := c.ObserveInput(nil, inputTarget()); return err }},
	} {
		t.Run(tc.name, func(t *testing.T) { requireErr(t, tc.call(), livekit.ErrInvalid) })
	}
	var nilClient *livekit.Client
	if _, err := nilClient.ObserveInput(context.Background(), inputTarget()); err != livekit.ErrInvalid {
		t.Fatal(err)
	}
	if err := nilClient.RemoveInput(context.Background(), inputTarget(), time.Now().Unix()); err != livekit.ErrInvalid {
		t.Fatal(err)
	}
	if err := nilClient.DeleteInputRoom(context.Background(), room); err != livekit.ErrInvalid {
		t.Fatal(err)
	}
	if _, err := nilClient.ObserveInputRoom(context.Background(), room); err != livekit.ErrInvalid {
		t.Fatal(err)
	}
	if calls.Load() != 0 {
		t.Fatalf("invalid calls reached network: %d", calls.Load())
	}
}

func TestLKI03StrictParticipantAndRoomProjection(t *testing.T) {
	var body string
	c, _, _ := fixtureClient(t, func(w http.ResponseWriter, _ *http.Request) { reply(w, body) })
	for name, raw := range map[string]string{
		"omitted defaults":  `{"identity":"` + publisherIdentity + `","sid":"PA_one"}`,
		"numeric enums":     `{"identity":"` + publisherIdentity + `","sid":"PA_one","state":2,"tracks":[{"sid":"TR_c","source":1,"type":1},{"sid":"TR_m","source":2}]}`,
		"muted independent": `{"identity":"` + publisherIdentity + `","sid":"PA_one","tracks":[{"sid":"TR_c","source":1,"type":1,"muted":true}]}`,
	} {
		t.Run(name, func(t *testing.T) {
			body = raw
			got, err := c.ObserveInput(context.Background(), inputTarget())
			if err != nil || got.RoomName != room || got.Identity != publisherIdentity || got.ParticipantID != "PA_one" {
				t.Fatalf("projection: %+v %v", got, err)
			}
			switch name {
			case "omitted defaults":
				if got.State != "JOINING" || got.CameraPublished || got.MicrophonePublished {
					t.Fatal(got)
				}
			case "numeric enums":
				if got.State != "ACTIVE" || !got.CameraPublished || !got.MicrophonePublished || got.CameraMuted || got.MicrophoneMuted {
					t.Fatal(got)
				}
			case "muted independent":
				if !got.CameraPublished || !got.CameraMuted || got.MicrophonePublished {
					t.Fatal(got)
				}
			}
		})
	}
	for name, raw := range map[string]string{
		"wrong identity":           `{"identity":"lcp_ffffffffffffffffffffffffffffffff","sid":"PA_one"}`,
		"bad sid":                  `{"identity":"` + publisherIdentity + `","sid":"PA_"}`,
		"null state":               `{"identity":"` + publisherIdentity + `","sid":"PA_one","state":null}`,
		"stringified state":        `{"identity":"` + publisherIdentity + `","sid":"PA_one","state":"2"}`,
		"unknown state":            `{"identity":"` + publisherIdentity + `","sid":"PA_one","state":4}`,
		"null tracks":              `{"identity":"` + publisherIdentity + `","sid":"PA_one","tracks":null}`,
		"source omitted":           `{"identity":"` + publisherIdentity + `","sid":"PA_one","tracks":[{"sid":"TR_one","type":"VIDEO"}]}`,
		"source type mismatch":     `{"identity":"` + publisherIdentity + `","sid":"PA_one","tracks":[{"sid":"TR_one","source":"CAMERA","type":"AUDIO"}]}`,
		"duplicate track SID":      `{"identity":"` + publisherIdentity + `","sid":"PA_one","tracks":[{"sid":"TR_one","source":1,"type":1},{"sid":"TR_one","source":2,"type":0}]}`,
		"duplicate source":         `{"identity":"` + publisherIdentity + `","sid":"PA_one","tracks":[{"sid":"TR_one","source":1,"type":1},{"sid":"TR_two","source":1,"type":1}]}`,
		"three tracks":             `{"identity":"` + publisherIdentity + `","sid":"PA_one","tracks":[{"sid":"TR_one","source":1,"type":1},{"sid":"TR_two","source":2,"type":0},{"sid":"TR_three","source":1,"type":1}]}`,
		"muted null":               `{"identity":"` + publisherIdentity + `","sid":"PA_one","tracks":[{"sid":"TR_one","source":1,"type":1,"muted":null}]}`,
		"duplicate ignored nested": `{"identity":"` + publisherIdentity + `","sid":"PA_one","ignored":{"x":1,"x":2}}`,
		"trailing object":          `{"identity":"` + publisherIdentity + `","sid":"PA_one"}{}`,
		"secret malformed":         `{"identity":"` + secretWord,
	} {
		t.Run(name, func(t *testing.T) {
			body = raw
			got, err := c.ObserveInput(context.Background(), inputTarget())
			inputResult(t, got, err, livekit.ErrUnavailable)
		})
	}
	for name, raw := range map[string]string{
		"empty":   `{"rooms":[]}`,
		"omitted": `{}`,
	} {
		t.Run("room/"+name, func(t *testing.T) {
			body = raw
			got, err := c.ObserveInputRoom(context.Background(), room)
			inputResult(t, got, err, livekit.ErrNotObserved)
		})
	}
	for name, raw := range map[string]string{
		"null":             `{"rooms":null}`,
		"nonarray":         `{"rooms":{}}`,
		"wrong room":       `{"rooms":[{"name":"lc_ffffffffffffffffffffffffffffffff","sid":"RM_one"}]}`,
		"bad SID":          `{"rooms":[{"name":"` + room + `","sid":"RM_"}]}`,
		"two rows":         `{"rooms":[{"name":"` + room + `","sid":"RM_one"},{"name":"` + room + `","sid":"RM_two"}]}`,
		"duplicate nested": `{"rooms":[{"name":"` + room + `","sid":"RM_one","ignored":{"x":1,"x":2}}]}`,
	} {
		t.Run("room/"+name, func(t *testing.T) {
			body = raw
			got, err := c.ObserveInputRoom(context.Background(), room)
			inputResult(t, got, err, livekit.ErrUnavailable)
		})
	}
}

func TestLKI03ResponseCeilingsAndDestructiveAck(t *testing.T) {
	var body []byte
	var compressed bool
	c, _, _ := fixtureClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if compressed {
			w.Header().Set("Content-Encoding", "gzip")
			z := gzip.NewWriter(w)
			_, _ = z.Write(body)
			_ = z.Close()
			return
		}
		_, _ = w.Write(body)
	})
	oversized := []byte(`{"identity":"` + publisherIdentity + `","sid":"PA_one","ignored":"` + strings.Repeat("a", 65537) + `"}`)
	invalidUTF8 := []byte(`{"identity":"` + publisherIdentity + `","sid":"PA_one","ignored":"`)
	invalidUTF8 = append(invalidUTF8, 0xff)
	invalidUTF8 = append(invalidUTF8, []byte(`"}`)...)
	for _, raw := range [][]byte{[]byte("{"), invalidUTF8, oversized} {
		body, compressed = raw, false
		got, err := c.ObserveInput(context.Background(), inputTarget())
		inputResult(t, got, err, livekit.ErrUnavailable)
		for _, method := range []string{"RemoveParticipant", "DeleteRoom"} {
			_, err := inputCall(c, method, context.Background())
			requireErr(t, err, livekit.ErrUnknown)
		}
	}
	body, compressed = oversized, true
	got, err := c.ObserveInputRoom(context.Background(), room)
	inputResult(t, got, err, livekit.ErrUnavailable)
	body, compressed = []byte(`{"unexpected":true}`), false
	for _, method := range []string{"RemoveParticipant", "DeleteRoom"} {
		_, err := inputCall(c, method, context.Background())
		requireErr(t, err, livekit.ErrUnknown)
	}
	body = []byte(`{}`)
	for _, method := range []string{"RemoveParticipant", "DeleteRoom"} {
		_, err := inputCall(c, method, context.Background())
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestLKI04HTTPFailuresDropHangCancelNoRedirectOrRetry(t *testing.T) {
	for _, method := range []string{"GetParticipant", "RemoveParticipant", "DeleteRoom", "ListRooms"} {
		want := livekit.ErrUnavailable
		if method == "RemoveParticipant" || method == "DeleteRoom" {
			want = livekit.ErrUnknown
		}
		for _, code := range []int{302, 404, 503, 200} {
			t.Run(fmt.Sprintf("%s/status-%d", method, code), func(t *testing.T) {
				redirected := new(atomic.Int32)
				second := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { redirected.Add(1); reply(w, `{}`) }))
				defer second.Close()
				first := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					if code == 302 {
						w.Header().Set("Location", second.URL+"/stolen")
					}
					if code == 200 {
						reply(w, `{"malformed":`)
						return
					}
					w.WriteHeader(code)
					_, _ = io.WriteString(w, secretWord)
				}))
				defer first.Close()
				var calls atomic.Int32
				transport := fixtureTransport(first, second)
				client, err := livekit.New(config(), roundTrip(func(r *http.Request) (*http.Response, error) { calls.Add(1); return transport.RoundTrip(r) }))
				if err != nil {
					t.Fatal(err)
				}
				got, err := inputCall(client, method, context.Background())
				inputResult(t, got, err, want)
				if calls.Load() != 1 || redirected.Load() != 0 {
					t.Fatalf("retried/followed redirect: %d %d", calls.Load(), redirected.Load())
				}
			})
		}
		t.Run(method+"/drop", func(t *testing.T) {
			c, _, calls := fixtureClient(t, func(w http.ResponseWriter, _ *http.Request) {
				conn, _, err := w.(http.Hijacker).Hijack()
				if err == nil {
					_ = conn.Close()
				}
			})
			got, err := inputCall(c, method, context.Background())
			inputResult(t, got, err, want)
			if calls.Load() != 1 {
				t.Fatalf("drop retried %d times", calls.Load())
			}
		})
		t.Run(method+"/hang-cancel", func(t *testing.T) {
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
				got any
				err error
			}
			finished := make(chan outcome, 1)
			go func() { got, err := inputCall(c, method, ctx); finished <- outcome{got, err} }()
			select {
			case <-entered:
			case <-time.After(2 * time.Second):
				t.Fatal("request did not reach TLS fixture")
			}
			cancel()
			select {
			case result := <-finished:
				inputResult(t, result.got, result.err, want)
			case <-time.After(2 * time.Second):
				t.Fatal("cancellation did not unwind")
			}
			if calls.Load() != 1 {
				t.Fatalf("hang retried %d times", calls.Load())
			}
		})
	}
}
