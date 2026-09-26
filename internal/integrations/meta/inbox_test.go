package meta

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func TestInboxOrderKeepsOriginalOrdinals(t *testing.T) {
	events := []Event{
		{Key: "b", PayloadHash: "2"},
		{Key: "a", PayloadHash: "2"},
		{Key: "a", PayloadHash: "1"},
		{Key: "a", PayloadHash: "1"},
	}
	want := []int{2, 3, 1, 0}
	if got := sortedEventOrder(events); !reflect.DeepEqual(got, want) {
		t.Fatalf("key/hash/ordinal lock order: got %v, want %v", got, want)
	}
	if events[0].Key != "b" || events[2].PayloadHash != "1" {
		t.Fatal("input events mutated")
	}
}

func TestInboxConstructorAndJobContract(t *testing.T) {
	if _, err := NewInbox(nil, nil, nil); !errors.Is(err, ErrConfig) {
		t.Fatalf("nil constructor: %v", err)
	}
	if _, err := NewInbox(context.Background(), nil, nil); !errors.Is(err, ErrConfig) {
		t.Fatalf("nil pool: %v", err)
	}
	if _, err := NewInboxHandler(nil, nil); !errors.Is(err, ErrConfig) {
		t.Fatalf("nil handler: %v", err)
	}
	args := inboxJobArgs{EventID: "11111111-2222-3333-4444-555555555555", Version: 1}
	encoded, err := json.Marshal(args)
	if err != nil || string(encoded) != `{"event_id":"11111111-2222-3333-4444-555555555555","version":1}` ||
		args.Kind() != "meta_inbox_v1" || inboxQueue != "meta_inbox" {
		t.Fatal("River job contract drifted")
	}
}

func TestInboxErrorsAndFormattingAreSafe(t *testing.T) {
	if err := (*Inbox)(nil).commit(context.Background(), Batch{}, nil); !errors.Is(err, ErrInboxStorage) {
		t.Fatalf("invalid commit: %v", err)
	}
	inbox := Inbox{}
	for _, rendered := range []string{fmt.Sprint(inbox), fmt.Sprintf("%+v", inbox), fmt.Sprintf("%#v", inbox)} {
		if !strings.Contains(rendered, "redacted") || strings.Contains(rendered, "pool:") {
			t.Fatal("inbox formatting exposed internals")
		}
	}
	encoded, err := json.Marshal(inbox)
	if err != nil || string(encoded) != `"meta.Inbox{redacted}"` {
		t.Fatal("inbox JSON exposed internals")
	}
	if strings.Contains(ErrInboxStorage.Error(), "SQL") || strings.Contains(ErrInboxStorage.Error(), "payload") {
		t.Fatal("storage error exposes details")
	}
}
