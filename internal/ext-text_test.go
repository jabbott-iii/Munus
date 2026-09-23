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
			err := ValidateTaskText(tc.title, tc.desc)
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
	if tasks, _ := db.ListTasks(); len(tasks) != 0 {
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
