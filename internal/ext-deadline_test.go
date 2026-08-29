package internal

import (
	"testing"
	"time"
)

func TestParseTimeUnit(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		value   int
		unit    string
		want    time.Duration
		wantErr bool
	}{
		{name: "minutes", value: 5, unit: "m", want: 5 * time.Minute},
		{name: "hours", value: 2, unit: "h", want: 2 * time.Hour},
		{name: "days", value: 3, unit: "d", want: 72 * time.Hour},
		{name: "weeks", value: 1, unit: "w", want: 7 * 24 * time.Hour},
		{name: "invalid unit", value: 1, unit: "x", wantErr: true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := ParseTimeUnit(tt.value, tt.unit)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ParseTimeUnit(%d, %q) expected error, got nil", tt.value, tt.unit)
				}
				return
			}

			if err != nil {
				t.Fatalf("ParseTimeUnit(%d, %q) unexpected error: %v", tt.value, tt.unit, err)
			}
			if got != tt.want {
				t.Fatalf("ParseTimeUnit(%d, %q) = %v, want %v", tt.value, tt.unit, got, tt.want)
			}
		})
	}
}

func TestParseRelativeTime(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    time.Duration
		wantErr bool
	}{
		{name: "minutes", input: "15m", want: 15 * time.Minute},
		{name: "hours and minutes", input: "2h 30m", want: 2*time.Hour + 30*time.Minute},
		{name: "days and hours", input: "1d 6h", want: 24*time.Hour + 6*time.Hour},
		{name: "weeks", input: "2w", want: 14 * 24 * time.Hour},
		{name: "months only", input: "1M"},
		{name: "combined months and units", input: "1M 2d 3h"},
		{name: "mixed spacing", input: "  2d\t3h 15m  "},
		{name: "empty", input: "", wantErr: true},
		{name: "invalid unit", input: "5x", wantErr: true},
		{name: "invalid characters", input: "2d+1h", wantErr: true},
		{name: "zero value", input: "0d", wantErr: true},
		{name: "negative-like format", input: "-1h", wantErr: true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := ParseRelativeTime(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ParseRelativeTime(%q) expected error, got nil", tt.input)
				}
				return
			}

			if err != nil {
				t.Fatalf("ParseRelativeTime(%q) unexpected error: %v", tt.input, err)
			}

			if tt.want > 0 && got != tt.want {
				t.Fatalf("ParseRelativeTime(%q) = %v, want %v", tt.input, got, tt.want)
			}
			if tt.want == 0 && got <= 0 {
				t.Fatalf("ParseRelativeTime(%q) = %v, want positive duration", tt.input, got)
			}
		})
	}
}

func TestParseDeadline(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name              string
		input             string
		wantErr           bool
		wantExact         bool
		relativeDuration  time.Duration // NEW: Store duration instead of absolute time
	}{
		{name: "absolute deadline", input: "2025-11-16 14:05", wantExact: true},
		{name: "relative deadline", input: "1h 30m", relativeDuration: 90 * time.Minute},
		{name: "trimmed input", input: " 2d ", relativeDuration: 48 * time.Hour},
		{name: "mixed spacing", input: "\t1h   15m ", relativeDuration: 75 * time.Minute},
		{name: "empty", input: "", wantErr: true},
		{name: "invalid format", input: "tomorrow", wantErr: true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			before := time.Now()
			got, err := ParseDeadline(tt.input)
			after := time.Now()

			if tt.wantErr {
				if err == nil {
					t.Fatalf("ParseDeadline(%q) expected error, got nil", tt.input)
				}
				return
			}

			if err != nil {
				t.Fatalf("ParseDeadline(%q) unexpected error: %v", tt.input, err)
			}
			if got == nil {
				t.Fatalf("ParseDeadline(%q) returned nil time", tt.input)
			}

			if tt.wantExact {
				want, parseErr := time.ParseInLocation("2006-01-02 15:05", tt.input, time.Local)
				if parseErr != nil {
					t.Fatalf("test setup parse failed: %v", parseErr)
				}
				if !got.Equal(want) {
					t.Fatalf("ParseDeadline(%q) = %v, want %v", tt.input, got, want)
				}
				return
			}

			// Calculate bounds dynamically using captured time
			if tt.relativeDuration > 0 {
				wantAfter := before.Add(tt.relativeDuration)
				wantBefore := after.Add(tt.relativeDuration + 2*time.Second)
				
				if got.Before(wantAfter) || got.After(wantBefore) {
					t.Fatalf("ParseDeadline(%q) = %v, want within [%v, %v]", tt.input, got, wantAfter, wantBefore)
				}
			}
		})
	}
}
