// Package notes turns a GitHub release body into the plain, single-cell lines
// the update dialog can draw. The daemon only carries the raw Markdown; keeping
// the shaping here means the display rules stay in the TUI and the wire payload
// stays presentation-free.
package notes

import (
	"html"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"tuick-tack/internal/tui/canvas"
)

// Kind is the role of one rendered line.
type Kind uint8

const (
	// Body is a paragraph line.
	Body Kind = iota
	// Heading is a "### Feature" line, drawn in the accent color.
	Heading
	// Bullet is a list item, with its marker in Text.
	Bullet
	// Quote is a blockquote line, with a "│ " marker in Text.
	Quote
)

// Line is one wrapped, drawable line. Text is guaranteed to contain only
// single-cell runes and to fit the width passed to Render. An empty Text is a
// vertical spacer.
type Line struct {
	Text string
	Kind Kind
}

const (
	// BulletMarker prefixes a wrapped list item.
	BulletMarker = "· "
	// QuoteMarker prefixes a blockquote line.
	QuoteMarker = "│ "
)

// htmlTag matches real HTML tags but not autolinks such as <https://example.test>.
var htmlTag = regexp.MustCompile(`</?[A-Za-z][A-Za-z0-9-]*(?:\s[^>]*)?/?>`)

// Render extracts the "What's new" section of a release body and wraps it to
// width columns. An empty result means there are no usable notes.
func Render(body string, width int) []Line {
	if width <= 0 {
		return nil
	}
	text := whatsNew(body)
	var out []Line
	inFence := false
	for _, raw := range strings.Split(text, "\n") {
		line := strings.TrimSpace(raw)
		if strings.HasPrefix(line, "```") || strings.HasPrefix(line, "~~~") {
			inFence = !inFence
			continue
		}
		if inFence {
			if line = cleanInline(line); line != "" {
				appendWrapped(&out, line, Body, "", "", width)
			} else {
				addSpacer(&out)
			}
			continue
		}
		if line == "" {
			addSpacer(&out)
			continue
		}
		if isRule(line) {
			addSpacer(&out)
			continue
		}
		if isTableSeparator(line) {
			continue
		}
		if strings.HasPrefix(line, "|") && strings.HasSuffix(line, "|") {
			if line = cleanInline(flattenTable(line)); line != "" {
				appendWrapped(&out, line, Body, "", "", width)
			}
			continue
		}
		if strings.HasPrefix(line, ">") {
			line = strings.TrimSpace(strings.TrimLeft(line, ">"))
			if line == "" {
				addSpacer(&out)
				continue
			}
			if isCallout(line) {
				continue
			}
			if line = cleanInline(line); line != "" {
				appendWrapped(&out, line, Quote, QuoteMarker, QuoteMarker, width)
			}
			continue
		}
		if h, ok := heading(line); ok {
			if h = cleanInline(h); h != "" {
				appendWrapped(&out, h, Heading, "", "", width)
			}
			continue
		}
		if prefix, rest, ok := listItem(line); ok {
			if rest = cleanInline(rest); rest != "" {
				appendWrapped(&out, rest, Bullet, prefix,
					strings.Repeat(" ", canvas.Width(prefix)), width)
			}
			continue
		}
		if line = cleanInline(line); line != "" {
			appendWrapped(&out, line, Body, "", "", width)
		}
	}
	trimSpacers(&out)
	return out
}

// whatsNew returns the first "What's new" section, from the heading after a
// level-two heading that mentions it until the next level-two heading. A body
// without that section falls back to the whole text.
func whatsNew(body string) string {
	text := normalize(body)
	lines := strings.Split(text, "\n")
	start := -1
	for i, line := range lines {
		if isSection(line) && mentionsWhatsNew(line) {
			start = i + 1
			break
		}
	}
	if start < 0 {
		return text
	}
	end := len(lines)
	for i := start; i < len(lines); i++ {
		if isSection(lines[i]) {
			end = i
			break
		}
	}
	return strings.Join(lines[start:end], "\n")
}

func isSection(line string) bool {
	t := strings.TrimSpace(line)
	return strings.HasPrefix(t, "## ") && !strings.HasPrefix(t, "### ")
}

func mentionsWhatsNew(line string) bool {
	t := strings.ToLower(line)
	return strings.Contains(t, "what's new") || strings.Contains(t, "whats new")
}

func normalize(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	s = strings.ReplaceAll(s, "\t", "    ")
	s = strings.ReplaceAll(s, "\u00a0", " ")
	s = strings.NewReplacer("‘", "'", "’", "'", "“", "\"", "”", "\"").Replace(s)
	for {
		start := strings.Index(s, "<!--")
		if start < 0 {
			break
		}
		end := strings.Index(s[start+4:], "-->")
		if end < 0 {
			s = s[:start]
			break
		}
		s = s[:start] + " " + s[start+4+end+3:]
	}
	return s
}

func cleanInline(s string) string {
	s = stripANSI(s)
	s = htmlTag.ReplaceAllString(s, " ")
	s = html.UnescapeString(s)
	s = stripLinks(s)
	s = strings.NewReplacer("**", "", "__", "", "~~", "", "`", "", "*", "").Replace(s)
	s = strings.Join(strings.Fields(s), " ")
	return cellSafe(s)
}

// stripLinks keeps a link's label and drops its target; an image keeps its alt
// text. The scan is deliberately small, not a full Markdown parser.
func stripLinks(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); {
		if s[i] == '!' && i+1 < len(s) && s[i+1] == '[' {
			i++ // keep the alt text, dropped by the '[' branch next.
			continue
		}
		if s[i] == '[' {
			close := strings.IndexByte(s[i+1:], ']')
			if close >= 0 {
				close += i + 1
				if close+1 < len(s) && s[close+1] == '(' {
					end := strings.IndexByte(s[close+2:], ')')
					if end >= 0 {
						b.WriteString(s[i+1 : close])
						i = close + 2 + end + 1
						continue
					}
				}
			}
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String()
}

// stripANSI removes CSI, OSC and other escape sequences so an edited release
// body can never smuggle terminal control text into the dialog.
func stripANSI(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); {
		if s[i] != 0x1b {
			b.WriteByte(s[i])
			i++
			continue
		}
		i++
		if i >= len(s) {
			break
		}
		switch s[i] {
		case '[':
			i++
			for i < len(s) && !(s[i] >= 0x40 && s[i] <= 0x7e) {
				i++
			}
			if i < len(s) {
				i++
			}
		case ']':
			i++
			for i < len(s) && s[i] != 0x07 && s[i] != 0x1b {
				i++
			}
			if i < len(s) && s[i] == 0x07 {
				i++
			} else if i+1 < len(s) && s[i] == 0x1b && s[i+1] == '\\' {
				i += 2
			}
		default:
			i++
		}
	}
	return b.String()
}

// cellSafe drops control characters and anything that is not exactly one
// terminal cell: emoji, CJK and combining marks would shift the canvas grid.
func cellSafe(s string) string {
	var b strings.Builder
	for _, r := range s {
		if unicode.IsControl(r) || !canvas.SingleCell(r) {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func heading(line string) (string, bool) {
	n := 0
	for n < len(line) && line[n] == '#' {
		n++
	}
	if n == 0 || n > 6 {
		return "", false
	}
	if n < len(line) && line[n] == ' ' {
		return strings.TrimSpace(line[n:]), true
	}
	return "", false
}

func listItem(line string) (prefix, rest string, ok bool) {
	for _, p := range []string{"- ", "* ", "+ "} {
		if strings.HasPrefix(line, p) {
			return BulletMarker, strings.TrimSpace(line[len(p):]), true
		}
	}
	i := 0
	for i < len(line) && line[i] >= '0' && line[i] <= '9' {
		i++
	}
	if i > 0 && i+1 < len(line) && (line[i] == '.' || line[i] == ')') && line[i+1] == ' ' {
		return line[:i+2], strings.TrimSpace(line[i+2:]), true
	}
	return "", "", false
}

func isCallout(line string) bool {
	t := strings.ToLower(strings.TrimSpace(line))
	for _, marker := range []string{"[!note]", "[!tip]", "[!important]", "[!warning]", "[!caution]"} {
		if strings.HasPrefix(t, marker) {
			return true
		}
	}
	return false
}

func isRule(line string) bool {
	if line == "" {
		return false
	}
	var ch byte
	count := 0
	for i := 0; i < len(line); i++ {
		if line[i] == ' ' {
			continue
		}
		if count == 0 {
			ch = line[i]
			if ch != '-' && ch != '*' && ch != '_' {
				return false
			}
		}
		if line[i] != ch {
			return false
		}
		count++
	}
	return count >= 3
}

func isTableSeparator(line string) bool {
	if len(line) < 2 || line[0] != '|' || line[len(line)-1] != '|' {
		return false
	}
	cells := strings.Split(strings.Trim(line, "|"), "|")
	seen := false
	for _, cell := range cells {
		cell = strings.TrimSpace(cell)
		if cell == "" {
			continue
		}
		seen = true
		cell = strings.Trim(cell, ":")
		if len(cell) < 3 {
			return false
		}
		for i := 0; i < len(cell); i++ {
			if cell[i] != '-' {
				return false
			}
		}
	}
	return seen
}

func flattenTable(line string) string {
	cells := strings.Split(strings.Trim(line, "|"), "|")
	parts := make([]string, 0, len(cells))
	for _, cell := range cells {
		if cell = strings.TrimSpace(cell); cell != "" {
			parts = append(parts, cell)
		}
	}
	return strings.Join(parts, " · ")
}

func appendWrapped(out *[]Line, text string, kind Kind, first, next string, width int) {
	for _, line := range wrap(text, first, next, width) {
		*out = append(*out, Line{Text: line, Kind: kind})
	}
}

// wrap breaks text into lines of at most width cells, prefixing the first line
// with first and every continuation with next.
func wrap(text, first, next string, width int) []string {
	if canvas.Width(first) >= width {
		first, next = "", ""
	}
	if canvas.Width(next) >= width {
		next = ""
	}
	var out []string
	prefix := first
	rest := strings.TrimSpace(text)
	for rest != "" {
		avail := width - canvas.Width(prefix)
		if avail < 1 {
			avail = 1
		}
		line, remaining := takeLine(rest, avail)
		if line == "" {
			break
		}
		out = append(out, prefix+line)
		rest = remaining
		prefix = next
	}
	return out
}

func takeLine(s string, width int) (string, string) {
	if width <= 0 {
		return "", s
	}
	if canvas.Width(s) <= width {
		return s, ""
	}
	used, cut, space := 0, 0, -1
	for i, r := range s {
		rw := canvas.Width(string(r))
		if used+rw > width {
			break
		}
		used += rw
		cut = i + len(string(r))
		if r == ' ' {
			space = i
		}
	}
	if cut == 0 {
		_, size := utf8.DecodeRuneInString(s)
		return s[:size], strings.TrimLeft(s[size:], " ")
	}
	if space > 0 {
		return strings.TrimRight(s[:space], " "), strings.TrimLeft(s[space:], " ")
	}
	return s[:cut], strings.TrimLeft(s[cut:], " ")
}

func addSpacer(out *[]Line) {
	if len(*out) == 0 || (*out)[len(*out)-1].Text == "" {
		return
	}
	*out = append(*out, Line{})
}

func trimSpacers(out *[]Line) {
	for len(*out) > 0 && (*out)[len(*out)-1].Text == "" {
		*out = (*out)[:len(*out)-1]
	}
}
