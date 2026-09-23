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
	"strings"
	"unicode"
	"unicode/utf8"
)

// ValidateTaskText enforces the shared title/description limits and rejects
// terminal control characters (for example ESC sequences) that could alter the
// terminal when the task is displayed. Descriptions may contain newlines and tabs.
func ValidateTaskText(title, description string) error {
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
	return unicode.IsControl(r)
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
