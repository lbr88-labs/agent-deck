package statedb

import (
	"encoding/json"
	"strings"
	"time"
)

const (
	codexBindingRevisionKey    = "codex_binding_revision"
	codexBindingConfirmedKey   = "codex_binding_confirmed"
	codexPromotionSourceKey    = "codex_promotion_source_id"
	codexPromotionOldRootKey   = "codex_promotion_old_root_id"
	codexPromotionTargetKey    = "codex_promotion_target_id"
	codexPromotionStartedKey   = "codex_promotion_started_at"
	codexPromotionCompletedKey = "codex_promotion_completed_at"
)

var codexPromotionKeys = []string{
	codexPromotionSourceKey,
	codexPromotionOldRootKey,
	codexPromotionTargetKey,
	codexPromotionStartedKey,
	codexPromotionCompletedKey,
}

func toolDataString(values map[string]any, key string) string {
	value, _ := values[key].(string)
	return strings.ToLower(strings.TrimSpace(value))
}

func toolDataPositiveInt64(values map[string]any, key string) int64 {
	value, ok := values[key].(float64)
	if !ok || value <= 0 || value != float64(int64(value)) {
		return 0
	}
	return int64(value)
}

func toolDataCodexBindingConfirmed(values map[string]any) bool {
	confirmed, _ := values[codexBindingConfirmedKey].(bool)
	return confirmed && toolDataString(values, "codex_session_id") != ""
}

// WriteCodexSessionBindingToToolData overwrites the serialized Codex tuple with
// a caller-captured snapshot. Storage uses this after instanceToRow so a
// concurrent in-memory edit cannot leak partially into an older save payload.
func WriteCodexSessionBindingToToolData(data json.RawMessage, sessionID string, detectedAt time.Time, revision int64, confirmed bool) json.RawMessage {
	var values map[string]any
	if len(data) == 0 {
		values = make(map[string]any)
	} else if json.Unmarshal(data, &values) != nil {
		return data
	}
	if values == nil {
		values = make(map[string]any)
	}
	sessionID = strings.ToLower(strings.TrimSpace(sessionID))
	values["codex_session_id"] = sessionID
	values[codexBindingConfirmedKey] = confirmed && sessionID != ""
	if detectedAt.IsZero() {
		delete(values, "codex_detected_at")
	} else {
		values["codex_detected_at"] = detectedAt.Unix()
	}
	if revision > 0 {
		values[codexBindingRevisionKey] = revision
	} else {
		delete(values, codexBindingRevisionKey)
	}
	out, err := json.Marshal(values)
	if err != nil {
		return data
	}
	return out
}

// ReadCodexSessionBindingFromToolData extracts the complete durable Codex
// binding tuple used by Storage after a whole-row save has been reconciled at
// SQLite's write boundary.
func ReadCodexSessionBindingFromToolData(data json.RawMessage) (string, time.Time, int64, bool) {
	var values map[string]any
	if json.Unmarshal(data, &values) != nil {
		return "", time.Time{}, 0, false
	}

	sessionID := toolDataString(values, "codex_session_id")
	detectedAt := time.Time{}
	if detectedAtUnix := toolDataPositiveInt64(values, "codex_detected_at"); detectedAtUnix > 0 {
		detectedAt = time.Unix(detectedAtUnix, 0)
	}
	return sessionID, detectedAt, toolDataPositiveInt64(values, codexBindingRevisionKey), toolDataCodexBindingConfirmed(values)
}

// stripCodexManagedExtras prevents MergeToolDataExtras from copying Codex
// identity fields individually as sticky or unknown keys. The entire tuple
// and promotion provenance are reconciled against the authoritative row in
// the active SQLite write transaction instead.
func stripCodexManagedExtras(data json.RawMessage) json.RawMessage {
	var values map[string]json.RawMessage
	if json.Unmarshal(data, &values) != nil || values == nil {
		return data
	}
	delete(values, "codex_session_id")
	delete(values, "codex_detected_at")
	delete(values, codexBindingRevisionKey)
	delete(values, codexBindingConfirmedKey)
	for _, key := range codexPromotionKeys {
		delete(values, key)
	}
	out, err := json.Marshal(values)
	if err != nil {
		return data
	}
	return out
}

func mergeToolDataExtrasWithoutCodexManaged(oldToolData, newToolData json.RawMessage) json.RawMessage {
	return MergeToolDataExtras(stripCodexManagedExtras(oldToolData), newToolData)
}

func ensureExplicitCodexBindingKey(data json.RawMessage) json.RawMessage {
	var values map[string]json.RawMessage
	if json.Unmarshal(data, &values) != nil {
		return data
	}
	if values == nil {
		values = make(map[string]json.RawMessage)
	}
	if _, ok := values["codex_session_id"]; !ok {
		values["codex_session_id"] = json.RawMessage(`""`)
	}
	out, err := json.Marshal(values)
	if err != nil {
		return data
	}
	return out
}

func copyCodexBinding(authoritative, incoming map[string]any, revision int64) {
	for _, key := range []string{"codex_session_id", "codex_detected_at"} {
		if value, ok := authoritative[key]; ok {
			incoming[key] = value
		} else {
			delete(incoming, key)
		}
	}
	incoming[codexBindingConfirmedKey] = toolDataCodexBindingConfirmed(authoritative)
	if revision > 0 {
		incoming[codexBindingRevisionKey] = revision
	} else {
		delete(incoming, codexBindingRevisionKey)
	}
	for _, key := range codexPromotionKeys {
		if value, ok := authoritative[key]; ok {
			incoming[key] = value
		}
	}
}

// reconcileCodexBindingToolData runs while a full-row save holds SQLite's
// write transaction. Targeted writers advance codex_binding_revision
// atomically, allowing this boundary to reject stale snapshots even after a
// completed promotion marker has been rotated away. The marker remains a
// compatibility defense for generation-zero promotions.
//
// Explicit user overrides are the one permitted way for a stale snapshot to
// supersede the authoritative binding: they receive a new generation greater
// than both versions and remove all migration provenance.
func reconcileCodexBindingToolData(existing, incoming json.RawMessage, explicitOverride bool) json.RawMessage {
	var oldValues, newValues map[string]any
	if json.Unmarshal(existing, &oldValues) != nil || json.Unmarshal(incoming, &newValues) != nil {
		return incoming
	}
	if newValues == nil {
		return incoming
	}

	// Promotion provenance is never accepted from a serialized process
	// snapshot. Only the authoritative row may contribute these managed keys.
	for _, key := range codexPromotionKeys {
		delete(newValues, key)
	}

	existingID := toolDataString(oldValues, "codex_session_id")
	incomingID := toolDataString(newValues, "codex_session_id")
	_, incomingHasID := newValues["codex_session_id"]
	existingRevision := toolDataPositiveInt64(oldValues, codexBindingRevisionKey)
	incomingRevision := toolDataPositiveInt64(newValues, codexBindingRevisionKey)

	if explicitOverride {
		if incomingRevision < existingRevision {
			incomingRevision = existingRevision
		}
		newValues[codexBindingRevisionKey] = incomingRevision + 1
	} else if !incomingHasID && (existingID != "" || existingRevision > 0) {
		// Omission still means an unaware writer, but preserve the whole
		// binding rather than attaching the old ID to incoming proof.
		copyCodexBinding(oldValues, newValues, existingRevision)
	} else if existingRevision > incomingRevision ||
		(existingRevision == incomingRevision &&
			((existingRevision > 0 && existingID != incomingID) ||
				(existingID == incomingID && toolDataCodexBindingConfirmed(oldValues) != toolDataCodexBindingConfirmed(newValues)))) {
		// A lower generation is stale. Equal committed generations cannot
		// change ownership proof without an explicit override, even for a
		// legacy generation-zero binding with the same ID.
		copyCodexBinding(oldValues, newValues, existingRevision)
	} else {
		sourceID := toolDataString(oldValues, codexPromotionSourceKey)
		oldRootID := toolDataString(oldValues, codexPromotionOldRootKey)
		targetID := toolDataString(oldValues, codexPromotionTargetKey)
		validPromotion := sourceID != "" && targetID != "" && existingID == targetID
		promotionPeer := incomingID == sourceID || incomingID == targetID || incomingID == "" ||
			(oldRootID != "" && incomingID == oldRootID)
		if validPromotion && promotionPeer {
			copyCodexBinding(oldValues, newValues, existingRevision)
		}
		// Otherwise the incoming generation/binding is newer (or this is a
		// generation-zero ordinary save) and intentionally rotates provenance.
	}
	if _, ok := newValues[codexBindingConfirmedKey]; ok {
		newValues[codexBindingConfirmedKey] = toolDataCodexBindingConfirmed(newValues)
	}

	merged, err := json.Marshal(newValues)
	if err != nil {
		return incoming
	}
	return merged
}
