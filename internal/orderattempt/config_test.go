// Purpose: pin OA1 startup key custody before the shared loader exists.
// Depends on: injected environment reads and the real plan/pay-entry codecs.
// Used by: API/worker configuration CI; no environment, database or provider I/O.
// Invariants: strict bounded JSON, separate purposes, retained versions, immutable
// loaded keys and fixed redacted failures. Local execution is NOT_RUN.
package orderattempt

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"
)

const configPlanActive = "LC_ORDER_ATTEMPT_PLAN_ACTIVE_KEY"
const configPlanKeys = "LC_ORDER_ATTEMPT_PLAN_KEYS"
const configPayActive = "LC_ORDER_ATTEMPT_PAY_ENTRY_ACTIVE_KEY"
const configPayKeys = "LC_ORDER_ATTEMPT_PAY_ENTRY_KEYS"

func configEncodedKey(b byte) string {
	return base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{b}, 32))
}

func configValues() map[string]string {
	return map[string]string{
		configPlanActive: "v1", configPlanKeys: `{"v1":"` + configEncodedKey(41) + `","v2":"` + configEncodedKey(42) + `"}`,
		configPayActive: "v1", configPayKeys: `{"v1":"` + configEncodedKey(51) + `","v2":"` + configEncodedKey(52) + `"}`,
	}
}

func configLoad(t *testing.T, values map[string]string) (*PlanKeys, *PayEntryKeys) {
	t.Helper()
	seen := map[string]int{}
	plans, pay, err := LoadKeys(func(name string) string {
		if _, ok := values[name]; !ok { t.Fatalf("unexpected environment read %q", name) }
		seen[name]++
		return values[name]
	})
	if err != nil || plans == nil || pay == nil { t.Fatalf("valid isolated key configuration rejected: %v", err) }
	if len(seen) != 4 { t.Fatal("loader did not read exactly the four dedicated configuration names") }
	return plans, pay
}

func configPlan() FrozenPlan {
	admitted := time.Date(2026, 10, 14, 1, 0, 0, 0, time.UTC)
	return FrozenPlan{Schema:1, AttemptID:"00000000-0000-4000-8000-000000000001", ScopeID:"00000000-0000-4000-8000-000000000002", Generation:1,
		TenantID:"00000000-0000-4000-8000-000000000003", StoreID:"00000000-0000-4000-8000-000000000004",
		PrincipalID:"00000000-0000-4000-8000-000000000005", BuyerOwnerID:"00000000-0000-4000-8000-000000000006",
		AdmittedAt:admitted, ExecuteBy:admitted.Add(15*time.Minute), RequestHash:sha256.Sum256([]byte("synthetic request")),
		FactsDigest:sha256.Sum256([]byte("synthetic facts")), Facts:json.RawMessage(`{"synthetic":true}`)}
}

func TestOrderAttemptConfigRotatesBothRingsAndRetainsOriginalMaterial(t *testing.T) {
	values := configValues()
	plans, pay := configLoad(t, values)
	plan := configPlan()
	binding := PayEntryBinding{TenantID:plan.TenantID, StoreID:plan.StoreID, OrderID:plan.AttemptID, Version:1}
	sealed, err := plans.Seal(plan)
	if err != nil || sealed.KeyVersion != "v1" { t.Fatal("loaded plan ring did not seal using its active version") }
	entry, err := pay.Issue(binding)
	if err != nil || entry.KeyVersion != "v1" { t.Fatal("loaded pay ring did not issue using its active version") }
	token, err := pay.Open(binding, entry)
	if err != nil || token == "" { t.Fatal("loaded pay ring cannot replay its material") }
	values[configPlanActive], values[configPayActive] = "v2", "v2"
	rotatedPlans, rotatedPay := configLoad(t, values)
	opened, err := rotatedPlans.Open(plan.Binding(), sealed)
	if err != nil || opened.AttemptID != plan.AttemptID { t.Fatal("rotation lost retained plan version") }
	replayed, err := rotatedPay.Open(binding, entry)
	if err != nil || replayed != token { t.Fatal("rotation lost original pay credential") }
	newPlan, err := rotatedPlans.Seal(plan)
	if err != nil || newPlan.KeyVersion != "v2" { t.Fatal("new plan did not use rotated version") }
	newEntry, err := rotatedPay.Issue(binding)
	if err != nil || newEntry.KeyVersion != "v2" { t.Fatal("new entry did not use rotated version") }
	// Changing every injected input after load cannot mutate either retained ring.
	values[configPlanKeys] = `{"v1":"` + configEncodedKey(61) + `","v2":"` + configEncodedKey(62) + `"}`
	values[configPayKeys] = `{"v1":"` + configEncodedKey(71) + `","v2":"` + configEncodedKey(72) + `"}`
	replacedPlans, replacedPay := configLoad(t, values)
	if _, err = replacedPlans.Open(plan.Binding(), sealed); err == nil { t.Fatal("changed plan key unexpectedly opens old material") }
	if _, err = replacedPay.Open(binding, entry); err == nil { t.Fatal("changed pay key unexpectedly opens old material") }
	if _, err = plans.Open(plan.Binding(), sealed); err != nil { t.Fatal("environment mutation changed original plan ring") }
	if got, err := pay.Open(binding, entry); err != nil || got != token { t.Fatal("environment mutation changed original pay ring") }
	// Exercise the real constructors with mutable source bytes, then destroy them.
	rawPlan, rawPay := bytes.Repeat([]byte{41},32), bytes.Repeat([]byte{51},32)
	planInput, payInput := map[string][]byte{"v1":rawPlan}, map[string][]byte{"v1":rawPay}
	directPlan, err := NewPlanKeys("v1",planInput)
	if err != nil { t.Fatal(err) }
	directPay, err := NewPayEntryKeys("v1",payInput)
	if err != nil { t.Fatal(err) }
	clear(rawPlan); clear(rawPay); delete(planInput,"v1"); delete(payInput,"v1")
	if _, err = directPlan.Open(plan.Binding(),sealed); err != nil { t.Fatal("constructor retained caller-owned plan bytes") }
	if got, err := directPay.Open(binding,entry); err != nil || got != token { t.Fatal("constructor retained caller-owned pay bytes") }
	for _, secret := range []string{configEncodedKey(41),configEncodedKey(51),token} {
		if strings.Contains(fmt.Sprintf("%v %#v %v %#v",plans,plans,pay,pay),secret) { t.Fatal("keyring formatting disclosed custody") }
	}
}

func TestOrderAttemptConfigRejectsMalformedOrMissingInputsWithOneRedactedError(t *testing.T) {
	_, _, initial := LoadKeys(nil)
	if initial == nil || initial.Error() == "" { t.Fatal("nil getenv accepted") }
	fixed := initial.Error()
	assertRejected := func(t *testing.T, values map[string]string) {
		t.Helper()
		plans, pay, err := LoadKeys(func(name string) string { return values[name] })
		if err == nil || plans != nil || pay != nil { t.Fatal("invalid configuration yielded a usable or partial ring") }
		if err.Error() != fixed { t.Fatal("configuration error is not a single fixed redacted value") }
		formatted := fmt.Sprintf("%v %+v %#v",err,err,err)
		for _, value := range values {
			if len(value)>8 && strings.Contains(formatted,value) { t.Fatal("formatted error leaked configuration input") }
		}
	}
	for _, name := range []string{configPlanActive,configPlanKeys,configPayActive,configPayKeys} {
		t.Run("missing_"+name,func(t *testing.T) { values:=configValues(); delete(values,name); assertRejected(t,values) })
	}
	for _, name := range []string{configPlanActive,configPayActive} {
		for _, bad := range []string{"", "missing", "bad key", "../unsafe", strings.Repeat("k",65)} {
			t.Run(name+"_"+bad,func(t *testing.T) { values:=configValues(); values[name]=bad; assertRejected(t,values) })
		}
	}
	valid := configEncodedKey(91)
	malformed := map[string]string{
		"empty":"", "null":"null", "array":"[]", "empty object":"{}", "scalar":"true", "malformed":"{",
		"nonstring":`{"v1":1}`, "null key":`{"v1":null}`, "nested key":`{"v1":{"key":"`+valid+`"}}`,
		"bad inactive ID":`{"v1":"`+valid+`","bad key":"`+configEncodedKey(92)+`"}`,
		"empty ID":`{"v1":"`+valid+`","":"`+configEncodedKey(92)+`"}`,
		"zero key":`{"v1":"`+base64.StdEncoding.EncodeToString(make([]byte,32))+`"}`,
		"short key":`{"v1":"`+base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{9},31))+`"}`,
		"long key":`{"v1":"`+base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{9},33))+`"}`,
		"invalid base64":`{"v1":"synthetic-not-base64-sensitive-value"}`,
		"unpadded":`{"v1":"`+strings.TrimRight(valid,"=")+`"}`,
		"embedded newline":`{"v1":"`+valid[:8]+`\n`+valid[8:]+`"}`,
		"invalid inactive key":`{"v1":"`+valid+`","v2":"not-a-key"}`,
		"duplicate identical":`{"v1":"`+valid+`","v1":"`+valid+`"}`,
		"duplicate different":`{"v1":"`+valid+`","v1":"`+configEncodedKey(92)+`"}`,
		"escaped duplicate":`{"v1":"`+valid+`","v\u0031":"`+valid+`"}`,
		"trailing object":`{"v1":"`+valid+`"} {}`, "trailing scalar":`{"v1":"`+valid+`"} true`,
		"oversized":strings.Repeat(" ",8193)+`{"v1":"`+valid+`"}`,
	}
	many:=map[string]string{}
	for i:=0;i<17;i++ { many[fmt.Sprintf("v%d",i)]=configEncodedKey(byte(90+i)) }
	encoded,_:=json.Marshal(many); malformed["too many versions"]=string(encoded)
	for _, name := range []string{configPlanKeys,configPayKeys} {
		for label,bad := range malformed {
			t.Run(name+"_"+label,func(t *testing.T) { values:=configValues(); values[name]=bad; assertRejected(t,values) })
		}
	}
}

func TestOrderAttemptConfigRejectsCrossPurposeReuseIncludingInactiveAliases(t *testing.T) {
	for _, tc := range []struct{ name, plan, pay string }{
		{"active", `{"v1":"`+configEncodedKey(41)+`"}`, `{"v1":"`+configEncodedKey(41)+`"}`},
		{"inactive", configValues()[configPlanKeys], `{"v1":"`+configEncodedKey(51)+`","different-id":"`+configEncodedKey(42)+`"}`},
		{"reverse inactive", `{"v1":"`+configEncodedKey(41)+`","old":"`+configEncodedKey(51)+`"}`, configValues()[configPayKeys]},
	} {
		t.Run(tc.name,func(t *testing.T) {
			values:=configValues(); values[configPlanKeys],values[configPayKeys]=tc.plan,tc.pay
			plans,pay,err:=LoadKeys(func(name string) string { return values[name] })
			if err==nil || plans!=nil || pay!=nil { t.Fatal("actual key material reused across plan and pay purposes") }
		})
	}
}
