package types

import (
	"fmt"
	"regexp"
	"strconv"
	"time"
)

// durationRE matches compound duration strings like "1d3M2h30m".
// Each component (d=day, M=month≈30d, h=hour, m=minute, s=second) is optional.
var durationRE = regexp.MustCompile(`^(?:(\d+)d)?(?:(\d+)M)?(?:(\d+)h)?(?:(\d+)m)?(?:(\d+)s)?$`)

// ParseDuration extends time.ParseDuration with day (d) and month (M) suffixes.
// Months are approximated as 30 days. Standard Go duration strings ("1h30m")
// are also accepted.
func ParseDuration(s string) (time.Duration, error) {
	if s == "" {
		return 0, fmt.Errorf("empty duration string")
	}

	// Try standard Go duration first.
	d, err := time.ParseDuration(s)
	if err == nil {
		return d, nil
	}

	matches := durationRE.FindStringSubmatch(s)
	if matches == nil {
		return 0, fmt.Errorf("invalid duration format: %s", s)
	}

	var total time.Duration
	if matches[1] != "" {
		days, _ := strconv.Atoi(matches[1])
		total += time.Duration(days) * 24 * time.Hour
	}
	if matches[2] != "" {
		months, _ := strconv.Atoi(matches[2])
		total += time.Duration(months) * 30 * 24 * time.Hour
	}
	if matches[3] != "" {
		hours, _ := strconv.Atoi(matches[3])
		total += time.Duration(hours) * time.Hour
	}
	if matches[4] != "" {
		minutes, _ := strconv.Atoi(matches[4])
		total += time.Duration(minutes) * time.Minute
	}
	if matches[5] != "" {
		seconds, _ := strconv.Atoi(matches[5])
		total += time.Duration(seconds) * time.Second
	}

	if total == 0 && s != "0s" && s != "0" {
		return 0, fmt.Errorf("invalid duration format: %s", s)
	}

	return total, nil
}

// MustParseDuration is like ParseDuration but panics on error.
// Only use for compile-time-known constants.
func MustParseDuration(s string) time.Duration {
	d, err := ParseDuration(s)
	if err != nil {
		panic(err)
	}
	return d
}

// FormatDuration returns a human-readable representation using days, hours,
// and minutes. Seconds are omitted.
func FormatDuration(d time.Duration) string {
	if d == 0 {
		return "0s"
	}

	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60

	if days > 0 {
		return fmt.Sprintf("%dd%dh%dm", days, hours, minutes)
	}
	if hours > 0 {
		return fmt.Sprintf("%dh%dm", hours, minutes)
	}
	return fmt.Sprintf("%dm", minutes)
}
