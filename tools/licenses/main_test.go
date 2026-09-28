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

package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func newModule(path, dir string, targets ...platform) *module {
	m := &module{path: path, dir: dir, pkgDirs: map[string]struct{}{dir: {}}, platforms: map[platform]struct{}{}}
	for _, p := range targets {
		m.platforms[p] = struct{}{}
	}
	return m
}

// testDockerfile mirrors the stages of the repository's Dockerfile on the
// Alpine release whose musl COPYRIGHT is embedded.
var testDockerfile = "FROM golang:1.26-alpine" + muslAlpineRelease + " AS source\nFROM source AS builder\nFROM source AS static-builder\nFROM scratch AS static\nFROM alpine:" + muslAlpineRelease + "\n"

func TestNormalizeText(t *testing.T) {
	tests := []struct {
		name, in, want string
	}{
		{"crlf and trailing spaces", "\r\n\r\nLine one  \r\nLine two\t\r\n\r\n", "Line one\nLine two\n"},
		{"lone carriage returns", "a\rb\r", "a\nb\n"},
		{"byte order mark", "\ufeffMIT License\n", "MIT License\n"},
		{"inner blank lines kept", "a\n\n\nb", "a\n\n\nb\n"},
		{"leading indentation kept", "   * item\n", "   * item\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeText([]byte(tt.in))
			if err != nil {
				t.Fatalf("normalizeText: %v", err)
			}
			if got != tt.want {
				t.Fatalf("normalizeText(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
	for _, bad := range []string{"", " \r\n\t\n", "\xff\xfe"} {
		if _, err := normalizeText([]byte(bad)); err == nil {
			t.Errorf("normalizeText(%q) succeeded, want an error", bad)
		}
	}
}

func TestReadLicenseFiles(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "LICENSE.txt"), "MIT text\r\n")
	writeFile(t, filepath.Join(dir, "COPYING"), "copying text\n")
	writeFile(t, filepath.Join(dir, "PATENTS"), "patents text\n")
	writeFile(t, filepath.Join(dir, "notice.go"), "package x\n")
	writeFile(t, filepath.Join(dir, "README.md"), "readme\n")
	writeFile(t, filepath.Join(dir, "LICENSES", "inner"), "not read\n")

	texts, err := readLicenseFiles(dir, "sub/")
	if err != nil {
		t.Fatalf("readLicenseFiles: %v", err)
	}
	var names []string
	for _, lt := range texts {
		names = append(names, lt.name)
	}
	if got, want := strings.Join(names, ","), "sub/COPYING,sub/LICENSE.txt,sub/PATENTS"; got != want {
		t.Fatalf("names = %s, want %s", got, want)
	}
	if texts[1].text != "MIT text\n" {
		t.Fatalf("LICENSE.txt text = %q, want normalised text", texts[1].text)
	}

	writeFile(t, filepath.Join(dir, "LICENSE"), "\xff\n")
	if _, err := readLicenseFiles(dir, ""); err == nil || !strings.Contains(err.Error(), "UTF-8") {
		t.Fatalf("readLicenseFiles with invalid UTF-8: err = %v, want a UTF-8 error", err)
	}
	if _, err := readLicenseFiles(filepath.Join(dir, "missing"), ""); err == nil {
		t.Fatal("readLicenseFiles of a missing directory succeeded")
	}
}

func TestModuleTexts(t *testing.T) {
	t.Run("root and package license files", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, "LICENSE"), "root license\n")
		writeFile(t, filepath.Join(dir, "internal", "vendored", "LICENSE"), "vendored license\n")
		writeFile(t, filepath.Join(dir, "plain", "plain.go"), "package plain\n")
		m := newModule("example.com/mod", dir)
		m.pkgDirs[filepath.Join(dir, "internal", "vendored")] = struct{}{}
		m.pkgDirs[filepath.Join(dir, "plain")] = struct{}{}
		texts, err := moduleTexts(m)
		if err != nil {
			t.Fatalf("moduleTexts: %v", err)
		}
		if len(texts) != 2 || texts[0].name != "LICENSE" || texts[1].name != "internal/vendored/LICENSE" {
			t.Fatalf("texts = %+v, want the root and the package license", texts)
		}
	})

	t.Run("no license file", func(t *testing.T) {
		m := newModule("example.com/unlicensed", t.TempDir())
		if _, err := moduleTexts(m); err == nil || !strings.Contains(err.Error(), "has no license file") {
			t.Fatalf("moduleTexts: err = %v, want a missing-license error", err)
		}
	})

	const localereader = "github.com/mattn/go-localereader"
	rl := readmeLicenses()[localereader]
	t.Run("reviewed README statement", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, "README.md"), "# go-localereader\r\n\r\n"+strings.ReplaceAll(rl.excerpt, "\n", "\r\n")+"\r\n")
		texts, err := moduleTexts(newModule(localereader, dir))
		if err != nil {
			t.Fatalf("moduleTexts: %v", err)
		}
		if len(texts) != 2 || !strings.Contains(texts[0].text, "    MIT\n") || !strings.HasPrefix(texts[1].text, "Permission is hereby granted") {
			t.Fatalf("texts = %+v, want the README statement and the MIT terms", texts)
		}
	})

	t.Run("changed README statement", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, "README.md"), "# go-localereader\n\n## License\n\nGPL\n")
		if _, err := moduleTexts(newModule(localereader, dir)); err == nil || !strings.Contains(err.Error(), "no longer contains") {
			t.Fatalf("moduleTexts: err = %v, want a changed-statement error", err)
		}
	})

	t.Run("missing README", func(t *testing.T) {
		if _, err := moduleTexts(newModule(localereader, t.TempDir())); err == nil {
			t.Fatal("moduleTexts without README succeeded")
		}
	})
}

func TestSQLiteNotice(t *testing.T) {
	header := "#ifndef USE_LIBSQLITE3\r\n/*\r\n** 2001-09-15\r\n**\r\n** " + sqliteDisclaimer + "  In place of\r\n** a legal notice, here is a blessing:\r\n**\r\n**    May you do good and not evil.\r\n**\r\n*************\r\n** API docs\r\n"
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, sqliteHeader), header)
	got, err := sqliteNotice(dir)
	if err != nil {
		t.Fatalf("sqliteNotice: %v", err)
	}
	want := "2001-09-15\n\n" + sqliteDisclaimer + "  In place of\na legal notice, here is a blessing:\n\n   May you do good and not evil.\n"
	if got != want {
		t.Fatalf("sqliteNotice = %q, want %q", got, want)
	}

	writeFile(t, filepath.Join(dir, sqliteHeader), "/*\n** Copyright someone\n*****\n")
	if _, err := sqliteNotice(dir); err == nil || !strings.Contains(err.Error(), "review the SQLite license") {
		t.Fatalf("sqliteNotice without the dedication: err = %v", err)
	}
	writeFile(t, filepath.Join(dir, sqliteHeader), "int x;\n")
	if _, err := sqliteNotice(dir); err == nil || !strings.Contains(err.Error(), "no header comment") {
		t.Fatalf("sqliteNotice without a comment: err = %v", err)
	}
	if _, err := sqliteNotice(t.TempDir()); err == nil {
		t.Fatal("sqliteNotice without the header succeeded")
	}
}

func TestCheckDockerfile(t *testing.T) {
	tests := []struct {
		name, content, wantErr string
	}{
		{"same release", testDockerfile, ""},
		{"platform flag", "FROM --platform=$BUILDPLATFORM golang:1.26-alpine" + muslAlpineRelease + " AS b\n", ""},
		{"other builder release", strings.Replace(testDockerfile, "alpine"+muslAlpineRelease, "alpine0.1", 1), "uses Alpine 0.1"},
		{"other runtime release", strings.Replace(testDockerfile, "alpine:"+muslAlpineRelease, "alpine:0.1", 1), "uses Alpine 0.1"},
		{"no alpine", "FROM debian:12\n", "no Alpine image"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, dockerfile), tt.content)
			err := checkDockerfile(root)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("checkDockerfile: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("checkDockerfile: err = %v, want %q", err, tt.wantErr)
			}
		})
	}
	if err := checkDockerfile(t.TempDir()); err == nil {
		t.Fatal("checkDockerfile without a Dockerfile succeeded")
	}
}

func TestIncludedIn(t *testing.T) {
	all := releasePlatforms()
	set := func(ps ...platform) map[platform]struct{} {
		m := map[platform]struct{}{}
		for _, p := range ps {
			m[p] = struct{}{}
		}
		return m
	}
	tests := []struct {
		name string
		set  map[platform]struct{}
		want string
	}{
		{"all", set(all...), "all binaries"},
		{"windows", set(platform{"windows", "amd64"}, platform{"windows", "arm64"}), "Windows binaries"},
		{"mixed", set(platform{"linux", "arm64"}, platform{"darwin", "amd64"}, platform{"darwin", "arm64"}), "linux/arm64, macOS binaries"},
	}
	for _, tt := range tests {
		if got := includedIn(tt.set, all); got != tt.want {
			t.Errorf("%s: includedIn = %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestAddPackages(t *testing.T) {
	linux := platform{"linux", "amd64"}
	stream := `{"Dir":"/goroot/src/fmt","Standard":true}
{"Dir":"/repo","Module":{"Path":"github.com/jabbott-iii/Munus","Main":true,"Dir":"/repo"}}
{"Dir":"/mod/a@v1/pkg","Module":{"Path":"example.com/a","Dir":"/mod/a@v1"}}
{"Dir":"/mod/a@v1/other","Module":{"Path":"example.com/a","Dir":"/mod/a@v1"}}
{"Dir":"/local/b/pkg","Module":{"Path":"example.com/b","Dir":"/mod/b@v1","Replace":{"Path":"../b","Dir":"/local/b"}}}
`
	byPath := map[string]*module{}
	if err := addPackages(byPath, strings.NewReader(stream), linux); err != nil {
		t.Fatalf("addPackages: %v", err)
	}
	if len(byPath) != 2 {
		t.Fatalf("modules = %v, want example.com/a and example.com/b only", byPath)
	}
	if a := byPath["example.com/a"]; a.dir != "/mod/a@v1" || len(a.pkgDirs) != 2 {
		t.Fatalf("example.com/a = %+v", a)
	}
	if b := byPath["example.com/b"]; b.dir != "/local/b" {
		t.Fatalf("replaced module dir = %q, want the replacement's", b.dir)
	}
	if err := addPackages(byPath, strings.NewReader(stream), platform{"windows", "arm64"}); err != nil {
		t.Fatalf("addPackages: %v", err)
	}
	if n := len(byPath["example.com/a"].platforms); n != 2 {
		t.Fatalf("platforms = %d, want 2", n)
	}

	if err := addPackages(map[string]*module{}, strings.NewReader(`{"Dir":"/x","Module":{"Path":"example.com/c"}}`), linux); err == nil || !strings.Contains(err.Error(), "go mod download") {
		t.Fatalf("module without dir: err = %v", err)
	}
	if err := addPackages(map[string]*module{}, strings.NewReader(`{"Dir":`), linux); err == nil {
		t.Fatal("truncated go list output was accepted")
	}
}

func TestNoticeProblems(t *testing.T) {
	notice := "Munus\r\n\r\nDirect dependencies:\r\n  - example.com/a Someone MIT License\r\n  - example.com/gone Someone MIT License\r\n\r\nAlso:\r\n  - SQLite, embedded in example.com/a, public domain\r\n"
	mods := []*module{newModule("example.com/a", "/a"), newModule("example.com/b", "/b"), newModule("example.com/a/v2", "/a2")}
	got := strings.Join(noticeProblems(notice, mods), "\n")
	want := "does not list example.com/b\ndoes not list example.com/a/v2\nlists example.com/gone, which no release binary includes"
	if got != want {
		t.Fatalf("noticeProblems =\n%s\nwant\n%s", got, want)
	}
	if p := noticeProblems("  - example.com/a\n", mods[:1]); len(p) != 0 {
		t.Fatalf("noticeProblems for a complete NOTICE = %v", p)
	}
}

func TestFirstDifference(t *testing.T) {
	if _, differs := firstDifference("a\nb\n", "a\nb\n"); differs {
		t.Fatal("equal texts differ")
	}
	if line, _ := firstDifference("a\nb\n", "a\nc\n"); line != 2 {
		t.Fatalf("line = %d, want 2", line)
	}
	if line, _ := firstDifference("a\n", "a\nb\n"); line != 2 {
		t.Fatalf("line = %d, want 2 for a longer text", line)
	}
}

// fakeRepository builds a repository root, GOROOT and two modules (one of
// them go-sqlite3) and returns them.
func fakeRepository(t *testing.T) (root, goroot string, mods []*module) {
	t.Helper()
	root = t.TempDir()
	writeFile(t, filepath.Join(root, dockerfile), testDockerfile)
	writeFile(t, filepath.Join(root, noticeFile), "  - "+sqliteModule+" Someone MIT License\r\n  - example.com/win Someone MIT License\r\n")
	goroot = t.TempDir()
	writeFile(t, filepath.Join(goroot, "LICENSE"), "Go license\n")
	writeFile(t, filepath.Join(goroot, "PATENTS"), "Go patents\n")
	sqliteDir := t.TempDir()
	writeFile(t, filepath.Join(sqliteDir, "LICENSE"), "sqlite3 driver license\n")
	writeFile(t, filepath.Join(sqliteDir, sqliteHeader), "/*\n** "+sqliteDisclaimer+"\n*****\n")
	winDir := t.TempDir()
	writeFile(t, filepath.Join(winDir, "LICENSE"), "windows module license\n")
	all := releasePlatforms()
	mods = []*module{
		newModule(sqliteModule, sqliteDir, all...),
		newModule("example.com/win", winDir, platform{"windows", "amd64"}, platform{"windows", "arm64"}),
	}
	return root, goroot, mods
}

func TestGenerate(t *testing.T) {
	root, goroot, mods := fakeRepository(t)
	got, err := generate(root, goroot, mods)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	again, err := generate(root, goroot, mods)
	if err != nil || again != got {
		t.Fatalf("generate is not deterministic (err = %v)", err)
	}
	order := []string{
		"Contents:\n  - Go standard library and runtime\n  - SQLite (compiled into " + sqliteModule + ")\n  - musl libc " + muslVersion + "\n  - " + sqliteModule + "\n  - example.com/win\n",
		"Go standard library and runtime\nIncluded in: all binaries\n",
		"--- LICENSE ---\n\nGo license\n",
		"--- PATENTS ---\n\nGo patents\n",
		"--- " + sqliteHeader + ", header comment ---\n\n" + sqliteDisclaimer + "\n",
		"musl libc " + muslVersion + "\nIncluded in: Linux release binaries",
		"--- COPYRIGHT ---\n\nmusl as a whole is licensed under the following standard MIT license:",
		sqliteModule + "\nIncluded in: all binaries\n",
		"sqlite3 driver license\n",
		"example.com/win\nIncluded in: Windows binaries\n",
		"windows module license\n",
	}
	pos := 0
	for _, part := range order {
		i := strings.Index(got[pos:], part)
		if i < 0 {
			t.Fatalf("output lacks %q after offset %d:\n%s", part, pos, got)
		}
		pos += i + len(part)
	}
	if strings.Contains(got, "\r") || !strings.HasSuffix(got, "\n") || strings.HasSuffix(got, "\n\n") {
		t.Fatal("output must use LF line endings and end with a single newline")
	}

	writeFile(t, filepath.Join(root, dockerfile), "FROM alpine:0.1\n")
	if _, err := generate(root, goroot, mods); err == nil {
		t.Fatal("generate accepted a Dockerfile on another Alpine release")
	}
	writeFile(t, filepath.Join(root, dockerfile), testDockerfile)
	if _, err := generate(root, t.TempDir(), mods); err == nil || !strings.Contains(err.Error(), "no license file found in GOROOT") {
		t.Fatalf("generate with an empty GOROOT: err = %v", err)
	}
	unlicensed := append(mods, newModule("example.com/none", t.TempDir()))
	if _, err := generate(root, goroot, unlicensed); err == nil {
		t.Fatal("generate accepted a module without a license")
	}
}

func TestFinish(t *testing.T) {
	root, goroot, mods := fakeRepository(t)
	content, err := generate(root, goroot, mods)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	out := filepath.Join(root, outputFile)

	var stdout, stderr bytes.Buffer
	if code := finish(root, true, content, mods, &stdout, &stderr); code != 1 || !strings.Contains(stderr.String(), outputFile) {
		t.Fatalf("check without %s: code %d, stderr %q", outputFile, code, stderr.String())
	}

	stderr.Reset()
	if code := finish(root, false, content, mods, &stdout, &stderr); code != 0 {
		t.Fatalf("write: code %d, stderr %q", code, stderr.String())
	}
	if data, err := os.ReadFile(out); err != nil || string(data) != content {
		t.Fatalf("written file differs (err = %v)", err)
	}
	if code := finish(root, true, content, mods, &stdout, &stderr); code != 0 {
		t.Fatalf("check of the written file: code %d, stderr %q", code, stderr.String())
	}

	// A Windows checkout converts the file to CRLF; the check still passes.
	writeFile(t, out, strings.ReplaceAll(content, "\n", "\r\n"))
	if code := finish(root, true, content, mods, &stdout, &stderr); code != 0 {
		t.Fatalf("check of a CRLF checkout: code %d, stderr %q", code, stderr.String())
	}

	writeFile(t, out, strings.Replace(content, "windows module license", "edited", 1))
	stderr.Reset()
	if code := finish(root, true, content, mods, &stdout, &stderr); code != 1 || !strings.Contains(stderr.String(), "out of date") {
		t.Fatalf("check of an edited file: code %d, stderr %q", code, stderr.String())
	}

	writeFile(t, out, content)
	writeFile(t, filepath.Join(root, noticeFile), "  - "+sqliteModule+"\n")
	stderr.Reset()
	if code := finish(root, true, content, mods, &stdout, &stderr); code != 1 || !strings.Contains(stderr.String(), "does not list example.com/win") {
		t.Fatalf("check with an incomplete NOTICE: code %d, stderr %q", code, stderr.String())
	}
	stderr.Reset()
	if code := finish(root, false, content, mods, &stdout, &stderr); code != 1 {
		t.Fatalf("write with an incomplete NOTICE: code %d, want 1", code)
	}
	if err := os.Remove(filepath.Join(root, noticeFile)); err != nil {
		t.Fatal(err)
	}
	if code := finish(root, true, content, mods, &stdout, &stderr); code != 1 {
		t.Fatalf("check without NOTICE: code %d, want 1", code)
	}
}

// TestListModulesForRepository runs `go list` on this repository.
func TestListModulesForRepository(t *testing.T) {
	if testing.Short() {
		t.Skip("runs the go command")
	}
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	mods, err := listModules(context.Background(), root, []platform{{"linux", "amd64"}})
	if err != nil {
		t.Fatalf("listModules: %v", err)
	}
	found := false
	for _, m := range mods {
		if m.path == "github.com/jabbott-iii/Munus" {
			t.Fatal("the main module was listed")
		}
		if m.path == sqliteModule {
			found = true
			if _, err := os.Stat(filepath.Join(m.dir, sqliteHeader)); err != nil {
				t.Fatalf("go-sqlite3 directory: %v", err)
			}
		}
	}
	if !found {
		t.Fatalf("%s not listed in %d modules", sqliteModule, len(mods))
	}
	if _, err := goEnv(context.Background(), root, "GOROOT"); err != nil {
		t.Fatalf("goEnv: %v", err)
	}
	if _, err := goEnv(context.Background(), root, "MUNUS_NO_SUCH_GO_ENV_VARIABLE"); err == nil {
		t.Fatal("goEnv of an unknown variable succeeded")
	}
}

// TestRepositoryLicensesUpToDate is the CI check of the committed
// THIRD_PARTY_LICENSES and NOTICE (integration test: runs the go command,
// which may download modules that are not in the module cache yet).
func TestRepositoryLicensesUpToDate(t *testing.T) {
	if testing.Short() {
		t.Skip("runs the go command")
	}
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	// The go command reads the sources, go.mod and go.sum in a child process,
	// which go test's result cache does not see. Listing every directory of
	// the repository records their entries' sizes and modification times, so
	// a changed import or dependency invalidates a cached pass.
	err = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && d.Name() == ".git" {
			return filepath.SkipDir
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk repository: %v", err)
	}
	var stdout, stderr bytes.Buffer
	if code := run(root, true, &stdout, &stderr); code != 0 {
		t.Fatalf("%s or %s is out of date (exit %d):\n%s", outputFile, noticeFile, code, stderr.String())
	}
}
