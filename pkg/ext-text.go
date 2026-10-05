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
	"io"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Tag limits.
const (
	maxTagLength   = 32
	maxTagsPerTask = 10
)

// untitledTaskTitle replaces an imported title that is blank once control
// characters are removed, so exports and backups that contain such a task
// still restore (default import only; --strict rejects it).
const untitledTaskTitle = "(untitled)"

// shortenToLimit returns s cut to at most limit bytes at a character (rune)
// boundary, and whether it was cut. s must be valid UTF-8.
func shortenToLimit(s string, limit int) (string, bool) {
	if len(s) <= limit {
		return s, false
	}
	cut := limit
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut], true
}

// isBlank reports whether s is empty or contains only whitespace.
func isBlank(s string) bool {
	return strings.TrimSpace(s) == ""
}

// validateTaskText enforces the shared title/description limits and rejects
// terminal control characters (for example ESC sequences) that could alter the
// terminal when the task is displayed. Descriptions may contain newlines and tabs.
func validateTaskText(title, description string) error {
	if len(title) > MaxTitleLength {
		return fmt.Errorf("title exceeds maximum length of %d", MaxTitleLength)
	}
	if len(description) > MaxDescriptionLength {
		return fmt.Errorf("description exceeds maximum length of %d", MaxDescriptionLength)
	}
	if !utf8.ValidString(title) || !utf8.ValidString(description) {
		return fmt.Errorf("task text must be valid UTF-8")
	}
	if containsControl(title, false) {
		return fmt.Errorf("title contains control characters")
	}
	if containsControl(description, true) {
		return fmt.Errorf("description contains control characters")
	}
	return nil
}

func containsControl(s string, allowLineBreaks bool) bool {
	for _, r := range s {
		if isDisallowedControl(r, allowLineBreaks) {
			return true
		}
	}
	return false
}

func isDisallowedControl(r rune, allowLineBreaks bool) bool {
	if allowLineBreaks && (r == '\n' || r == '\t') {
		return false
	}
	return unicode.IsControl(r) || isBidiControl(r)
}

// isBidiControl reports whether r is a bidirectional embedding, override or
// isolate control (U+202A–U+202E, U+2066–U+2069). They can make displayed text
// read differently from what is stored, so they are treated like control
// characters. Other format characters, such as the U+200D joiner used in emoji
// sequences, are allowed.
func isBidiControl(r rune) bool {
	return (r >= '\u202a' && r <= '\u202e') || (r >= '\u2066' && r <= '\u2069')
}

// sanitizeForTerminal replaces control characters with U+FFFD so that stored
// text (including data saved before validation existed) cannot emit terminal
// escape sequences when rendered.
func sanitizeForTerminal(s string, allowLineBreaks bool) string {
	s = strings.ToValidUTF8(s, string(unicode.ReplacementChar))
	if !containsControl(s, allowLineBreaks) {
		return s
	}
	return strings.Map(func(r rune) rune {
		if isDisallowedControl(r, allowLineBreaks) {
			return unicode.ReplacementChar
		}
		return r
	}, s)
}

// terminalSafeWriter passes everything written to it through
// sanitizeForTerminal (line breaks and tabs kept). The root command writes its
// errors through it, so error text that carries stored or imported data, such
// as a message raised by a SQLite trigger in a crafted database, cannot emit
// terminal escape sequences.
type terminalSafeWriter struct {
	w io.Writer
}

func (s terminalSafeWriter) Write(p []byte) (int, error) {
	if _, err := io.WriteString(s.w, sanitizeForTerminal(string(p), true)); err != nil {
		return 0, err
	}
	return len(p), nil
}

// stripControlCharacters removes control characters from imported text so that
// exports and backups of older data (for example CRLF line endings) can still be
// restored. Line breaks are normalised to "\n" where they are allowed.
func stripControlCharacters(s string, allowLineBreaks bool) string {
	s = strings.ToValidUTF8(s, string(unicode.ReplacementChar))
	if allowLineBreaks {
		s = strings.ReplaceAll(s, "\r\n", "\n")
	}
	if !containsControl(s, allowLineBreaks) {
		return s
	}
	return strings.Map(func(r rune) rune {
		if isDisallowedControl(r, allowLineBreaks) {
			return -1
		}
		return r
	}, s)
}

// normalizeTags trims, lowercases, de-duplicates and sorts tag names and
// validates them: 1–32 characters of letters, digits, '-' or '_', at most 10.
func normalizeTags(tags []string) ([]string, error) {
	if len(tags) == 0 {
		return nil, nil
	}
	seen := make(map[string]struct{}, len(tags))
	out := make([]string, 0, len(tags))
	for _, raw := range tags {
		tag := strings.ToLower(strings.TrimSpace(raw))
		if err := validateTag(tag); err != nil {
			return nil, err
		}
		if _, ok := seen[tag]; ok {
			continue
		}
		seen[tag] = struct{}{}
		out = append(out, tag)
	}
	if len(out) > maxTagsPerTask {
		return nil, fmt.Errorf("too many tags: %d (maximum %d)", len(out), maxTagsPerTask)
	}
	sort.Strings(out)
	return out, nil
}

func validateTag(tag string) error {
	if tag == "" {
		return fmt.Errorf("tag must not be empty")
	}
	if !utf8.ValidString(tag) {
		return fmt.Errorf("tag must be valid UTF-8")
	}
	if n := utf8.RuneCountInString(tag); n > maxTagLength {
		return fmt.Errorf("tag %q exceeds maximum length of %d", tag, maxTagLength)
	}
	for _, r := range tag {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '-' && r != '_' {
			return fmt.Errorf("tag %q may only contain letters, digits, '-' and '_'", tag)
		}
	}
	return nil
}
