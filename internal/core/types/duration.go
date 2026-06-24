package types

import (
	"fmt"
	"regexp"
	"strconv"
	"time"
)

var durationRE = regexp.MustCompile(`^(?:(\d+)d)?(?:(\d+)M)?(?:(\d+)h)?(?:(\d+)m)?(?:(\d+)s)?$`)

func ParseDuration(s string) (time.Duration, error) {
	if s == "" {
		return 0, fmt.Errorf("empty duration string")
	}

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

func MustParseDuration(s string) time.Duration {
	d, err := ParseDuration(s)
	if err != nil {
		panic(err)
	}
	return d
}

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
