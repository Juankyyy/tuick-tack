package notes

import (
	"strings"
	"testing"

	"tuick-tack/internal/tui/canvas"
)

func joined(lines []Line) string {
	var b strings.Builder
	for _, line := range lines {
		b.WriteString(line.Text)
		b.WriteByte('\n')
	}
	return b.String()
}

func assertFits(t *testing.T, lines []Line, width int) {
	t.Helper()
	for i, line := range lines {
		if got := canvas.Width(line.Text); got > width {
			t.Errorf("line %d width = %d, want <= %d (%q)", i, got, width, line.Text)
		}
		for _, r := range line.Text {
			if !canvas.SingleCell(r) {
				t.Errorf("line %d contains a non-single-cell rune %q (%q)", i, r, line.Text)
			}
		}
	}
}

func TestRenderExtractsWhatsNew(t *testing.T) {
	body := "## 🎉 What's new in v0.2.0\n\n" +
		"### 🎨 A pixel identity\n\n" +
		"The tray icons are pixel art.\n\n" +
		"- One bullet\n- Another bullet\n\n" +
		"## ⬆️ Upgrading from v0.1.2\n\n" +
		"You must quit first.\n\n" +
		"## 📦 Download\n\n" +
		"| File | Size |\n|---|---:|\n| tuick-tack.exe | 10 MB |\n"
	lines := Render(body, 60)
	text := joined(lines)
	for _, want := range []string{"A pixel identity", "The tray icons are pixel art.", "One bullet", "Another bullet"} {
		if !strings.Contains(text, want) {
			t.Errorf("notes missing %q:\n%s", want, text)
		}
	}
	for _, unwanted := range []string{"Upgrading", "quit first", "Download", "10 MB", "---"} {
		if strings.Contains(text, unwanted) {
			t.Errorf("notes should stop at the next section, found %q:\n%s", unwanted, text)
		}
	}
	assertFits(t, lines, 60)
}

func TestRenderFallsBackToWholeBody(t *testing.T) {
	body := "### Changed\n\nA change without a What's new heading.\n"
	text := joined(Render(body, 60))
	if !strings.Contains(text, "Changed") || !strings.Contains(text, "A change without") {
		t.Fatalf("fallback lost the body:\n%s", text)
	}
}

func TestRenderCleansMarkdownAndTerminalText(t *testing.T) {
	body := "## What's new\n\n" +
		"A **bold** [link](https://example.test) and `code`.\n\n" +
		"![alt](x.png) 🎉 日本語\n\n" +
		"<span>html</span> &amp; text\n\n" +
		"\x1b[31mred\x1b[0m\n\n" +
		"<https://example.test/keep>\n"
	lines := Render(body, 80)
	text := joined(lines)
	for _, want := range []string{"A bold link and code.", "alt", "html & text", "red", "<https://example.test/keep>"} {
		if !strings.Contains(text, want) {
			t.Errorf("cleaned notes missing %q:\n%s", want, text)
		}
	}
	for _, unwanted := range []string{"**", "[link]", "`", "🎉", "日本", "\x1b", "[31m", "<span>", "&amp;"} {
		if strings.Contains(text, unwanted) {
			t.Errorf("cleaned notes still contain %q:\n%s", unwanted, text)
		}
	}
	assertFits(t, lines, 80)
}

func TestRenderWrapsWithHangingIndent(t *testing.T) {
	body := "## What's new\n\n- The update dialog now shows the release notes for the version you are about to install.\n"
	lines := Render(body, 24)
	assertFits(t, lines, 24)
	first := -1
	for i, line := range lines {
		if line.Kind == Bullet {
			first = i
			break
		}
	}
	if first < 0 {
		t.Fatalf("expected a wrapped bullet, got %#v", lines)
	}
	if !strings.HasPrefix(lines[first].Text, BulletMarker) {
		t.Fatalf("first bullet line = %#v", lines[first])
	}
	if len(lines[first:]) < 2 {
		t.Fatalf("bullet did not wrap: %#v", lines[first:])
	}
	for _, line := range lines[first+1:] {
		if line.Text == "" {
			continue
		}
		if !strings.HasPrefix(line.Text, "  ") {
			t.Fatalf("continuation does not keep the hanging indent: %q", line.Text)
		}
	}
}

func TestRenderTables(t *testing.T) {
	body := "## What's new\n\n| File | Size |\n|---|---:|\n| app.exe | 10 MB |\n"
	text := joined(Render(body, 60))
	if !strings.Contains(text, "File · Size") || !strings.Contains(text, "app.exe · 10 MB") {
		t.Fatalf("table was not flattened:\n%s", text)
	}
	if strings.Contains(text, "---") {
		t.Fatalf("table separator leaked:\n%s", text)
	}
}

func TestRenderQuotesAndCallouts(t *testing.T) {
	body := "## What's new\n\n> [!IMPORTANT]\n> Quoted text.\n\n---\n\nAfter.\n"
	lines := Render(body, 40)
	text := joined(lines)
	if strings.Contains(text, "IMPORTANT") {
		t.Fatalf("callout marker leaked:\n%s", text)
	}
	if !strings.Contains(text, QuoteMarker+"Quoted text.") {
		t.Fatalf("quote line missing its marker:\n%s", text)
	}
	if !strings.Contains(text, "After.") {
		t.Fatalf("body after the rule missing:\n%s", text)
	}
	assertFits(t, lines, 40)
}

func TestRenderNumberedList(t *testing.T) {
	body := "## What's new\n\n1. First step\n2. Second step\n"
	lines := Render(body, 40)
	if len(lines) < 2 || !strings.HasPrefix(lines[0].Text, "1. First step") ||
		!strings.HasPrefix(lines[1].Text, "2. Second step") {
		t.Fatalf("numbered list = %#v", lines)
	}
}

func TestRenderCurlyApostropheHeading(t *testing.T) {
	body := "## What’s new\n\nA real change.\n\n## Download\n\nNope.\n"
	text := joined(Render(body, 40))
	if !strings.Contains(text, "A real change.") || strings.Contains(text, "Nope.") {
		t.Fatalf("curly heading did not select the section:\n%s", text)
	}
}

func TestRenderEmpty(t *testing.T) {
	if lines := Render("", 40); len(lines) != 0 {
		t.Fatalf("empty notes = %#v", lines)
	}
	if lines := Render("## What's new\n\n## Download\n", 40); len(lines) != 0 {
		t.Fatalf("notes with an empty section = %#v", lines)
	}
	if lines := Render("text", 0); len(lines) != 0 {
		t.Fatalf("width 0 = %#v", lines)
	}
}

func TestRenderLongWord(t *testing.T) {
	body := "## What's new\n\n" + strings.Repeat("a", 90) + "\n"
	lines := Render(body, 20)
	assertFits(t, lines, 20)
	if len(lines) < 5 {
		t.Fatalf("long word was not broken: %#v", lines)
	}
}

func TestRenderNeverLeaksWideRunes(t *testing.T) {
	body := "## What's new\n\n" +
		"Emoji 🚀🎉 and CJK 日本語 and combining e\u0301 and bidi \u202eRTL\u202c.\n" +
		"\x1b]0;title\x07 control \x00 text.\n"
	for _, width := range []int{20, 36, 56, 60} {
		lines := Render(body, width)
		assertFits(t, lines, width)
	}
}
