/*
Copyright 2026 Joseph Anthony Abbott III

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package internal

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// deadlineInputLayout is the absolute deadline format accepted from users.
const deadlineInputLayout = "2006-01-02 15:04"

// Upper bounds keep relative deadlines far from time.Duration overflow and
// stop oversized inputs from doing unbounded work.
const (
	maxDeadlineMonths   = 1200                       // 100 years
	maxRelativeDuration = 100 * 365 * 24 * time.Hour // ~100 years for m/h/d/w units
)

// relativeTokenRegex matches one "<number><unit>" token at the start of the input.
// Uppercase M means months; m/h/d/w (either case) mean minutes/hours/days/weeks.
var relativeTokenRegex = regexp.MustCompile(`^(\d+)([mhdwMHDW])`)

// ParseDeadline accepts multiple deadline formats:
// 1. Absolute: "YYYY-MM-DD HH:MM" (e.g., "2025-11-16 14:30")
// 2. Single units: "1d", "2h", "3w", "4m", "1M" (from now)
// 3. Combinations: "2d 1h", "1w 2d" (from now)
func ParseDeadline(input string) (*time.Time, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return nil, fmt.Errorf("deadline cannot be empty")
	}

	if t, err := time.ParseInLocation(deadlineInputLayout, input, time.Local); err == nil {
		return &t, nil
	}

	duration, err := ParseRelativeTime(input)
	if err != nil {
		return nil, fmt.Errorf("invalid deadline format: %v\nSupported formats:\n  - Absolute: YYYY-MM-DD HH:MM (e.g., 2025-11-16 14:30)\n  - Relative: 1d, 2h, 3w, 1M (e.g., 2d 3h 20m)", err)
	}

	deadline := time.Now().Add(duration)
	return &deadline, nil
}

// ParseRelativeTime parses a whitespace-separated sequence of "<number><unit>"
// tokens (e.g. "2M 1w 3d 4h 30m") into a duration from now.
func ParseRelativeTime(input string) (time.Duration, error) {
	rest := strings.TrimSpace(input)
	if rest == "" {
		return 0, fmt.Errorf("no valid time units found (use: m, h, d, w, M)")
	}

	months := 0
	var total time.Duration
	for tokens := 0; rest != ""; tokens++ {
		loc := relativeTokenRegex.FindStringSubmatchIndex(rest)
		if loc == nil {
			if tokens == 0 {
				return 0, fmt.Errorf("no valid time units found (use: m, h, d, w, M)")
			}
			return 0, fmt.Errorf("contains invalid characters or format")
		}

		value, err := strconv.Atoi(rest[loc[2]:loc[3]])
		if err != nil {
			return 0, fmt.Errorf("invalid number: %s", rest[loc[2]:loc[3]])
		}
		if value <= 0 {
			return 0, fmt.Errorf("time values must be positive")
		}

		unit := rest[loc[4]:loc[5]]
		if unit == "M" {
			if value > maxDeadlineMonths-months {
				return 0, fmt.Errorf("deadline too far in the future (max %d months)", maxDeadlineMonths)
			}
			months += value
		} else {
			unitDuration, err := ParseTimeUnit(value, strings.ToLower(unit))
			if err != nil {
				return 0, err
			}
			if unitDuration > maxRelativeDuration-total {
				return 0, fmt.Errorf("deadline too far in the future (max ~100 years)")
			}
			total += unitDuration
		}

		rest = strings.TrimLeft(rest[loc[1]:], " \t")
	}

	if months > 0 {
		now := time.Now()
		total += now.AddDate(0, months, 0).Sub(now)
	}

	if total <= 0 {
		return 0, fmt.Errorf("total duration must be positive")
	}

	return total, nil
}

// ParseTimeUnit converts value units of m, h, d or w into a duration. It
// rejects values that would overflow time.Duration.
func ParseTimeUnit(value int, unit string) (time.Duration, error) {
	var unitDuration time.Duration
	switch unit {
	case "m":
		unitDuration = time.Minute
	case "h":
		unitDuration = time.Hour
	case "d":
		unitDuration = 24 * time.Hour
	case "w":
		unitDuration = 7 * 24 * time.Hour
	default:
		return 0, fmt.Errorf("invalid time unit: %s (use: m, h, d, w, M)", unit)
	}
	if value < 0 || int64(value) > math.MaxInt64/int64(unitDuration) {
		return 0, fmt.Errorf("time value out of range: %d%s", value, unit)
	}
	return time.Duration(value) * unitDuration, nil
}
