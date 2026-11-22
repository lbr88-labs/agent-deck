package session

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/asheshgoplani/agent-deck/internal/atomicfile"
)

// LastSavedWaitResponse reads a provider-owned response. Screen repaint, prompt
// echo, and editor drafts are deliberately excluded from completion evidence.
func (i *Instance) LastSavedWaitResponse(peers []*Instance) (*ResponseOutput, error) {
	if i.IsSSH() || i.IsSandboxed() {
		return nil, fmt.Errorf("wait requires a locally bound transcript for %s", i.Tool)
	}
	var path, expectedID string
	switch {
	case IsClaudeCompatible(i.Tool):
		if !IsBareClaudeSessionUUID(i.ClaudeSessionID) {
			return nil, os.ErrNotExist
		}
		if _, err := i.GetJSONLPathChecked(peers); err != nil {
			return nil, err
		}
		path = ClaudeTranscriptPathForInstance(i)
	case IsCodexCompatible(i.Tool):
		if !isCodexSessionUUID(i.CodexSessionID) {
			return nil, os.ErrNotExist
		}
		if !i.CodexBindingConfirmed {
			return nil, os.ErrNotExist
		}
		expectedID = i.CodexSessionID
		home := i.getCodexHomeDir()
		for _, peer := range peers {
			if peer != nil && peer.ID != i.ID && IsCodexCompatible(peer.Tool) && strings.EqualFold(peer.CodexSessionID, i.CodexSessionID) && !peer.IsSSH() && peer.getCodexHomeDir() == home {
				return nil, fmt.Errorf("Codex conversation is shared by sessions %s and %s", i.ID, peer.ID)
			}
		}
		path = codexRolloutPathInHome(i.CodexSessionID, home)
	case i.Tool == "omp":
		active, err := verifiedOmpTranscript(context.Background(), i)
		if err != nil {
			return nil, err
		}
		path, expectedID = active.File, active.SessionID
	case i.Tool == "cursor":
		return readCursorWaitResponse(i.ID)
	case i.Tool == "gemini":
		r, err := i.getGeminiLastResponse()
		if err == nil && r != nil && r.SessionID != i.GeminiSessionID {
			return nil, errChatSearchIdentity
		}
		return r, err
	default:
		return nil, fmt.Errorf("%s does not expose a bound timestamped assistant transcript to session wait", i.Tool)
	}
	if path == "" {
		return nil, os.ErrNotExist
	}
	file, err := openOmpRegularFile(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	if expectedID != "" {
		err = verifyWaitTranscriptHeader(file, expectedID, IsCodexCompatible(i.Tool))
		if err != nil {
			return nil, err
		}
	}
	lines, err := transcriptTailLinesFromFile(file, 128)
	if err != nil {
		return nil, err
	}
	return parseWaitTranscript(i, lines), nil
}

// Check identity and read output through the same descriptor. A concurrent
// rename must never put the header of one conversation on the tail of another.
func verifyWaitTranscriptHeader(file *os.File, expectedID string, codex bool) error {
	const headerLimit = 256 << 10
	reader := bufio.NewReaderSize(io.LimitReader(file, headerLimit), headerLimit)
	line, err := reader.ReadSlice('\n')
	if err == io.EOF && len(line) < headerLimit {
		return os.ErrNotExist
	} // producer has not finished the header
	if err != nil {
		return errChatSearchIdentity
	}
	var h struct {
		Type, ID string
		Payload  struct{ ID string }
	}
	if json.Unmarshal(line, &h) != nil {
		return errChatSearchIdentity
	}
	if codex && h.Type == "session_meta" && strings.EqualFold(h.Payload.ID, expectedID) {
		return nil
	}
	if !codex && h.Type == "session" && h.ID == expectedID {
		return nil
	}
	return errChatSearchIdentity
}

func parseWaitTranscript(i *Instance, lines []string) *ResponseOutput {
	for j := len(lines) - 1; j >= 0; j-- {
		var record struct {
			Type        string
			SessionID   string `json:"sessionId"`
			IsSidechain bool
			Timestamp   string
			Payload     struct {
				Type             string
				LastAgentMessage string `json:"last_agent_message"`
			}
			Message struct {
				Role             string
				StopReason       string
				ClaudeStopReason string `json:"stop_reason"`
				Content          []struct {
					Type string
					Text string
				}
			}
		}
		if json.Unmarshal([]byte(lines[j]), &record) != nil {
			continue
		}
		var content string
		ompFinal := i.Tool == "omp" && record.Type == "message" && record.Message.StopReason == "stop"
		claudeFinal := IsClaudeCompatible(i.Tool) && record.Type == "assistant" && !record.IsSidechain && record.SessionID == i.ClaudeSessionID && (record.Message.ClaudeStopReason == "end_turn" || record.Message.ClaudeStopReason == "stop_sequence")
		if (ompFinal || claudeFinal) && record.Message.Role == "assistant" {
			var parts []string
			for _, block := range record.Message.Content {
				if block.Type == "text" {
					parts = append(parts, block.Text)
				}
			}
			content = strings.Join(parts, "\n")
		} else if IsCodexCompatible(i.Tool) && record.Type == "event_msg" && record.Payload.Type == "task_complete" {
			content = record.Payload.LastAgentMessage
		}
		if content != "" {
			return &ResponseOutput{Tool: i.Tool, Role: "assistant", Content: content, Timestamp: record.Timestamp}
		}
	}
	return nil
}

type cursorWaitResponse struct {
	Generation string
	TurnID     string
	Reply      ResponseOutput
}

// ObserveCursorWaitTurn pairs beforeSubmitPrompt and afterAgentResponse. The
// prompt timestamp excludes previous turns whose response hook arrived late.
func ObserveCursorWaitTurn(instanceID, conversationID, turnID, text string) error {
	if !validHookSessionAnchorInstanceID(instanceID) || conversationID == "" || turnID == "" {
		return errChatSearchIdentity
	}
	release, err := AcquireHookSessionLock(instanceID)
	if err != nil {
		return err
	}
	defer release()
	generation := strings.TrimSpace(os.Getenv("AGENTDECK_HOOK_GENERATION"))
	current, authority := hookGenerationForInstance(instanceID)
	if !hookGenerationRecordAccepted(generation, current, authority) {
		return errChatSearchIdentity
	}
	dir := filepath.Join(GetHooksDir(), ".cursor-responses")
	path := filepath.Join(dir, instanceID+".json")
	record, err := loadCursorWaitTurn(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	match := record.Generation == generation && record.TurnID == turnID && record.Reply.SessionID == conversationID
	if text == "" {
		if match {
			return nil
		}
		record = cursorWaitResponse{Generation: generation, TurnID: turnID, Reply: ResponseOutput{Tool: "cursor", Role: "assistant", Timestamp: time.Now().UTC().Format(time.RFC3339Nano), SessionID: conversationID}}
	} else {
		if !match {
			return errChatSearchIdentity
		}
		record.Reply.Content = text
	}
	data, err := json.Marshal(record)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	return atomicfile.WriteFile(path, data, 0600)
}

func loadCursorWaitTurn(path string) (cursorWaitResponse, error) {
	var record cursorWaitResponse
	data, err := readOmpBoundedRegularFile(path, 8<<20)
	if err == nil {
		err = json.Unmarshal(data, &record)
	}
	return record, err
}

func readCursorWaitResponse(instanceID string) (*ResponseOutput, error) {
	if !validHookSessionAnchorInstanceID(instanceID) {
		return nil, errChatSearchIdentity
	}
	record, err := loadCursorWaitTurn(filepath.Join(GetHooksDir(), ".cursor-responses", instanceID+".json"))
	if err != nil {
		return nil, err
	}
	current, authority := hookGenerationForInstance(instanceID)
	if !hookGenerationRecordAccepted(record.Generation, current, authority) || record.TurnID == "" || record.Reply.Content == "" {
		return nil, os.ErrNotExist
	}
	return &record.Reply, nil
}
