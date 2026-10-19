package scripts

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

func TestLighthouseAckLabelDescriptionFitsGitHubLimit(t *testing.T) {
	repoRoot := filepath.Clean("..")
	workflow, err := os.ReadFile(filepath.Join(repoRoot, ".github/workflows/lighthouse-ci.yml"))
	if err != nil {
		t.Fatalf("read Lighthouse workflow: %v", err)
	}

	description := regexp.MustCompile(`--description "([^"]+)"`).FindSubmatch(workflow)
	if description == nil {
		t.Fatal("Lighthouse workflow has no ack-label description")
	}
	if got := len(description[1]); got > 100 {
		t.Fatalf("Lighthouse ack-label description is %d characters; GitHub permits at most 100", got)
	}
}
