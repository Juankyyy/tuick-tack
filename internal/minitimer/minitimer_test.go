package minitimer

import (
	"testing"
	"time"

	"tuick-tack/internal/engine"
	"tuick-tack/internal/tui/canvas"
	"tuick-tack/internal/tui/theme"
)

func TestCellForDPI(t *testing.T) {
	tests := []struct {
		dpi  int
		want int
	}{
		{0, 3}, {72, 3}, {96, 3}, {110, 3}, {120, 4}, {132, 4},
		{144, 5}, {168, 5}, {192, 6}, {240, 6},
	}
	for _, tt := range tests {
		if got := CellForDPI(tt.dpi); got != tt.want {
			t.Errorf("CellForDPI(%d) = %d, want %d", tt.dpi, got, tt.want)
		}
	}
}

func TestLayoutFor(t *testing.T) {
	tests := []struct {
		cell int
		w, h int
	}{
		{2, 128, 30},
		{3, 128, 30},
		{4, 170, 40},
		{5, 212, 50},
		{7, 254, 60},
	}
	for _, tt := range tests {
		got := LayoutFor(tt.cell)
		if got.Width != tt.w || got.Height != tt.h {
			t.Errorf("LayoutFor(%d) = %dx%d, want %dx%d",
				tt.cell, got.Width, got.Height, tt.w, tt.h)
		}
	}
	if got := LayoutFor(3); got.ContentX() != 1 || got.ContentY() != 3 || got.Cells != Cols {
		t.Errorf("base layout = %+v, want content 1,3 and %d cells", got, Cols)
	}
}

func TestWidthCells(t *testing.T) {
	tests := []struct {
		state engine.State
		want  int
	}{
		{engine.State{Status: engine.Running, Remaining: 4*time.Minute + 32*time.Second}, 42},
		{engine.State{Status: engine.Running, Remaining: time.Hour + 2*time.Minute + 3*time.Second}, 42},
		{engine.State{Status: engine.Running, Remaining: 9*time.Hour + 59*time.Minute}, 42},
		{engine.State{Status: engine.Running, Remaining: 10 * time.Hour}, 47},
		{engine.State{Status: engine.Finished}, 42},
	}
	for _, tt := range tests {
		if got := WidthCells(tt.state); got != tt.want {
			t.Errorf("WidthCells(%+v) = %d, want %d", tt.state, got, tt.want)
		}
	}
}

func TestLayoutForState(t *testing.T) {
	st := engine.State{Status: engine.Running, Remaining: 10 * time.Hour}
	got := LayoutForState(3, st)
	if got.Cells != 47 || got.Width != 143 || got.Height != 30 {
		t.Fatalf("LayoutForState = %+v, want 47 cells, 143x30", got)
	}
}

func TestShouldShow(t *testing.T) {
	env := Environment{TaskbarVisible: true, TaskbarHorizontal: true}
	tests := []struct {
		name    string
		enabled bool
		status  engine.Status
		env     Environment
		want    bool
	}{
		{"disabled", false, engine.Running, env, false},
		{"idle", true, engine.Idle, env, false},
		{"running", true, engine.Running, env, true},
		{"paused", true, engine.Paused, env, true},
		{"finished", true, engine.Finished, env, true},
		{"taskbar hidden", true, engine.Running,
			Environment{TaskbarVisible: false, TaskbarHorizontal: true}, false},
		{"vertical taskbar", true, engine.Running,
			Environment{TaskbarVisible: true, TaskbarHorizontal: false}, false},
		{"fullscreen", true, engine.Running,
			Environment{TaskbarVisible: true, TaskbarHorizontal: true, Fullscreen: true}, false},
		{"tui focused", true, engine.Running,
			Environment{TaskbarVisible: true, TaskbarHorizontal: true, TUIFocused: true}, false},
		{"shell overlay", true, engine.Running,
			Environment{TaskbarVisible: true, TaskbarHorizontal: true, ShellOverlay: true}, false},
	}
	for _, tt := range tests {
		if got := ShouldShow(tt.enabled, engine.State{Status: tt.status}, tt.env); got != tt.want {
			t.Errorf("%s: ShouldShow = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestFinishWindow(t *testing.T) {
	base := time.Date(2026, 11, 27, 10, 0, 0, 0, time.UTC)
	finished := func(seq int64) engine.State {
		return engine.State{Status: engine.Finished, FinishSeq: seq}
	}

	var w finishWindow
	if w.closed(base) {
		t.Fatal("a window that never saw a finish reports closed")
	}
	w.observe(engine.State{Status: engine.Running}, base)
	if !w.at.IsZero() || w.closed(base.Add(time.Hour)) {
		t.Fatal("a running state started the finished window")
	}

	w.observe(finished(1), base)
	tests := []struct {
		name string
		at   time.Time
		want bool
	}{
		{"start", base, false},
		{"below hold", base.Add(FinishedHold - time.Millisecond), false},
		{"at hold", base.Add(FinishedHold), true},
		{"after hold", base.Add(FinishedHold + time.Minute), true},
	}
	for _, tt := range tests {
		if got := w.closed(tt.at); got != tt.want {
			t.Errorf("%s: closed = %v, want %v", tt.name, got, tt.want)
		}
	}

	// Seeing the same finish again must keep the original deadline.
	w.observe(finished(1), base.Add(5*time.Second))
	if !w.closed(base.Add(FinishedHold)) {
		t.Fatal("the same finish restarted the window")
	}

	// A new finish gets its own window.
	w.observe(finished(2), base.Add(20*time.Second))
	if w.closed(base.Add(20*time.Second + FinishedHold - time.Millisecond)) {
		t.Fatal("a new finish closed before its hold")
	}
	if !w.closed(base.Add(20*time.Second + FinishedHold)) {
		t.Fatal("a new finish did not restart the window")
	}

	// Leaving the finished state clears the window.
	w.observe(engine.State{Status: engine.Idle}, base.Add(40*time.Second))
	if w.closed(base.Add(time.Hour)) {
		t.Fatal("an idle state kept the finished window")
	}
}

func TestFinishWindowElapsed(t *testing.T) {
	base := time.Date(2026, 11, 27, 10, 0, 0, 0, time.UTC)
	var w finishWindow
	if _, ok := w.elapsed(base); ok {
		t.Fatal("a window that never saw a finish reported an age")
	}
	w.observe(engine.State{Status: engine.Finished, FinishSeq: 7}, base)
	if got, ok := w.elapsed(base.Add(750 * time.Millisecond)); !ok || got != 750*time.Millisecond {
		t.Fatalf("elapsed = %v, %v, want 750ms, true", got, ok)
	}
	// Seeing the same finish again must not restart the burst clock.
	w.observe(engine.State{Status: engine.Finished, FinishSeq: 7}, base.Add(2*time.Second))
	if got, _ := w.elapsed(base.Add(2 * time.Second)); got != 2*time.Second {
		t.Fatalf("the same finish restarted the clock at %v", got)
	}
	w.observe(engine.State{Status: engine.Finished, FinishSeq: 8}, base.Add(20*time.Second))
	if got, _ := w.elapsed(base.Add(20 * time.Second)); got != 0 {
		t.Fatalf("a new finish started at %v, want 0", got)
	}
	w.observe(engine.State{Status: engine.Idle}, base.Add(40*time.Second))
	if _, ok := w.elapsed(base.Add(41 * time.Second)); ok {
		t.Fatal("an idle state kept the finish clock")
	}
}

func TestBurstActive(t *testing.T) {
	tests := []struct {
		name string
		age  time.Duration
		ok   bool
		want bool
	}{
		{"no finish", 0, false, false},
		{"before the finish", -time.Millisecond, true, false},
		{"start", 0, true, true},
		{"last frame", BurstDuration - time.Millisecond, true, true},
		{"over", BurstDuration, true, false},
	}
	for _, tt := range tests {
		if got := burstActive(tt.age, tt.ok); got != tt.want {
			t.Errorf("%s: burstActive = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestTimeText(t *testing.T) {
	tests := []struct {
		state engine.State
		want  string
	}{
		{engine.State{Status: engine.Idle}, ""},
		{engine.State{Status: engine.Running, Remaining: 4*time.Minute + 32*time.Second}, "04:32"},
		{engine.State{Status: engine.Paused, Remaining: 90 * time.Second}, "01:30"},
		{engine.State{Status: engine.Running, Remaining: time.Hour + 2*time.Minute + 3*time.Second}, "1:02:03"},
		{engine.State{Status: engine.Finished}, "00:00"},
	}
	for _, tt := range tests {
		if got := TimeText(tt.state); got != tt.want {
			t.Errorf("TimeText(%+v) = %q, want %q", tt.state, got, tt.want)
		}
	}
}

func TestProgress(t *testing.T) {
	tests := []struct {
		state engine.State
		want  float64
	}{
		{engine.State{Status: engine.Idle}, 0},
		{engine.State{Status: engine.Running, Duration: 4 * time.Minute, Remaining: 4 * time.Minute}, 0},
		{engine.State{Status: engine.Running, Duration: 4 * time.Minute, Remaining: 2 * time.Minute}, 0.5},
		{engine.State{Status: engine.Paused, Duration: 4 * time.Minute, Remaining: time.Minute}, 0.75},
		{engine.State{Status: engine.Finished, Duration: 4 * time.Minute}, 1},
		{engine.State{Status: engine.Running, Duration: 0, Remaining: 0}, 0},
		{engine.State{Status: engine.Running, Duration: time.Minute, Remaining: -time.Second}, 1},
		{engine.State{Status: engine.Running, Duration: time.Minute, Remaining: 2 * time.Minute}, 0},
	}
	for _, tt := range tests {
		if got := Progress(tt.state); got != tt.want {
			t.Errorf("Progress(%+v) = %v, want %v", tt.state, got, tt.want)
		}
	}
}

func TestPaletteFor(t *testing.T) {
	pal := theme.Get("midnight")
	if got := PaletteFor(pal, engine.State{Status: engine.Running}); got.Top != pal.Accent ||
		got.Bottom != pal.Accent2 || got.Head != pal.Text {
		t.Fatalf("running palette = %+v", got)
	}
	if got := PaletteFor(pal, engine.State{Status: engine.Paused}); got.Top != pal.Warn ||
		got.Bottom != pal.Warn || got.Head != pal.Warn {
		t.Fatalf("paused palette = %+v", got)
	}
	if got := PaletteFor(pal, engine.State{Status: engine.Finished}); got.Top != pal.Warn ||
		got.Bottom != pal.Danger || got.Head != pal.Text {
		t.Fatalf("finished palette = %+v", got)
	}
}

func TestRender(t *testing.T) {
	pal := theme.Get("midnight")
	st := engine.State{
		Status:    engine.Running,
		Duration:  4 * time.Minute,
		Remaining: 2 * time.Minute,
	}
	c := Render(st, pal, 0)
	if c.W != Cols || c.H != Rows {
		t.Fatalf("canvas = %dx%d, want %dx%d", c.W, c.H, Cols, Rows)
	}
	p := PaletteFor(pal, st)
	barX := (Cols - BarCells) / 2
	if got := c.At(barX+BarCells-1, barRow).BG; got != p.Track {
		t.Errorf("bar track = %+v, want %+v", got, p.Track)
	}
	if got := c.At(barX, barRow).BG; got != p.Top {
		t.Errorf("first filled cell = %+v, want %+v", got, p.Top)
	}
	if got := c.At(barX+17, barRow).BG; got != p.Head {
		t.Errorf("bar head = %+v, want %+v", got, p.Head)
	}
	if !hasInk(c) {
		t.Fatal("time glyphs were not drawn")
	}
}

func TestRenderLongTime(t *testing.T) {
	pal := theme.Get("midnight")
	st := engine.State{
		Status:    engine.Running,
		Duration:  24 * time.Hour,
		Remaining: 10 * time.Hour,
	}
	c := Render(st, pal, 0)
	if c.W != 47 || c.H != Rows {
		t.Fatalf("long-time canvas = %dx%d, want 47x%d", c.W, c.H, Rows)
	}
	if !hasInk(c) {
		t.Fatal("long-time glyphs were not drawn")
	}
	barX := (c.W - BarCells) / 2
	if got := c.At(barX, barRow).BG; got != PaletteFor(pal, st).Top {
		t.Errorf("long-time first filled cell = %+v", got)
	}
	if got := c.At(c.W-1, barRow).HasBG; got {
		t.Error("bar should stay centered, not stretch to the new width")
	}
}

func TestRenderFinished(t *testing.T) {
	pal := theme.Get("ember")
	st := engine.State{Status: engine.Finished, Duration: 5 * time.Minute}
	c := Render(st, pal, 0)
	if TimeText(st) != "00:00" {
		t.Fatalf("finished text = %q", TimeText(st))
	}
	p := PaletteFor(pal, st)
	barX := (Cols - BarCells) / 2
	for i := 0; i < BarCells; i++ {
		if !c.At(barX+i, barRow).HasBG {
			t.Fatalf("finished bar cell %d has no background", i)
		}
	}
	if got := c.At(barX+BarCells-1, barRow).BG; got != p.Head {
		t.Errorf("finished head = %+v, want %+v", got, p.Head)
	}
}

func TestRenderPausedUsesWarn(t *testing.T) {
	pal := theme.Get("mono")
	st := engine.State{
		Status:    engine.Paused,
		Duration:  4 * time.Minute,
		Remaining: time.Minute,
	}
	c := Render(st, pal, 0)
	p := PaletteFor(pal, st)
	barX := (Cols - BarCells) / 2
	if got := c.At(barX, barRow).BG; got != p.Top {
		t.Errorf("paused fill = %+v, want %+v", got, p.Top)
	}
}

func TestRenderBurst(t *testing.T) {
	pal := theme.Get("midnight")
	st := engine.State{Status: engine.Finished, Duration: 5 * time.Minute}
	calm := RenderAt(st, pal, 0, Cols)
	running := engine.State{Status: engine.Running, Duration: time.Minute, Remaining: 30 * time.Second}
	runningCalm := RenderAt(running, pal, 0, Cols)

	tests := []struct {
		name  string
		state engine.State
		age   time.Duration
		calm  *canvas.Canvas
		same  bool
	}{
		{"before the finish", st, -time.Millisecond, calm, true},
		{"burst start", st, 0, calm, false},
		{"mid burst", st, 700 * time.Millisecond, calm, false},
		{"burst over", st, BurstDuration, calm, true},
		{"after the burst", st, BurstDuration + time.Minute, calm, true},
		{"not finished", running, 700 * time.Millisecond, runningCalm, true},
	}
	for _, tt := range tests {
		got := RenderBurstAt(tt.state, pal, 0, Cols, tt.age)
		if same := sameCanvas(got, tt.calm); same != tt.same {
			t.Errorf("%s: same as the calm frame = %v, want %v", tt.name, same, tt.same)
		}
	}
}

func TestRenderBurstFlashesTheBar(t *testing.T) {
	pal := theme.Get("ember")
	st := engine.State{Status: engine.Finished, Duration: 5 * time.Minute}
	calm := RenderAt(st, pal, 0, Cols)
	c := RenderBurstAt(st, pal, 0, Cols, burstFlash/2)
	barX := (Cols - BarCells) / 2
	if c.At(barX, barRow) == calm.At(barX, barRow) {
		t.Fatal("the burst did not lift the bar")
	}
	if got := c.At(barX+BarCells-1, barRow).BG; got != PaletteFor(pal, st).Head {
		t.Errorf("the burst moved the bar head: %+v", got)
	}
}

func TestRenderBurstCoversDigits(t *testing.T) {
	pal := theme.Get("midnight")
	st := engine.State{Status: engine.Finished, Duration: 5 * time.Minute}
	calm := RenderAt(st, pal, 0, Cols)
	c := RenderBurstAt(st, pal, 0, Cols, 700*time.Millisecond)
	covered := 0
	for y := 0; y < c.H; y++ {
		for x := 0; x < c.W; x++ {
			before, after := calm.At(x, y), c.At(x, y)
			if before == after {
				continue
			}
			// The widget painter draws BG and then FG, so a piece over a
			// digit must carry both colors to stay visible.
			if !after.HasFG || after.FG != after.BG {
				t.Fatalf("cell %d,%d is not a solid burst block: %+v", x, y, after)
			}
			if before.HasFG {
				covered++
			}
		}
	}
	if covered == 0 {
		t.Fatal("the burst did not paint over any digit cell")
	}
}

func TestRenderBurstDeterministic(t *testing.T) {
	pal := theme.Get("midnight")
	st := engine.State{Status: engine.Finished, Duration: time.Minute}
	age := 450 * time.Millisecond
	if !sameCanvas(RenderBurstAt(st, pal, 0, Cols, age), RenderBurstAt(st, pal, 0, Cols, age)) {
		t.Fatal("the burst is not deterministic")
	}
}

func TestRenderBurstKeepsTheBarCentred(t *testing.T) {
	pal := theme.Get("mono")
	st := engine.State{Status: engine.Finished, Duration: 24 * time.Hour, FinishSeq: 1}
	calm := RenderAt(st, pal, 0, 47)
	c := RenderBurstAt(st, pal, 0, 47, burstFlash/2)
	if c.W != 47 {
		t.Fatalf("canvas width = %d, want 47", c.W)
	}
	barX := (47 - BarCells) / 2
	if c.At(barX, barRow) == calm.At(barX, barRow) {
		t.Error("the burst did not lift the centred bar")
	}
	for y := 0; y < c.H; y++ {
		for x := 0; x < c.W; x++ {
			if ch := c.At(x, y).Ch; ch != ' ' && ch != '█' {
				t.Fatalf("cell %d,%d has glyph %q", x, y, ch)
			}
		}
	}
}

func sameCanvas(a, b *canvas.Canvas) bool {
	if a.W != b.W || a.H != b.H {
		return false
	}
	for y := 0; y < a.H; y++ {
		for x := 0; x < a.W; x++ {
			if a.At(x, y) != b.At(x, y) {
				return false
			}
		}
	}
	return true
}

func hasInk(c *canvas.Canvas) bool {
	for y := 0; y < c.H; y++ {
		for x := 0; x < c.W; x++ {
			if c.At(x, y).HasFG {
				return true
			}
		}
	}
	return false
}
