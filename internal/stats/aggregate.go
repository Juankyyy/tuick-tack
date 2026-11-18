package stats

import (
	"sort"
	"strings"
	"time"

	"tuick-tack/internal/protocol"
)

// Aggregate builds the wire summary the Statistics tab renders. now fixes the
// daily window and loc buckets local days, hours and weekdays.
func (s *Store) Aggregate(now time.Time, loc *time.Location) protocol.StatsPayload {
	if loc == nil {
		loc = time.Local
	}
	recs := s.Records()
	local := now.In(loc)
	today := time.Date(local.Year(), local.Month(), local.Day(), 12, 0, 0, 0, loc)

	p := protocol.StatsPayload{
		Daily:    make([]protocol.StatsDay, DailyWindow),
		Hourly:   make([]protocol.StatsBucket, 24),
		Weekday:  make([]protocol.StatsBucket, 7),
		Retained: len(recs),
		Records: protocol.StatsRecords{
			BusiestWeekday: -1,
			BusiestHour:    -1,
		},
	}
	index := make(map[string]int, DailyWindow)
	for i := 0; i < DailyWindow; i++ {
		day := today.AddDate(0, 0, -(DailyWindow - 1 - i))
		key := day.Format(dateLayout)
		p.Daily[i].Date = key
		index[key] = i
	}

	activeDays := make(map[string]bool, len(recs))
	labels := make(map[string]*protocol.StatsLabel)
	for _, rec := range recs {
		started := time.UnixMilli(rec.StartedMS).In(loc)
		key := started.Format(dateLayout)
		activeDays[key] = true

		hour := started.Hour()
		p.Hourly[hour].MS += rec.ElapsedMS
		p.Hourly[hour].N++
		weekday := (int(started.Weekday()) + 6) % 7
		p.Weekday[weekday].MS += rec.ElapsedMS
		p.Weekday[weekday].N++

		daysAgo := DailyWindow // older than the retained window
		if idx, ok := index[key]; ok {
			daysAgo = DailyWindow - 1 - idx
			p.Daily[idx].MS += rec.ElapsedMS
			p.Daily[idx].N++
		}

		addRange(&p.Totals.All, rec)
		if daysAgo <= 6 {
			addRange(&p.Totals.D7, rec)
		}
		if daysAgo <= 29 {
			addRange(&p.Totals.D30, rec)
		}
		if daysAgo <= 89 {
			addRange(&p.Totals.D90, rec)
		}

		label := strings.TrimSpace(rec.Label)
		entry := labels[label]
		if entry == nil {
			entry = &protocol.StatsLabel{Label: label}
			labels[label] = entry
		}
		entry.MS += rec.ElapsedMS
		entry.N++
		switch rec.Outcome {
		case Completed:
			entry.Completed++
		case Stopped:
			entry.Stopped++
		}

		if rec.ElapsedMS > p.Records.LongestMS {
			p.Records.LongestMS = rec.ElapsedMS
			p.Records.LongestLabel = label
		}
	}

	p.Records.StreakDays = currentStreak(activeDays, today)
	p.Records.BestStreakDays = bestStreak(activeDays, loc)
	if len(recs) > 0 {
		p.Records.BusiestWeekday = busiest(p.Weekday)
		p.Records.BusiestHour = busiest(p.Hourly)
	}
	p.Labels = sortedLabels(labels, LabelCount)

	recent := append([]Record(nil), recs...)
	sort.SliceStable(recent, func(i, j int) bool {
		return recent[i].StartedMS > recent[j].StartedMS
	})
	if len(recent) > RecentCount {
		recent = recent[:RecentCount]
	}
	for _, rec := range recent {
		p.Recent = append(p.Recent, protocol.StatsEvent{
			StartedMS: rec.StartedMS,
			PlannedMS: rec.PlannedMS,
			ElapsedMS: rec.ElapsedMS,
			Label:     rec.Label,
			Outcome:   string(rec.Outcome),
		})
	}
	return p
}

func addRange(r *protocol.StatsRange, rec Record) {
	r.Sessions++
	r.MS += rec.ElapsedMS
	switch rec.Outcome {
	case Completed:
		r.Completed++
	case Stopped:
		r.Stopped++
	case Replaced:
		r.Replaced++
	}
}

func sortedLabels(labels map[string]*protocol.StatsLabel, limit int) []protocol.StatsLabel {
	out := make([]protocol.StatsLabel, 0, len(labels))
	for _, entry := range labels {
		out = append(out, *entry)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].MS != out[j].MS {
			return out[i].MS > out[j].MS
		}
		if out[i].N != out[j].N {
			return out[i].N > out[j].N
		}
		return out[i].Label < out[j].Label
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}

func busiest(buckets []protocol.StatsBucket) int {
	if len(buckets) == 0 {
		return -1
	}
	best := 0
	for i := 1; i < len(buckets); i++ {
		if buckets[i].MS > buckets[best].MS ||
			(buckets[i].MS == buckets[best].MS && buckets[i].N > buckets[best].N) {
			best = i
		}
	}
	if buckets[best].N == 0 {
		return -1
	}
	return best
}

// currentStreak counts days with at least one session, walking back from today
// (or yesterday when today is still empty, so the streak survives the morning).
func currentStreak(days map[string]bool, today time.Time) int {
	start := today
	if !days[start.Format(dateLayout)] {
		start = start.AddDate(0, 0, -1)
		if !days[start.Format(dateLayout)] {
			return 0
		}
	}
	streak := 0
	for days[start.Format(dateLayout)] {
		streak++
		start = start.AddDate(0, 0, -1)
	}
	return streak
}

func bestStreak(days map[string]bool, loc *time.Location) int {
	keys := make([]string, 0, len(days))
	for key := range days {
		keys = append(keys, key)
	}
	if len(keys) == 0 {
		return 0
	}
	sort.Strings(keys)
	best, run := 1, 1
	for i := 1; i < len(keys); i++ {
		prev, err := time.ParseInLocation(dateLayout, keys[i-1], loc)
		if err == nil && prev.AddDate(0, 0, 1).Format(dateLayout) == keys[i] {
			run++
		} else {
			run = 1
		}
		if run > best {
			best = run
		}
	}
	return best
}
