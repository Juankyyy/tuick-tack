package tui

import (
	"fmt"
	"image/color"
	"math"
	"sort"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"tuick-tack/internal/engine"
	"tuick-tack/internal/protocol"
	"tuick-tack/internal/stats"
	"tuick-tack/internal/tui/canvas"
	"tuick-tack/internal/tui/copy"
	"tuick-tack/internal/tui/theme"
)

// Statistics tab sections.
const (
	statsTabOverview = iota
	statsTabActivity
	statsTabPresets
	statsTabRecent
	statsTabCount
)

// Selectable overview ranges.
const (
	statsRange7 = iota
	statsRange30
	statsRange90
	statsRangeAll
	statsRangeCount
)

// The activity heatmap always spans a year (GitHub-style). At wide sizes each
// day is two cells so the square reads as a square in a terminal cell.
const (
	heatWeeks    = 53
	heatDays     = 7
	heatCellWide = 2
)

// heatRamp is the five-step tint of the heatmap, from an almost empty day to
// the brightest one. The empty step is deliberately above the panel so the
// whole grid stays visible.
var heatRamp = [...]float64{0.10, 0.28, 0.46, 0.64, 0.86}

// heatGrid is the last geometry of the activity heatmap. drawStats fills x and
// y; heatDayAt uses it to map the pointer to a date without hitboxes.
type heatGrid struct {
	x, y     int
	row      int
	cellW    int
	weeks    int
	firstMon time.Time
}

// openStats opens the tab and asks the daemon for a fresh summary. The previous
// summary stays cached until the reply arrives.
func (m *Model) openStats() tea.Cmd {
	if !m.connected {
		m.showToast(copy.T(copy.ErrorOffline))
		return nil
	}
	m.modal = modalStats
	m.statsTab = statsTabOverview
	m.statsRange = statsRange7
	m.statsScroll = 0
	m.statsHeatHover = ""
	m.statsHeat = heatGrid{}
	return m.requestStats()
}

// requestStats asks the daemon for the aggregated history.
func (m *Model) requestStats() tea.Cmd {
	if !m.connected {
		return nil
	}
	m.statsLoading = true
	return m.send(protocol.ClientMessage{Type: protocol.TypeGetStats})
}

// statsChanged reports whether a state snapshot may have added a record, so the
// open Statistics tab refreshes itself without polling. A finish closes the tab
// for the celebration; a stop or a tray replacement keeps it open.
func statsChanged(prev, next protocol.StateMessage) bool {
	wasActive := prev.Status == string(engine.Running) || prev.Status == string(engine.Paused)
	if !wasActive {
		return false
	}
	if next.Status == string(engine.Idle) {
		return true
	}
	return next.Status == string(engine.Running) &&
		(next.DurationMS != prev.DurationMS || next.Label != prev.Label)
}

func (m *Model) setStatsTab(tab int) {
	if tab < 0 || tab >= statsTabCount {
		return
	}
	m.statsTab = tab
	m.statsScroll = 0
	m.statsHeatHover = ""
}

func (m *Model) scrollStats(delta int) {
	m.statsScroll += delta
	m.clampStatsScroll()
}

func (m *Model) clampStatsScroll() {
	if m.modal != modalStats {
		m.statsScroll = 0
		return
	}
	max := m.statsMaxScroll()
	if m.statsScroll > max {
		m.statsScroll = max
	}
	if m.statsScroll < 0 {
		m.statsScroll = 0
	}
}

func (m *Model) statsMaxScroll() int {
	if m.statsData == nil {
		return 0
	}
	_, w, _, h := m.statsContentArea()
	rows := m.statsRows(w, h)
	if len(rows) <= h {
		return 0
	}
	return len(rows) - h
}

func (m *Model) statsPage() int {
	_, _, _, h := m.statsContentArea()
	if h < 4 {
		return 3
	}
	return h - 2
}

// handleStatsKey drives the tab. The tab is a view, not a command surface, so
// nothing here changes timer or config state.
func (m *Model) handleStatsKey(key tea.Key) tea.Cmd {
	text := key.Text
	switch {
	case key.Code == tea.KeyEscape || text == "s":
		m.modal = modalNone
	case key.Code == tea.KeyLeft:
		m.setStatsTab((m.statsTab + statsTabCount - 1) % statsTabCount)
	case key.Code == tea.KeyRight, key.Code == tea.KeyTab:
		m.setStatsTab((m.statsTab + 1) % statsTabCount)
	case text == "t" && m.statsTab == statsTabOverview:
		// The range only summarizes the overview; on the other sections it
		// would change a value that is not on screen.
		m.statsRange = (m.statsRange + 1) % statsRangeCount
		m.statsScroll = 0
	case len(text) == 1 && text[0] >= '1' && text[0] <= '4':
		m.setStatsTab(int(text[0] - '1'))
	case key.Code == tea.KeyUp:
		m.scrollStats(-1)
	case key.Code == tea.KeyDown:
		m.scrollStats(1)
	case key.Code == tea.KeyPgUp:
		m.scrollStats(-m.statsPage())
	case key.Code == tea.KeyPgDown:
		m.scrollStats(m.statsPage())
	case key.Code == tea.KeyHome:
		m.statsScroll = 0
	case key.Code == tea.KeyEnd:
		m.statsScroll = m.statsMaxScroll()
	}
	return nil
}

// statsRect is the sheet geometry. It uses most of the terminal so the charts
// get room, but keeps a margin for the dimmed stage behind it.
func (m *Model) statsRect() rect {
	baseW, baseH := m.viewSize()
	// The year heatmap needs room for 53 two-cell days at 120 columns.
	w := minInt(114, baseW-6)
	h := minInt(26, baseH-2)
	if w < 24 {
		w = baseW
	}
	if h < 6 {
		h = baseH
	}
	return rect{x: (m.width - w) / 2, y: (m.height - h) / 2, w: w, h: h}
}

// statsContentArea returns the inner rectangle used by the section content.
func (m *Model) statsContentArea() (x, w, y, h int) {
	r := m.statsRect()
	contentY := r.y + 3
	return r.x + 2, r.w - 4, contentY, (r.y + r.h - 3) - contentY
}

func (m *Model) statsLocation() *time.Location {
	loc := m.now().Location()
	if loc == nil {
		return time.Local
	}
	return loc
}

// statCell is one styled run inside a statistics row.
type statCell struct {
	text  string
	fg    color.RGBA
	hasFG bool
	bg    color.RGBA
	hasBG bool
	bold  bool
}

type statLine []statCell

func statTxt(s string, fg color.RGBA) statCell {
	return statCell{text: s, fg: fg, hasFG: true}
}

func statBold(s string, fg color.RGBA) statCell {
	return statCell{text: s, fg: fg, hasFG: true, bold: true}
}

func statBG(s string, fg, bg color.RGBA) statCell {
	return statCell{text: s, fg: fg, hasFG: true, bg: bg, hasBG: true}
}

func statPad(n int) statCell {
	if n <= 0 {
		return statCell{}
	}
	return statCell{text: strings.Repeat(" ", n)}
}

func lineWidth(line statLine) int {
	total := 0
	for _, cell := range line {
		total += canvas.Width(cell.text)
	}
	return total
}

func padRight(s string, n int) string {
	if w := canvas.Width(s); w < n {
		return s + strings.Repeat(" ", n-w)
	}
	return s
}

func padLeft(s string, n int) string {
	if w := canvas.Width(s); w < n {
		return strings.Repeat(" ", n-w) + s
	}
	return s
}

// statDuration renders milliseconds compactly for charts and tables.
func statDuration(ms int64) string {
	if ms < 0 {
		ms = 0
	}
	secs := (ms + 500) / 1000
	if secs == 0 {
		return "0m"
	}
	if secs < 60 {
		return fmt.Sprintf("%ds", secs)
	}
	mins := secs / 60
	if mins < 60 {
		return fmt.Sprintf("%dm", mins)
	}
	hours := mins / 60
	mins %= 60
	if mins == 0 {
		return fmt.Sprintf("%dh", hours)
	}
	return fmt.Sprintf("%dh %02dm", hours, mins)
}

func statsTabKey(tab int) copy.Key {
	switch tab {
	case statsTabActivity:
		return copy.StatsActivity
	case statsTabPresets:
		return copy.StatsPresets
	case statsTabRecent:
		return copy.StatsRecent
	default:
		return copy.StatsOverview
	}
}

func (m *Model) statsRangeLabel() string {
	switch m.statsRange {
	case statsRange30:
		return copy.T(copy.StatsRange30)
	case statsRange90:
		return copy.T(copy.StatsRange90)
	case statsRangeAll:
		return copy.T(copy.StatsRangeAll)
	default:
		return copy.T(copy.StatsRange7)
	}
}

func (m *Model) statsRangeTotals() protocol.StatsRange {
	switch m.statsRange {
	case statsRange30:
		return m.statsData.Totals.D30
	case statsRange90:
		return m.statsData.Totals.D90
	case statsRangeAll:
		return m.statsData.Totals.All
	default:
		return m.statsData.Totals.D7
	}
}

func (m *Model) statsRangeDays() int {
	switch m.statsRange {
	case statsRange30:
		return 30
	case statsRange90:
		return 90
	default:
		return 7
	}
}

// statsPrevMS sums the window before the selected one from the daily series.
func (m *Model) statsPrevMS(days int) int64 {
	if days <= 0 || m.statsData == nil {
		return 0
	}
	daily := m.statsData.Daily
	if len(daily) < 2*days {
		return 0
	}
	var sum int64
	for _, day := range daily[len(daily)-2*days : len(daily)-days] {
		sum += day.MS
	}
	return sum
}

// statsRows builds the scrollable content of the active section. The width is
// the inner sheet width and the height is the visible content rows, so charts
// use the room the terminal offers.
func (m *Model) statsRows(w, h int) []statLine {
	if m.statsData == nil || w <= 0 {
		return nil
	}
	switch m.statsTab {
	case statsTabActivity:
		return m.statsActivityRows(w, h)
	case statsTabPresets:
		return m.statsPresetsRows(w)
	case statsTabRecent:
		return m.statsRecentRows(w)
	default:
		return m.statsOverviewRows(w, h)
	}
}

// statsHeader is a section title with a quiet right-aligned note.
func (m *Model) statsHeader(left, right string, w int) statLine {
	line := statLine{statTxt(" "+left, m.pal.Text)}
	if right == "" {
		return line
	}
	pad := w - 1 - canvas.Width(left) - canvas.Width(right)
	if pad < 1 {
		pad = 1
	}
	line = append(line, statPad(pad), statTxt(right, m.pal.Muted))
	return line
}

func (m *Model) statsOverviewRows(w, h int) []statLine {
	p := m.statsData
	kpi := m.statsKPILines(w)
	rows := []statLine{
		m.statsHeader(copy.T(copy.StatsOverview), m.statsRangeLabel(), w),
	}
	rows = append(rows, kpi...)
	rows = append(rows, statLine{})

	days := 7
	barW := 2
	gap := 1
	weekdays := true
	if m.statsRange != statsRange7 {
		days = 30
		barW = 1
		weekdays = false
	} else if fit := (w+gap)/7 - gap; fit > barW {
		// Give the seven daily bars the room the terminal offers.
		barW = minInt(8, fit)
	}
	cols := days
	if fit := (w + gap) / (barW + gap); cols > fit {
		cols = fit
	}
	if cols < 1 {
		cols = 1
	}
	window := p.Daily
	if len(window) > cols {
		window = window[len(window)-cols:]
	}
	vals := make([]int64, len(window))
	var max int64
	for i, day := range window {
		vals[i] = day.MS
		if day.MS > max {
			max = day.MS
		}
	}
	if max <= 0 {
		max = 1
	}

	// The bars are always the recent daily window; the section header shows
	// the range the KPI above them summarizes.
	note := copy.T(copy.StatsMax, statDuration(max)) + " · " + copy.T(copy.StatsLastDays, len(window))
	rows = append(rows, m.statsHeader(copy.T(copy.StatsFocusPerDay), note, w))

	// Reserve the fixed rows so the chart grows into whatever the terminal
	// leaves without pushing the records off the sheet.
	fixed := 1 + len(kpi) + 1 + 1 + 1 + 1 + 1
	chartH := h - fixed
	if chartH > 10 {
		chartH = 10
	}
	if chartH < 3 {
		chartH = 3
	}
	rows = append(rows, statChart(m, statBars{
		values:   vals,
		max:      max,
		height:   chartH,
		barW:     barW,
		gap:      gap,
		accent:   m.pal.Accent,
		last:     m.pal.Accent2,
		markLast: true,
	})...)
	rows = append(rows, m.statsDayLabels(window, barW, gap, weekdays))
	rows = append(rows, m.statsHeader(copy.T(copy.StatsRecordsHead), copy.T(copy.StatsRangeAll), w))
	if rec := m.statsRecordsLine(w); len(rec) > 0 {
		rows = append(rows, rec)
	}
	return rows
}

// statsKPILines packs the summary into one line when it fits and into two when
// the sheet is narrow, instead of letting the last figure get clipped.
func (m *Model) statsKPILines(w int) []statLine {
	tr := m.statsRangeTotals()
	line := statLine{
		statPad(1),
		statBold(statDuration(tr.MS), m.pal.Text),
		statTxt(" "+copy.T(copy.StatsFocused), m.pal.Muted),
		statTxt(" · ", m.pal.Muted),
		statBold(fmt.Sprintf("%d", tr.Sessions), m.pal.Text),
		statTxt(" "+copy.T(copy.StatsSessions), m.pal.Muted),
	}
	var parts []statLine
	if done := tr.Completed + tr.Stopped; done > 0 {
		pct := int(math.Round(100 * float64(tr.Completed) / float64(done)))
		parts = append(parts, statLine{
			statBold(fmt.Sprintf("%d%%", pct), m.pal.Text),
			statTxt(" "+copy.T(copy.StatsOfFinished), m.pal.Muted),
		})
	}
	if m.statsRange != statsRangeAll {
		days := m.statsRangeDays()
		if prev := m.statsPrevMS(days); prev > 0 {
			delta := int(math.Round(100 * float64(tr.MS-prev) / float64(prev)))
			col := m.pal.Accent
			if delta < 0 {
				col = m.pal.Warn
			}
			parts = append(parts, statLine{
				statBold(fmt.Sprintf("%+d%%", delta), col),
				statTxt(" "+copy.T(copy.StatsVsPrev, m.statsRangeLabel()), m.pal.Muted),
			})
		}
	}
	lines := []statLine{line}
	for _, part := range parts {
		last := len(lines) - 1
		candidate := append(append(statLine{}, lines[last]...), statTxt(" · ", m.pal.Muted))
		candidate = append(candidate, part...)
		if lineWidth(candidate) <= w {
			lines[last] = candidate
			continue
		}
		if len(lines) == 1 {
			next := append(statLine{statPad(1)}, part...)
			if lineWidth(next) <= w {
				lines = append(lines, next)
			}
		}
	}
	return lines
}

// statBars describes one vertical bar chart. gap is the number of empty cells
// between bars; the hour chart uses none so its 24 columns line up with the
// 24-character axis.
type statBars struct {
	values   []int64
	max      int64
	height   int
	barW     int
	gap      int
	accent   color.RGBA
	last     color.RGBA
	markLast bool
}

func statBarsWidth(n, barW, gap int) int {
	if n <= 0 {
		return 0
	}
	return n*barW + (n-1)*gap
}

// statChart draws vertical bars with half-block resolution. Bars are barW cells
// wide and separated by gap cells. markLast paints the newest column with last,
// which only the daily chart uses for "today".
func statChart(m *Model, c statBars) []statLine {
	if c.height <= 0 || len(c.values) == 0 {
		return nil
	}
	rows := make([]statLine, 0, c.height)
	last := len(c.values) - 1
	for r := 0; r < c.height; r++ {
		line := statLine{}
		for i, v := range c.values {
			if i > 0 && c.gap > 0 {
				line = append(line, statCell{text: strings.Repeat(" ", c.gap)})
			}
			halves := 0
			if c.max > 0 {
				halves = int(math.Round(float64(v) / float64(c.max) * float64(c.height*2)))
			}
			// A day with a little focus must not look empty; rounding a small
			// share down to zero would hide it completely.
			if v > 0 && halves < 1 {
				halves = 1
			}
			need := 2 * (c.height - r)
			text := strings.Repeat(" ", c.barW)
			col := c.accent
			switch {
			case halves >= need:
				text = strings.Repeat("█", c.barW)
			case halves == need-1:
				text = strings.Repeat("▄", c.barW)
			case r == c.height-1 && v == 0:
				left := maxInt(0, (c.barW-1)/2)
				text = strings.Repeat(" ", left) + "·" + strings.Repeat(" ", maxInt(0, c.barW-1-left))
				col = theme.Lerp(m.pal.Panel, m.pal.Muted, 0.5)
			}
			if c.markLast && i == last {
				col = c.last
			}
			line = append(line, statTxt(text, col))
		}
		rows = append(rows, line)
	}
	return rows
}

// statsDayLabels builds a label row with the exact width of the bar chart. The
// labels are stamped at their bar's column; appending them would drift because
// a day number is wider than a one-cell bar.
func (m *Model) statsDayLabels(window []protocol.StatsDay, barW, gap int, weekdays bool) statLine {
	width := statBarsWidth(len(window), barW, gap)
	if width <= 0 {
		return nil
	}
	cells := make([]statCell, width)
	loc := m.statsLocation()
	var lastMonth time.Month
	for i, day := range window {
		t, err := time.ParseInLocation("2006-01-02", day.Date, loc)
		if err != nil {
			continue
		}
		label := ""
		if weekdays {
			label = copy.WeekdayNarrow((int(t.Weekday()) + 6) % 7)
		} else if i%7 == 0 {
			label = fmt.Sprintf("%02d", t.Day())
			if lastMonth == 0 || t.Month() != lastMonth {
				label = copy.MonthShort(t.Month()) + " " + label
				lastMonth = t.Month()
			}
		}
		if label == "" {
			continue
		}
		col := i * (barW + gap)
		off := (barW - canvas.Width(label)) / 2
		if off < 0 {
			off = 0
		}
		for k, r := range []rune(label) {
			pos := col + off + k
			if pos < 0 || pos >= width {
				continue
			}
			cells[pos] = statCell{text: string(r), fg: m.pal.Muted, hasFG: true}
		}
	}
	line := make(statLine, 0, width)
	for _, cell := range cells {
		if cell.text == "" {
			cell = statCell{text: " "}
		}
		line = append(line, cell)
	}
	return line
}

func firstRunes(s string, n int) string {
	if n <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) > n {
		runes = runes[:n]
	}
	return string(runes)
}

func (m *Model) statsRecordsLine(w int) statLine {
	rec := m.statsData.Records
	line := statLine{statPad(1)}
	added := 0
	// appendPart adds the first candidate that fits, so a long detail can fall
	// back to its compact form instead of dropping the whole value.
	appendPart := func(parts ...statLine) {
		for _, part := range parts {
			candidate := append(statLine{}, line...)
			if added > 0 {
				candidate = append(candidate, statTxt(" · ", m.pal.Muted))
			}
			candidate = append(candidate, part...)
			if lineWidth(candidate) > w {
				continue
			}
			line = candidate
			added++
			return
		}
	}
	if rec.StreakDays > 0 {
		appendPart(statLine{statBold(copy.T(copy.StatsStreak, rec.StreakDays), m.pal.Accent)})
	}
	appendPart(statLine{statTxt(copy.T(copy.StatsBestStreak, rec.BestStreakDays), m.pal.Muted)})
	if rec.BusiestWeekday >= 0 && rec.BusiestHour >= 0 {
		appendPart(statLine{statTxt(copy.T(copy.StatsBusiest,
			copy.WeekdayShort(rec.BusiestWeekday), rec.BusiestHour), m.pal.Muted)})
	}
	longest := copy.T(copy.StatsLongest, statDuration(rec.LongestMS))
	if rec.LongestLabel != "" {
		appendPart(
			statLine{statTxt(longest+" ("+fitText(rec.LongestLabel, 12)+")", m.pal.Muted)},
			statLine{statTxt(longest, m.pal.Muted)},
		)
	} else {
		appendPart(statLine{statTxt(longest, m.pal.Muted)})
	}
	if added == 0 {
		return nil
	}
	return line
}

func (m *Model) statsActivityRows(w, h int) []statLine {
	p := m.statsData
	// statsMaxScroll also builds the rows; keep the drawn origin so a mouse
	// motion before the next draw still maps to the right cells.
	lastX, lastY := m.statsHeat.x, m.statsHeat.y
	m.statsHeat = heatGrid{}
	if w <= 3 {
		return nil
	}
	loc := m.statsLocation()
	byDate := make(map[string]protocol.StatsDay, len(p.Daily))
	for _, day := range p.Daily {
		byDate[day.Date] = day
	}
	now := m.now().In(loc)
	today := time.Date(now.Year(), now.Month(), now.Day(), 12, 0, 0, 0, loc)
	thisMonday := today.AddDate(0, 0, -((int(today.Weekday()) + 6) % 7))

	// At 120 columns each day gets two cells, which reads as a square in a
	// terminal; narrower sheets keep one cell and show as many of the 53 weeks
	// as fit.
	cellW := 1
	if w-3 >= heatWeeks*heatCellWide {
		cellW = heatCellWide
	}
	weeks := (w - 3) / cellW
	if weeks > heatWeeks {
		weeks = heatWeeks
	}
	if weeks < 1 {
		weeks = 1
	}
	for weeks > 1 && 3+weeks*cellW > w {
		weeks--
	}
	firstMonday := thisMonday.AddDate(0, 0, -7*(weeks-1))
	firstKey := firstMonday.Format("2006-01-02")
	todayKey := today.Format("2006-01-02")
	if m.statsHeatHover != "" && (m.statsHeatHover < firstKey || m.statsHeatHover > todayKey) {
		// A refresh or midnight can move the window out from under a hover.
		m.statsHeatHover = ""
	}

	// Collect the visible window once: its total feeds the summary and its
	// active days feed the quartile thresholds of the tint ramp.
	active := make([]int64, 0, weeks*heatDays)
	var totalMS int64
	var totalN int
	for i := 0; i < weeks; i++ {
		monday := firstMonday.AddDate(0, 0, 7*i)
		for d := 0; d < heatDays; d++ {
			day := monday.AddDate(0, 0, d)
			if day.After(today) {
				continue
			}
			value := byDate[day.Format("2006-01-02")]
			if value.MS > 0 {
				active = append(active, value.MS)
			}
			totalMS += value.MS
			totalN += value.N
		}
	}
	thresholds := heatThresholds(active)

	rows := []statLine{m.statsHeader(copy.T(copy.StatsActivity), copy.T(copy.StatsWeeks, weeks), w)}
	rows = append(rows, m.heatMonthLabels(firstMonday, weeks, cellW))

	gridRow := len(rows)
	for d := 0; d < heatDays; d++ {
		label := ""
		switch d {
		case 0, 2, 4:
			label = firstRunes(copy.WeekdayShort(d), 1)
		}
		// Keep the gutter at three cells even on rows without a label, or the
		// unlabelled rows would shift one column left of the labelled ones.
		line := statLine{statTxt(" "+padRight(label, 1)+" ", m.pal.Muted)}
		for i := 0; i < weeks; i++ {
			day := firstMonday.AddDate(0, 0, 7*i+d)
			key := day.Format("2006-01-02")
			bg := m.heatColor(0)
			if day.After(today) {
				// Leave the future days empty instead of a dark square.
				bg = m.pal.Panel
			} else if value := byDate[key]; value.MS > 0 {
				bg = m.heatColor(heatLevel(value.MS, thresholds))
			}
			if key == m.statsHeatHover {
				bg = theme.Lerp(bg, m.pal.Text, 0.32)
			}
			line = append(line, statBG(strings.Repeat(" ", cellW), m.pal.Text, bg))
		}
		rows = append(rows, line)
	}
	m.statsHeat = heatGrid{
		x:        lastX,
		y:        lastY,
		row:      gridRow,
		cellW:    cellW,
		weeks:    weeks,
		firstMon: firstMonday,
	}

	rows = append(rows, m.heatSummaryLines(w, cellW, totalMS, totalN)...)
	rows = append(rows, m.heatDetailLine(w, byDate, today))
	if h >= 16 {
		rows = append(rows, statLine{})
	}
	rows = append(rows, m.statsHeader(copy.T(copy.StatsWhenFocus), copy.T(copy.StatsRangeAll), w))

	hourVals := make([]int64, 24)
	for i := range hourVals {
		if i < len(p.Hourly) {
			hourVals[i] = p.Hourly[i].MS
		}
	}
	weekdayVals := make([]int64, 7)
	for i := range weekdayVals {
		if i < len(p.Weekday) {
			weekdayVals[i] = p.Weekday[i].MS
		}
	}
	hourMax := maxValue(hourVals)
	weekdayMax := maxValue(weekdayVals)

	hourBarW, hourGap := 1, 0
	if w >= 100 {
		hourBarW, hourGap = heatCellWide, 1
	}
	const (
		weekdayBarW = 2
		weekdayGap  = 1
		columnGap   = 4
	)
	hourW := statBarsWidth(len(hourVals), hourBarW, hourGap)
	weekdayW := statBarsWidth(len(weekdayVals), weekdayBarW, weekdayGap)

	hourChart := statChart(m, statBars{
		values: hourVals, max: hourMax, height: 3,
		barW: hourBarW, gap: hourGap, accent: m.pal.Accent,
	})
	weekdayChart := statChart(m, statBars{
		values: weekdayVals, max: weekdayMax, height: 3,
		barW: weekdayBarW, gap: weekdayGap, accent: m.pal.Accent,
	})

	if w >= 1+hourW+columnGap+weekdayW {
		sub := statLine{statPad(1), statTxt(copy.T(copy.StatsByHour), m.pal.Muted)}
		if pad := 1 + hourW + columnGap - lineWidth(sub); pad > 0 {
			sub = append(sub, statPad(pad))
		}
		sub = append(sub, statTxt(copy.T(copy.StatsByWeekday), m.pal.Muted))
		rows = append(rows, sub)
		for i := 0; i < 3; i++ {
			line := append(statLine{statPad(1)}, hourChart[i]...)
			line = append(line, statPad(columnGap))
			line = append(line, weekdayChart[i]...)
			rows = append(rows, line)
		}
		axis := statLine{
			statPad(1), statTxt(hourAxis(hourBarW, hourGap), m.pal.Muted),
			statPad(columnGap), statTxt(weekdayAxis(), m.pal.Muted),
		}
		rows = append(rows, axis)
	} else {
		rows = append(rows, statLine{statPad(1), statTxt(copy.T(copy.StatsByHour), m.pal.Muted)})
		for _, row := range hourChart {
			rows = append(rows, append(statLine{statPad(1)}, row...))
		}
		rows = append(rows, statLine{statPad(1), statTxt(hourAxis(hourBarW, hourGap), m.pal.Muted)})
		rows = append(rows, statLine{statPad(1), statTxt(copy.T(copy.StatsByWeekday), m.pal.Muted)})
		for _, row := range weekdayChart {
			rows = append(rows, append(statLine{statPad(1)}, row...))
		}
		rows = append(rows, statLine{statPad(1), statTxt(weekdayAxis(), m.pal.Muted)})
	}

	if insight := m.statsInsightLine(w); insight != nil {
		switch room := h - len(rows); {
		case room >= 2:
			rows = append(rows, statLine{}, insight)
		case room == 1:
			rows = append(rows, insight)
		}
	}
	return rows
}

// heatThresholds returns the quartiles of the active days. A handful of
// sessions should not be flattened by one outlier, which is what a
// max-relative ramp does; the thresholds adapt to the user's own history.
func heatThresholds(active []int64) [3]int64 {
	if len(active) == 0 {
		return [3]int64{}
	}
	sort.Slice(active, func(i, j int) bool { return active[i] < active[j] })
	// Index by p*n rather than p*(n-1): with only two or three active days the
	// latter collapses the thresholds and paints every square at the top step.
	quartile := func(p float64) int64 {
		idx := int(p * float64(len(active)))
		if idx < 0 {
			idx = 0
		}
		if idx >= len(active) {
			idx = len(active) - 1
		}
		return active[idx]
	}
	return [3]int64{quartile(0.25), quartile(0.5), quartile(0.75)}
}

// heatLevel maps a day to its tint step. Values at or above a quartile use the
// higher step, so a single active day is still the brightest square.
func heatLevel(ms int64, thresholds [3]int64) int {
	switch {
	case ms <= 0:
		return 0
	case ms >= thresholds[2]:
		return 4
	case ms >= thresholds[1]:
		return 3
	case ms >= thresholds[0]:
		return 2
	default:
		return 1
	}
}

func (m *Model) heatColor(level int) color.RGBA {
	if level < 0 {
		level = 0
	}
	if level >= len(heatRamp) {
		level = len(heatRamp) - 1
	}
	return theme.Lerp(m.pal.Panel, m.pal.Accent, heatRamp[level])
}

// heatMonthLabels draws one short month name per month run. A partial first
// month is skipped, like GitHub does, and a label that would touch the previous
// one is dropped instead of overlapping it.
func (m *Model) heatMonthLabels(firstMonday time.Time, weeks, cellW int) statLine {
	width := weeks * cellW
	if width <= 0 {
		return nil
	}
	cells := make([]rune, width)
	for i := range cells {
		cells[i] = ' '
	}
	type monthRun struct {
		year  int
		month time.Month
		start int
		weeks int
	}
	runs := make([]monthRun, 0, 13)
	for i := 0; i < weeks; i++ {
		monday := firstMonday.AddDate(0, 0, 7*i)
		if len(runs) == 0 {
			runs = append(runs, monthRun{year: monday.Year(), month: monday.Month(), start: i, weeks: 1})
			continue
		}
		last := &runs[len(runs)-1]
		if monday.Year() == last.year && monday.Month() == last.month {
			last.weeks++
			continue
		}
		runs = append(runs, monthRun{year: monday.Year(), month: monday.Month(), start: i, weeks: 1})
	}
	lastEnd := -2
	for _, run := range runs {
		if run.weeks < 2 {
			continue
		}
		label := []rune(copy.MonthShort(run.month))
		at := run.start * cellW
		if at <= lastEnd+1 || at+len(label) > width {
			continue
		}
		for k, r := range label {
			cells[at+k] = r
		}
		lastEnd = at + len(label) - 1
	}
	return statLine{statPad(3), statTxt(string(cells), m.pal.Muted)}
}

// heatSummaryLines puts the focus total and the less/more legend on one row
// when they fit, or on two rows on a narrow sheet.
func (m *Model) heatSummaryLines(w, cellW int, ms int64, n int) []statLine {
	word := copy.T(copy.StatsSessions)
	if n == 1 {
		word = copy.T(copy.StatsSession)
	}
	left := statLine{
		statPad(1),
		statBold(statDuration(ms), m.pal.Text),
		statTxt(" "+copy.T(copy.StatsFocused)+" · ", m.pal.Muted),
		statBold(fmt.Sprintf("%d", n), m.pal.Text),
		statTxt(" "+word, m.pal.Muted),
	}
	legend := m.heatLegendLine(cellW)
	if lineWidth(left)+lineWidth(legend)+1 <= w {
		pad := w - lineWidth(left) - lineWidth(legend)
		left = append(left, statPad(pad))
		left = append(left, legend...)
		return []statLine{left}
	}
	legend = append(statLine{statPad(1)}, legend...)
	return []statLine{left, legend}
}

func (m *Model) heatLegendLine(cellW int) statLine {
	line := statLine{statTxt(copy.T(copy.StatsHeatLess)+" ", m.pal.Muted)}
	for level := 0; level <= 4; level++ {
		if level > 0 {
			line = append(line, statPad(1))
		}
		line = append(line, statBG(strings.Repeat(" ", cellW), m.pal.Text, m.heatColor(level)))
	}
	line = append(line, statTxt(" "+copy.T(copy.StatsHeatMore), m.pal.Muted))
	return line
}

// heatDetailLine is the GitHub tooltip equivalent: the selected day (or today
// when the pointer is outside the grid) with its focus time and sessions.
func (m *Model) heatDetailLine(w int, byDate map[string]protocol.StatsDay, today time.Time) statLine {
	loc := m.statsLocation()
	key := m.statsHeatHover
	if key == "" {
		key = today.Format("2006-01-02")
	}
	day, err := time.ParseInLocation("2006-01-02", key, loc)
	if err != nil {
		return nil
	}
	entry := byDate[key]
	build := func(withYear bool) statLine {
		date := copy.WeekdayShort((int(day.Weekday())+6)%7) + " " +
			fmt.Sprintf("%02d", day.Day()) + " " + copy.MonthShort(day.Month())
		if withYear {
			date += fmt.Sprintf(" %d", day.Year())
		}
		line := statLine{
			statPad(1),
			statTxt("› ", m.pal.Accent),
			statTxt(date, m.pal.Text),
			statTxt(" · ", m.pal.Muted),
		}
		if entry.MS <= 0 {
			return append(line, statTxt(copy.T(copy.StatsNoSessions), m.pal.Muted))
		}
		word := copy.T(copy.StatsSessions)
		if entry.N == 1 {
			word = copy.T(copy.StatsSession)
		}
		return append(line,
			statTxt(statDuration(entry.MS), m.pal.Text),
			statTxt(" · "+fmt.Sprintf("%d %s", entry.N, word), m.pal.Muted),
		)
	}
	line := build(day.Year() != today.Year())
	if lineWidth(line) > w && day.Year() != today.Year() {
		line = build(false)
	}
	if lineWidth(line) > w {
		line = statLine{statTxt(fitText(lineText(line), w), m.pal.Muted)}
	}
	return line
}

// heatDayAt maps a terminal coordinate to the heatmap date under it. The grid
// geometry comes from the last draw, so no hitbox is allocated per day.
func (m *Model) heatDayAt(x, y int) string {
	g := m.statsHeat
	if g.weeks <= 0 || g.cellW <= 0 {
		return ""
	}
	_, _, contentY, contentH := m.statsContentArea()
	if y < contentY || y >= contentY+contentH {
		return ""
	}
	if x < g.x || x >= g.x+g.weeks*g.cellW || y < g.y || y >= g.y+heatDays {
		return ""
	}
	week := (x - g.x) / g.cellW
	day := y - g.y
	loc := m.statsLocation()
	now := m.now().In(loc)
	today := time.Date(now.Year(), now.Month(), now.Day(), 12, 0, 0, 0, loc)
	date := g.firstMon.AddDate(0, 0, week*heatDays+day)
	if date.After(today) {
		return ""
	}
	return date.Format("2006-01-02")
}

func lineText(line statLine) string {
	var b strings.Builder
	for _, cell := range line {
		b.WriteString(cell.text)
	}
	return b.String()
}

func maxValue(vals []int64) int64 {
	var max int64
	for _, v := range vals {
		if v > max {
			max = v
		}
	}
	if max <= 0 {
		max = 1
	}
	return max
}

// hourAxis lays the labels under the 24 bars. barW and gap set the stride so
// the labels stay under their hour at both chart widths.
func hourAxis(barW, gap int) string {
	width := statBarsWidth(24, barW, gap)
	if width <= 0 {
		return ""
	}
	runes := []rune(strings.Repeat(" ", width))
	stride := barW + gap
	place := func(hour int, label string) {
		at := hour * stride
		if at+len([]rune(label)) > width {
			at = width - len([]rune(label))
		}
		if at < 0 {
			at = 0
		}
		for k, r := range []rune(label) {
			if at+k < len(runes) {
				runes[at+k] = r
			}
		}
	}
	place(0, "0")
	place(6, "6")
	place(12, "12")
	place(18, "18")
	place(23, "23")
	return string(runes)
}

func weekdayAxis() string {
	parts := make([]string, 7)
	for i := range parts {
		parts[i] = padRight(copy.WeekdayNarrow(i), 2)
	}
	return strings.Join(parts, " ")
}

func (m *Model) statsInsightLine(w int) statLine {
	rec := m.statsData.Records
	if rec.BusiestWeekday < 0 || rec.BusiestHour < 0 {
		return nil
	}
	full := " " + copy.T(copy.StatsInsight, copy.WeekdayShort(rec.BusiestWeekday), rec.BusiestHour)
	if canvas.Width(full) <= w {
		return statLine{statTxt(full, m.pal.Muted)}
	}
	short := " " + copy.T(copy.StatsBusiest, copy.WeekdayShort(rec.BusiestWeekday), rec.BusiestHour)
	if canvas.Width(short) <= w {
		return statLine{statTxt(short, m.pal.Accent)}
	}
	return statLine{statTxt(fitText(short, w), m.pal.Muted)}
}

func (m *Model) statsPresetsRows(w int) []statLine {
	p := m.statsData
	rows := []statLine{
		m.statsHeader(copy.T(copy.StatsPresetsHead), copy.T(copy.StatsRangeAll), w),
		{},
	}
	if len(p.Labels) == 0 {
		rows = append(rows, statLine{statTxt(" "+copy.T(copy.StatsEmptyTitle), m.pal.Muted)})
		return rows
	}
	max := maxValue(labelMS(p.Labels))
	track := theme.Lerp(m.pal.Panel, m.pal.Border, 0.7)
	showRuns := w >= 46
	showPct := w >= 58
	labelW := 10
	if w >= 60 {
		labelW = 14
	}
	if w >= 80 {
		labelW = 18
	}
	for _, label := range p.Labels {
		line := statLine{
			statPad(1),
			statTxt(padRight(fitText(label.Label, labelW-1), labelW), m.pal.Text),
		}
		right := statLine{statTxt(padLeft(statDuration(label.MS), 7), m.pal.Text)}
		if showRuns {
			right = append(right,
				statTxt(" · ", m.pal.Muted),
				statTxt(padLeft(fmt.Sprintf("%d", label.N), 3), m.pal.Text),
				statTxt(" "+copy.T(copy.StatsRuns), m.pal.Muted),
			)
		}
		if showPct {
			if done := label.Completed + label.Stopped; done > 0 {
				pct := int(math.Round(100 * float64(label.Completed) / float64(done)))
				col := m.pal.Accent
				if pct < 60 {
					col = m.pal.Warn
				}
				right = append(right,
					statTxt(" · ", m.pal.Muted),
					statTxt(fmt.Sprintf("%d%%", pct), col),
					statTxt(" "+copy.T(copy.StatsOfFinished), m.pal.Muted),
				)
			}
		}
		barW := w - 1 - labelW - 1 - lineWidth(right)
		if barW > 30 {
			barW = 30
		}
		if barW < 4 {
			barW = 4
		}
		fill := int(math.Round(float64(label.MS) / float64(max) * float64(barW)))
		if fill < 1 {
			fill = 1
		}
		if fill > barW {
			fill = barW
		}
		line = append(line,
			statTxt(strings.Repeat("█", fill), m.pal.Accent),
			statTxt(strings.Repeat("░", barW-fill), track),
			statPad(1),
		)
		line = append(line, right...)
		rows = append(rows, line)
	}

	rows = append(rows, statLine{})
	footer := copy.T(copy.StatsMostUsed, p.Labels[0].Label)
	if p.Totals.All.Sessions > 0 {
		avg := p.Totals.All.MS / int64(p.Totals.All.Sessions)
		extra := " · " + copy.T(copy.StatsAvgSession, statDuration(avg))
		if canvas.Width(" "+footer+extra) <= w {
			footer += extra
		}
	}
	rows = append(rows, statLine{statTxt(" "+footer, m.pal.Muted)})
	return rows
}

func labelMS(labels []protocol.StatsLabel) []int64 {
	out := make([]int64, len(labels))
	for i := range labels {
		out[i] = labels[i].MS
	}
	return out
}

func (m *Model) statsRecentRows(w int) []statLine {
	p := m.statsData
	rows := []statLine{
		m.statsHeader(copy.T(copy.StatsRecent), fmt.Sprintf("%d %s", p.Retained, copy.T(copy.StatsSessions)), w),
		{},
	}
	// The label column is the first to go at narrow widths; the outcome tells
	// more than the label when there is only room for one. The start column
	// only grows when "yesterday" fits.
	const plannedW = 7
	whenW := 12
	longWhen := false
	if w >= 66 {
		whenW = 15
		longWhen = true
	}
	labelW := 10
	if w >= 70 {
		labelW = 16
	}
	if w >= 86 {
		labelW = 20
	}
	showLabel := w >= 54
	header := statLine{statPad(1)}
	header = append(header,
		statTxt(padRight(copy.T(copy.StatsWhen), whenW), m.pal.Muted),
		statPad(1),
		statTxt(padLeft(copy.T(copy.StatsPlanned), plannedW), m.pal.Muted),
		statPad(1),
	)
	if showLabel {
		header = append(header, statTxt(padRight(copy.T(copy.StatsLabel), labelW), m.pal.Muted), statPad(1))
	}
	header = append(header, statTxt(copy.T(copy.StatsOutcome), m.pal.Muted))
	rows = append(rows, header)
	outcomeW := w - 1 - whenW - 1 - plannedW - 1
	if showLabel {
		outcomeW -= labelW + 1
	}
	appendOutcome := func(line *statLine, text string, col color.RGBA) {
		if outcomeW > 0 {
			text = fitText(text, outcomeW)
		}
		*line = append(*line, statTxt(text, col))
	}
	for _, ev := range p.Recent {
		line := statLine{statPad(1)}
		line = append(line,
			statTxt(padRight(m.statsWhen(ev.StartedMS, longWhen), whenW), m.pal.Text),
			statPad(1),
			statTxt(padLeft(fitText(statDuration(ev.PlannedMS), plannedW), plannedW), m.pal.Text),
			statPad(1),
		)
		if showLabel {
			line = append(line, statTxt(padRight(fitText(ev.Label, labelW-1), labelW), m.pal.Text), statPad(1))
		}
		elapsed := statDuration(ev.ElapsedMS)
		switch ev.Outcome {
		case string(stats.Completed):
			appendOutcome(&line, copy.T(copy.StatsDone), m.pal.Accent)
		case string(stats.Stopped):
			appendOutcome(&line, copy.T(copy.StatsStopped, elapsed), m.pal.Warn)
		default:
			appendOutcome(&line, copy.T(copy.StatsReplaced, elapsed), m.pal.Accent2)
		}
		rows = append(rows, line)
	}
	rows = append(rows, statLine{})
	rows = append(rows, statLine{statTxt(" "+copy.T(copy.StatsShowing, len(p.Recent), p.Retained), m.pal.Muted)})
	return rows
}

func (m *Model) statsWhen(startedMS int64, long bool) string {
	loc := m.statsLocation()
	t := time.UnixMilli(startedMS).In(loc)
	now := m.now().In(loc)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	day := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, loc)
	switch {
	case day.Equal(today):
		return copy.T(copy.StatsToday, t.Format("15:04"))
	case day.Equal(today.AddDate(0, 0, -1)):
		if long {
			return copy.T(copy.StatsYesterday, t.Format("15:04"))
		}
		return copy.T(copy.StatsYesterdayShort, t.Format("15:04"))
	default:
		return fmt.Sprintf("%02d %s %s", t.Day(), copy.MonthShort(t.Month()), t.Format("15:04"))
	}
}

// drawStats paints the whole tab: title, section content, tab bar and hint.
func (m *Model) drawStats(c *canvas.Canvas, base rect) {
	r := m.statsRect()
	m.drawOverlay(c, r, copy.T(copy.StatsTitle))
	x, w, y, h := m.statsContentArea()

	switch {
	case !m.statsLoaded || m.statsData == nil:
		m.statsHeat = heatGrid{}
		m.drawCenteredIn(c, x, y+h/2, w, copy.T(copy.StatsLoading), canvas.Fg(m.pal.Muted))
	case m.statsData.Retained == 0:
		m.statsHeat = heatGrid{}
		m.drawCenteredIn(c, x, y+h/2-1, w, copy.T(copy.StatsEmptyTitle),
			canvas.Style{FG: m.pal.Text, HasFG: true, Bold: true})
		m.drawCenteredIn(c, x, y+h/2, w, copy.T(copy.StatsEmptyBody), canvas.Fg(m.pal.Muted))
	default:
		rows := m.statsRows(w, h)
		scroll := m.statsScroll
		if max := len(rows) - h; scroll > max {
			scroll = max
		}
		if scroll < 0 {
			scroll = 0
		}
		for i := 0; i < h && scroll+i < len(rows); i++ {
			m.drawStatLine(c, x, y+i, rows[scroll+i], w)
		}
		if len(rows) > h {
			m.drawStatsBar(c, r, y, h, scroll, len(rows))
		}
		if m.statsTab == statsTabActivity && m.statsHeat.weeks > 0 {
			m.statsHeat.x = x + 3
			m.statsHeat.y = y + m.statsHeat.row - scroll
		}
	}

	m.drawStatsTabs(c, x, r.y+r.h-3, w)
	hint := copy.T(copy.StatsHint)
	narrow := copy.StatsHintShort
	if m.statsTab != statsTabOverview {
		hint = copy.T(copy.StatsHintSection)
		narrow = copy.StatsHintNarrow
		if m.statsTab == statsTabActivity {
			hint = copy.T(copy.StatsHintActivity)
		}
	}
	if canvas.Width(hint) > w {
		hint = copy.T(narrow)
	}
	hintCol := m.pal.Muted
	if !m.connected {
		hint = copy.T(copy.Disconnected)
		hintCol = m.pal.Danger
	}
	m.drawCenteredIn(c, x, r.y+r.h-2, w, fitText(hint, w), canvas.Fg(hintCol))
}

func (m *Model) drawStatLine(c *canvas.Canvas, x, y int, line statLine, w int) {
	col := x
	for _, cell := range line {
		if col >= x+w {
			return
		}
		st := canvas.Style{
			FG:    cell.fg,
			BG:    cell.bg,
			HasFG: cell.hasFG,
			HasBG: cell.hasBG,
			Bold:  cell.bold,
		}
		for _, r := range cell.text {
			if col >= x+w {
				return
			}
			if !canvas.SingleCell(r) {
				r = '?'
			}
			c.Set(col, y, r, st)
			col++
		}
	}
}

func (m *Model) drawStatsBar(c *canvas.Canvas, r rect, y, h, scroll, total int) {
	if h <= 0 || total <= h {
		return
	}
	barX := r.x + r.w - 1
	track := canvas.Fg(theme.Lerp(m.pal.Panel, m.pal.Border, 0.9))
	for row := 0; row < h; row++ {
		c.Set(barX, y+row, '│', track)
	}
	thumb := h * h / total
	if thumb < 1 {
		thumb = 1
	}
	if thumb > h {
		thumb = h
	}
	pos := 0
	if max := total - h; max > 0 {
		pos = scroll * (h - thumb) / max
	}
	for i := 0; i < thumb; i++ {
		c.Set(barX, y+pos+i, '█', canvas.Fg(m.pal.Accent))
	}
}

func (m *Model) drawStatsTabs(c *canvas.Canvas, x, y, w int) {
	names := make([]string, statsTabCount)
	for i := range names {
		names[i] = copy.T(statsTabKey(i))
	}
	measure := func(sep string, marker bool) int {
		total := 0
		for i, name := range names {
			if i > 0 {
				total += canvas.Width(sep)
			}
			total += canvas.Width(name)
			if marker && i == m.statsTab {
				total += 2
			}
		}
		return total
	}
	sep := " · "
	marker := true
	total := measure(sep, marker)
	if total > w {
		sep = " "
		total = measure(sep, marker)
	}
	if total > w {
		marker = false
		total = measure(sep, marker)
	}
	cx := x + (w-total)/2
	if cx < x {
		cx = x
	}
	for i, name := range names {
		if i > 0 {
			c.Text(cx, y, sep, canvas.Fg(m.pal.Muted))
			cx += canvas.Width(sep)
		}
		if marker && i == m.statsTab {
			c.Text(cx, y, "› ", canvas.Fg(m.pal.Accent))
			cx += 2
		}
		st := canvas.Fg(theme.Lerp(m.pal.BG, m.pal.Muted, 0.7))
		if i == m.statsTab {
			st = canvas.Style{FG: m.pal.Accent, HasFG: true, Bold: true}
		}
		c.Text(cx, y, name, st)
		m.addHit(cx, y, canvas.Width(name), 1, hitStatsTab, i)
		cx += canvas.Width(name)
	}
}
