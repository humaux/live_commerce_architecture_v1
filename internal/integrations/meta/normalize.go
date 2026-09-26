package meta

import "time"

func (v *Verifier) normalize(root map[string]any, b *Batch) error {
	if stringField(root, "object") != v.object {
		return v.emit(b, "", "quarantine", "", "object_mismatch", root, nil)
	}
	for key := range root {
		if key != "object" && key != "entry" {
			if err := v.emit(b, "", "quarantine", "", "unknown_root_field", root, nil); err != nil {
				return err
			}
			break
		}
	}
	entries, ok := root["entry"].([]any)
	if !ok || len(entries) == 0 {
		return v.emit(b, "", "quarantine", "", "empty_or_invalid_entries", root, nil)
	}
	for _, rawEntry := range entries {
		entry, ok := object(rawEntry)
		if !ok {
			if err := v.emit(b, "", "quarantine", "", "invalid_entry", rawEntry, nil); err != nil {
				return err
			}
			continue
		}
		assetID := stringField(entry, "id")
		if !digits(assetID) {
			if err := v.emit(b, "", "quarantine", "", "invalid_asset_id", entry, seconds(entry["time"])); err != nil {
				return err
			}
			continue
		}
		at := seconds(entry["time"])
		context := map[string]any{"id": assetID}
		if source, present := entry["time"]; present {
			if at == nil {
				if err := v.emit(b, assetID, "quarantine", "", "invalid_entry_time", entry, nil); err != nil {
					return err
				}
			} else {
				context["time"] = source
			}
		}
		for key := range entry {
			if key != "id" && key != "time" && key != "changes" && key != "messaging" {
				if err := v.emit(b, assetID, "quarantine", "", "unknown_entry_field", entry, at); err != nil {
					return err
				}
				break
			}
		}
		children := 0
		if value, present := entry["changes"]; present {
			units, valid := value.([]any)
			if !valid {
				children++
				if err := v.emit(b, assetID, "quarantine", "", "invalid_changes", entry, at); err != nil {
					return err
				}
			} else {
				for _, unit := range units {
					children++
					if err := v.change(b, assetID, context, unit, at); err != nil {
						return err
					}
				}
			}
		}
		if value, present := entry["messaging"]; present {
			units, valid := value.([]any)
			if !valid {
				children++
				if err := v.emit(b, assetID, "quarantine", "", "invalid_messaging", entry, at); err != nil {
					return err
				}
			} else {
				for _, unit := range units {
					children++
					if err := v.message(b, assetID, context, unit, at); err != nil {
						return err
					}
				}
			}
		}
		if children == 0 {
			if err := v.emit(b, assetID, "quarantine", "", "empty_entry", entry, at); err != nil {
				return err
			}
		}
	}
	return nil
}

func quarantineUnit(context map[string]any, unit any) any {
	// Keep only delivery context; copying every sibling into each quarantine
	// would amplify a bounded request into quadratic retained payload bytes.
	return map[string]any{"entry": context, "unit": unit}
}

func (v *Verifier) change(b *Batch, assetID string, context map[string]any, unit any, fallback *time.Time) error {
	change, ok := object(unit)
	if !ok {
		return v.emit(b, assetID, "quarantine", "", "unsupported_change", quarantineUnit(context, unit), fallback)
	}
	value, ok := object(change["value"])
	if !ok {
		return v.emit(b, assetID, "quarantine", "", "unsupported_change", quarantineUnit(context, unit), fallback)
	}
	if v.object == "page" && stringField(change, "field") == "feed" && stringField(value, "item") == "comment" {
		verb := stringField(value, "verb")
		id := stringField(value, "comment_id")
		if (verb == "add" || verb == "edit" || verb == "remove") && commentID(id) {
			at := fallback
			if source, present := value["created_time"]; present {
				at = seconds(source)
			}
			return v.emit(b, assetID, "page_comment_"+verb, id, "", change, at)
		}
	}
	if v.object == "instagram" {
		field := stringField(change, "field")
		id := stringField(value, "id")
		if (field == "comments" || field == "live_comments") && digits(id) {
			kind := "instagram_comment"
			if field == "live_comments" {
				kind = "instagram_live_comment"
			}
			return v.emit(b, assetID, kind, id, "", change, fallback)
		}
	}
	return v.emit(b, assetID, "quarantine", "", "unsupported_change", quarantineUnit(context, unit), fallback)
}

func commentID(s string) bool {
	if len(s) < 1 || len(s) > 200 {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !((c >= '0' && c <= '9') || (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || c == '_' || c == '-' || c == ':' || c == '.') {
			return false
		}
	}
	return true
}

func messageID(s string) bool {
	if len(s) < 1 || len(s) > 1024 {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < 0x20 || s[i] > 0x7e {
			return false
		}
	}
	return true
}

func (v *Verifier) message(b *Batch, assetID string, context map[string]any, unit any, fallback *time.Time) error {
	m, ok := object(unit)
	if !ok {
		return v.emit(b, assetID, "quarantine", "", "unsupported_messaging", quarantineUnit(context, unit), fallback)
	}
	sender, senderOK := object(m["sender"])
	recipient, recipientOK := object(m["recipient"])
	message, messageOK := object(m["message"])
	if !senderOK || !recipientOK || !messageOK {
		return v.emit(b, assetID, "quarantine", "", "unsupported_messaging", quarantineUnit(context, unit), fallback)
	}
	senderID, recipientID := stringField(sender, "id"), stringField(recipient, "id")
	mid := stringField(message, "mid")
	echo, echoPresent := message["is_echo"]
	if !digits(senderID) || !digits(recipientID) || senderID == assetID || recipientID != assetID || !messageID(mid) || (echoPresent && echo != false) {
		return v.emit(b, assetID, "quarantine", "", "unsupported_messaging", quarantineUnit(context, unit), fallback)
	}
	kind := "page_message"
	if v.object == "instagram" {
		kind = "instagram_message"
	}
	at := fallback
	if source, present := m["timestamp"]; present {
		at = millis(source)
	}
	return v.emit(b, assetID, kind, mid, "", m, at)
}
