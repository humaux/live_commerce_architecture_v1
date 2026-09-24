package storefront

import (
	"encoding/json"
	"math"
	"strings"
	"testing"
)

func TestDestinationInputValidation(t *testing.T) {
	home := DestinationInput{
		CartVersion: 1, Kind: "home", Country: "TW", RecipientName: "Buyer", Phone: "+886 912 345 678",
		HomeAddress: HomeAddress{City: "Taipei", Line1: "Main street 1"},
	}
	cvs := DestinationInput{
		CartVersion: 1, Kind: "cvs_711", Country: "TW", RecipientName: "Buyer", Phone: "0912345678",
		PickupID: "00000000-0000-0000-0000-000000000001",
	}
	if !validDestinationInput(home) || !validDestinationInput(cvs) {
		t.Fatal("valid home and CVS inputs must pass")
	}
	tests := []struct {
		name string
		edit func(*DestinationInput)
	}{
		{"negative expected version", func(v *DestinationInput) { v.ExpectedVersion = -1 }},
		{"overflow expected version", func(v *DestinationInput) { v.ExpectedVersion = math.MaxInt64 }},
		{"zero cart version", func(v *DestinationInput) { v.CartVersion = 0 }},
		{"lowercase country", func(v *DestinationInput) { v.Country = "tw" }},
		{"leading name space", func(v *DestinationInput) { v.RecipientName = " Buyer" }},
		{"blank name", func(v *DestinationInput) { v.RecipientName = "" }},
		{"format character", func(v *DestinationInput) { v.RecipientName = "A\u200bB" }},
		{"invalid UTF8", func(v *DestinationInput) { v.RecipientName = string([]byte{0xff}) }},
		{"long name", func(v *DestinationInput) { v.RecipientName = strings.Repeat("界", 121) }},
		{"short phone", func(v *DestinationInput) { v.Phone = "12345" }},
		{"nondigit phone", func(v *DestinationInput) { v.Phone = "12345x678" }},
		{"too many phone digits", func(v *DestinationInput) { v.Phone = strings.Repeat("1", 21) }},
		{"missing city", func(v *DestinationInput) { v.HomeAddress.City = "" }},
		{"trimmed line", func(v *DestinationInput) { v.HomeAddress.Line1 = " Main" }},
		{"long line", func(v *DestinationInput) { v.HomeAddress.Line1 = strings.Repeat("界", 201) }},
		{"home pickup", func(v *DestinationInput) { v.PickupID = cvs.PickupID }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			v := home
			tc.edit(&v)
			if validDestinationInput(v) {
				t.Fatal("invalid home input accepted")
			}
		})
	}
	for _, tc := range []struct {
		name string
		edit func(*DestinationInput)
	}{
		{"wrong country", func(v *DestinationInput) { v.Country = "US" }},
		{"missing pickup ID", func(v *DestinationInput) { v.PickupID = "" }},
		{"bad pickup ID", func(v *DestinationInput) { v.PickupID = "123" }},
		{"buyer-supplied address", func(v *DestinationInput) { v.HomeAddress.City = "Taipei" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v := cvs
			tc.edit(&v)
			if validDestinationInput(v) {
				t.Fatal("invalid CVS input accepted")
			}
		})
	}
}

func TestDestinationReceiptOmitsPII(t *testing.T) {
	body, err := json.Marshal(destinationReceipt{ID: "00000000-0000-0000-0000-000000000001", Version: 1})
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != `{"id":"00000000-0000-0000-0000-000000000001","version":1}` {
		t.Fatalf("unexpected receipt fields: %s", body)
	}
}
