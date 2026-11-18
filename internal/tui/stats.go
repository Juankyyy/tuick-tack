package tui

import (
	"fmt"
	"image/color"
	"math"
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
	case text == "t":
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
	w := minInt(96, baseW-6)
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
		return m.statsActivityRows(w)
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
	rows := []statLine{m.statsKPILine(w), {}}

	days := 7
	barW := 2
	if m.statsRange != statsRange7 {
		days = 30
		barW = 1
	}
	cols := days
	if fit := (w + 1) / (barW + 1); cols > fit {
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

	note := copy.T(copy.StatsMax, statDuration(max)) + " · " + m.statsRangeLabel()
	rows = append(rows, m.statsHeader(copy.T(copy.StatsFocusPerDay), note, w))
	chartH := 6
	switch {
	case h >= 19:
		chartH = 10
	case h >= 15:
		chartH = 8
	}
	rows = append(rows, statChart(m, vals, max, chartH, barW, m.pal.Accent, m.pal.Accent2)...)
	rows = append(rows, m.statsDayLabels(window, barW))
	rows = append(rows, statLine{})
	if rec := m.statsRecordsLine(w); len(rec) > 0 {
		rows = append(rows, rec)
	}
	return rows
}

func (m *Model) statsKPILine(w int) statLine {
	tr := m.statsRangeTotals()
	line := statLine{statPad(1)}
	line = append(line,
		statBold(statDuration(tr.MS), m.pal.Text),
		statTxt(" "+copy.T(copy.StatsFocused), m.pal.Muted),
		statTxt(" · ", m.pal.Muted),
		statBold(fmt.Sprintf("%d", tr.Sessions), m.pal.Text),
		statTxt(" "+copy.T(copy.StatsSessions), m.pal.Muted),
	)
	if done := tr.Completed + tr.Stopped; done > 0 {
		pct := int(math.Round(100 * float64(tr.Completed) / float64(done)))
		candidate := statLine{
			statTxt(" · ", m.pal.Muted),
			statBold(fmt.Sprintf("%d%%", pct), m.pal.Text),
			statTxt(" "+copy.T(copy.StatsCompleted), m.pal.Muted),
		}
		if lineWidth(line)+lineWidth(candidate) <= w {
			line = append(line, candidate...)
		}
	}
	if m.statsRange != statsRangeAll {
		days := m.statsRangeDays()
		if prev := m.statsPrevMS(days); prev > 0 {
			delta := int(math.Round(100 * float64(tr.MS-prev) / float64(prev)))
			col := m.pal.Accent
			if delta < 0 {
				col = m.pal.Warn
			}
			candidate := statLine{
				statTxt(" · ", m.pal.Muted),
				statBold(fmt.Sprintf("%+d%%", delta), col),
				statTxt(" "+copy.T(copy.StatsVsPrev, m.statsRangeLabel()), m.pal.Muted),
			}
			if lineWidth(line)+lineWidth(candidate) <= w {
				line = append(line, candidate...)
			}
		}
	}
	return line
}

// statChart draws vertical bars with half-block resolution. Each bar is barW
// cells wide and the newest column uses the quieter accent as "today".
func statChart(m *Model, vals []int64, max int64, height, barW int, accent, outer color.RGBA) []statLine {
	rows := make([]statLine, 0, height)
	last := len(vals) - 1
	for r := 0; r < height; r++ {
		line := statLine{}
		for i, v := range vals {
			if i > 0 {
				line = append(line, statCell{text: " "})
			}
			halves := 0
			if max > 0 {
				halves = int(math.Round(float64(v) / float64(max) * float64(height*2)))
			}
			need := 2 * (height - r)
			text := strings.Repeat(" ", barW)
			col := accent
			switch {
			case halves >= need:
				text = strings.Repeat("█", barW)
			case halves == need-1:
				text = strings.Repeat("▄", barW)
			case r == height-1 && v == 0:
				text = "·" + strings.Repeat(" ", maxInt(0, barW-1))
				col = theme.Lerp(m.pal.Panel, m.pal.Muted, 0.5)
			}
			if i == last {
				col = outer
			}
			line = append(line, statTxt(text, col))
		}
		rows = append(rows, line)
	}
	return rows
}

func (m *Model) statsDayLabels(window []protocol.StatsDay, barW int) statLine {
	line := statLine{}
	slotStep := 1
	if barW == 1 {
		slotStep = 7
	}
	loc := m.statsLocation()
	for i, day := range window {
		if i > 0 {
			line = append(line, statCell{text: " "})
		}
		label := ""
		if t, err := time.ParseInLocation("2006-01-02", day.Date, loc); err == nil {
			if barW == 2 {
				short := copy.WeekdayShort((int(t.Weekday()) + 6) % 7)
				if canvas.Width(short) >= 2 {
					label = short[:2]
				}
			} else if i%slotStep == 0 {
				label = fmt.Sprintf("%02d", t.Day())
			}
		}
		if label == "" {
			line = append(line, statPad(barW))
		} else {
			line = append(line, statTxt(label, m.pal.Muted))
		}
	}
	return line
}

func (m *Model) statsRecordsLine(w int) statLine {
	rec := m.statsData.Records
	line := statLine{statPad(1)}
	added := 0
	appendPart := func(part statLine) {
		candidate := append(statLine{}, line...)
		if added > 0 {
			candidate = append(candidate, statTxt(" · ", m.pal.Muted))
		}
		candidate = append(candidate, part...)
		if lineWidth(candidate) > w {
			return
		}
		line = candidate
		added++
	}
	if rec.StreakDays > 0 {
		appendPart(statLine{statBold(copy.T(copy.StatsStreak, rec.StreakDays), m.pal.Accent)})
	}
	appendPart(statLine{statTxt(copy.T(copy.StatsBestStreak, rec.BestStreakDays), m.pal.Muted)})
	appendPart(statLine{statTxt(copy.T(copy.StatsLongest, statDuration(rec.LongestMS)), m.pal.Muted)})
	if rec.BusiestWeekday >= 0 && rec.BusiestHour >= 0 {
		appendPart(statLine{statTxt(copy.T(copy.StatsBusiest,
			copy.WeekdayShort(rec.BusiestWeekday), rec.BusiestHour), m.pal.Muted)})
	}
	if added == 0 {
		return nil
	}
	return line
}

func (m *Model) statsActivityRows(w int) []statLine {
	p := m.statsData
	loc := m.statsLocation()
	rows := []statLine{m.statsHeader(copy.T(copy.StatsActivity), copy.T(copy.StatsWeeks, 26), w)}

	byDate := make(map[string]int64, len(p.Daily))
	for _, day := range p.Daily {
		byDate[day.Date] = day.MS
	}
	now := m.now().In(loc)
	today := time.Date(now.Year(), now.Month(), now.Day(), 12, 0, 0, 0, loc)
	weeks := 26
	if fit := w - 3; weeks > fit {
		weeks = fit
	}
	if weeks < 4 {
		weeks = 4
	}
	thisMonday := today.AddDate(0, 0, -((int(today.Weekday()) + 6) % 7))
	firstMonday := thisMonday.AddDate(0, 0, -7*(weeks-1))

	var maxMS int64
	for i := 0; i < weeks; i++ {
		monday := firstMonday.AddDate(0, 0, 7*i)
		for d := 0; d < 7; d++ {
			day := monday.AddDate(0, 0, d)
			if day.After(today) {
				continue
			}
			if v := byDate[day.Format("2006-01-02")]; v > maxMS {
				maxMS = v
			}
		}
	}

	months := make([]rune, weeks)
	for i := range months {
		months[i] = ' '
	}
	placeMonth := func(at int, label string) {
		for k, r := range label {
			if at+k < weeks {
				months[at+k] = r
			}
		}
	}
	lastMonth := ""
	lastCol := -10
	for i := 0; i < weeks; i++ {
		label := firstMonday.AddDate(0, 0, 7*i).Format("Jan")
		if label == lastMonth {
			continue
		}
		switch {
		case i+3 <= weeks && i >= lastCol+4:
			placeMonth(i, label)
			lastMonth = label
			lastCol = i
		case i+3 > weeks:
			// The last month label is right-aligned into the grid when it fits
			// without touching the previous one.
			start := weeks - 3
			if start >= 0 && start >= lastCol+4 {
				placeMonth(start, label)
				lastMonth = label
				lastCol = start
			}
		}
	}
	rows = append(rows, statLine{statPad(3), statTxt(string(months), m.pal.Muted)})

	dayLabels := [7]string{"M", "", "W", "", "F", "", ""}
	for d := 0; d < 7; d++ {
		line := statLine{statTxt(" "+dayLabels[d]+" ", m.pal.Muted)}
		for i := 0; i < weeks; i++ {
			day := firstMonday.AddDate(0, 0, 7*i+d)
			level := 0
			if !day.After(today) && maxMS > 0 {
				if v := byDate[day.Format("2006-01-02")]; v > 0 {
					ratio := float64(v) / float64(maxMS)
					level = 1 + int(math.Min(3, math.Floor(ratio*3.999)))
				}
			}
			bg := theme.Lerp(m.pal.Panel, m.pal.BG, 0.55)
			if day.After(today) {
				bg = m.pal.BG
			} else if level > 0 {
				bg = theme.Lerp(m.pal.Panel, m.pal.Accent, 0.18+0.2*float64(level))
			}
			line = append(line, statBG(" ", m.pal.Text, bg))
		}
		rows = append(rows, line)
	}

	legend := statLine{statPad(1), statTxt(copy.T(copy.StatsHeatLess)+" ", m.pal.Muted)}
	for level := 0; level <= 4; level++ {
		bg := theme.Lerp(m.pal.Panel, m.pal.BG, 0.55)
		if level > 0 {
			bg = theme.Lerp(m.pal.Panel, m.pal.Accent, 0.18+0.2*float64(level))
		}
		legend = append(legend, statBG(" ", m.pal.Text, bg))
	}
	legend = append(legend, statTxt(" "+copy.T(copy.StatsHeatMore), m.pal.Muted))
	rows = append(rows, legend, statLine{})

	hourVals := make([]int64, len(p.Hourly))
	for i := range p.Hourly {
		hourVals[i] = p.Hourly[i].MS
	}
	weekdayVals := make([]int64, len(p.Weekday))
	for i := range p.Weekday {
		weekdayVals[i] = p.Weekday[i].MS
	}
	hourMax := maxValue(hourVals)
	weekdayMax := maxValue(weekdayVals)

	if w >= 58 {
		header := statLine{statTxt(" "+copy.T(copy.StatsByHour), m.pal.Text)}
		if pad := 26 - lineWidth(header); pad > 0 {
			header = append(header, statPad(pad))
		}
		header = append(header, statTxt(copy.T(copy.StatsByWeekday), m.pal.Text))
		rows = append(rows, header)
		hourRows := statChart(m, hourVals, hourMax, 3, 1, m.pal.Accent, m.pal.Accent2)
		weekdayRows := statChart(m, weekdayVals, weekdayMax, 3, 2, m.pal.Accent, m.pal.Accent2)
		for i := 0; i < 3; i++ {
			line := append(statLine{}, hourRows[i]...)
			line = append(line, statPad(2))
			line = append(line, weekdayRows[i]...)
			rows = append(rows, line)
		}
		axis := statLine{statTxt(hourAxis(), m.pal.Muted)}
		axis = append(axis, statPad(2), statTxt(weekdayAxis(), m.pal.Muted))
		rows = append(rows, axis)
	} else {
		rows = append(rows, statLine{statTxt(" "+copy.T(copy.StatsByHour), m.pal.Text)})
		rows = append(rows, statChart(m, hourVals, hourMax, 3, 1, m.pal.Accent, m.pal.Accent2)...)
		rows = append(rows, statLine{statTxt(hourAxis(), m.pal.Muted)})
		rows = append(rows, statLine{statTxt(" "+copy.T(copy.StatsByWeekday), m.pal.Text)})
		rows = append(rows, statChart(m, weekdayVals, weekdayMax, 3, 2, m.pal.Accent, m.pal.Accent2)...)
		rows = append(rows, statLine{statTxt(weekdayAxis(), m.pal.Muted)})
	}

	if insight := m.statsInsightLine(w); insight != nil {
		rows = append(rows, statLine{}, insight)
	}
	return rows
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

func hourAxis() string {
	runes := []rune(strings.Repeat(" ", 24))
	runes[0] = '0'
	runes[6] = '6'
	runes[12] = '1'
	runes[13] = '2'
	runes[18] = '1'
	runes[19] = '8'
	return string(runes)
}

func weekdayAxis() string { return "Mo Tu We Th Fr Sa Su" }

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
	showRuns := w >= 52
	showPct := w >= 60
	const labelW = 10
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
				col := m.pal.Text
				if pct < 60 {
					col = m.pal.Warn
				}
				right = append(right, statTxt(" · ", m.pal.Muted), statTxt(fmt.Sprintf("%d%%", pct), col))
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
	// more than the label when there is only room for one.
	const whenW, plannedW, labelW = 12, 7, 10
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
	for _, ev := range p.Recent {
		line := statLine{statPad(1)}
		line = append(line,
			statTxt(padRight(m.statsWhen(ev.StartedMS), whenW), m.pal.Text),
			statPad(1),
			statTxt(padLeft(statDuration(ev.PlannedMS), plannedW), m.pal.Text),
			statPad(1),
		)
		if showLabel {
			line = append(line, statTxt(padRight(fitText(ev.Label, labelW-1), labelW), m.pal.Text), statPad(1))
		}
		elapsed := statDuration(ev.ElapsedMS)
		switch ev.Outcome {
		case string(stats.Completed):
			line = append(line, statTxt(copy.T(copy.StatsDone), m.pal.Accent))
		case string(stats.Stopped):
			line = append(line, statTxt(copy.T(copy.StatsStopped, elapsed), m.pal.Warn))
		default:
			line = append(line, statTxt(copy.T(copy.StatsReplaced, elapsed), m.pal.Accent2))
		}
		rows = append(rows, line)
	}
	rows = append(rows, statLine{})
	rows = append(rows, statLine{statTxt(" "+copy.T(copy.StatsShowing, len(p.Recent), p.Retained), m.pal.Muted)})
	return rows
}

func (m *Model) statsWhen(startedMS int64) string {
	loc := m.statsLocation()
	t := time.UnixMilli(startedMS).In(loc)
	now := m.now().In(loc)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	day := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, loc)
	switch {
	case day.Equal(today):
		return copy.T(copy.StatsToday, t.Format("15:04"))
	case day.Equal(today.AddDate(0, 0, -1)):
		return copy.T(copy.StatsYesterday, t.Format("15:04"))
	default:
		return t.Format("02 Jan 15:04")
	}
}

// drawStats paints the whole tab: title, section content, tab bar and hint.
func (m *Model) drawStats(c *canvas.Canvas, base rect) {
	r := m.statsRect()
	m.drawOverlay(c, r, copy.T(copy.StatsTitle))
	x, w, y, h := m.statsContentArea()

	switch {
	case !m.statsLoaded || m.statsData == nil:
		m.drawCenteredIn(c, x, y+h/2, w, copy.T(copy.StatsLoading), canvas.Fg(m.pal.Muted))
	case m.statsData.Retained == 0:
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
	}

	m.drawStatsTabs(c, x, r.y+r.h-3, w)
	hint := copy.T(copy.StatsHint)
	if canvas.Width(hint) > w {
		hint = copy.T(copy.StatsHintShort)
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
