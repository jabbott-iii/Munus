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

package pkg

import (
	"bytes"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"
)

func TestValidateTaskText(t *testing.T) {
	cases := []struct {
		name        string
		title, desc string
		wantErr     string
	}{
		{name: "plain", title: "Title", desc: "Description"},
		{name: "unicode", title: "Café ✓ 日本", desc: "naïve"},
		{name: "multiline description", title: "T", desc: "line 1\n\tline 2"},
		{name: "escape in title", title: "a\x1b[2Jb", wantErr: "title contains control characters"},
		{name: "newline in title", title: "a\nb", wantErr: "title contains control characters"},
		{name: "bell in description", title: "T", desc: "a\x07", wantErr: "description contains control characters"},
		{name: "C1 control in description", title: "T", desc: "a\u009bb", wantErr: "description contains control characters"},
		{name: "DEL in title", title: "a\x7f", wantErr: "title contains control characters"},
		{name: "title too long", title: strings.Repeat("a", MaxTitleLength+1), wantErr: "title exceeds maximum length"},
		{name: "description too long", title: "T", desc: strings.Repeat("a", MaxDescriptionLength+1), wantErr: "description exceeds maximum length"},
		{name: "invalid UTF-8 (raw C1 CSI byte)", title: "A\x9b31mRED", wantErr: "valid UTF-8"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateTaskText(tc.title, tc.desc)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("expected error containing %q, got %v", tc.wantErr, err)
			}
		})
	}
}

func TestSanitizeForTerminal(t *testing.T) {
	if got := sanitizeForTerminal("safe text", false); got != "safe text" {
		t.Fatalf("expected unchanged text, got %q", got)
	}
	if got := sanitizeForTerminal("a\x1b]0;x\x07b", false); strings.ContainsAny(got, "\x1b\x07") {
		t.Fatalf("expected control characters removed, got %q", got)
	}
	if got := sanitizeForTerminal("a\nb", false); got != "a�b" {
		t.Fatalf("expected newline replaced in single-line text, got %q", got)
	}
	if got := sanitizeForTerminal("a\n\tb", true); got != "a\n\tb" {
		t.Fatalf("expected newline/tab kept in multi-line text, got %q", got)
	}
	if got := sanitizeForTerminal("A\x9b31mRED", false); strings.Contains(got, "\x9b") || !utf8.ValidString(got) {
		t.Fatalf("expected invalid UTF-8 byte replaced, got %q", got)
	}
}

func TestPrintListSanitizesStoredText(t *testing.T) {
	var buf bytes.Buffer
	PrintList(&buf, []*ItemModel{{ID: 1, Title: "evil\x1b]0;PWNED\x07", Description: "x\x1b[2J"}})
	if bytes.ContainsAny(buf.Bytes(), "\x1b\x07") {
		t.Fatalf("expected no raw control characters in list output, got %q", buf.String())
	}
}

func TestRenderTaskSanitizesStoredText(t *testing.T) {
	list := NewListModel(&MockStorage{})
	task := &ItemModel{ID: 7, Title: "evil\x1b]0;PWNED\x07", Description: "d\x1b[2J"}
	list.expanded[task.ID] = true
	plain := lipgloss.NewStyle()
	out := list.RenderTask(task, 0, false, plain, plain, plain, plain, plain, plain)
	if strings.Contains(out, "PWNED\x07") || strings.Contains(out, "\x1b]0;") || strings.Contains(out, "\x1b[2J") {
		t.Fatalf("expected sanitized render, got %q", out)
	}
}

func TestAddCmdRejectsControlCharacters(t *testing.T) {
	db := NewMockModel()
	cmd := NewAddCmd(db)
	cmd.SetArgs([]string{"-t", "bad\x1b[2J", "-d", "desc"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "control characters") {
		t.Fatalf("expected control character error, got %v", err)
	}
	if tasks, _ := db.ListTasks(t.Context()); len(tasks) != 0 {
		t.Fatalf("expected no task to be created, got %d", len(tasks))
	}
}

func TestSubmitFormRejectsControlCharacters(t *testing.T) {
	storage := &MockStorage{}
	form := NewFormModel(storage)
	form.fields[titleField] = "bad\x07"
	form.fields[descriptionField] = "desc"
	if err := form.submitForm(); err == nil || !strings.Contains(err.Error(), "control characters") {
		t.Fatalf("expected control character error, got %v", err)
	}
	if len(storage.tasks) != 0 {
		t.Fatalf("expected no task to be created")
	}
}

func TestDeleteDialogSanitizesTitle(t *testing.T) {
	list := NewListModel(&MockStorage{})
	list.loading = false
	list.confirmingDelete = true
	list.taskToDelete = &ItemModel{ID: 1, Title: "evil\x1b]0;PWNED\x07"}
	if view := list.View(); strings.Contains(view, "\x1b]0;PWNED") {
		t.Fatalf("expected delete dialog title to be sanitized, got %q", view)
	}
}

func TestNormalizeTags(t *testing.T) {
	cases := []struct {
		name    string
		in      []string
		want    []string
		wantErr string
	}{
		{name: "nil", in: nil, want: nil},
		{name: "trim, lowercase, dedupe, sort", in: []string{" Work", "home", "WORK", "a-b_c"}, want: []string{"a-b_c", "home", "work"}},
		{name: "unicode letters", in: []string{"café", "日本"}, want: []string{"café", "日本"}},
		{name: "empty", in: []string{" "}, wantErr: "must not be empty"},
		{name: "space inside", in: []string{"two words"}, wantErr: "letters, digits"},
		{name: "punctuation", in: []string{"a,b"}, wantErr: "letters, digits"},
		{name: "control character", in: []string{"a\x1b"}, wantErr: "letters, digits"},
		{name: "too long", in: []string{strings.Repeat("x", maxTagLength+1)}, wantErr: "maximum length"},
		{name: "max length ok", in: []string{strings.Repeat("é", maxTagLength)}, want: []string{strings.Repeat("é", maxTagLength)}},
		{name: "too many", in: []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k"}, wantErr: "too many tags"},
		{name: "duplicates do not count twice", in: []string{"a", "A", "b", "c", "d", "e", "f", "g", "h", "i", "j"}, want: []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := normalizeTags(tc.in)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("expected error containing %q, got %v", tc.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if strings.Join(got, ",") != strings.Join(tc.want, ",") {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}

// ============================== plan 3: text rules ==============================

// P-034 / SEC-013 (D-7): bidi embedding, override and isolate controls are
// treated as control characters; other format characters are allowed.
func TestBidiControlsAreControlCharacters(t *testing.T) {
	for _, r := range []rune{'\u202a', '\u202b', '\u202c', '\u202d', '\u202e', '\u2066', '\u2067', '\u2068', '\u2069'} {
		s := "a" + string(r) + "b"
		if err := validateTaskText(s, ""); err == nil {
			t.Errorf("%U: expected validation to reject it", r)
		}
		if got := sanitizeForTerminal(s, true); got != "a�b" {
			t.Errorf("%U: sanitizeForTerminal = %q", r, got)
		}
		if got := stripControlCharacters(s, true); got != "ab" {
			t.Errorf("%U: stripControlCharacters = %q", r, got)
		}
	}
	family := "👨\u200d👩\u200d👧 \u200bzero-width \u200e"
	if err := validateTaskText(family, family); err != nil {
		t.Errorf("joiners and marks must stay allowed: %v", err)
	}
	if got := sanitizeForTerminal(family, false); got != family {
		t.Errorf("sanitizeForTerminal changed allowed text: %q", got)
	}
}

func TestTerminalSafeWriter(t *testing.T) {
	var buf bytes.Buffer
	w := terminalSafeWriter{w: &buf}
	in := "Error: \x1b]0;PWNED\x07 \u202e\n\tnext\n"
	n, err := w.Write([]byte(in))
	if err != nil || n != len(in) {
		t.Fatalf("Write = %d, %v; want %d, nil", n, err, len(in))
	}
	if want := "Error: �]0;PWNED� �\n\tnext\n"; buf.String() != want {
		t.Fatalf("got %q, want %q", buf.String(), want)
	}
}

// P-043: sanitised text never contains a disallowed control character, is
// valid UTF-8 and is stable when sanitised again.
func FuzzSanitizeForTerminal(f *testing.F) {
	for _, s := range []string{"a\x1b]0;x\x07b", "\xff\xfe", "line\nbreak\ttab\r", "\u009b31m", "\u202eevil", "👨\u200d👩"} {
		f.Add(s, true)
		f.Add(s, false)
	}
	f.Fuzz(func(t *testing.T, s string, lineBreaks bool) {
		out := sanitizeForTerminal(s, lineBreaks)
		if !utf8.ValidString(out) {
			t.Fatalf("invalid UTF-8 output for %q", s)
		}
		for _, r := range out {
			if isDisallowedControl(r, lineBreaks) {
				t.Fatalf("control %U survived for %q", r, s)
			}
		}
		if again := sanitizeForTerminal(out, lineBreaks); again != out {
			t.Fatalf("not stable for %q: %q then %q", s, out, again)
		}
		if stripped := stripControlCharacters(s, lineBreaks); containsControl(stripped, lineBreaks) {
			t.Fatalf("stripControlCharacters left a control character in %q", stripped)
		}
	})
}

// P-051: text is cut at a character boundary, never inside a UTF-8 sequence.
func TestShortenToLimit(t *testing.T) {
	cases := []struct {
		in    string
		limit int
		want  string
		cut   bool
	}{
		{"abc", 3, "abc", false},
		{"abcd", 3, "abc", true},
		{strings.Repeat("é", 60), 100, strings.Repeat("é", 50), true}, // 2-byte runes, cut on a boundary
		{strings.Repeat("€", 40), 100, strings.Repeat("€", 33), true}, // 3-byte runes: 99 bytes
		{strings.Repeat("😀", 30), 100, strings.Repeat("😀", 25), true}, // 4-byte runes
		{"a😀", 3, "a", true},
		{"😀", 0, "", true},
	}
	for _, tc := range cases {
		got, cut := shortenToLimit(tc.in, tc.limit)
		if got != tc.want || cut != tc.cut || !utf8.ValidString(got) || len(got) > tc.limit {
			t.Errorf("shortenToLimit(%q, %d) = %q, %v; want %q, %v", tc.in, tc.limit, got, cut, tc.want, tc.cut)
		}
	}
}
