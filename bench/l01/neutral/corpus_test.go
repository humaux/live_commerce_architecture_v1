// Purpose: verify the persisted L01-W1 language-neutral corpus and calibrate negative mutations via the public API.
// Depends on: Go standard library and livecommerce/bench/l01/neutral public symbols only; no DB, network or provider calls.
// Used by: go test ./bench/l01/neutral during offline corpus verification and future T19 integration checks.
// Invariants: I01, I02, I11, I18; persisted output must round-trip byte-identically and every body/signature/replay mutation must fail closed.
package neutral_test

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"livecommerce/bench/l01/neutral"
)

func TestPersistedCorpusPublicLoadAndDeterminism(t *testing.T) {
	persistedBytes, err := os.ReadFile(neutral.DefaultFilename)
	if err != nil {
		t.Fatalf("read persisted corpus %s: %v", neutral.DefaultFilename, err)
	}
	loaded, err := neutral.LoadFile(neutral.DefaultFilename)
	if err != nil {
		t.Fatalf("public LoadFile(%s) failed: %v", neutral.DefaultFilename, err)
	}

	rebuilt := neutral.Build(neutral.DefaultSeed)
	encoded, err := neutral.Encode(rebuilt)
	if err != nil {
		t.Fatalf("Encode(Build(%d)) failed: %v", neutral.DefaultSeed, err)
	}
	if !bytes.Equal(persistedBytes, encoded) {
		t.Fatalf("persisted %s drifted from deterministic Encode(Build(%d))", neutral.DefaultFilename, neutral.DefaultSeed)
	}

	var uniqueValid, duplicateReplay, invalidSig, admitCount, intentSum int
	var uniformBodyLen int
	for i, tc := range loaded.Cases {
		if i == 0 {
			uniformBodyLen = tc.BodyBytes
		} else if tc.BodyBytes != uniformBodyLen {
			t.Fatalf("case %d body_bytes=%d want uniform %d", i, tc.BodyBytes, uniformBodyLen)
		}
		switch tc.Classification {
		case neutral.ClassUniqueValid:
			uniqueValid++
		case neutral.ClassDuplicateReplay:
			duplicateReplay++
		case neutral.ClassInvalidSignature:
			invalidSig++
		default:
			t.Fatalf("unexpected classification %q", tc.Classification)
		}
		if tc.ExpectedAdmit {
			admitCount++
		}
		intentSum += tc.ExpectedIntentDelta
	}

	if uniqueValid != 60 || duplicateReplay != 20 || invalidSig != 20 || admitCount != 80 || intentSum != 60 {
		t.Fatalf("unexpected counts unique=%d replay=%d invalid=%d admit=%d intentSum=%d",
			uniqueValid, duplicateReplay, invalidSig, admitCount, intentSum)
	}
}

func TestVerifierNegativeMutationsAndPersistedRoundTrip(t *testing.T) {
	base, err := neutral.LoadFile(neutral.DefaultFilename)
	if err != nil {
		t.Fatalf("LoadFile baseline: %v", err)
	}

	firstIdx := func(class string) int {
		for i, tc := range base.Cases {
			if tc.Classification == class {
				return i
			}
		}
		t.Fatalf("missing classification %s", class)
		return -1
	}

	validIdx := firstIdx(neutral.ClassUniqueValid)
	replayIdx := firstIdx(neutral.ClassDuplicateReplay)
	invalidIdx := firstIdx(neutral.ClassInvalidSignature)
	otherValidIdx := -1
	for i, tc := range base.Cases {
		if tc.Classification == neutral.ClassUniqueValid && i != *base.Cases[replayIdx].ReplayOfIndex {
			otherValidIdx = i
			break
		}
	}
	if otherValidIdx < 0 {
		t.Fatal("missing distinct unique_valid case")
	}

	cases := []struct {
		name    string
		mutate  func(*neutral.Corpus)
		wantErr error
	}{
		{
			name: "mutated_body_without_digest_update",
			mutate: func(c *neutral.Corpus) {
				c.Cases[validIdx].RawBody = strings.Replace(c.Cases[validIdx].RawBody, `"time":123`, `"time":124`, 1)
			},
			wantErr: neutral.ErrBodyDigestMismatch,
		},
		{
			name: "mutated_body_with_recomputed_sha256_breaks_hmac",
			mutate: func(c *neutral.Corpus) {
				mutated := strings.Replace(c.Cases[validIdx].RawBody, `"time":123`, `"time":124`, 1)
				sum := sha256.Sum256([]byte(mutated))
				c.Cases[validIdx].RawBody = mutated
				c.Cases[validIdx].BodyBytes = len(mutated)
				c.Cases[validIdx].BodySHA256 = hex.EncodeToString(sum[:])
			},
			wantErr: neutral.ErrSignatureMismatch,
		},
		{
			name: "mutated_signature_on_unique_valid",
			mutate: func(c *neutral.Corpus) {
				sig := []byte(c.Cases[validIdx].SignatureHeader)
				if sig[len(sig)-1] == '0' {
					sig[len(sig)-1] = '1'
				} else {
					sig[len(sig)-1] = '0'
				}
				c.Cases[validIdx].SignatureHeader = string(sig)
			},
			wantErr: neutral.ErrSignatureMismatch,
		},
		{
			name: "valid_hmac_on_invalid_signature_case",
			mutate: func(c *neutral.Corpus) {
				c.Cases[invalidIdx].SignatureHeader = testSign(c.HMACSecret, []byte(c.Cases[invalidIdx].RawBody))
			},
			wantErr: neutral.ErrSignatureMismatch,
		},
		{
			name: "replay_mismatch_against_causal_primary",
			mutate: func(c *neutral.Corpus) {
				other := c.Cases[otherValidIdx]
				ref := *c.Cases[replayIdx].ReplayOfIndex
				c.Cases[replayIdx] = other
				c.Cases[replayIdx].Index = replayIdx
				c.Cases[replayIdx].Classification = neutral.ClassDuplicateReplay
				c.Cases[replayIdx].ReplayOfIndex = &ref
				c.Cases[replayIdx].ExpectedAdmit = true
				c.Cases[replayIdx].ExpectedIntentDelta = 0
			},
			wantErr: neutral.ErrReplayMismatch,
		},
		{
			name: "classification_token_leaked_into_signed_body",
			mutate: func(c *neutral.Corpus) {
				leakedID := fmt.Sprintf("m.w1.%s.000", neutral.ClassUniqueValid)
				raw := []byte(fmt.Sprintf(
					`{"object":"page","entry":[{"id":%q,"time":123,"messaging":[{"sender":{"id":"4"},"recipient":{"id":%q},"message":{"mid":%q,"text":%q}}]}]}`,
					c.AuthoritativeAssetID, c.AuthoritativeAssetID, leakedID, neutral.FixedBodyText,
				))
				sum := sha256.Sum256(raw)
				c.Cases[invalidIdx].EventID = leakedID
				c.Cases[invalidIdx].RawBody = string(raw)
				c.Cases[invalidIdx].BodyBytes = len(raw)
				c.Cases[invalidIdx].BodySHA256 = hex.EncodeToString(sum[:])
				c.Cases[invalidIdx].SignatureHeader = "sha256=" + strings.Repeat("0", 64)
			},
			wantErr: neutral.ErrClassificationInBody,
		},
		{
			name: "empty_or_truncated_cases",
			mutate: func(c *neutral.Corpus) {
				c.Cases = c.Cases[:len(c.Cases)-1]
			},
			wantErr: neutral.ErrInvalidCorpus,
		},
	}

	tmpDir := t.TempDir()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			bad := cloneCorpus(base)
			tc.mutate(&bad)
			err := neutral.Verify(bad)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("Verify error=%v, want %v", err, tc.wantErr)
			}
			// Also prove the public persisted file path (LoadFile -> Parse -> Verify) rejects tampered JSON on disk.
			raw, mErr := marshalUncheckedJSON(bad)
			if mErr != nil {
				t.Fatalf("marshal mutated corpus: %v", mErr)
			}
			badPath := filepath.Join(tmpDir, tc.name+".json")
			if wErr := os.WriteFile(badPath, raw, 0o600); wErr != nil {
				t.Fatalf("write mutated file: %v", wErr)
			}
			if _, loadErr := neutral.LoadFile(badPath); !errors.Is(loadErr, tc.wantErr) {
				t.Fatalf("LoadFile error=%v, want %v", loadErr, tc.wantErr)
			}
		})
	}

	t.Run("duplicate_json_key_rejected_by_parse", func(t *testing.T) {
		dupJSON := []byte(`{"schema":"livecommerce/bench-l01-w1-neutral-corpus/v1","schema":"tampered"}`)
		if _, err := neutral.Parse(dupJSON); !errors.Is(err, neutral.ErrInvalidCorpus) {
			t.Fatalf("Parse duplicate key err=%v, want ErrInvalidCorpus", err)
		}
	})
}

func TestStrictJSONMemberSpellingAndMandatoryFields(t *testing.T) {
	raw, err := os.ReadFile(neutral.DefaultFilename)
	if err != nil {
		t.Fatalf("read persisted fixture: %v", err)
	}
	replaceOnce := func(t *testing.T, old, repl string) []byte {
		t.Helper()
		if !bytes.Contains(raw, []byte(old)) {
			t.Fatalf("fixture anchor not found: %s", old)
		}
		return bytes.Replace(raw, []byte(old), []byte(repl), 1)
	}

	rejections := []struct {
		name string
		raw  []byte
	}{
		{
			name: "case_folded_alias_overrides_expected_admit",
			raw:  replaceOnce(t, `"expected_admit":true,`, `"expected_admit":false,"EXPECTED_ADMIT":true,`),
		},
		{
			name: "case_folded_alias_replaces_expected_admit",
			raw:  replaceOnce(t, `"expected_admit":true,`, `"EXPECTED_ADMIT":true,`),
		},
		{
			name: "envelope_case_folded_seed_alias",
			raw:  replaceOnce(t, `"seed": 1901101,`, `"SEED": 1901101,`),
		},
		{
			name: "counts_case_folded_total_alias",
			raw:  replaceOnce(t, `"total":100,`, `"TOTAL":100,`),
		},
		{
			name: "case_folded_replay_of_index_alias",
			raw:  replaceOnce(t, `"replay_of_index":10,`, `"REPLAY_OF_INDEX":10,`),
		},
		{
			name: "case_index_omitted",
			raw:  replaceOnce(t, `"index":0,`, ``),
		},
		{
			name: "case_index_null",
			raw:  replaceOnce(t, `"index":0,`, `"index":null,`),
		},
		{
			name: "case_expected_intent_delta_omitted",
			raw:  replaceOnce(t, `"expected_intent_delta":0,`, ``),
		},
		{
			name: "case_expected_intent_delta_null",
			raw:  replaceOnce(t, `"expected_intent_delta":0,`, `"expected_intent_delta":null,`),
		},
		{
			name: "case_expected_admit_false_omitted",
			raw:  replaceOnce(t, `"expected_admit":false,`, ``),
		},
		{
			name: "case_expected_admit_null",
			raw:  replaceOnce(t, `"expected_admit":false,`, `"expected_admit":null,`),
		},
		{
			name: "replay_case_replay_of_index_null",
			raw:  replaceOnce(t, `"replay_of_index":10,`, `"replay_of_index":null,`),
		},
		{
			name: "replay_case_replay_of_index_omitted",
			raw:  replaceOnce(t, `"replay_of_index":10,`, ``),
		},
		{
			name: "non_replay_case_replay_of_index_zero",
			raw:  replaceOnce(t, `"index":0,"classification":"unique_valid",`, `"index":0,"classification":"unique_valid","replay_of_index":0,`),
		},
		{
			name: "non_replay_case_replay_of_index_null",
			raw:  replaceOnce(t, `"index":0,"classification":"unique_valid",`, `"index":0,"classification":"unique_valid","replay_of_index":null,`),
		},
		{
			name: "cases_array_null_element",
			raw:  replaceOnce(t, `{"index":0,`, `null,{"index":0,`),
		},
		{
			name: "envelope_seed_omitted",
			raw:  replaceOnce(t, `"seed": 1901101,`, ``),
		},
		{
			name: "envelope_seed_null",
			raw:  replaceOnce(t, `"seed": 1901101,`, `"seed": null,`),
		},
		{
			name: "envelope_synthetic_only_null",
			raw:  replaceOnce(t, `"synthetic_only": true,`, `"synthetic_only": null,`),
		},
		{
			name: "envelope_counts_null",
			raw:  replaceOnce(t, `"counts": {"total":100,"unique_valid":60,"duplicate_replay":20,"invalid_signature":20},`, `"counts": null,`),
		},
		{
			name: "counts_unique_valid_omitted",
			raw:  replaceOnce(t, `"unique_valid":60,`, ``),
		},
		{
			name: "counts_invalid_signature_null",
			raw:  replaceOnce(t, `"invalid_signature":20`, `"invalid_signature":null`),
		},
	}

	tmpDir := t.TempDir()
	for _, tc := range rejections {
		t.Run("reject_"+tc.name, func(t *testing.T) {
			if _, pErr := neutral.Parse(tc.raw); !errors.Is(pErr, neutral.ErrInvalidCorpus) {
				t.Errorf("Parse(%s) err=%v, want ErrInvalidCorpus", tc.name, pErr)
			}
			p := filepath.Join(tmpDir, tc.name+".json")
			if wErr := os.WriteFile(p, tc.raw, 0o600); wErr != nil {
				t.Fatalf("write %s: %v", p, wErr)
			}
			if _, lErr := neutral.LoadFile(p); !errors.Is(lErr, neutral.ErrInvalidCorpus) {
				t.Errorf("LoadFile(%s) err=%v, want ErrInvalidCorpus", tc.name, lErr)
			}
		})
	}

	t.Run("accept_seed_zero_explicit_false_and_zero_including_replay_of_zero", func(t *testing.T) {
		var sawReplayZero bool
		for _, seed := range []int64{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, -1, 42} {
			built := neutral.Build(seed)
			for _, tc := range built.Cases {
				if tc.ReplayOfIndex != nil && *tc.ReplayOfIndex == 0 {
					sawReplayZero = true
				}
			}
			enc, err := neutral.Encode(built)
			if err != nil {
				t.Fatalf("Encode(Build(%d)): %v", seed, err)
			}
			parsed, err := neutral.Parse(enc)
			if err != nil || parsed.Seed != seed {
				t.Fatalf("Parse(Encode(Build(%d))) seed=%d err=%v", seed, parsed.Seed, err)
			}
			if seed == 0 {
				p := filepath.Join(tmpDir, "seed_zero.json")
				if err := os.WriteFile(p, enc, 0o600); err != nil {
					t.Fatalf("write seed_zero: %v", err)
				}
				loaded, err := neutral.LoadFile(p)
				if err != nil || loaded.Seed != 0 {
					t.Fatalf("LoadFile(seed_zero) seed=%d err=%v", loaded.Seed, err)
				}
			}
		}
		if !sawReplayZero {
			t.Fatal("expected at least one deterministic seed to exercise replay_of_index=0")
		}
	})

	t.Run("accept_reordered_fields_whitespace_and_optional_replay_pointer", func(t *testing.T) {
		base, err := neutral.LoadFile(neutral.DefaultFilename)
		if err != nil {
			t.Fatalf("LoadFile baseline: %v", err)
		}
		reordered := marshalReorderedIndentedJSON(base)
		parsed, err := neutral.Parse(reordered)
		if err != nil {
			t.Fatalf("Parse(reordered) failed: %v", err)
		}
		p := filepath.Join(tmpDir, "reordered.json")
		if err := os.WriteFile(p, reordered, 0o600); err != nil {
			t.Fatalf("write reordered: %v", err)
		}
		loaded, err := neutral.LoadFile(p)
		if err != nil {
			t.Fatalf("LoadFile(reordered) failed: %v", err)
		}
		canonical, err := neutral.Encode(loaded)
		if err != nil || !bytes.Equal(canonical, raw) || parsed.Seed != base.Seed {
			t.Fatalf("reordered corpus did not round-trip to canonical fixture bytes: err=%v", err)
		}
	})
}

func cloneCorpus(src neutral.Corpus) neutral.Corpus {
	out := src
	out.Cases = make([]neutral.Case, len(src.Cases))
	for i, tc := range src.Cases {
		out.Cases[i] = tc
		if tc.ReplayOfIndex != nil {
			ref := *tc.ReplayOfIndex
			out.Cases[i].ReplayOfIndex = &ref
		}
	}
	return out
}

func testSign(secret string, raw []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(raw)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func marshalUncheckedJSON(c neutral.Corpus) ([]byte, error) {
	// Bypass Encode's pre-Verify guard so LoadFile/Parse must catch persisted corruption itself.
	var buf bytes.Buffer
	buf.WriteString(`{"schema":` + fmt.Sprintf("%q", c.Schema) +
		`,"evidence_class":` + fmt.Sprintf("%q", c.EvidenceClass) +
		`,"seed":` + fmt.Sprintf("%d", c.Seed) +
		`,"synthetic_only":` + fmt.Sprintf("%v", c.SyntheticOnly) +
		`,"app_id":` + fmt.Sprintf("%q", c.AppID) +
		`,"object":` + fmt.Sprintf("%q", c.Object) +
		`,"hmac_secret":` + fmt.Sprintf("%q", c.HMACSecret) +
		`,"authoritative_asset_id":` + fmt.Sprintf("%q", c.AuthoritativeAssetID) +
		`,"authoritative_tenant_id":` + fmt.Sprintf("%q", c.AuthoritativeTenantID) +
		`,"spoofed_tenant_header":` + fmt.Sprintf("%q", c.SpoofedTenantHeader) +
		fmt.Sprintf(`,"counts":{"total":%d,"unique_valid":%d,"duplicate_replay":%d,"invalid_signature":%d},"cases":[`,
			c.Counts.Total, c.Counts.UniqueValid, c.Counts.DuplicateReplay, c.Counts.InvalidSignature))
	for i, tc := range c.Cases {
		if i > 0 {
			buf.WriteByte(',')
		}
		replayField := ""
		if tc.ReplayOfIndex != nil {
			replayField = fmt.Sprintf(`,"replay_of_index":%d`, *tc.ReplayOfIndex)
		}
		buf.WriteString(fmt.Sprintf(
			`{"index":%d,"classification":%q%s,"expected_admit":%v,"expected_intent_delta":%d,"event_id":%q,"content_type":%q,"signature_header":%q,"untrusted_tenant_header":%q,"body_bytes":%d,"body_sha256":%q,"raw_body":%q}`,
			tc.Index, tc.Classification, replayField, tc.ExpectedAdmit, tc.ExpectedIntentDelta,
			tc.EventID, tc.ContentType, tc.SignatureHeader, tc.UntrustedTenantHeader,
			tc.BodyBytes, tc.BodySHA256, tc.RawBody,
		))
	}
	buf.WriteString("]}\n")
	return buf.Bytes(), nil
}

func marshalReorderedIndentedJSON(c neutral.Corpus) []byte {
	var buf bytes.Buffer
	buf.WriteString("{\n\t\"cases\": [\n")
	for i, tc := range c.Cases {
		if i > 0 {
			buf.WriteString(",\n")
		}
		replayLine := ""
		if tc.ReplayOfIndex != nil {
			replayLine = fmt.Sprintf("\n\t\t\t\"replay_of_index\": %d,", *tc.ReplayOfIndex)
		}
		buf.WriteString(fmt.Sprintf(
			"\t\t{\n\t\t\t\"raw_body\": %q,\n\t\t\t\"body_sha256\": %q,\n\t\t\t\"body_bytes\": %d,\n\t\t\t\"untrusted_tenant_header\": %q,\n\t\t\t\"signature_header\": %q,\n\t\t\t\"content_type\": %q,\n\t\t\t\"event_id\": %q,%s\n\t\t\t\"expected_intent_delta\": %d,\n\t\t\t\"expected_admit\": %v,\n\t\t\t\"classification\": %q,\n\t\t\t\"index\": %d\n\t\t}",
			tc.RawBody, tc.BodySHA256, tc.BodyBytes, tc.UntrustedTenantHeader,
			tc.SignatureHeader, tc.ContentType, tc.EventID, replayLine,
			tc.ExpectedIntentDelta, tc.ExpectedAdmit, tc.Classification, tc.Index,
		))
	}
	buf.WriteString(fmt.Sprintf(
		"\n\t],\n\t\"counts\": {\n\t\t\"invalid_signature\": %d,\n\t\t\"duplicate_replay\": %d,\n\t\t\"unique_valid\": %d,\n\t\t\"total\": %d\n\t},\n\t\"spoofed_tenant_header\": %q,\n\t\"authoritative_tenant_id\": %q,\n\t\"authoritative_asset_id\": %q,\n\t\"hmac_secret\": %q,\n\t\"object\": %q,\n\t\"app_id\": %q,\n\t\"synthetic_only\": %v,\n\t\"seed\": %d,\n\t\"evidence_class\": %q,\n\t\"schema\": %q\n}\n",
		c.Counts.InvalidSignature, c.Counts.DuplicateReplay, c.Counts.UniqueValid, c.Counts.Total,
		c.SpoofedTenantHeader, c.AuthoritativeTenantID, c.AuthoritativeAssetID,
		c.HMACSecret, c.Object, c.AppID, c.SyntheticOnly, c.Seed, c.EvidenceClass, c.Schema,
	))
	return buf.Bytes()
}
