package statedb

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func openCodexBindingTestDB(t *testing.T) *StateDB {
	t.Helper()
	db, err := Open(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := db.Migrate(); err != nil {
		_ = db.Close()
		t.Fatalf("Migrate: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func codexBindingRow(id, sessionID string) *InstanceRow {
	toolData := json.RawMessage(`{}`)
	if sessionID != "" {
		toolData = json.RawMessage(`{"codex_session_id":"` + sessionID + `","codex_detected_at":1}`)
	}
	return &InstanceRow{
		ID: id, Title: "test", ProjectPath: "/tmp/test", GroupPath: "default",
		Command: "codex", Tool: "codex", Status: "idle", CreatedAt: time.Now(), ToolData: toolData,
	}
}

func readCodexBindingToolData(t *testing.T, db *StateDB, id string) json.RawMessage {
	t.Helper()
	var raw string
	if err := db.db.QueryRow("SELECT tool_data FROM instances WHERE id = ?", id).Scan(&raw); err != nil {
		t.Fatalf("read tool_data: %v", err)
	}
	return json.RawMessage(raw)
}

func TestCodexSessionBindingToolDataHelpers(t *testing.T) {
	detectedAt := time.Unix(1_725_000_123, 0)
	raw := json.RawMessage(`{
		"codex_session_id":"AAAAAAAA-AAAA-4AAA-8AAA-AAAAAAAAAAAA",
		"codex_detected_at":1725000123,
		"unrelated":"preserved"
	}`)
	raw = WriteCodexSessionBindingToToolData(raw, "AAAAAAAA-AAAA-4AAA-8AAA-AAAAAAAAAAAA", detectedAt, 7, false)

	id, gotDetectedAt, revision, confirmed := ReadCodexSessionBindingFromToolData(raw)
	if id != "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa" {
		t.Fatalf("session ID = %q, want normalized lowercase", id)
	}
	if !gotDetectedAt.Equal(detectedAt) {
		t.Fatalf("detected at = %v, want %v", gotDetectedAt, detectedAt)
	}
	if revision != 7 {
		t.Fatalf("binding tuple revision = %d, want 7", revision)
	}
	if confirmed {
		t.Fatal("revision alone must not confirm a legacy binding")
	}

	raw = WriteCodexSessionBindingToToolData(raw, id, detectedAt, 0, false)
	if _, _, got, _ := ReadCodexSessionBindingFromToolData(raw); got != 0 {
		t.Fatalf("removed revision = %d, want 0", got)
	}
	var values map[string]any
	if err := json.Unmarshal(raw, &values); err != nil {
		t.Fatalf("unmarshal helper result: %v", err)
	}
	if _, ok := values[codexBindingRevisionKey]; ok {
		t.Fatal("non-positive revision should remove key")
	}
	if values["unrelated"] != "preserved" {
		t.Fatalf("unrelated field = %v, want preserved", values["unrelated"])
	}
}

func TestTargetedCodexBindingWritesAdvanceRevisionAndRotatePromotion(t *testing.T) {
	const (
		id       = "targeted-revision"
		sourceID = "11111111-1111-4111-8111-111111111111"
		oldRoot  = "22222222-2222-4222-8222-222222222222"
		targetID = "33333333-3333-4333-8333-333333333333"
		newID    = "44444444-4444-4444-8444-444444444444"
	)
	db := openCodexBindingTestDB(t)
	if err := db.SaveInstance(codexBindingRow(id, sourceID)); err != nil {
		t.Fatalf("seed: %v", err)
	}

	promotionRevision, matched, err := db.WriteCodexSessionPromotion(
		id, targetID, time.Unix(100, 0), sourceID, oldRoot, time.Unix(90, 0), time.Unix(100, 0),
		0, true,
	)
	if err != nil || !matched || promotionRevision != 1 {
		t.Fatalf("promotion: revision=%d matched=%v err=%v", promotionRevision, matched, err)
	}
	bindingRevision, matched, err := db.WriteCodexSessionBinding(id, newID, time.Unix(200, 0), promotionRevision, false)
	if err != nil || !matched || bindingRevision != 2 {
		t.Fatalf("binding: revision=%d matched=%v err=%v", bindingRevision, matched, err)
	}

	gotID, gotDetectedAt, gotRevision, confirmed, err := db.ReadCodexSessionBinding(id)
	if err != nil {
		t.Fatalf("read binding: %v", err)
	}
	if gotID != newID || !gotDetectedAt.Equal(time.Unix(200, 0)) || gotRevision != 2 || confirmed {
		t.Fatalf("binding = (%q, %v, %d, %v), want (%q, %v, 2, false)", gotID, gotDetectedAt, gotRevision, confirmed, newID, time.Unix(200, 0))
	}
	var values map[string]any
	if err := json.Unmarshal(readCodexBindingToolData(t, db, id), &values); err != nil {
		t.Fatalf("unmarshal tool_data: %v", err)
	}
	for _, key := range codexPromotionKeys {
		if _, ok := values[key]; ok {
			t.Errorf("ordinary targeted binding retained promotion key %q", key)
		}
	}
}

func TestCodexExplicitClearRevisionSurvivesStaleFullRowWriters(t *testing.T) {
	const (
		id       = "durable-clear"
		sourceID = "55555555-5555-4555-8555-555555555555"
		oldRoot  = "66666666-6666-4666-8666-666666666666"
		targetID = "77777777-7777-4777-8777-777777777777"
	)

	for _, saveOne := range []bool{false, true} {
		name := "SaveInstances"
		if saveOne {
			name = "SaveInstance"
		}
		t.Run(name, func(t *testing.T) {
			db := openCodexBindingTestDB(t)
			if err := db.SaveInstance(codexBindingRow(id, sourceID)); err != nil {
				t.Fatalf("seed: %v", err)
			}
			promotionRevision, matched, err := db.WriteCodexSessionPromotion(
				id, targetID, time.Unix(100, 0), sourceID, oldRoot, time.Unix(90, 0), time.Unix(100, 0),
				0, true,
			)
			if err != nil || !matched {
				t.Fatalf("promote: revision=%d matched=%v err=%v", promotionRevision, matched, err)
			}
			stale := codexBindingRow(id, targetID)
			stale.ToolData = append(json.RawMessage(nil), readCodexBindingToolData(t, db, id)...)

			clear := codexBindingRow(id, "")
			clear.CodexBindingOverrideIntent = true
			if saveOne {
				err = db.SaveInstance(clear)
			} else {
				err = db.SaveInstances([]*InstanceRow{clear})
			}
			if err != nil {
				t.Fatalf("explicit clear: %v", err)
			}
			if _, _, got, _ := ReadCodexSessionBindingFromToolData(clear.ToolData); got != promotionRevision+1 {
				t.Fatalf("returned clear revision = %d, want %d", got, promotionRevision+1)
			}

			// A process serialized before the clear still carries the target and
			// completed-promotion keys at the previous generation.
			if saveOne {
				err = db.SaveInstance(stale)
			} else {
				err = db.SaveInstances([]*InstanceRow{stale})
			}
			if err != nil {
				t.Fatalf("stale save: %v", err)
			}
			gotID, _, gotRevision, confirmed, err := db.ReadCodexSessionBinding(id)
			if err != nil {
				t.Fatalf("read after stale save: %v", err)
			}
			if gotID != "" || gotRevision != promotionRevision+1 || confirmed {
				t.Fatalf("stale save restored (%q, rev %d, confirmed %v), want unconfirmed clear at rev %d", gotID, gotRevision, confirmed, promotionRevision+1)
			}
			var values map[string]any
			if err := json.Unmarshal(readCodexBindingToolData(t, db, id), &values); err != nil {
				t.Fatalf("unmarshal final tool_data: %v", err)
			}
			for _, key := range codexPromotionKeys {
				if _, ok := values[key]; ok {
					t.Errorf("stale save resurrected promotion key %q", key)
				}
			}
		})
	}
}

func TestCodexBindingReconcileUsesRevisionOrdering(t *testing.T) {
	const (
		id          = "revision-order"
		authorityID = "88888888-8888-4888-8888-888888888888"
		conflictID  = "99999999-9999-4999-8999-999999999999"
		newerID     = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	)
	db := openCodexBindingTestDB(t)
	seed := codexBindingRow(id, authorityID)
	seed.ToolData = WriteCodexSessionBindingToToolData(seed.ToolData, authorityID, time.Unix(1, 0), 4, false)
	if err := db.SaveInstance(seed); err != nil {
		t.Fatalf("seed: %v", err)
	}

	equalConflict := codexBindingRow(id, conflictID)
	equalConflict.ToolData = WriteCodexSessionBindingToToolData(equalConflict.ToolData, conflictID, time.Unix(1, 0), 4, false)
	if err := db.SaveInstances([]*InstanceRow{equalConflict}); err != nil {
		t.Fatalf("equal-generation conflict save: %v", err)
	}
	gotID, _, gotRevision, _, err := db.ReadCodexSessionBinding(id)
	if err != nil {
		t.Fatalf("read equal conflict: %v", err)
	}
	if gotID != authorityID || gotRevision != 4 {
		t.Fatalf("equal conflict committed (%q, rev %d), want authoritative (%q, rev 4)", gotID, gotRevision, authorityID)
	}

	newer := codexBindingRow(id, newerID)
	newer.ToolData = WriteCodexSessionBindingToToolData(newer.ToolData, newerID, time.Unix(1, 0), 5, false)
	if err := db.SaveInstances([]*InstanceRow{newer}); err != nil {
		t.Fatalf("newer-generation save: %v", err)
	}
	gotID, _, gotRevision, _, err = db.ReadCodexSessionBinding(id)
	if err != nil {
		t.Fatalf("read newer binding: %v", err)
	}
	if gotID != newerID || gotRevision != 5 {
		t.Fatalf("newer save committed (%q, rev %d), want (%q, rev 5)", gotID, gotRevision, newerID)
	}
}

func TestSaveInstancesPreservesCompletedCodexPromotionAtWriteBoundary(t *testing.T) {
	const (
		id       = "instance"
		sourceID = "11111111-1111-4111-8111-111111111111"
		oldRoot  = "22222222-2222-4222-8222-222222222222"
		targetID = "33333333-3333-4333-8333-333333333333"
	)

	for _, staleID := range []string{sourceID, oldRoot, ""} {
		t.Run("stale_"+staleID, func(t *testing.T) {
			db := openCodexBindingTestDB(t)
			if err := db.SaveInstance(codexBindingRow(id, sourceID)); err != nil {
				t.Fatalf("seed: %v", err)
			}
			revision, matched, err := db.WriteCodexSessionPromotion(
				id, targetID, time.Now(), sourceID, oldRoot,
				time.Now().Add(-time.Second), time.Now(),
				0, true,
			)
			if err != nil || !matched {
				t.Fatalf("promote: revision=%d matched=%v err=%v", revision, matched, err)
			}

			// This row represents a long-running process that serialized its stale
			// snapshot before the targeted promotion committed.
			stale := codexBindingRow(id, staleID)
			stale.Title = "unrelated stale-process edit"
			if err := db.SaveInstances([]*InstanceRow{stale}); err != nil {
				t.Fatalf("SaveInstances: %v", err)
			}
			got, _, _, confirmed, err := db.ReadCodexSessionBinding(id)
			if err != nil {
				t.Fatalf("ReadCodexSessionBinding: %v", err)
			}
			if got != targetID || !confirmed {
				t.Fatalf("stale %q replaced promoted target: got (%q, confirmed %v), want (%q, confirmed true)", staleID, got, confirmed, targetID)
			}
		})
	}
}

func TestSaveInstancesAllowsExplicitCodexBindingClearAfterPromotion(t *testing.T) {
	const (
		id       = "instance-clear"
		sourceID = "44444444-4444-4444-8444-444444444444"
		oldRoot  = "55555555-5555-4555-8555-555555555555"
		targetID = "66666666-6666-4666-8666-666666666666"
	)
	db := openCodexBindingTestDB(t)
	if err := db.SaveInstance(codexBindingRow(id, sourceID)); err != nil {
		t.Fatalf("seed: %v", err)
	}
	_, matched, err := db.WriteCodexSessionPromotion(id, targetID, time.Now(), sourceID, oldRoot, time.Now(), time.Now(), 0, true)
	if err != nil || !matched {
		t.Fatalf("promote: matched=%v err=%v", matched, err)
	}

	clearRow := codexBindingRow(id, "")
	clearRow.CodexBindingOverrideIntent = true
	if err := db.SaveInstances([]*InstanceRow{clearRow}); err != nil {
		t.Fatalf("SaveInstances clear: %v", err)
	}
	got, _, _, confirmed, err := db.ReadCodexSessionBinding(id)
	if err != nil {
		t.Fatalf("ReadCodexSessionBinding: %v", err)
	}
	if got != "" || confirmed {
		t.Fatalf("explicit clear preserved promoted target (%q, confirmed %v)", got, confirmed)
	}
}

func TestWriteCodexBindingReportsMissingRow(t *testing.T) {
	db := openCodexBindingTestDB(t)
	revision, matched, err := db.WriteCodexSessionBinding("missing", "77777777-7777-4777-8777-777777777777", time.Now(), 0, true)
	if err != nil {
		t.Fatalf("WriteCodexSessionBinding: %v", err)
	}
	if matched {
		t.Fatal("missing row reported a successful binding write")
	}
	if revision != 0 {
		t.Fatalf("missing row revision = %d, want 0", revision)
	}
}

func TestCodexBindingCASRejectsStaleRevision(t *testing.T) {
	const (
		id      = "cas-conflict"
		oldID   = "11111111-aaaa-4aaa-8aaa-111111111111"
		staleID = "22222222-bbbb-4bbb-8bbb-222222222222"
		oldRoot = "33333333-cccc-4ccc-8ccc-333333333333"
	)
	db := openCodexBindingTestDB(t)
	if err := db.SaveInstance(codexBindingRow(id, oldID)); err != nil {
		t.Fatalf("seed: %v", err)
	}
	clearRevision, matched, err := db.WriteCodexSessionBindingOverride(id, "", time.Unix(200, 0), false)
	if err != nil || !matched || clearRevision != 1 {
		t.Fatalf("explicit clear: revision=%d matched=%v err=%v", clearRevision, matched, err)
	}

	if revision, matched, err := db.WriteCodexSessionBinding(id, staleID, time.Unix(300, 0), 0, true); err != nil || matched || revision != 0 {
		t.Fatalf("stale binding CAS: revision=%d matched=%v err=%v", revision, matched, err)
	}
	if revision, matched, err := db.WriteCodexSessionPromotion(
		id, staleID, time.Unix(300, 0), oldID, oldRoot, time.Unix(250, 0), time.Unix(300, 0), 0, true,
	); err != nil || matched || revision != 0 {
		t.Fatalf("stale promotion CAS: revision=%d matched=%v err=%v", revision, matched, err)
	}

	gotID, _, gotRevision, confirmed, err := db.ReadCodexSessionBinding(id)
	if err != nil {
		t.Fatalf("read after stale CAS: %v", err)
	}
	if gotID != "" || gotRevision != clearRevision || confirmed {
		t.Fatalf("stale CAS changed binding: id=%q revision=%d confirmed=%v, want empty unconfirmed revision=%d", gotID, gotRevision, confirmed, clearRevision)
	}
}

func TestExplicitCodexOverrideOnNewRowStartsAtRevisionOne(t *testing.T) {
	for _, saveOne := range []bool{false, true} {
		t.Run(map[bool]string{false: "SaveInstances", true: "SaveInstance"}[saveOne], func(t *testing.T) {
			db := openCodexBindingTestDB(t)
			row := codexBindingRow("new-explicit", "")
			row.CodexBindingOverrideIntent = true
			var err error
			if saveOne {
				err = db.SaveInstance(row)
			} else {
				err = db.SaveInstances([]*InstanceRow{row})
			}
			if err != nil {
				t.Fatalf("save explicit new row: %v", err)
			}
			if _, _, got, _ := ReadCodexSessionBindingFromToolData(row.ToolData); got != 1 {
				t.Fatalf("new explicit row revision = %d, want 1", got)
			}
			if row.CodexBindingOverrideIntent {
				t.Fatal("committed row retained transient override intent")
			}
		})
	}
}

func TestCodexBindingConfirmationRequiresExplicitMarker(t *testing.T) {
	const sessionID = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	for _, tc := range []struct {
		name      string
		raw       string
		revision  int64
		confirmed bool
	}{
		{"legacy-zero", `{"codex_session_id":"` + sessionID + `","codex_detected_at":100}`, 0, false},
		{"legacy-one", `{"codex_session_id":"` + sessionID + `","codex_detected_at":100,"codex_binding_revision":1}`, 1, false},
		{"confirmed-zero", `{"codex_session_id":"` + sessionID + `","codex_binding_confirmed":true}`, 0, true},
		{"confirmed-one", `{"codex_session_id":"` + sessionID + `","codex_binding_revision":1,"codex_binding_confirmed":true}`, 1, true},
		{"revoked", `{"codex_session_id":"` + sessionID + `","codex_binding_revision":1,"codex_binding_confirmed":false}`, 1, false},
		{"string-is-not-proof", `{"codex_session_id":"` + sessionID + `","codex_binding_confirmed":"true"}`, 0, false},
		{"integer-is-not-proof", `{"codex_session_id":"` + sessionID + `","codex_binding_confirmed":1}`, 0, false},
		{"empty", `{"codex_session_id":"","codex_binding_revision":1,"codex_binding_confirmed":true}`, 1, false},
		{"missing-id", `{"codex_binding_revision":1,"codex_binding_confirmed":true}`, 1, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, _, revision, confirmed := ReadCodexSessionBindingFromToolData(json.RawMessage(tc.raw))
			if revision != tc.revision || confirmed != tc.confirmed {
				t.Fatalf("raw binding revision=%d confirmed=%v, want %d %v", revision, confirmed, tc.revision, tc.confirmed)
			}
			db := openCodexBindingTestDB(t)
			row := codexBindingRow("marker", "")
			row.ToolData = json.RawMessage(tc.raw)
			if err := db.SaveInstance(row); err != nil {
				t.Fatalf("seed: %v", err)
			}
			_, _, revision, confirmed, err := db.ReadCodexSessionBinding(row.ID)
			if err != nil || revision != tc.revision || confirmed != tc.confirmed {
				t.Fatalf("persisted binding revision=%d confirmed=%v err=%v, want %d %v", revision, confirmed, err, tc.revision, tc.confirmed)
			}
		})
	}
}

func TestWriteCodexSessionBindingToToolDataKeepsConfirmationWithTuple(t *testing.T) {
	raw := json.RawMessage(`{"codex_session_id":"aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa","codex_binding_confirmed":true,"unrelated":"kept"}`)
	for _, tc := range []struct {
		name      string
		id        string
		confirmed bool
		wantID    string
		wantProof bool
	}{
		{"replace", "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb", false, "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb", false},
		{"confirm", " BBBBBBBB-BBBB-4BBB-8BBB-BBBBBBBBBBBB ", true, "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb", true},
		{"clear", " ", true, "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			written := WriteCodexSessionBindingToToolData(raw, tc.id, time.Unix(200, 0), 2, tc.confirmed)
			id, at, revision, confirmed := ReadCodexSessionBindingFromToolData(written)
			if id != tc.wantID || !at.Equal(time.Unix(200, 0)) || revision != 2 || confirmed != tc.wantProof {
				t.Fatalf("binding = (%q, %v, %d, %v), want (%q, 200, 2, %v)", id, at, revision, confirmed, tc.wantID, tc.wantProof)
			}
			var values map[string]any
			if err := json.Unmarshal(written, &values); err != nil {
				t.Fatal(err)
			}
			if values["unrelated"] != "kept" || values["codex_binding_confirmed"] != tc.wantProof {
				t.Fatalf("serialized tuple or unrelated data changed: %s", written)
			}
		})
	}
}

func TestTargetedCodexBindingConfirmationChangesAtomically(t *testing.T) {
	const (
		sessionID = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
		otherID   = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"
	)
	for _, writer := range []string{"cas", "override", "promotion"} {
		t.Run(writer, func(t *testing.T) {
			db := openCodexBindingTestDB(t)
			row := codexBindingRow("atomic-proof", sessionID)
			if err := db.SaveInstance(row); err != nil {
				t.Fatal(err)
			}
			for step, tc := range []struct {
				id        string
				confirmed bool
				wantProof bool
			}{
				{sessionID, true, true},
				{sessionID, false, false},
				{sessionID, true, true},
				{otherID, false, false},
				{"", true, false},
			} {
				at := time.Unix(int64(100+step), 0)
				var revision int64
				var matched bool
				var err error
				switch writer {
				case "cas":
					revision, matched, err = db.WriteCodexSessionBinding(row.ID, tc.id, at, int64(step), tc.confirmed)
				case "override":
					revision, matched, err = db.WriteCodexSessionBindingOverride(row.ID, tc.id, at, tc.confirmed)
				case "promotion":
					revision, matched, err = db.WriteCodexSessionPromotion(row.ID, tc.id, at, sessionID, "", at, at, int64(step), tc.confirmed)
				}
				if err != nil || !matched || revision != int64(step+1) {
					t.Fatalf("step %d write: revision=%d matched=%v err=%v", step, revision, matched, err)
				}
				id, detectedAt, gotRevision, confirmed, err := db.ReadCodexSessionBinding(row.ID)
				if err != nil || id != tc.id || !detectedAt.Equal(at) || gotRevision != int64(step+1) || confirmed != tc.wantProof {
					t.Fatalf("step %d tuple = (%q, %v, %d, %v), err=%v", step, id, detectedAt, gotRevision, confirmed, err)
				}
				var values map[string]any
				if err := json.Unmarshal(readCodexBindingToolData(t, db, row.ID), &values); err != nil {
					t.Fatal(err)
				}
				if values["codex_binding_confirmed"] != tc.wantProof {
					t.Fatalf("step %d marker is not a JSON boolean: %v", step, values["codex_binding_confirmed"])
				}
			}
		})
	}
}

func TestCodexFullRowConfirmationStaysWithWinningTuple(t *testing.T) {
	const (
		sessionID = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
		otherID   = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"
	)
	for _, writer := range []string{"SaveInstance", "SaveInstances", "UpsertInstances"} {
		t.Run(writer, func(t *testing.T) {
			for _, tc := range []struct {
				name             string
				existingRevision int64
				existingProof    bool
				incomingID       string
				incomingRevision int64
				incomingProof    bool
				override         bool
				wantID           string
				wantRevision     int64
				wantProof        bool
			}{
				{"equal-regrant", 1, false, sessionID, 1, true, false, sessionID, 1, false},
				{"equal-revoke", 1, true, sessionID, 1, false, false, sessionID, 1, true},
				{"zero-regrant", 0, false, sessionID, 0, true, false, sessionID, 0, false},
				{"zero-revoke", 0, true, sessionID, 0, false, false, sessionID, 0, true},
				{"new-uuid", 1, true, otherID, 2, false, false, otherID, 2, false},
				{"legacy-new-uuid", 0, true, otherID, 0, false, false, otherID, 0, false},
				{"stale-proof", 2, false, otherID, 1, true, false, sessionID, 2, false},
				{"stale-revocation", 2, true, sessionID, 1, false, false, sessionID, 2, true},
				{"explicit-regrant", 1, false, sessionID, 1, true, true, sessionID, 2, true},
				{"explicit-revoke", 1, true, sessionID, 1, false, true, sessionID, 2, false},
				{"explicit-clear", 1, true, "", 1, true, true, "", 2, false},
			} {
				t.Run(tc.name, func(t *testing.T) {
					db := openCodexBindingTestDB(t)
					seed := codexBindingRow("full-row-proof", sessionID)
					seed.ToolData = WriteCodexSessionBindingToToolData(seed.ToolData, sessionID, time.Unix(100, 0), tc.existingRevision, tc.existingProof)
					if err := db.SaveInstance(seed); err != nil {
						t.Fatal(err)
					}
					incoming := codexBindingRow(seed.ID, tc.incomingID)
					incoming.ToolData = WriteCodexSessionBindingToToolData(incoming.ToolData, tc.incomingID, time.Unix(200, 0), tc.incomingRevision, tc.incomingProof)
					incoming.CodexBindingOverrideIntent = tc.override
					var err error
					switch writer {
					case "SaveInstance":
						err = db.SaveInstance(incoming)
					case "SaveInstances":
						err = db.SaveInstances([]*InstanceRow{incoming})
					case "UpsertInstances":
						err = db.UpsertInstances([]*InstanceRow{incoming})
					}
					if err != nil {
						t.Fatal(err)
					}
					id, _, revision, confirmed, err := db.ReadCodexSessionBinding(seed.ID)
					if err != nil || id != tc.wantID || revision != tc.wantRevision || confirmed != tc.wantProof {
						t.Fatalf("committed tuple = (%q, %d, %v), err=%v, want (%q, %d, %v)", id, revision, confirmed, err, tc.wantID, tc.wantRevision, tc.wantProof)
					}
					returnedID, _, returnedRevision, returnedProof := ReadCodexSessionBindingFromToolData(incoming.ToolData)
					if returnedID != tc.wantID || returnedRevision != tc.wantRevision || returnedProof != tc.wantProof {
						t.Fatalf("returned tuple = (%q, %d, %v), want committed tuple", returnedID, returnedRevision, returnedProof)
					}
				})
			}
		})
	}
}

func TestCodexConfirmationMigrationPreservesOnlyExplicitMarker(t *testing.T) {
	for _, tc := range []struct {
		name      string
		id        string
		marker    string
		confirmed bool
	}{
		{"legacy", "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", "", false},
		{"confirmed", "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", `,"codex_binding_confirmed":true`, true},
		{"revoked", "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", `,"codex_binding_confirmed":false`, false},
		{"empty", "", `,"codex_binding_confirmed":true`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := openCodexBindingTestDB(t)
			path := filepath.Join(t.TempDir(), "sessions.json")
			raw := `{"instances":[{"id":"migrated","title":"test","tool":"codex","group_path":"default","codex_session_id":"` + tc.id + `","codex_detected_at":"2026-01-01T00:00:00Z"` + tc.marker + `}]}`
			if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, _, err := MigrateFromJSON(path, db); err != nil {
				t.Fatal(err)
			}
			id, _, revision, confirmed, err := db.ReadCodexSessionBinding("migrated")
			if err != nil || id != tc.id || revision != 0 || confirmed != tc.confirmed {
				t.Fatalf("migrated tuple = (%q, %d, %v), err=%v, want (%q, 0, %v)", id, revision, confirmed, err, tc.id, tc.confirmed)
			}
		})
	}
}

func TestCodexFullRowOmittedIDPreservesAtomicBinding(t *testing.T) {
	const sessionID = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	for _, writer := range []string{"SaveInstance", "SaveInstances", "UpsertInstances"} {
		t.Run(writer, func(t *testing.T) {
			for _, confirmed := range []bool{false, true} {
				for _, raw := range []string{
					`{"latest_prompt":"updated"}`,
					`{"latest_prompt":"updated","codex_binding_revision":2,"codex_binding_confirmed":true}`,
				} {
					db := openCodexBindingTestDB(t)
					seed := codexBindingRow("omitted-id", sessionID)
					seed.ToolData = WriteCodexSessionBindingToToolData(seed.ToolData, sessionID, time.Unix(100, 0), 1, confirmed)
					if err := db.SaveInstance(seed); err != nil {
						t.Fatal(err)
					}
					incoming := codexBindingRow(seed.ID, "")
					incoming.ToolData = json.RawMessage(raw)
					var err error
					switch writer {
					case "SaveInstance":
						err = db.SaveInstance(incoming)
					case "SaveInstances":
						err = db.SaveInstances([]*InstanceRow{incoming})
					case "UpsertInstances":
						err = db.UpsertInstances([]*InstanceRow{incoming})
					}
					if err != nil {
						t.Fatal(err)
					}
					id, at, revision, proof, err := db.ReadCodexSessionBinding(seed.ID)
					if err != nil || id != sessionID || !at.Equal(time.Unix(100, 0)) || revision != 1 || proof != confirmed {
						t.Fatalf("omitted ID split tuple: (%q, %v, %d, %v), err=%v, incoming=%s", id, at, revision, proof, err, raw)
					}
					var values map[string]any
					if err := json.Unmarshal(incoming.ToolData, &values); err != nil {
						t.Fatal(err)
					}
					if values["latest_prompt"] != "updated" {
						t.Fatalf("binding merge lost unrelated update: %s", incoming.ToolData)
					}
				}
			}
		})
	}
}
