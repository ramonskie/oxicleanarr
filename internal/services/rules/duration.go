package rules

import (
	"fmt"
	"regexp"
	"strconv"
	"time"
)

// parseDuration parses duration strings like "90d", "24h", "30m", "60s",
// or special values "never"/"0d" which disable retention (returns 0, nil).
func parseDuration(s string) (time.Duration, error) {
	if s == "" {
		return 0, fmt.Errorf("empty duration string")
	}

	// Special values that disable retention
	if s == "never" || s == "0d" {
		return 0, nil
	}

	re := regexp.MustCompile(`^(\d+)([dhms])$`)
	matches := re.FindStringSubmatch(s)
	if len(matches) != 3 {
		return 0, fmt.Errorf("invalid duration format: %s (expected format: 90d, 24h, 30m, or 'never')", s)
	}

	value, err := strconv.Atoi(matches[1])
	if err != nil {
		return 0, fmt.Errorf("invalid duration value: %w", err)
	}

	switch matches[2] {
	case "d":
		return time.Duration(value) * 24 * time.Hour, nil
	case "h":
		return time.Duration(value) * time.Hour, nil
	case "m":
		return time.Duration(value) * time.Minute, nil
	case "s":
		return time.Duration(value) * time.Second, nil
	default:
		return 0, fmt.Errorf("unknown duration unit: %s", matches[2])
	}
}
