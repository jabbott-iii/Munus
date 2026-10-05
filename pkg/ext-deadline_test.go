package pkg

import (
	"fmt"
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
		{name: "largest in-range days", value: 106751, unit: "d", want: 106751 * 24 * time.Hour},
		{name: "days overflow rejected", value: 213504, unit: "d", wantErr: true},
		{name: "overflow rejected", value: 1 << 40, unit: "w", wantErr: true},
		{name: "negative rejected", value: -1, unit: "h", wantErr: true},
	}

	for _, tt := range tests {
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

// fixedNow is the clock for deterministic deadline tests (UTC has no DST).
func fixedNow() time.Time { return time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC) }

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
		{name: "no separator between tokens", input: "2d1h", want: 49 * time.Hour},
		{name: "uppercase hour unit", input: "2H", want: 2 * time.Hour},
		{name: "multiple months", input: "2M"},
		{name: "twelve months", input: "12M 1d"},
		{name: "repeated month tokens", input: "1M 1M"},
		{name: "trailing garbage", input: "1d abc", wantErr: true},
		{name: "number without unit", input: "5", wantErr: true},
		{name: "months beyond limit", input: "1201M", wantErr: true},
		{name: "huge month count", input: "99999999999M", wantErr: true},
		{name: "number too large to parse", input: "99999999999999999999d", wantErr: true},
		{name: "days beyond limit", input: "213504d", wantErr: true},
		{name: "cumulative units beyond limit", input: "5000w 5000w 5000w", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			deadline, err := relativeDeadline(tt.input, fixedNow())
			got := deadline.Sub(fixedNow())
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
		name             string
		input            string
		wantErr          bool
		wantExact        bool
		relativeDuration time.Duration // NEW: Store duration instead of absolute time
	}{
		{name: "absolute deadline", input: "2025-11-16 14:05", wantExact: true},
		{name: "absolute deadline keeps minutes", input: "2025-11-16 14:30", wantExact: true},
		{name: "absolute deadline invalid minute", input: "2025-11-16 14:75", wantErr: true},
		{name: "relative deadline", input: "1h 30m", relativeDuration: 90 * time.Minute},
		{name: "trimmed input", input: " 2d ", relativeDuration: 48 * time.Hour},
		{name: "mixed spacing", input: "\t1h   15m ", relativeDuration: 75 * time.Minute},
		{name: "empty", input: "", wantErr: true},
		{name: "invalid format", input: "tomorrow", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := parseDeadlineAt(tt.input, fixedNow())

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
				want, parseErr := time.ParseInLocation("2006-01-02 15:04", tt.input, fixedNow().Location())
				if parseErr != nil {
					t.Fatalf("test setup parse failed: %v", parseErr)
				}
				if !got.Equal(want) {
					t.Fatalf("ParseDeadline(%q) = %v, want %v", tt.input, got, want)
				}
				if got.Second() != 0 {
					t.Fatalf("ParseDeadline(%q) = %v, want zero seconds", tt.input, got)
				}
				return
			}

			if tt.relativeDuration > 0 {
				if want := fixedNow().Add(tt.relativeDuration); !got.Equal(want) {
					t.Fatalf("ParseDeadline(%q) = %v, want %v", tt.input, got, want)
				}
			}
		})
	}
}

func TestParseRelativeTimeMonthsMatchCalendar(t *testing.T) {
	start := time.Date(2026, 1, 31, 9, 0, 0, 0, time.UTC)
	for months, want := range map[int]time.Time{
		1:  time.Date(2026, 3, 3, 9, 0, 0, 0, time.UTC), // Go's AddDate normalises Feb 31
		2:  time.Date(2026, 3, 31, 9, 0, 0, 0, time.UTC),
		12: time.Date(2027, 1, 31, 9, 0, 0, 0, time.UTC),
	} {
		got, err := relativeDeadline(fmt.Sprintf("%dM", months), start)
		if err != nil || !got.Equal(want) {
			t.Fatalf("%dM from %v = %v, %v; want %v", months, start, got, err, want)
		}
	}
}

// P-040 / N-031 (D-6): days and weeks are calendar units, so a deadline keeps
// its clock time across a DST change; hours and minutes are elapsed time.
func TestRelativeDeadlineUsesCalendarDays(t *testing.T) {
	ny, err := time.LoadLocation("America/New_York") // embedded via time/tzdata
	if err != nil {
		t.Fatalf("load zone: %v", err)
	}
	cases := []struct {
		now   time.Time
		input string
		want  time.Time
		label string
	}{
		// Spring forward (2026-03-08): the day has 23 hours.
		{time.Date(2026, 3, 7, 23, 30, 0, 0, ny), "1d", time.Date(2026, 3, 8, 23, 30, 0, 0, ny), "Due tomorrow"},
		{time.Date(2026, 3, 7, 23, 30, 0, 0, ny), "24h", time.Date(2026, 3, 9, 0, 30, 0, 0, ny), "2 days left"},
		{time.Date(2026, 3, 7, 23, 30, 0, 0, ny), "1d 2h", time.Date(2026, 3, 9, 1, 30, 0, 0, ny), "2 days left"},
		// Fall back (2026-11-01): the day has 25 hours.
		{time.Date(2026, 11, 1, 0, 30, 0, 0, ny), "1d", time.Date(2026, 11, 2, 0, 30, 0, 0, ny), "Due tomorrow"},
		{time.Date(2026, 10, 31, 9, 0, 0, 0, ny), "1w", time.Date(2026, 11, 7, 9, 0, 0, 0, ny), ""},
	}
	for _, tc := range cases {
		got, err := parseDeadlineAt(tc.input, tc.now)
		if err != nil || !got.Equal(tc.want) {
			t.Errorf("%q at %v = %v, %v; want %v", tc.input, tc.now, got, err, tc.want)
			continue
		}
		if tc.label != "" {
			if label, _ := deadlineLabel(*got, tc.now); label != tc.label {
				t.Errorf("%q at %v: label %q, want %q", tc.input, tc.now, label, tc.label)
			}
		}
	}
}

// The exported entry points count from the real clock.
func TestParseDeadlineUsesCurrentTime(t *testing.T) {
	before := time.Now()
	got, err := ParseDeadline("1d")
	after := time.Now()
	if err != nil {
		t.Fatalf("ParseDeadline failed: %v", err)
	}
	if got.Before(before.AddDate(0, 0, 1)) || got.After(after.AddDate(0, 0, 1)) {
		t.Fatalf("ParseDeadline(1d) = %v, want the same clock time tomorrow (%v)", got, before.AddDate(0, 0, 1))
	}
	if d, err := ParseRelativeTime("90m"); err != nil || d < 90*time.Minute-time.Second || d > 90*time.Minute {
		t.Fatalf("ParseRelativeTime(90m) = %v, %v", d, err)
	}
}

// P-043: deadline parsing never panics, and accepted relative deadlines stay
// within the documented bounds (~100 years of m/h/d/w plus 1200 months).
func FuzzParseDeadline(f *testing.F) {
	for _, s := range []string{"1d", "2M 1w 3d 4h 30m", "2026-01-02 03:04", "99999999999M", "1200M", "0d", "1m1m1m", "  5h  ", "1M 1M", "106751d"} {
		f.Add(s)
	}
	limit := maxRelativeDuration + time.Duration(maxDeadlineMonths)*31*24*time.Hour
	f.Fuzz(func(t *testing.T, s string) {
		d, err := ParseDeadline(s)
		if err == nil && d == nil {
			t.Fatalf("nil deadline without an error for %q", s)
		}
		if dur, err := ParseRelativeTime(s); err == nil && (dur <= 0 || dur > limit) {
			t.Fatalf("relative duration out of bounds for %q: %v", s, dur)
		}
	})
}
