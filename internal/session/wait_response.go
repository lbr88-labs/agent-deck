package session

import (
	"context"
	"encoding/json"
	"fmt"
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
	var path string
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
			return nil, fmt.Errorf("Codex conversation binding for %s is unconfirmed", i.ID)
		}
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
		path = active.File
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
	lines, err := TranscriptTailLines(path, 128)
	if err != nil {
		return nil, err
	}
	return parseWaitTranscript(i, lines), nil
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
	Reply      ResponseOutput
}

// SaveCursorWaitResponse stores the text delivered by Cursor's afterAgentResponse hook.
func SaveCursorWaitResponse(instanceID, conversationID, text string) error {
	if !validHookSessionAnchorInstanceID(instanceID) || conversationID == "" || strings.TrimSpace(text) == "" {
		return errChatSearchIdentity
	}
	generation := strings.TrimSpace(os.Getenv("AGENTDECK_HOOK_GENERATION"))
	current, authority := hookGenerationForInstance(instanceID)
	if !hookGenerationRecordAccepted(generation, current, authority) {
		return errChatSearchIdentity
	}
	record := cursorWaitResponse{Generation: generation, Reply: ResponseOutput{Tool: "cursor", Role: "assistant", Timestamp: time.Now().UTC().Format(time.RFC3339Nano), SessionID: conversationID, Content: text}}
	data, err := json.Marshal(record)
	if err != nil {
		return err
	}
	dir := filepath.Join(GetHooksDir(), ".cursor-responses")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	return atomicfile.WriteFile(filepath.Join(dir, instanceID+".json"), data, 0600)
}

func readCursorWaitResponse(instanceID string) (*ResponseOutput, error) {
	if !validHookSessionAnchorInstanceID(instanceID) {
		return nil, errChatSearchIdentity
	}
	path := filepath.Join(GetHooksDir(), ".cursor-responses", instanceID+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var record cursorWaitResponse
	if err := json.Unmarshal(data, &record); err != nil {
		return nil, err
	}
	current, authority := hookGenerationForInstance(instanceID)
	if !hookGenerationRecordAccepted(record.Generation, current, authority) {
		return nil, os.ErrNotExist
	}
	return &record.Reply, nil
}
