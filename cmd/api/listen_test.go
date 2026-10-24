package main

import "testing"

// TestListenAddrZeroPortRequiresLiteralLoopback pins the production config rule CMD-1 relies on: a zero port is the
// test-only ephemeral affordance and must name a literal loopback host, exactly like cmd/claims-worker's console
// bridge (validConsoleAddr). Fixed ports keep their previous host-agnostic bind semantics.
func TestListenAddrZeroPortRequiresLiteralLoopback(t *testing.T) {
	for _, addr := range []string{"127.0.0.1:0", "[::1]:0"} {
		if err := validateListenAddr(addr); err != nil {
			t.Errorf("loopback ephemeral listener %q refused: %v", addr, err)
		}
	}
	for _, addr := range []string{"0.0.0.0:0", ":0", "[::]:0", "localhost:0", "10.1.2.3:0"} {
		if err := validateListenAddr(addr); err == nil {
			t.Errorf("non-loopback ephemeral listener %q accepted", addr)
		}
	}
	for _, addr := range []string{"0.0.0.0:8080", ":8080", "127.0.0.1:8080", "[::]:8080", "localhost:8080"} {
		if err := validateListenAddr(addr); err != nil {
			t.Errorf("fixed listener %q refused: %v", addr, err)
		}
	}
	for _, addr := range []string{"127.0.0.1", "127.0.0.1:notaport", "127.0.0.1:70000"} {
		if err := validateListenAddr(addr); err == nil {
			t.Errorf("malformed listener %q accepted", addr)
		}
	}
}
