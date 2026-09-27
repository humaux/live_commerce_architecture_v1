package main

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestRecoveryReleaseGate(t *testing.T) {
	readEnd, writeEnd, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer readEnd.Close()
	done := make(chan bool, 1)
	go func() { done <- awaitRecoveryRelease(readEnd) }()
	select {
	case <-done:
		t.Fatal("child passed before release")
	case <-time.After(30 * time.Millisecond):
	}
	if _, err := writeEnd.Write([]byte{1}); err != nil {
		t.Fatal(err)
	}
	select {
	case allowed := <-done:
		if !allowed {
			t.Fatal("release rejected")
		}
	case <-time.After(time.Second):
		t.Fatal("release not received")
	}
	_ = writeEnd.Close()
}

func TestRecoveryReleaseEOFRejects(t *testing.T) {
	readEnd, writeEnd, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer readEnd.Close()
	_ = writeEnd.Close()
	if awaitRecoveryRelease(readEnd) {
		t.Fatal("EOF became release")
	}
}

func TestRecoveryChildEnvironmentDropsRecoveryDSN(t *testing.T) {
	t.Setenv("COMMERCE_MEDIA_RECOVERY_DATABASE_URL", "secret-recovery-dsn")
	for _, entry := range sanitizedNativeEnvironment() {
		if strings.Contains(entry, "secret-recovery-dsn") {
			t.Fatal("recovery DSN leaked to child")
		}
	}
}
