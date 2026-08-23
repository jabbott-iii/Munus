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
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	monthRegex = regexp.MustCompile(`(\d+)M`)
	unitRegex  = regexp.MustCompile(`(\d+)([mhdw])`)
)

// ParseDeadline accepts multiple deadline formats:
// 1. Absolute: "YYYY-MM-DD HH:MM" (e.g., "2025-11-16 14:30")
// 2. Single units: "1d", "2h", "3w", "4m", "1M" (from now)
// 3. Combinations: "2d 1h", "1w 2d" (from now)
func ParseDeadline(input string) (*time.Time, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return nil, fmt.Errorf("deadline cannot be empty")
	}

	if t, err := time.ParseInLocation("2006-01-02 15:05", input, time.Local); err == nil {
		return &t, nil
	}

	duration, err := ParseRelativeTime(input)
	if err != nil {
		return nil, fmt.Errorf("invalid deadline format: %v\nSupported formats:\n  - Absolute: YYYY-MM-DD HH:MM (e.g., 2025-11-16 14:30)\n  - Relative: 1d, 2h, 3w, 1M (e.g., 2d 3h 20m)", err)
	}

	deadline := time.Now().Add(duration)
	return &deadline, nil
}

func ParseRelativeTime(input string) (time.Duration, error) {
	originalInput := input

	input = strings.ToLower(input)

	months := 0

	monthMatches := monthRegex.FindAllStringSubmatch(originalInput, -1)
	for _, match := range monthMatches {
		value, err := strconv.Atoi(match[1])
		if err != nil {
			return 0, fmt.Errorf("invalid number, %s", match[1])
		}
		if value <= 0 {
			return 0, fmt.Errorf("time values must be positive")
		}
		months += value
	}

	processedInput := monthRegex.ReplaceAllString(originalInput, "")
	processedInput = strings.ToLower(processedInput)

	matches := unitRegex.FindAllStringSubmatch(processedInput, -1)
	if len(matches) == 0 && months == 0 {
		return 0, fmt.Errorf("no valid time units found (use: m, h, d, w, M)")
	}

	var reconstructed strings.Builder
	for _, match := range matches {
		reconstructed.WriteString(match[0])
	}
	for i := 0; i < months; i++ {
		reconstructed.WriteString("M")
	}

	inputNoSpace := strings.ReplaceAll(strings.ReplaceAll(originalInput, " ", ""), "\t", "")
	inputNoSpace = strings.ToLower(inputNoSpace)
	for _, match := range monthMatches {
		inputNoSpace = strings.Replace(inputNoSpace, strings.ToLower(match[0]), "M", 1)
	}
	reconstructedNoSpaces := strings.ToLower(reconstructed.String())

	if len(reconstructedNoSpaces) != len(inputNoSpace) {
		return 0, fmt.Errorf("contains invalid characters or format")
	}

	var totalDuration time.Duration
	for _, match := range matches {
		value, err := strconv.Atoi(match[1])
		if err != nil {
			return 0, fmt.Errorf("invalid number: %s", match[1])
		}

		if value <= 0 {
			return 0, fmt.Errorf("time values must be positive")
		}

		unit := match[2]

		unitDuration, err := ParseTimeUnit(value, unit)
		if err != nil {
			return 0, err
		}
		totalDuration += unitDuration
	}

	if months > 0 {
		now := time.Now()
		targetTime := now.AddDate(0, months, 0)
		monthsDuration := targetTime.Sub(now)
		totalDuration += monthsDuration
	}

	if totalDuration <= 0 && months == 0 {
		return 0, fmt.Errorf("total duration must be positive")
	}

	return totalDuration, nil
}

func ParseTimeUnit(value int, unit string) (time.Duration, error) {
	switch unit {
	case "m":
		return time.Duration(value) * time.Minute, nil
	case "h":
		return time.Duration(value) * time.Hour, nil
	case "d":
		return time.Duration(value) * 24 * time.Hour, nil
	case "w":
		return time.Duration(value) * 7 * 24 * time.Hour, nil
	default:
		return 0, fmt.Errorf("invalid time unit: %s (use: m, h, d, w, M)", unit)
	}
}
