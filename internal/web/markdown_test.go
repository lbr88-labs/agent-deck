package web

import (
	"strings"
	"testing"
)

func TestRenderMarkdownBasics(t *testing.T) {
	in := "# Title\n\nSome **bold** and *italic* and `code`.\n\n- one\n- two\n\n1. first\n2. second\n"
	out := renderMarkdown(in)
	for _, want := range []string{
		"<h1>Title</h1>",
		"<strong>bold</strong>",
		"<em>italic</em>",
		"<code>code</code>",
		"<ul>",
		"<li>one</li>",
		"<ol>",
		"<li>first</li>",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output, got:\n%s", want, out)
		}
	}
}

func TestRenderMarkdownFencedCode(t *testing.T) {
	in := "```\nplain <script>alert(1)</script>\n```\n"
	out := renderMarkdown(in)
	if !strings.Contains(out, "<pre><code>") {
		t.Fatalf("expected code block, got: %s", out)
	}
	if strings.Contains(out, "<script>") {
		t.Fatalf("code block must escape HTML, got: %s", out)
	}
	if !strings.Contains(out, "&lt;script&gt;") {
		t.Fatalf("expected escaped script tag, got: %s", out)
	}
}

func TestRenderMarkdownEscapesRawHTML(t *testing.T) {
	in := "A paragraph with <img src=x onerror=alert(1)> and <b>raw</b>.\n"
	out := renderMarkdown(in)
	if strings.Contains(out, "<img") || strings.Contains(out, "onerror") && strings.Contains(out, "<img") {
		t.Fatalf("raw HTML must be escaped, got: %s", out)
	}
	if !strings.Contains(out, "&lt;img") {
		t.Fatalf("expected escaped img, got: %s", out)
	}
}

func TestRenderMarkdownSafeLinks(t *testing.T) {
	out := renderMarkdown("[ok](https://example.com) and [bad](javascript:alert(1))\n")
	if !strings.Contains(out, `href="https://example.com"`) {
		t.Fatalf("expected safe link rendered, got: %s", out)
	}
	if strings.Contains(out, "javascript:") {
		t.Fatalf("javascript: scheme must be dropped, got: %s", out)
	}
	// The visible link text for the dropped link is still present.
	if !strings.Contains(out, "bad") {
		t.Fatalf("expected dropped-link text retained, got: %s", out)
	}
}

func TestRenderMarkdownHR(t *testing.T) {
	out := renderMarkdown("above\n\n---\n\nbelow\n")
	if !strings.Contains(out, "<hr/>") {
		t.Fatalf("expected hr, got: %s", out)
	}
}
