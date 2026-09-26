package livekit

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strconv"
	"unicode/utf8"
)

var errWire = errors.New("invalid provider wire data")

func decodeObject(body []byte) (map[string]any, error) {
	if len(body) > maxResponse || !utf8.Valid(body) {
		return nil, errWire
	}
	d := json.NewDecoder(bytes.NewReader(body))
	d.UseNumber()
	v, err := parseValue(d, 1)
	if err != nil {
		return nil, errWire
	}
	if _, err = d.Token(); err != io.EOF {
		return nil, errWire
	}
	obj, ok := v.(map[string]any)
	if !ok {
		return nil, errWire
	}
	return obj, nil
}

// Token parsing catches duplicate keys even inside ignored provider fields.
func parseValue(d *json.Decoder, depth int) (any, error) {
	if depth > 32 {
		return nil, errWire
	}
	tok, err := d.Token()
	if err != nil {
		return nil, err
	}
	switch tok {
	case json.Delim('{'):
		obj := make(map[string]any)
		for d.More() {
			keyToken, err := d.Token()
			if err != nil {
				return nil, err
			}
			key, ok := keyToken.(string)
			if !ok {
				return nil, errWire
			}
			if _, duplicate := obj[key]; duplicate {
				return nil, errWire
			}
			value, err := parseValue(d, depth+1)
			if err != nil {
				return nil, err
			}
			obj[key] = value
		}
		end, err := d.Token()
		if err != nil || end != json.Delim('}') {
			return nil, errWire
		}
		return obj, nil
	case json.Delim('['):
		values := make([]any, 0)
		for d.More() {
			value, err := parseValue(d, depth+1)
			if err != nil {
				return nil, err
			}
			values = append(values, value)
		}
		end, err := d.Token()
		if err != nil || end != json.Delim(']') {
			return nil, errWire
		}
		return values, nil
	case json.Delim('}'), json.Delim(']'):
		return nil, errWire
	default:
		return tok, nil
	}
}

func field(obj map[string]any, snake, camel string) (any, bool, error) {
	a, haveA := obj[snake]
	b, haveB := obj[camel]
	if snake != camel && haveA && haveB {
		return nil, false, errWire
	}
	if haveA {
		return a, true, nil
	}
	return b, haveB, nil
}

func stringField(obj map[string]any, snake, camel string) (string, error) {
	v, present, err := field(obj, snake, camel)
	if err != nil {
		return "", err
	}
	if !present {
		return "", nil
	}
	s, ok := v.(string)
	if !ok {
		return "", errWire
	}
	return s, nil
}

func intField(obj map[string]any, snake, camel string) (int64, error) {
	v, present, err := field(obj, snake, camel)
	if err != nil {
		return 0, err
	}
	if !present {
		return 0, nil
	}
	var s string
	switch n := v.(type) {
	case string:
		s = n
	case json.Number:
		s = string(n)
	default:
		return 0, errWire
	}
	if s == "" {
		return 0, errWire
	}
	for i := range s {
		if s[i] < '0' || s[i] > '9' {
			return 0, errWire
		}
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, errWire
	}
	return n, nil
}

var statuses = [...]string{"EGRESS_STARTING", "EGRESS_ACTIVE", "EGRESS_ENDING", "EGRESS_COMPLETE", "EGRESS_FAILED", "EGRESS_ABORTED", "EGRESS_LIMIT_REACHED"}

func statusField(obj map[string]any) (string, error) {
	v, present := obj["status"]
	if !present {
		return statuses[0], nil
	}
	switch s := v.(type) {
	case string:
		for _, known := range statuses {
			if s == known {
				return s, nil
			}
		}
	case json.Number:
		if len(s) != 1 || s[0] < '0' || s[0] > '6' {
			return "", errWire
		}
		return statuses[int(s[0]-'0')], nil
	}
	return "", errWire
}

func observation(obj map[string]any) (Observation, error) {
	var out Observation
	var err error
	if out.EgressID, err = stringField(obj, "egress_id", "egressId"); err != nil || !idPattern.MatchString(out.EgressID) {
		return Observation{}, errWire
	}
	if out.RoomName, err = stringField(obj, "room_name", "roomName"); err != nil || !roomPattern.MatchString(out.RoomName) {
		return Observation{}, errWire
	}
	if out.Status, err = statusField(obj); err != nil {
		return Observation{}, errWire
	}
	if out.StartedAtNS, err = intField(obj, "started_at", "startedAt"); err != nil {
		return Observation{}, errWire
	}
	if out.UpdatedAtNS, err = intField(obj, "updated_at", "updatedAt"); err != nil {
		return Observation{}, errWire
	}
	if out.EndedAtNS, err = intField(obj, "ended_at", "endedAt"); err != nil {
		return Observation{}, errWire
	}
	return out, nil
}

func decodeObservation(body []byte) (Observation, error) {
	obj, err := decodeObject(body)
	if err != nil {
		return Observation{}, errWire
	}
	return observation(obj)
}

func decodeList(body []byte, target Target) (Observation, error) {
	obj, err := decodeObject(body)
	if err != nil {
		return Observation{}, ErrUnavailable
	}
	page, present, err := field(obj, "next_page_token", "nextPageToken")
	if err != nil {
		return Observation{}, ErrUnavailable
	}
	if present {
		pagination, ok := page.(map[string]any)
		if !ok {
			return Observation{}, ErrUnavailable
		}
		if len(pagination) > 1 {
			return Observation{}, ErrUnavailable
		}
		if token, exists := pagination["token"]; exists {
			s, ok := token.(string)
			if !ok || s != "" {
				return Observation{}, ErrUnavailable
			}
		}
	}
	items, present := obj["items"]
	if !present {
		return Observation{}, ErrNotObserved
	}
	rows, ok := items.([]any)
	if !ok {
		return Observation{}, ErrUnavailable
	}
	if len(rows) == 0 {
		return Observation{}, ErrNotObserved
	}
	if len(rows) != 1 {
		return Observation{}, ErrUnavailable
	}
	row, ok := rows[0].(map[string]any)
	if !ok {
		return Observation{}, ErrUnavailable
	}
	out, err := observation(row)
	if err != nil || out.RoomName != target.RoomName || out.EgressID != target.EgressID {
		return Observation{}, ErrUnavailable
	}
	return out, nil
}
