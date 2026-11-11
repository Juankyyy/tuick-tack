package tui

import (
	"testing"

	"tuick-tack/internal/protocol"
)

func BenchmarkViewIdle(b *testing.B) {
	m, _ := testModel(120, 40)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = m.View()
	}
}

func BenchmarkViewRunning(b *testing.B) {
	m, _ := testModel(120, 40)
	applyRunning(m)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = m.View()
	}
}

func BenchmarkViewCelebration(b *testing.B) {
	m, _ := testModel(120, 40)
	applyRunning(m)
	m.applyState(protocol.StateMessage{
		Type: protocol.TypeState, Status: "finished",
		DurationMS: 300000, Label: "5 min", FinishSeq: 1,
	})
	for i := 0; i < 20; i++ {
		if m.confetti != nil {
			m.confetti.Update(0.033)
		}
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = m.View()
	}
}
