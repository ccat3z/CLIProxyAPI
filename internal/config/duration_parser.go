package config

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var dayPattern = regexp.MustCompile(`^(\d+(?:\.\d+)?)d$`)

// ParseDurationWithDays extends time.ParseDuration to support day suffix ("d").
// Examples: "1d" -> 24h, "7d" -> 168h, "5h" -> 5h, "2h30m" -> 2h30m.
func ParseDurationWithDays(s string) (time.Duration, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return 0, fmt.Errorf("empty duration")
	}
	if m := dayPattern.FindStringSubmatch(s); m != nil {
		days, err := strconv.ParseFloat(m[1], 64)
		if err != nil {
			return 0, fmt.Errorf("invalid day count %q: %w", m[1], err)
		}
		return time.Duration(days * float64(24*time.Hour)), nil
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, fmt.Errorf("invalid duration %q: %w", s, err)
	}
	return d, nil
}

// ParseTokenAmount parses human-friendly token amount strings.
// Suffixes: "m" = million, "k" = thousand. No suffix = plain integer.
// Examples: "5m" -> 5000000, "500k" -> 500000, "1000" -> 1000.
func ParseTokenAmount(s string) (int64, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return 0, nil
	}

	last := s[len(s)-1]
	numberPart := s

	switch last {
	case 'm':
		numberPart = s[:len(s)-1]
		v, err := strconv.ParseFloat(numberPart, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid token amount %q: %w", s, err)
		}
		return int64(v * 1_000_000), nil
	case 'k':
		numberPart = s[:len(s)-1]
		v, err := strconv.ParseFloat(numberPart, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid token amount %q: %w", s, err)
		}
		return int64(v * 1_000), nil
	default:
		v, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid token amount %q: %w", s, err)
		}
		return v, nil
	}
}

// parseFloatOptional parses a float string, treating empty/whitespace-only as 0.
func parseFloatOptional(s string) (float64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, nil
	}
	return strconv.ParseFloat(s, 64)
}
