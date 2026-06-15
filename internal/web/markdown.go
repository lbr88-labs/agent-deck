package web

import (
	"html"
	"regexp"
	"strings"
)

// Minimal, dependency-free, safe Markdown→HTML renderer for the Command Center
// detail pages. It renders the small subset conductors actually use in their
// outputs/ docs (headings, lists, fenced/inline code, bold/italic, links,
// paragraphs, hr) and HTML-escapes everything else. It is deliberately NOT a
// full CommonMark implementation: the goal is a faithful, injection-safe inline
// view of a conductor's own status docs, not arbitrary user HTML.
//
// Security: every text fragment is html.EscapeString-ed before any tag is
// emitted, so no raw HTML/script from a doc reaches the DOM. Only the link href
// of an [text](url) is special-cased, and it is restricted to http/https/mailto/
// relative — javascript: and data: hrefs are dropped to plain text.

var (
	mdHeading    = regexp.MustCompile(`^(#{1,6})\s+(.*)$`)
	mdULItem     = regexp.MustCompile(`^[-*+]\s+(.*)$`)
	mdOLItem     = regexp.MustCompile(`^\d+[.)]\s+(.*)$`)
	mdHR         = regexp.MustCompile(`^(\*{3,}|-{3,}|_{3,})$`)
	mdInlineCode = regexp.MustCompile("`([^`]+)`")
	mdBold       = regexp.MustCompile(`\*\*([^*]+)\*\*`)
	mdItalic     = regexp.MustCompile(`(^|[^*])\*([^*]+)\*`)
	mdLink       = regexp.MustCompile(`\[([^\]]+)\]\(([^)\s]+)\)`)
	mdSafeHref   = regexp.MustCompile(`^(https?://|mailto:|/|\./|#)`)
)

// renderMarkdown converts a small Markdown subset to safe HTML.
func renderMarkdown(src string) string {
	lines := strings.Split(strings.ReplaceAll(src, "\r\n", "\n"), "\n")
	var b strings.Builder

	listType := "" // "ul" | "ol" | ""
	inCode := false
	var codeBuf strings.Builder
	var paraBuf []string

	closeList := func() {
		if listType != "" {
			b.WriteString("</" + listType + ">\n")
			listType = ""
		}
	}
	flushPara := func() {
		if len(paraBuf) == 0 {
			return
		}
		b.WriteString("<p>" + renderInline(strings.Join(paraBuf, " ")) + "</p>\n")
		paraBuf = nil
	}

	for _, raw := range lines {
		line := strings.TrimRight(raw, " \t")

		// Fenced code blocks: ``` toggles a verbatim, fully-escaped block.
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			if inCode {
				b.WriteString("<pre><code>" + html.EscapeString(codeBuf.String()) + "</code></pre>\n")
				codeBuf.Reset()
				inCode = false
			} else {
				flushPara()
				closeList()
				inCode = true
			}
			continue
		}
		if inCode {
			codeBuf.WriteString(raw + "\n")
			continue
		}

		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			flushPara()
			closeList()
			continue
		}

		if mdHR.MatchString(trimmed) {
			flushPara()
			closeList()
			b.WriteString("<hr/>\n")
			continue
		}

		if m := mdHeading.FindStringSubmatch(trimmed); m != nil {
			flushPara()
			closeList()
			level := len(m[1])
			b.WriteString("<h" + itoa(level) + ">" + renderInline(m[2]) + "</h" + itoa(level) + ">\n")
			continue
		}

		if m := mdULItem.FindStringSubmatch(trimmed); m != nil {
			flushPara()
			if listType != "ul" {
				closeList()
				b.WriteString("<ul>\n")
				listType = "ul"
			}
			b.WriteString("<li>" + renderInline(m[1]) + "</li>\n")
			continue
		}
		if m := mdOLItem.FindStringSubmatch(trimmed); m != nil {
			flushPara()
			if listType != "ol" {
				closeList()
				b.WriteString("<ol>\n")
				listType = "ol"
			}
			b.WriteString("<li>" + renderInline(m[1]) + "</li>\n")
			continue
		}

		closeList()
		paraBuf = append(paraBuf, trimmed)
	}

	if inCode {
		// Unterminated fence: still emit what we have, escaped.
		b.WriteString("<pre><code>" + html.EscapeString(codeBuf.String()) + "</code></pre>\n")
	}
	flushPara()
	closeList()
	return b.String()
}

// renderInline escapes text then re-applies the small set of safe inline spans.
// Order matters: escape first (so no raw HTML survives), then code spans (which
// must not be further formatted), then links, bold, italic.
func renderInline(s string) string {
	s = html.EscapeString(s)

	// Inline code: wrap, and within the span re-escape is unnecessary (already
	// escaped). Use a placeholder so bold/italic don't touch code content.
	type span struct{ ph, htmlOut string }
	var spans []span
	idx := 0
	s = mdInlineCode.ReplaceAllStringFunc(s, func(m string) string {
		inner := mdInlineCode.FindStringSubmatch(m)[1]
		ph := "\x00CODE" + itoa(idx) + "\x00"
		spans = append(spans, span{ph, "<code>" + inner + "</code>"})
		idx++
		return ph
	})

	// Links: [text](url) — text already escaped; url validated.
	s = mdLink.ReplaceAllStringFunc(s, func(m string) string {
		sub := mdLink.FindStringSubmatch(m)
		text, href := sub[1], sub[2]
		// href came through EscapeString already (so & -> &amp;); unescape for
		// the scheme check, then re-escape for the attribute.
		rawHref := html.UnescapeString(href)
		if !mdSafeHref.MatchString(rawHref) {
			return text // drop unsafe scheme, keep visible text
		}
		return `<a href="` + html.EscapeString(rawHref) + `" target="_blank" rel="noopener noreferrer">` + text + `</a>`
	})

	s = mdBold.ReplaceAllString(s, "<strong>$1</strong>")
	s = mdItalic.ReplaceAllString(s, "$1<em>$2</em>")

	for _, sp := range spans {
		s = strings.Replace(s, sp.ph, sp.htmlOut, 1)
	}
	return s
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [12]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
