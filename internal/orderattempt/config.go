// Purpose: load the two dedicated OA1 keyrings from injected configuration.
// Depends on: standard JSON/base64 parsing and the existing immutable codecs.
// Used by: API and claims-worker startup after their own activation decision.
// Invariants: no ambient I/O, duplicate or trailing JSON, cross-purpose key
// reuse, partial keyrings, or configuration values in returned errors.

package orderattempt

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"strings"
)

var errKeysConfig = errors.New("order_attempt_keys_invalid")

// LoadKeys reads only the four dedicated names. Callers own enablement and must
// fail startup when an enabled executor cannot load its required key custody.
func LoadKeys(getenv func(string) string) (*PlanKeys, *PayEntryKeys, error) {
	if getenv == nil {
		return nil, nil, errKeysConfig
	}
	planActive := getenv("LC_ORDER_ATTEMPT_PLAN_ACTIVE_KEY")
	planJSON := getenv("LC_ORDER_ATTEMPT_PLAN_KEYS")
	payActive := getenv("LC_ORDER_ATTEMPT_PAY_ENTRY_ACTIVE_KEY")
	payJSON := getenv("LC_ORDER_ATTEMPT_PAY_ENTRY_KEYS")
	if !validPlanKeyVersion(planActive) || !validPlanKeyVersion(payActive) {
		return nil, nil, errKeysConfig
	}
	planRaw, err := parseConfiguredKeys(planJSON)
	if err != nil {
		return nil, nil, errKeysConfig
	}
	defer clearConfiguredKeys(planRaw)
	payRaw, err := parseConfiguredKeys(payJSON)
	if err != nil {
		return nil, nil, errKeysConfig
	}
	defer clearConfiguredKeys(payRaw)
	// Compare actual material across every retained version, including aliases.
	for _, planKey := range planRaw {
		for _, payKey := range payRaw {
			if bytes.Equal(planKey, payKey) {
				return nil, nil, errKeysConfig
			}
		}
	}
	plans, err := NewPlanKeys(planActive, planRaw)
	if err != nil {
		return nil, nil, errKeysConfig
	}
	pay, err := NewPayEntryKeys(payActive, payRaw)
	if err != nil {
		return nil, nil, errKeysConfig
	}
	return plans, pay, nil
}

// parseConfiguredKeys consumes object members individually so duplicate decoded
// names cannot disappear through map unmarshalling (including escaped aliases).
func parseConfiguredKeys(raw string) (map[string][]byte, error) {
	if len(raw) == 0 || len(raw) > 8192 {
		return nil, errKeysConfig
	}
	decoder := json.NewDecoder(strings.NewReader(raw))
	start, err := decoder.Token()
	if err != nil || start != json.Delim('{') {
		return nil, errKeysConfig
	}
	keys := make(map[string][]byte)
	valid := false
	defer func() {
		if !valid {
			clearConfiguredKeys(keys)
		}
	}()
	for decoder.More() {
		name, err := decoder.Token()
		id, ok := name.(string)
		if err != nil || !ok || !validPlanKeyVersion(id) || len(keys) >= 16 {
			return nil, errKeysConfig
		}
		if _, duplicate := keys[id]; duplicate {
			return nil, errKeysConfig
		}
		var encoded string
		if err := decoder.Decode(&encoded); err != nil || len(encoded) != 44 {
			return nil, errKeysConfig
		}
		key, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil || len(key) != 32 || base64.StdEncoding.EncodeToString(key) != encoded {
			clear(key)
			return nil, errKeysConfig
		}
		keys[id] = key
	}
	end, err := decoder.Token()
	if err != nil || end != json.Delim('}') || len(keys) == 0 {
		return nil, errKeysConfig
	}
	var trailing any
	if decoder.Decode(&trailing) != io.EOF {
		return nil, errKeysConfig
	}
	valid = true
	return keys, nil
}

// Constructors have copied/expanded successful input; discard temporary bytes
// on both success and failure, without retaining a raw configuration structure.
func clearConfiguredKeys(keys map[string][]byte) {
	for _, key := range keys {
		clear(key)
	}
}
