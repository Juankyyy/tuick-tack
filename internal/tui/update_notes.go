package tui

import (
	"strconv"
	"strings"

	"tuick-tack/internal/tui/canvas"
	"tuick-tack/internal/tui/copy"
	"tuick-tack/internal/tui/notes"
)

// updateChooserLayout is the deterministic geometry of the update chooser. The
// view and the key handler share it so the notes pane, the type rows and the
// buttons cannot drift apart when the terminal is resized.
type updateChooserLayout struct {
	rect     rect
	innerX   int
	innerW   int
	typeY    int
	typeRows int
	noteY    int
	noteRows int
	headerY  int
	bodyTop  int
	bodyH    int
	textW    int
	hasNotes bool
}

// updateHasNotes reports whether the release carries a body. The TUI decides
// whether that body has usable lines when it renders them.
func (m *Model) updateHasNotes() bool {
	return m.update != nil && strings.TrimSpace(m.update.Notes) != ""
}

// updateChooserHeight grows the chooser when a notes pane is present: 22 rows
// at 80x24 and 30 rows at 120x40, clamped to the terminal.
func (m *Model) updateChooserHeight() int {
	if !m.updateHasNotes() {
		return 18
	}
	_, h := m.viewSize()
	height := 30
	if max := h - 2; height > max {
		height = max
	}
	if height < 18 {
		height = 18
	}
	return height
}

// updateChooserLayout computes the chooser rows and caches the rendered notes
// for the current width. It also clamps the scroll position when the width or
// the terminal height changes.
func (m *Model) updateChooserLayout() updateChooserLayout {
	m.ensureUpdateType()
	w, h := m.viewSize()
	r := m.modalRect(rect{x: 0, y: 0, w: w, h: h}, 66, m.updateChooserHeight())
	innerX, innerW := r.x+3, r.w-6
	y := r.y + 3
	typeY := y + 3
	typeRows := m.updateTypeRows(innerW)
	noteY := typeY + typeRows
	noteRows := len(m.updateTypeNoteLines(innerW))
	if noteRows < 1 {
		noteRows = 1
	}
	headerY := noteY + noteRows + 1
	bodyTop := headerY + 1
	bodyBottom := r.y + r.h - 5
	bodyH := bodyBottom - bodyTop + 1
	if bodyH < 0 {
		bodyH = 0
	}
	textW := innerW - 4
	if textW < 1 {
		textW = 1
	}
	hasNotes := m.updateHasNotes() && bodyH > 0
	if hasNotes {
		m.ensureUpdateNotes(textW)
		hasNotes = len(m.updateNotes) > 0
	}
	if hasNotes {
		max := len(m.updateNotes) - bodyH
		if max < 0 {
			max = 0
		}
		if m.notesScroll < 0 {
			m.notesScroll = 0
		}
		if m.notesScroll > max {
			m.notesScroll = max
		}
	} else {
		m.notesScroll = 0
	}
	return updateChooserLayout{
		rect:     r,
		innerX:   innerX,
		innerW:   innerW,
		typeY:    typeY,
		typeRows: typeRows,
		noteY:    noteY,
		noteRows: noteRows,
		headerY:  headerY,
		bodyTop:  bodyTop,
		bodyH:    bodyH,
		textW:    textW,
		hasNotes: hasNotes,
	}
}

func (m *Model) ensureUpdateNotes(width int) {
	if m.update == nil || width <= 0 {
		m.updateNotes = nil
		m.updateNotesKey = ""
		return
	}
	key := m.update.Latest + "|" + strconv.Itoa(width) + "|" + strconv.Itoa(len(m.update.Notes))
	if key == m.updateNotesKey {
		return
	}
	m.updateNotes = notes.Render(m.update.Notes, width)
	m.updateNotesKey = key
}

func (m *Model) updateTypeLabel(updateType string) string {
	label := copy.T(copy.UpdateTypePortable)
	if updateType == updateTypeSetup {
		label = copy.T(copy.UpdateTypeSetup)
	}
	return label + " · " + m.updateTypeStatus(updateType)
}

func (m *Model) updateTypePairWidth() int {
	if m.update == nil {
		return 0
	}
	current := m.currentUpdateType()
	prefix := ""
	if m.updateTypeAvailable(m.updateType) {
		prefix = "› "
	}
	return canvas.Width(prefix+m.updateTypeLabel(current)) + 4 +
		canvas.Width(m.updateTypeLabel(m.otherUpdateType(current)))
}

// updateTypeRows returns 1 when both type chips fit on one centered row and 2
// when they must stack. macOS has no type chooser, so it uses no rows.
func (m *Model) updateTypeRows(width int) int {
	if m.singleUpdate {
		return 0
	}
	if m.updateTypePairWidth() <= width {
		return 1
	}
	return 2
}

func (m *Model) bothUpdateTypes() bool {
	if m.update == nil || m.singleUpdate {
		return false
	}
	current := m.currentUpdateType()
	return m.updateTypeAvailable(current) && m.updateTypeAvailable(m.otherUpdateType(current))
}

func (m *Model) updateTypeNoteText() (string, bool) {
	if m.update == nil {
		return "", false
	}
	if !m.updateTypeAvailable(m.updateType) {
		return copy.T(copy.UpdateNoAssets), false
	}
	if m.updateType != m.currentUpdateType() {
		return copy.T(copy.UpdateOtherShort), true
	}
	if m.updateType == updateTypeSetup {
		return copy.T(copy.UpdateCurrentSetup), false
	}
	return copy.T(m.updateText().current), false
}

func (m *Model) updateTypeNoteLines(width int) []string {
	text, _ := m.updateTypeNoteText()
	if text == "" || width <= 0 {
		return nil
	}
	lines := wrapPlain(text, width)
	if len(lines) > 3 {
		lines = lines[:3]
	}
	return lines
}

// updateChooserHint joins the shortcut fragments that fit the inner width,
// preferring Tab and the notes over the github shortcut.
func (m *Model) updateChooserHint(layout updateChooserLayout) string {
	both := m.bothUpdateTypes()
	hasNotes := layout.hasNotes
	type variant struct{ tab, notes, github bool }
	variants := []variant{
		{both, hasNotes, true},
		{both, hasNotes, false},
		{both, false, false},
		{false, hasNotes, false},
		{false, false, false},
	}
	for _, v := range variants {
		if v.tab && !both {
			continue
		}
		if v.notes && !hasNotes {
			continue
		}
		parts := []string{copy.T(copy.UpdateHintSelect)}
		if v.tab {
			parts = append(parts, copy.T(copy.UpdateHintTab))
		}
		if v.notes {
			parts = append(parts, copy.T(copy.UpdateHintNotes))
		}
		if v.github {
			parts = append(parts, copy.T(copy.UpdateHintGithub))
		}
		parts = append(parts, copy.T(copy.UpdateHintLater))
		hint := strings.Join(parts, " · ")
		if canvas.Width(hint) <= layout.innerW {
			return hint
		}
	}
	return fitText(copy.T(copy.UpdateHintSelect)+" · "+copy.T(copy.UpdateHintLater), layout.innerW)
}

func (m *Model) scrollUpdateNotes(delta int) {
	layout := m.updateChooserLayout()
	if !layout.hasNotes {
		return
	}
	max := len(m.updateNotes) - layout.bodyH
	if max < 0 {
		max = 0
	}
	m.notesScroll += delta
	if m.notesScroll < 0 {
		m.notesScroll = 0
	}
	if m.notesScroll > max {
		m.notesScroll = max
	}
}

func (m *Model) updateNotesPage() int {
	layout := m.updateChooserLayout()
	if !layout.hasNotes {
		return 0
	}
	return layout.bodyH
}

func (m *Model) updateNotesMaxScroll() int {
	layout := m.updateChooserLayout()
	if !layout.hasNotes {
		return 0
	}
	max := len(m.updateNotes) - layout.bodyH
	if max < 0 {
		return 0
	}
	return max
}

// wrapPlain is the small word wrapper used by the one-line copy around the
// notes pane. It is not the release-notes renderer.
func wrapPlain(s string, width int) []string {
	if width <= 0 {
		return nil
	}
	var out []string
	line := ""
	for _, word := range strings.Fields(s) {
		for canvas.Width(word) > width {
			if line != "" {
				out = append(out, line)
				line = ""
			}
			head, rest := splitPlain(word, width)
			if head == "" {
				break
			}
			out = append(out, head)
			word = rest
		}
		if word == "" {
			continue
		}
		switch {
		case line == "":
			line = word
		case canvas.Width(line)+1+canvas.Width(word) <= width:
			line += " " + word
		default:
			out = append(out, line)
			line = word
		}
	}
	if line != "" {
		out = append(out, line)
	}
	return out
}

func splitPlain(s string, width int) (string, string) {
	used := 0
	for i, r := range s {
		rw := canvas.Width(string(r))
		if used+rw > width {
			return s[:i], s[i:]
		}
		used += rw
	}
	return s, ""
}
