package checkout

import (
	"context"
	"encoding/json"
	"net/url"
	"strings"
	"testing"
	"time"

	"livecommerce/internal/command"
	"livecommerce/internal/integrations/psp/payuni"
)

func TestHostedFormIsBoundedSingleValueJSON(t *testing.T) {
	encrypted, hash := strings.Repeat("a", 16), strings.Repeat("A", 64)
	raw := payuni.HostedForm{Action: "https://sandbox-api.payuni.com.tw/api/upp", Fields: url.Values{
		"MerID": {"merchant"}, "Version": {"2.0"}, "EncryptInfo": {encrypted}, "HashInfo": {hash},
	}}
	form, err := boundedHostedForm(raw, "PROVIDER_MOCK")
	if err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(form)
	if err != nil || string(body) != `{"action":"https://sandbox-api.payuni.com.tw/api/upp","fields":{"EncryptInfo":"`+encrypted+`","HashInfo":"`+hash+`","MerID":"merchant","Version":"2.0"}}` {
		t.Fatalf("hosted form escaped singleton shape: %s, %v", body, err)
	}
	raw.Fields["MerID"] = []string{"merchant", "other"}
	if _, err := boundedHostedForm(raw, "PROVIDER_MOCK"); err == nil {
		t.Fatal("multi-value provider field accepted")
	}
	raw.Fields["MerID"] = []string{"merchant"}
	raw.Fields["Extra"] = []string{"value"}
	if _, err := boundedHostedForm(raw, "PROVIDER_MOCK"); err == nil {
		t.Fatal("extra provider field accepted")
	}
	delete(raw.Fields, "Extra")
	for _, bad := range []struct{ key, value string }{{"MerID", "bad merchant"},
		{"EncryptInfo", strings.Repeat("a", 24578)}, {"EncryptInfo", strings.Repeat("A", 16)},
		{"HashInfo", strings.Repeat("a", 64)}, {"HashInfo", strings.Repeat("A", 63)}} {
		valid := raw.Fields[bad.key][0]
		raw.Fields[bad.key] = []string{bad.value}
		if _, err := boundedHostedForm(raw, "PROVIDER_MOCK"); err == nil {
			t.Fatalf("invalid %s accepted", bad.key)
		}
		raw.Fields[bad.key] = []string{valid}
	}
	if _, err := boundedHostedForm(raw, "unknown"); err == nil {
		t.Fatal("unknown provider profile accepted")
	}
}

func TestHostedHandoffNeverReturnsFormAfterIssued(t *testing.T) {
	form := &HostedForm{Action: "https://sandbox-api.payuni.com.tw/api/upp", Fields: map[string]string{
		"MerID": "merchant", "Version": "2.0", "EncryptInfo": strings.Repeat("a", 16), "HashInfo": strings.Repeat("A", 64),
	}}
	issued := HostedHandoff{OrderID: testID, Disposition: "ISSUED", ExpiresAt: time.Now().Add(time.Minute), Form: form}
	if !validHostedHandoff(issued, testID, "SANDBOX") {
		t.Fatal("first committed handoff rejected")
	}
	issued.Disposition = "ALREADY_ISSUED"
	if validHostedHandoff(issued, testID, "SANDBOX") {
		t.Fatal("repeated handoff retained form")
	}
	issued.Form = nil
	if !validHostedHandoff(issued, testID, "SANDBOX") {
		t.Fatal("form-free replay rejected")
	}
	if _, err := NewHostedPaymentStarter(context.Background(), nil, nil, "LIVE", nil,
		HostedConfig{ReturnURL: "https://pay.example.com/return", NotifyURL: "https://pay.example.com/notify"}); err != command.ErrInvalid {
		t.Fatalf("missing signer reached database: %v", err)
	}
}
