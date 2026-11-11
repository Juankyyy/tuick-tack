// Package duration parses the free-form durations typed in the TUI and
// formats remaining time for display.
package duration

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

const (
	// Min is the shortest accepted countdown.
	Min = time.Second
	// Max is the longest accepted countdown (24 hours).
	Max = 24 * time.Hour
)

var (
	// ErrInvalid reports a syntactically invalid duration.
	ErrInvalid = errors.New("invalid duration")
	// ErrOutOfRange reports a valid duration outside 1s..24h.
	ErrOutOfRange = errors.New("duration out of range")
)

// Parse accepts the formats documented in SPEC §9:
//
//	25       25 minutes
//	25m      25 minutes
//	90s      90 seconds
//	1h30m    1 hour 30 minutes
//	12:30    12 minutes 30 seconds
//	1:02:03  1 hour 2 minutes 3 seconds
//
// It rejects decimals, empty segments, repeated units and values outside
// 1 second..24 hours.
func Parse(input string) (time.Duration, error) {
	s := strings.TrimSpace(input)
	if s == "" {
		return 0, ErrInvalid
	}
	if strings.ContainsAny(s, ",.\u00a0") {
		return 0, ErrInvalid
	}

	var total time.Duration
	var err error
	if strings.Contains(s, ":") {
		total, err = parseClock(s)
	} else if isAllDigits(s) {
		total, err = parseMinutes(s)
	} else {
		total, err = parseUnits(strings.ToLower(s))
	}
	if err != nil {
		return 0, err
	}
	if total < Min || total > Max {
		return 0, ErrOutOfRange
	}
	return total, nil
}

func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func parseMinutes(s string) (time.Duration, error) {
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil || n > int64(Max/time.Minute)+1 {
		return 0, ErrOutOfRange
	}
	return time.Duration(n) * time.Minute, nil
}

func parseClock(s string) (time.Duration, error) {
	parts := strings.Split(s, ":")
	if len(parts) != 2 && len(parts) != 3 {
		return 0, ErrInvalid
	}
	nums := make([]int64, len(parts))
	for i, p := range parts {
		if !isAllDigits(p) {
			return 0, ErrInvalid
		}
		n, err := strconv.ParseInt(p, 10, 64)
		if err != nil {
			return 0, ErrInvalid
		}
		nums[i] = n
	}

	var hours, minutes, seconds int64
	if len(nums) == 2 {
		minutes, seconds = nums[0], nums[1]
		if seconds > 59 {
			return 0, ErrInvalid
		}
	} else {
		hours, minutes, seconds = nums[0], nums[1], nums[2]
		if minutes > 59 || seconds > 59 {
			return 0, ErrInvalid
		}
	}
	if hours > int64(Max/time.Hour)+1 {
		return 0, ErrOutOfRange
	}
	return time.Duration(hours)*time.Hour +
		time.Duration(minutes)*time.Minute +
		time.Duration(seconds)*time.Second, nil
}

func parseUnits(s string) (time.Duration, error) {
	var total time.Duration
	seen := map[byte]bool{}
	i := 0
	for i < len(s) {
		start := i
		for i < len(s) && s[i] >= '0' && s[i] <= '9' {
			i++
		}
		if start == i || i >= len(s) {
			return 0, ErrInvalid
		}
		n, err := strconv.ParseInt(s[start:i], 10, 64)
		if err != nil {
			return 0, ErrOutOfRange
		}
		unit := s[i]
		i++
		if seen[unit] {
			return 0, ErrInvalid
		}
		seen[unit] = true
		switch unit {
		case 'h':
			if n > int64(Max/time.Hour)+1 {
				return 0, ErrOutOfRange
			}
			total += time.Duration(n) * time.Hour
		case 'm':
			if n > int64(Max/time.Minute)+1 {
				return 0, ErrOutOfRange
			}
			total += time.Duration(n) * time.Minute
		case 's':
			if n > int64(Max/time.Second)+1 {
				return 0, ErrOutOfRange
			}
			total += time.Duration(n) * time.Second
		default:
			return 0, ErrInvalid
		}
	}
	if len(seen) == 0 {
		return 0, ErrInvalid
	}
	return total, nil
}

// Format renders a duration for the countdown. Values below one hour use
// MM:SS; from one hour on they use H:MM:SS. Remaining seconds round up so the
// display never shows 00:00 while time is still left.
func Format(d time.Duration) string {
	secs := int64(math.Ceil(d.Seconds()))
	if secs < 0 {
		secs = 0
	}
	if secs >= 3600 {
		return fmt.Sprintf("%d:%02d:%02d", secs/3600, (secs%3600)/60, secs%60)
	}
	return fmt.Sprintf("%02d:%02d", secs/60, secs%60)
}

// Short renders a duration for menus and labels: "5 min", "90s", "1h 30m".
func Short(d time.Duration) string {
	secs := int64(math.Round(d.Seconds()))
	if secs < 0 {
		secs = 0
	}
	if secs == 0 {
		return "0s"
	}
	if secs >= 3600 {
		h := secs / 3600
		m := (secs % 3600) / 60
		s := secs % 60
		switch {
		case m == 0 && s == 0:
			return fmt.Sprintf("%dh", h)
		case s == 0:
			return fmt.Sprintf("%dh %dm", h, m)
		default:
			return fmt.Sprintf("%dh %dm %ds", h, m, s)
		}
	}
	if secs%60 == 0 {
		return fmt.Sprintf("%d min", secs/60)
	}
	if secs < 60 {
		return fmt.Sprintf("%ds", secs)
	}
	return fmt.Sprintf("%d:%02d", secs/60, secs%60)
}
