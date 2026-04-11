package watcher

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"
	"unicode/utf8"
)

// triagePromptTemplate is the canonical prompt sent to the triage Claude session (D-06/Q2).
// It instructs the session to write a single JSON file to the exact output path.
const triagePromptTemplate = `You are a routing classifier. Read the event below and decide which conductor should handle it.

EVENT:
  Sender:  {{.Sender}}
  Subject: {{.Subject}}
  Body:    {{.BodyTruncated}}

KNOWN CONDUCTORS (from clients.json):
{{.ClientsList}}
Your task: write ONE JSON file at the EXACT path below and then exit.
Do not write any other files. Do not write to any other location.

OUTPUT PATH: {{.ResultPath}}

OUTPUT SCHEMA (strict — all fields required):
{
  "route_to": "<conductor-name-or-empty>",
  "group": "<group-path-or-empty>",
  "name": "<human-readable-name-or-empty>",
  "sender": "{{.Sender}}",
  "summary": "<one-line summary>",
  "confidence": "<high|medium|low>",
  "should_persist": <true|false>
}

After writing the file, print "DONE" and exit. Do not open a new session.`

// triagePromptData holds the template variables for BuildPrompt.
type triagePromptData struct {
	Sender        string
	Subject       string
	BodyTruncated string
	ClientsList   string
	ResultPath    string
}

// maxBodyRunes is the maximum body length before truncation (D-06).
const maxBodyRunes = 4000

// BuildPrompt renders the triage prompt template with the given event, clients list,
// and absolute result path. The event body is truncated to maxBodyRunes if longer.
func BuildPrompt(event Event, clientsList map[string]ClientEntry, resultPath string) (string, error) {
	// Truncate body to maxBodyRunes runes.
	bodyRunes := []rune(event.Body)
	bodyTruncated := event.Body
	if utf8.RuneCountInString(event.Body) > maxBodyRunes {
		bodyTruncated = string(bodyRunes[:maxBodyRunes]) + "…"
	}

	// Build a human-readable conductors list.
	var sb strings.Builder
	for sender, entry := range clientsList {
		fmt.Fprintf(&sb, "  - %s: %s/%s (%s)\n", sender, entry.Conductor, entry.Group, entry.Name)
	}
	conductorsList := sb.String()
	if conductorsList == "" {
		conductorsList = "  (none configured yet)\n"
	}

	data := triagePromptData{
		Sender:        event.Sender,
		Subject:       event.Subject,
		BodyTruncated: bodyTruncated,
		ClientsList:   conductorsList,
		ResultPath:    resultPath,
	}

	tmpl, err := template.New("triage").Parse(triagePromptTemplate)
	if err != nil {
		return "", fmt.Errorf("triage_prompt: parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("triage_prompt: execute template: %w", err)
	}

	return buf.String(), nil
}
