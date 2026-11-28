package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"tuick-tack/internal/config"
	"tuick-tack/internal/engine"
	"tuick-tack/internal/protocol"
	"tuick-tack/internal/stats"
	"tuick-tack/internal/tui/canvas"
	"tuick-tack/internal/tui/copy"
	"tuick-tack/internal/tui/theme"
)

// testStatsPayload builds a small, deterministic summary for the fixed test
// clock (2026-11-11 14:00 local).
func testStatsPayload(now time.Time) protocol.StatsPayload {
	loc := now.Location()
	day := func(daysAgo int) string {
		return time.Date(now.Year(), now.Month(), now.Day(), 12, 0, 0, 0, loc).
			AddDate(0, 0, -daysAgo).Format("2006-01-02")
	}
	daily := make([]protocol.StatsDay, stats.DailyWindow)
	for i := range daily {
		daily[i].Date = day(stats.DailyWindow - 1 - i)
	}
	daily[len(daily)-1].MS = 35 * 60000
	daily[len(daily)-1].N = 2
	daily[len(daily)-2].MS = 50 * 60000
	daily[len(daily)-2].N = 1
	daily[len(daily)-3].MS = 5 * 60000
	daily[len(daily)-3].N = 1
	daily[len(daily)-7].MS = 60 * 60000
	daily[len(daily)-7].N = 1

	hourly := make([]protocol.StatsBucket, 24)
	hourly[9] = protocol.StatsBucket{MS: 25 * 60000, N: 1}
	hourly[14] = protocol.StatsBucket{MS: 95 * 60000, N: 3}
	weekday := make([]protocol.StatsBucket, 7)
	weekday[1] = protocol.StatsBucket{MS: 95 * 60000, N: 3}
	weekday[2] = protocol.StatsBucket{MS: 35 * 60000, N: 2}

	return protocol.StatsPayload{
		Daily:   daily,
		Hourly:  hourly,
		Weekday: weekday,
		Labels: []protocol.StatsLabel{
			{Label: "5 min", MS: 125 * 60000, N: 14, Completed: 13, Stopped: 1},
			{Label: "Focus", MS: 50 * 60000, N: 4, Completed: 3, Stopped: 1},
			{Label: "10 min", MS: 38 * 60000, N: 5, Completed: 4},
		},
		Records: protocol.StatsRecords{
			StreakDays: 4, BestStreakDays: 12, LongestMS: 50 * 60000,
			LongestLabel: "Focus", BusiestWeekday: 1, BusiestHour: 14,
		},
		Recent: []protocol.StatsEvent{
			{StartedMS: now.Add(-8 * time.Hour).UnixMilli(), PlannedMS: 25 * 60000, ElapsedMS: 25 * 60000, Label: "5 min", Outcome: string(stats.Completed)},
			{StartedMS: now.Add(-26 * time.Hour).UnixMilli(), PlannedMS: 25 * 60000, ElapsedMS: 10 * 60000, Label: "5 min", Outcome: string(stats.Stopped)},
			{StartedMS: now.Add(-50 * time.Hour).UnixMilli(), PlannedMS: 50 * 60000, ElapsedMS: 8 * 60000, Label: "Focus", Outcome: string(stats.Replaced)},
		},
		Totals: protocol.StatsTotals{
			All: protocol.StatsRange{Sessions: 23, Completed: 20, Stopped: 2, Replaced: 1, MS: 213 * 60000},
			D7:  protocol.StatsRange{Sessions: 9, Completed: 8, Stopped: 1, MS: 90 * 60000},
			D30: protocol.StatsRange{Sessions: 15, Completed: 13, Stopped: 2, MS: 150 * 60000},
			D90: protocol.StatsRange{Sessions: 23, Completed: 20, Stopped: 2, Replaced: 1, MS: 213 * 60000},
		},
		Retained: 23,
	}
}

func openTestStats(t *testing.T, w, h int) (*Model, *testClock, protocol.StatsPayload) {
	t.Helper()
	m, clock := testModel(w, h)
	m.openStats()
	if m.modal != modalStats {
		t.Fatalf("modal = %v, want stats", m.modal)
	}
	payload := testStatsPayload(clock.Now())
	m.handleServerEvent(serverEventMsg{msg: protocol.NewStats(payload)})
	if m.statsData == nil || !m.statsLoaded {
		t.Fatal("stats reply was not stored")
	}
	return m, clock, payload
}

func TestStatsOpensWithSAndMutesWithM(t *testing.T) {
	m, _ := testModel(80, 24)
	press(m, 's', "s")
	if m.modal != modalStats {
		t.Fatalf("s in idle opened %v, want the statistics tab", m.modal)
	}
	press(m, tea.KeyEscape, "")
	if m.modal != modalNone {
		t.Fatal("escape should close the statistics tab")
	}

	// While a duration is being typed, s and m are still unit suffixes.
	press(m, '9', "9")
	press(m, '0', "0")
	press(m, 's', "s")
	if m.input != "90s" || m.modal != modalNone {
		t.Fatalf("typing 90s opened %v with input %q", m.modal, m.input)
	}
	press(m, tea.KeyEscape, "")
	press(m, '2', "2")
	press(m, '5', "5")
	press(m, 'm', "m")
	if m.input != "25m" || m.modal != modalNone {
		t.Fatalf("typing 25m opened %v with input %q", m.modal, m.input)
	}
	press(m, tea.KeyEscape, "")

	// m alone toggles mute; uppercase S no longer does.
	before := m.cfg.Sound
	press(m, 'm', "m")
	if m.cfg.Sound == before {
		t.Fatal("m did not toggle mute in idle")
	}
	muted := m.cfg.Sound
	press(m, 'S', "S")
	if m.cfg.Sound != muted {
		t.Fatal("uppercase S should not toggle mute anymore")
	}

	applyRunning(m)
	press(m, 's', "s")
	if m.modal != modalStats {
		t.Fatalf("s while running opened %v, want stats", m.modal)
	}
	press(m, tea.KeyEscape, "")
	press(m, 'm', "m")
	if m.cfg.Sound == muted {
		t.Fatal("m while running did not toggle mute")
	}

	m.applyState(protocol.StateMessage{Type: protocol.TypeState, Status: string(engine.Finished), DurationMS: 60000})
	press(m, 's', "s")
	if m.modal != modalStats {
		t.Fatalf("s while finished opened %v, want stats", m.modal)
	}
}

func TestStatsTabKeys(t *testing.T) {
	m, _, _ := openTestStats(t, 48, 22)
	if m.statsTab != statsTabOverview || m.statsRange != statsRange7 {
		t.Fatalf("initial tab/range = %d/%d", m.statsTab, m.statsRange)
	}

	press(m, tea.KeyRight, "")
	if m.statsTab != statsTabActivity {
		t.Fatalf("right moved to tab %d", m.statsTab)
	}
	press(m, tea.KeyLeft, "")
	if m.statsTab != statsTabOverview {
		t.Fatalf("left moved to tab %d", m.statsTab)
	}
	press(m, '3', "3")
	if m.statsTab != statsTabPresets {
		t.Fatalf("3 moved to tab %d", m.statsTab)
	}
	press(m, tea.KeyTab, "")
	if m.statsTab != statsTabRecent {
		t.Fatalf("tab moved to %d", m.statsTab)
	}

	// The range only belongs to the overview; changing it on another section
	// would alter a value that is not on screen.
	press(m, 't', "t")
	if m.statsRange != statsRange7 {
		t.Fatalf("t on recent changed the range to %d", m.statsRange)
	}
	press(m, '1', "1")
	press(m, 't', "t")
	if m.statsRange != statsRange30 {
		t.Fatalf("range = %d, want 30 days", m.statsRange)
	}
	for i := 0; i < statsRangeCount; i++ {
		press(m, 't', "t")
	}
	if m.statsRange != statsRange30 {
		t.Fatalf("range wrapped wrong: %d", m.statsRange)
	}

	m.setStatsTab(statsTabActivity)
	press(m, tea.KeyPgDown, "")
	if m.statsScroll <= 0 {
		t.Fatal("PgDn did not scroll the activity section")
	}
	press(m, tea.KeyHome, "")
	if m.statsScroll != 0 {
		t.Fatal("Home did not return to the top")
	}
	press(m, tea.KeyEnd, "")
	if m.statsScroll != m.statsMaxScroll() {
		t.Fatalf("End scroll = %d, want %d", m.statsScroll, m.statsMaxScroll())
	}

	press(m, 's', "s")
	if m.modal != modalNone {
		t.Fatal("s should close the statistics tab")
	}
}

func TestStatsSectionsRender(t *testing.T) {
	for _, size := range [][2]int{{48, 22}, {80, 24}, {120, 40}} {
		w, h := size[0], size[1]
		m, _, _ := openTestStats(t, w, h)
		for tab := 0; tab < statsTabCount; tab++ {
			m.setStatsTab(tab)
			plain := assertFrame(t, m, w, h)
			if !strings.Contains(plain, copy.T(copy.StatsTitle)) {
				t.Fatalf("%dx%d tab %d missing the title:\n%s", w, h, tab, plain)
			}
			if !strings.Contains(plain, copy.T(copy.StatsHint)) &&
				!strings.Contains(plain, copy.T(copy.StatsHintSection)) &&
				!strings.Contains(plain, copy.T(copy.StatsHintActivity)) &&
				!strings.Contains(plain, copy.T(copy.StatsHintNarrow)) &&
				!strings.Contains(plain, copy.T(copy.StatsHintShort)) {
				t.Fatalf("%dx%d tab %d missing the hint:\n%s", w, h, tab, plain)
			}
			if tab == statsTabOverview {
				for _, want := range []string{copy.T(copy.StatsFocusPerDay), "focused", copy.T(copy.StatsStreak, 4)} {
					if !strings.Contains(plain, want) {
						t.Fatalf("%dx%d overview missing %q:\n%s", w, h, want, plain)
					}
				}
			}
			if tab == statsTabActivity && !strings.Contains(plain, copy.T(copy.StatsByHour)) {
				t.Fatalf("%dx%d activity missing the hour chart:\n%s", w, h, plain)
			}
			if tab == statsTabPresets && !strings.Contains(plain, copy.T(copy.StatsPresetsHead)) {
				t.Fatalf("%dx%d presets missing its header:\n%s", w, h, plain)
			}
			if tab == statsTabRecent {
				for _, want := range []string{copy.T(copy.StatsShowing, 3, 23), copy.T(copy.StatsStopped, "10m")} {
					if !strings.Contains(plain, want) {
						t.Fatalf("%dx%d recent missing %q:\n%s", w, h, want, plain)
					}
				}
			}
		}
		// The narrow hint falls back to the short variant.
		m.setStatsTab(statsTabOverview)
		plain := assertFrame(t, m, w, h)
		if w < 64 && !strings.Contains(plain, copy.T(copy.StatsHintShort)) {
			t.Fatalf("%dx%d should use the short hint:\n%s", w, h, plain)
		}
		if w >= 64 && !strings.Contains(plain, copy.T(copy.StatsHint)) {
			t.Fatalf("%dx%d should use the full hint:\n%s", w, h, plain)
		}
	}
}

func TestStatsEmptyState(t *testing.T) {
	m, clock := testModel(80, 24)
	m.openStats()
	payload := protocol.StatsPayload{
		Daily:   make([]protocol.StatsDay, stats.DailyWindow),
		Hourly:  make([]protocol.StatsBucket, 24),
		Weekday: make([]protocol.StatsBucket, 7),
	}
	for i := range payload.Daily {
		payload.Daily[i].Date = time.Date(clock.Now().Year(), clock.Now().Month(), clock.Now().Day(), 12, 0, 0, 0, time.Local).
			AddDate(0, 0, -(stats.DailyWindow - 1 - i)).Format("2006-01-02")
	}
	m.handleServerEvent(serverEventMsg{msg: protocol.NewStats(payload)})
	plain := assertFrame(t, m, 80, 24)
	for _, want := range []string{copy.T(copy.StatsEmptyTitle), copy.T(copy.StatsEmptyBody)} {
		if !strings.Contains(plain, want) {
			t.Fatalf("empty state missing %q:\n%s", want, plain)
		}
	}
}

func TestStatsRefreshesOnStop(t *testing.T) {
	m, _, _ := openTestStats(t, 80, 24)
	m.applyState(protocol.StateMessage{Type: protocol.TypeState, Status: string(engine.Running), DurationMS: 60000, RemainingMS: 30000})
	if m.modal != modalStats {
		t.Fatal("a running snapshot closed the statistics tab")
	}

	// A finish closes the tab so the celebration is visible.
	m.handleServerEvent(serverEventMsg{msg: protocol.StateMessage{
		Type: protocol.TypeState, Status: string(engine.Finished),
		DurationMS: 60000, FinishSeq: 1,
	}})
	if m.modal != modalNone {
		t.Fatal("finishing should close the statistics tab for the celebration")
	}

	// A stop leaves the tab open and refreshes its data.
	m.openStats()
	m.applyState(protocol.StateMessage{Type: protocol.TypeState, Status: string(engine.Running), DurationMS: 60000, RemainingMS: 30000})
	m.statsLoading = false
	m.handleServerEvent(serverEventMsg{msg: protocol.StateMessage{
		Type: protocol.TypeState, Status: string(engine.Idle),
	}})
	if !m.statsLoading {
		t.Fatal("stopping with the tab open did not request fresh statistics")
	}

	// A preset started from the tray replaces the running countdown; that also
	// adds a record while the tab stays open.
	m.applyState(protocol.StateMessage{Type: protocol.TypeState, Status: string(engine.Running), DurationMS: 60000, RemainingMS: 30000})
	m.statsLoading = false
	m.handleServerEvent(serverEventMsg{msg: protocol.StateMessage{
		Type: protocol.TypeState, Status: string(engine.Running),
		DurationMS: 600000, RemainingMS: 600000, Label: "10 min",
	}})
	if !m.statsLoading {
		t.Fatal("a replaced countdown did not request fresh statistics")
	}
}

func TestStatsUnavailableClosesTab(t *testing.T) {
	m, _, _ := openTestStats(t, 80, 24)
	m.handleServerEvent(serverEventMsg{msg: protocol.NewError(protocol.CodeInvalidCommand, "unknown command")})
	if m.modal != modalNone {
		t.Fatal("an old daemon should close the statistics tab")
	}
	if m.toast == "" {
		t.Fatal("the tab should explain that statistics are unavailable")
	}
}

func TestStatsMouseTabs(t *testing.T) {
	m, _, _ := openTestStats(t, 80, 24)
	render(m) // fills the hitboxes
	var tabHit *hitbox
	for i := range m.hits {
		if m.hits[i].action == hitStatsTab && m.hits[i].index == statsTabActivity {
			tabHit = &m.hits[i]
			break
		}
	}
	if tabHit == nil {
		t.Fatal("no hitbox for the activity tab")
	}
	m.handleMouse(tea.MouseClickMsg{Button: tea.MouseLeft, X: tabHit.x + 1, Y: tabHit.y})
	if m.statsTab != statsTabActivity {
		t.Fatalf("clicking the activity tab selected %d", m.statsTab)
	}
	m.handleMouseWheel(tea.MouseWheelMsg{Button: tea.MouseWheelDown})
}

func TestStatsCompactIgnoresS(t *testing.T) {
	m, _ := testModel(40, 12)
	press(m, 's', "s")
	if m.modal != modalNone {
		t.Fatal("compact mode should not open an undrawable tab")
	}
	if m.input != "" {
		t.Fatalf("compact 's' leaked into the input: %q", m.input)
	}
}

func TestStatsUsesOnlySingleCellRunes(t *testing.T) {
	m, _, _ := openTestStats(t, 80, 24)
	for tab := 0; tab < statsTabCount; tab++ {
		m.setStatsTab(tab)
		_, w, _, h := m.statsContentArea()
		for _, line := range m.statsRows(w, h) {
			for _, cell := range line {
				for _, r := range cell.text {
					if !canvas.SingleCell(r) {
						t.Fatalf("tab %d contains non-single-cell rune %q", tab, r)
					}
				}
			}
		}
	}
}

func TestStatsHeatmapUsesBackgroundCells(t *testing.T) {
	m, _, _ := openTestStats(t, 80, 24)
	m.setStatsTab(statsTabActivity)
	c := canvas.New(80, 24)
	m.drawStats(c, rect{x: 0, y: 0, w: 80, h: 24})
	x, w, y, h := m.statsContentArea()
	tinted := 0
	for row := 0; row < h; row++ {
		for col := x; col < x+w; col++ {
			cell := c.At(col, y+row)
			if cell.HasBG && cell.BG != m.pal.Panel && cell.BG != m.pal.BG {
				tinted++
			}
		}
	}
	if tinted == 0 {
		t.Fatal("the activity heatmap painted no tinted background cells")
	}
}

func TestStatsRefreshKeepsTimerState(t *testing.T) {
	m, _ := testModel(80, 24)
	m.applyConfig(protocol.ConfigMessage{ConfigPayload: protocol.NewConfigPayload(config.Default())})
	applyRunning(m)
	m.openStats()
	if m.status() != engine.Running {
		t.Fatal("opening the statistics tab changed the timer")
	}
	if m.HasActiveTimer() != true {
		t.Fatal("opening the statistics tab dropped the active timer")
	}
}

func TestStatsRangesRender(t *testing.T) {
	m, _, _ := openTestStats(t, 80, 24)
	for r := 0; r < statsRangeCount; r++ {
		m.statsRange = r
		plain := assertFrame(t, m, 80, 24)
		if !strings.Contains(plain, m.statsRangeLabel()) {
			t.Fatalf("range %d missing its label %q:\n%s", r, m.statsRangeLabel(), plain)
		}
		// The bars are always the recent daily window; the note must say how
		// many days it actually paints, not the selected range.
		shown := 7
		if r != statsRange7 {
			shown = 30
		}
		if !strings.Contains(plain, copy.T(copy.StatsLastDays, shown)) {
			t.Fatalf("range %d chart note does not say %d days:\n%s", r, shown, plain)
		}
	}
}

func statLineText(line statLine) string {
	var b strings.Builder
	for _, cell := range line {
		b.WriteString(cell.text)
	}
	return b.String()
}

func TestStatsRowsFitContent(t *testing.T) {
	for _, size := range [][2]int{{48, 22}, {56, 22}, {80, 24}, {120, 40}} {
		w, h := size[0], size[1]
		for tab := 0; tab < statsTabCount; tab++ {
			for r := 0; r < statsRangeCount; r++ {
				m, _, _ := openTestStats(t, w, h)
				m.setStatsTab(tab)
				m.statsRange = r
				_, cw, _, ch := m.statsContentArea()
				for i, line := range m.statsRows(cw, ch) {
					if got := lineWidth(line); got > cw {
						t.Fatalf("%dx%d tab %d range %d row %d width %d > %d: %q",
							w, h, tab, r, i, got, cw, statLineText(line))
					}
				}
			}
		}
	}
}

func TestStatsActivityColumnsAlign(t *testing.T) {
	m, _, _ := openTestStats(t, 80, 24)
	m.setStatsTab(statsTabActivity)
	_, w, _, h := m.statsContentArea()
	rows := m.statsRows(w, h)

	hourW := statBarsWidth(24, 1, 0)
	weekdayW := statBarsWidth(7, 2, 1)
	const columnGap = 4
	blockW := 1 + hourW + columnGap + weekdayW
	if w < blockW {
		t.Fatalf("test width %d cannot show both charts", w)
	}
	var section, header, axis, chart statLine
	for _, row := range rows {
		text := statLineText(row)
		switch {
		case strings.Contains(text, copy.T(copy.StatsWhenFocus)):
			section = row
		case strings.Contains(text, copy.T(copy.StatsByHour)) &&
			strings.Contains(text, copy.T(copy.StatsByWeekday)):
			header = row
		case strings.Contains(text, weekdayAxis()):
			axis = row
		case chart == nil && strings.ContainsAny(text, "█▄·") && lineWidth(row) == blockW:
			chart = row
		}
	}
	if section == nil || header == nil || axis == nil || chart == nil {
		t.Fatalf("activity rows missing section/header/axis/chart: section=%v header=%v axis=%v chart=%v",
			section != nil, header != nil, axis != nil, chart != nil)
	}
	weekdayAt := 1 + hourW + columnGap
	headerText := statLineText(header)
	if at := strings.Index(headerText, copy.T(copy.StatsByWeekday)); at < 0 || canvas.Width(headerText[:at]) != weekdayAt {
		t.Fatalf("weekday header starts at %d, want %d: %q", canvas.Width(headerText[:maxInt(0, at)]), weekdayAt, headerText)
	}
	axisText := statLineText(axis)
	if at := strings.Index(axisText, copy.WeekdayNarrow(0)); at < 0 || canvas.Width(axisText[:at]) != weekdayAt {
		t.Fatalf("weekday axis starts at %d, want %d: %q", canvas.Width(axisText[:maxInt(0, at)]), weekdayAt, axisText)
	}
	if got := lineWidth(chart); got != blockW {
		t.Fatalf("chart block width = %d, want %d", got, blockW)
	}
	if got := hourAxis(1, 0); canvas.Width(got) != hourW || !strings.HasSuffix(got, "23") {
		t.Fatalf("hour axis = %q, want width %d ending in 23", got, hourW)
	}
}

func TestStatsActivityWideHourChart(t *testing.T) {
	m, _, _ := openTestStats(t, 120, 40)
	m.setStatsTab(statsTabActivity)
	_, w, _, h := m.statsContentArea()
	rows := m.statsRows(w, h)

	hourW := statBarsWidth(24, 2, 1)
	weekdayW := statBarsWidth(7, 2, 1)
	const columnGap = 4
	blockW := 1 + hourW + columnGap + weekdayW
	var header, axis, chart statLine
	for _, row := range rows {
		text := statLineText(row)
		switch {
		case strings.Contains(text, copy.T(copy.StatsByHour)) &&
			strings.Contains(text, copy.T(copy.StatsByWeekday)):
			header = row
		case strings.Contains(text, weekdayAxis()):
			axis = row
		case chart == nil && strings.ContainsAny(text, "█▄·") && lineWidth(row) == blockW:
			chart = row
		}
	}
	if header == nil || axis == nil || chart == nil {
		t.Fatalf("wide activity missing header/axis/chart: header=%v axis=%v chart=%v",
			header != nil, axis != nil, chart != nil)
	}
	weekdayAt := 1 + hourW + columnGap
	if at := strings.Index(statLineText(header), copy.T(copy.StatsByWeekday)); at < 0 || canvas.Width(statLineText(header)[:at]) != weekdayAt {
		t.Fatalf("wide weekday header starts at %d, want %d", at, weekdayAt)
	}
	if at := strings.Index(statLineText(axis), copy.WeekdayNarrow(0)); at < 0 || canvas.Width(statLineText(axis)[:at]) != weekdayAt {
		t.Fatalf("wide weekday axis starts at %d, want %d", at, weekdayAt)
	}
	if got := lineWidth(chart); got != blockW {
		t.Fatalf("wide chart block width = %d, want %d", got, blockW)
	}
	if got := hourAxis(2, 1); canvas.Width(got) != hourW || !strings.HasSuffix(got, "23") {
		t.Fatalf("wide hour axis = %q, want width %d ending in 23", got, hourW)
	}
}

func TestStatsHeatmapSpansAYear(t *testing.T) {
	for _, tc := range []struct {
		w, h, weeks, cellW int
	}{
		{120, 40, heatWeeks, heatCellWide},
		{80, 24, heatWeeks, 1},
	} {
		m, _, payload := openTestStats(t, tc.w, tc.h)
		payload.Daily[100].MS = 90 * 60000
		payload.Daily[100].N = 3
		m.handleServerEvent(serverEventMsg{msg: protocol.NewStats(payload)})
		m.setStatsTab(statsTabActivity)
		assertFrame(t, m, tc.w, tc.h)

		if m.statsHeat.weeks != tc.weeks || m.statsHeat.cellW != tc.cellW {
			t.Fatalf("%dx%d heatmap = %d weeks x %d cells, want %d x %d",
				tc.w, tc.h, m.statsHeat.weeks, m.statsHeat.cellW, tc.weeks, tc.cellW)
		}
		now := m.now().In(m.statsLocation())
		thisMonday := time.Date(now.Year(), now.Month(), now.Day(), 12, 0, 0, 0, m.statsLocation()).
			AddDate(0, 0, -((int(now.Weekday()) + 6) % 7))
		wantFirst := thisMonday.AddDate(0, 0, -7*(tc.weeks-1))
		if !m.statsHeat.firstMon.Equal(wantFirst) {
			t.Fatalf("%dx%d first Monday = %v, want %v", tc.w, tc.h, m.statsHeat.firstMon, wantFirst)
		}

		day, err := time.ParseInLocation("2006-01-02", payload.Daily[100].Date, m.statsLocation())
		if err != nil {
			t.Fatal(err)
		}
		first := time.Date(m.statsHeat.firstMon.Year(), m.statsHeat.firstMon.Month(),
			m.statsHeat.firstMon.Day(), 0, 0, 0, 0, m.statsHeat.firstMon.Location())
		week := int(day.Sub(first).Hours()/24) / heatDays
		weekday := (int(day.Weekday()) + 6) % heatDays
		c := canvas.New(tc.w, tc.h)
		m.drawStats(c, rect{x: 0, y: 0, w: tc.w, h: tc.h})
		cell := c.At(m.statsHeat.x+week*m.statsHeat.cellW, m.statsHeat.y+weekday)
		if !cell.HasBG || cell.BG == m.heatColor(0) {
			t.Fatalf("%dx%d old day %s at week %d weekday %d is not tinted", tc.w, tc.h, payload.Daily[100].Date, week, weekday)
		}
	}
}

func TestStatsHeatHoverShowsDay(t *testing.T) {
	m, _, payload := openTestStats(t, 120, 40)
	m.setStatsTab(statsTabActivity)
	render(m)

	target := payload.Daily[len(payload.Daily)-7]
	day, err := time.ParseInLocation("2006-01-02", target.Date, m.statsLocation())
	if err != nil {
		t.Fatal(err)
	}
	first := time.Date(m.statsHeat.firstMon.Year(), m.statsHeat.firstMon.Month(),
		m.statsHeat.firstMon.Day(), 0, 0, 0, 0, m.statsHeat.firstMon.Location())
	week := int(day.Sub(first).Hours()/24) / heatDays
	weekday := (int(day.Weekday()) + 6) % heatDays
	x := m.statsHeat.x + week*m.statsHeat.cellW + 1
	y := m.statsHeat.y + weekday
	m.handleMouseMotion(tea.MouseMotionMsg{X: x, Y: y})
	if m.statsHeatHover != target.Date {
		t.Fatalf("hover = %q, want %q", m.statsHeatHover, target.Date)
	}
	wantDate := copy.WeekdayShort((int(day.Weekday())+6)%7) + " " +
		fmt.Sprintf("%02d", day.Day()) + " " + copy.MonthShort(day.Month())
	plain := assertFrame(t, m, 120, 40)
	if !strings.Contains(plain, wantDate) || !strings.Contains(plain, "1h") || !strings.Contains(plain, "1 session") {
		t.Fatalf("hovered day missing its detail %q:\n%s", wantDate, plain)
	}

	m.handleMouseMotion(tea.MouseMotionMsg{X: 0, Y: 0})
	if m.statsHeatHover != "" {
		t.Fatalf("leaving the grid kept hover %q", m.statsHeatHover)
	}
}

func TestStatsHeatLevels(t *testing.T) {
	values := make([]int64, 100)
	for i := range values {
		values[i] = int64(i + 1)
	}
	thresholds := heatThresholds(append([]int64(nil), values...))
	if thresholds != [3]int64{26, 51, 76} {
		t.Fatalf("thresholds = %v, want [26 51 76]", thresholds)
	}
	levels := make([]int, len(values))
	for i, v := range values {
		levels[i] = heatLevel(v, thresholds)
	}
	if levels[0] != 1 || levels[len(levels)-1] != 4 {
		t.Fatalf("extreme levels = %d..%d, want 1..4", levels[0], levels[len(levels)-1])
	}
	for i := 1; i < len(levels); i++ {
		if levels[i] < levels[i-1] {
			t.Fatalf("levels are not monotonic at %d: %v", i, levels)
		}
	}

	two := heatThresholds([]int64{10, 40})
	if heatLevel(10, two) == heatLevel(40, two) {
		t.Fatalf("two active days share a level: %v", two)
	}
	three := heatThresholds([]int64{10, 20, 30})
	if a, b, c := heatLevel(10, three), heatLevel(20, three), heatLevel(30, three); a != 2 || b != 3 || c != 4 {
		t.Fatalf("three active day levels = [%d %d %d], want [2 3 4]", a, b, c)
	}
	repeated := heatThresholds([]int64{10, 10, 10})
	if heatLevel(10, repeated) != 4 {
		t.Fatalf("equal active days do not reach the top step")
	}

	if got := heatLevel(42*60000, heatThresholds([]int64{42 * 60000})); got != 4 {
		t.Fatalf("a single active day has level %d, want 4", got)
	}
	if got := heatLevel(0, thresholds); got != 0 {
		t.Fatalf("empty day has level %d, want 0", got)
	}
}

func TestStatsHeatmapNarrowSheet(t *testing.T) {
	m, _, _ := openTestStats(t, 48, 22)
	m.setStatsTab(statsTabActivity)
	render(m)
	if m.statsHeat.weeks != 35 || m.statsHeat.cellW != 1 {
		t.Fatalf("48x22 heatmap = %d weeks x %d cells, want 35 x 1", m.statsHeat.weeks, m.statsHeat.cellW)
	}
}

func TestStatsHeatHoverIgnoresFutureAndTracksScroll(t *testing.T) {
	m, _, _ := openTestStats(t, 120, 40)
	m.setStatsTab(statsTabActivity)
	render(m)
	grid := m.statsHeat
	futureX := grid.x + (grid.weeks-1)*grid.cellW
	futureY := grid.y + 6
	if got := m.heatDayAt(futureX, futureY); got != "" {
		t.Fatalf("future day maps to %q, want empty", got)
	}

	scrolled, _, _ := openTestStats(t, 80, 24)
	scrolled.setStatsTab(statsTabActivity)
	scrolled.scrollStats(1)
	render(scrolled)
	grid = scrolled.statsHeat
	want := grid.firstMon.AddDate(0, 0, 5).Format("2006-01-02")
	if got := scrolled.heatDayAt(grid.x, grid.y+5); got != want {
		t.Fatalf("scrolled heatmap maps to %q, want %q", got, want)
	}
}

func TestStatsHeatHoverClearsOutsideWindow(t *testing.T) {
	m, _, _ := openTestStats(t, 120, 40)
	m.setStatsTab(statsTabActivity)
	render(m)
	m.statsHeatHover = "2000-01-01"
	_, w, _, h := m.statsContentArea()
	m.statsRows(w, h)
	if m.statsHeatHover != "" {
		t.Fatalf("stale hover %q survived the window rebuild", m.statsHeatHover)
	}
}

func TestStatsHeatEmptyCellIsVisible(t *testing.T) {
	m, _, _ := openTestStats(t, 120, 40)
	for _, name := range config.Themes {
		m.cfg.Theme = name
		m.pal = theme.Get(name)
		empty := m.heatColor(0)
		if empty == m.pal.Panel {
			t.Fatalf("%s: empty heat cell is the panel color", name)
		}
		if theme.Contrast(empty, m.pal.Panel) < 1.05 {
			t.Fatalf("%s: empty heat cell is invisible against the panel", name)
		}
		previous := theme.Luminance(empty)
		for level := 1; level <= 4; level++ {
			lum := theme.Luminance(m.heatColor(level))
			if lum <= previous {
				t.Fatalf("%s: heat level %d is not lighter than the previous one", name, level)
			}
			previous = lum
		}
	}
}

func TestStatsDayLabelsMatchChartWidth(t *testing.T) {
	for _, size := range [][2]int{{48, 22}, {80, 24}} {
		m, _, _ := openTestStats(t, size[0], size[1])
		m.statsRange = statsRange30
		_, w, _, h := m.statsContentArea()
		rows := m.statsRows(w, h)
		var labels, chart statLine
		for _, row := range rows {
			text := statLineText(row)
			if !strings.Contains(text, copy.T(copy.StatsFocusPerDay)) &&
				(strings.Contains(text, copy.MonthShort(time.October)) ||
					strings.Contains(text, copy.MonthShort(time.November))) {
				labels = row
			}
			if chart == nil && strings.ContainsAny(text, "█▄") {
				chart = row
			}
		}
		if labels == nil || chart == nil {
			t.Fatalf("%dx%d missing day labels or chart", size[0], size[1])
		}
		if got, want := lineWidth(labels), lineWidth(chart); got != want {
			t.Fatalf("%dx%d labels width %d, chart width %d", size[0], size[1], got, want)
		}
		text := statLineText(labels)
		novAt := 42
		if size[0] == 48 {
			novAt = 28
		}
		if got := strings.Index(text, copy.MonthShort(time.October)); got != 0 {
			t.Fatalf("%dx%d October label at %d: %q", size[0], size[1], got, text)
		}
		if got := strings.Index(text, copy.MonthShort(time.November)); got != novAt {
			t.Fatalf("%dx%d November label at %d, want %d: %q", size[0], size[1], got, novAt, text)
		}
	}
}

func TestStatsMinimumSizeKeepsTheKPI(t *testing.T) {
	m, _, _ := openTestStats(t, 48, 22)
	_, w, _, h := m.statsContentArea()
	lines := m.statsKPILines(w)
	if len(lines) != 2 {
		t.Fatalf("48x22 KPI lines = %d, want 2", len(lines))
	}
	var joined string
	for _, line := range lines {
		if lineWidth(line) > w {
			t.Fatalf("KPI line width %d > %d: %q", lineWidth(line), w, statLineText(line))
		}
		joined += statLineText(line)
	}
	if !strings.Contains(joined, copy.T(copy.StatsOfFinished)) {
		t.Fatalf("48x22 KPI lost the completion rate: %q", joined)
	}
	if rows := m.statsRows(w, h); len(rows) > h {
		t.Fatalf("overview needs %d rows at 48x22, only %d visible", len(rows), h)
	}
}

func TestStatsTinyDayStillDraws(t *testing.T) {
	m, clock := testModel(80, 24)
	m.openStats()
	payload := testStatsPayload(clock.Now())
	for i := range payload.Daily {
		payload.Daily[i].MS = 0
	}
	payload.Daily[len(payload.Daily)-1].MS = 60 * 60000
	payload.Daily[len(payload.Daily)-6].MS = 1
	m.handleServerEvent(serverEventMsg{msg: protocol.NewStats(payload)})
	m.statsRange = statsRange7
	_, w, _, h := m.statsContentArea()
	var chart string
	for _, row := range m.statsRows(w, h) {
		chart += statLineText(row) + "\n"
	}
	if !strings.Contains(chart, "▄") {
		t.Fatalf("a nonzero day with almost no focus drew nothing:\n%s", chart)
	}
}

func TestStatsRecentUsesLongYesterdayAtWideSizes(t *testing.T) {
	for _, tc := range []struct {
		w, h int
		want string
	}{
		{80, 24, "yesterday 12:00"},
		{48, 22, "yest 12:00"},
	} {
		m, _, _ := openTestStats(t, tc.w, tc.h)
		m.setStatsTab(statsTabRecent)
		plain := assertFrame(t, m, tc.w, tc.h)
		if !strings.Contains(plain, tc.want) {
			t.Fatalf("%dx%d recent missing %q:\n%s", tc.w, tc.h, tc.want, plain)
		}
	}
}

func TestStatsRecentLongOutcomeFits(t *testing.T) {
	m, clock := testModel(48, 22)
	m.openStats()
	payload := testStatsPayload(clock.Now())
	payload.Recent[1].PlannedMS = (2 * time.Hour).Milliseconds()
	payload.Recent[1].ElapsedMS = (time.Hour + 5*time.Minute).Milliseconds()
	m.handleServerEvent(serverEventMsg{msg: protocol.NewStats(payload)})
	m.setStatsTab(statsTabRecent)
	_, w, _, h := m.statsContentArea()
	for _, row := range m.statsRows(w, h) {
		if got := lineWidth(row); got > w {
			t.Fatalf("recent row width %d > %d: %q", got, w, statLineText(row))
		}
	}
	plain := assertFrame(t, m, 48, 22)
	if !strings.Contains(plain, "stopped at 1h…") {
		t.Fatalf("long stopped outcome was not shortened:\n%s", plain)
	}
}

func TestStatsPresetsClarifyCompletion(t *testing.T) {
	m, _, _ := openTestStats(t, 80, 24)
	m.setStatsTab(statsTabPresets)
	plain := assertFrame(t, m, 80, 24)
	for _, want := range []string{
		"93% of finished", "14 sessions", "top: 5 min",
	} {
		if !strings.Contains(plain, want) {
			t.Fatalf("presets missing %q:\n%s", want, plain)
		}
	}
}

func TestStatsHintDependsOnSection(t *testing.T) {
	m, _, _ := openTestStats(t, 80, 24)
	plain := assertFrame(t, m, 80, 24)
	if !strings.Contains(plain, copy.T(copy.StatsHint)) {
		t.Fatalf("overview missing the range hint:\n%s", plain)
	}
	m.setStatsTab(statsTabActivity)
	plain = assertFrame(t, m, 80, 24)
	if strings.Contains(plain, "[t] range") {
		t.Fatalf("activity hint advertises the overview range key:\n%s", plain)
	}
	if !strings.Contains(plain, copy.T(copy.StatsHintActivity)) {
		t.Fatalf("activity missing its hint:\n%s", plain)
	}
}

func TestFirstRunesDoesNotSplit(t *testing.T) {
	for _, tc := range []struct {
		in   string
		n    int
		want string
	}{
		{"Mon", 2, "Mo"},
		{"mié", 2, "mi"},
		{"sáb", 2, "sá"},
		{"M", 2, "M"},
		{"", 2, ""},
	} {
		if got := firstRunes(tc.in, tc.n); got != tc.want {
			t.Fatalf("firstRunes(%q, %d) = %q, want %q", tc.in, tc.n, got, tc.want)
		}
	}
}

func TestStatsRendersInEveryTheme(t *testing.T) {
	for _, name := range config.Themes {
		m, _, _ := openTestStats(t, 80, 24)
		m.cfg.Theme = name
		m.pal = theme.Get(name)
		for tab := 0; tab < statsTabCount; tab++ {
			m.setStatsTab(tab)
			assertFrame(t, m, 80, 24)
		}
	}
}
