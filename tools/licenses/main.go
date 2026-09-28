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

// Command licenses writes THIRD_PARTY_LICENSES, the full license texts of the
// third-party software compiled into the Munus release binaries and Docker
// image, and checks that the committed file and NOTICE are up to date.
//
// Run it from the repository root:
//
//	go run ./tools/licenses          # rewrite THIRD_PARTY_LICENSES
//	go run ./tools/licenses -check   # fail if THIRD_PARTY_LICENSES or NOTICE is out of date
//
// The module set is the union of the packages `go list -deps` reports for
// every release platform with cgo enabled (Linux also with the release build
// tags), ignoring go.work files and GOFLAGS. Module versions are not written
// (a binary records them; see `go version -m`), so the file only changes when
// a module is added or removed or a license text changes.
package main

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	outputFile = "THIRD_PARTY_LICENSES"
	noticeFile = "NOTICE"
	dockerfile = "Dockerfile"

	// The Linux release binaries are built in the Dockerfile's Alpine builder
	// and link musl statically (plan 4, P-047). An Alpine release keeps one
	// upstream musl version, so when the Dockerfile moves to another Alpine
	// release, replace musl-COPYRIGHT with the COPYRIGHT file of that release's
	// musl version and update both constants; the Dockerfile check below fails
	// until then.
	muslVersion       = "1.2.6"
	muslAlpineRelease = "3.24"

	// linuxReleaseTags are the build tags of the Linux release binaries
	// (Dockerfile, static-builder stage). The Docker image's binary is built
	// without them, so Linux packages are listed both ways.
	linuxReleaseTags = "sqlite_omit_load_extension,osusergo,netgo"

	sqliteModule     = "github.com/mattn/go-sqlite3"
	sqliteHeader     = "sqlite3-binding.h"
	sqliteDisclaimer = "The author disclaims copyright to this source code."

	commandTimeout = 5 * time.Minute
	ruleWidth      = 80
)

// muslCopyright is musl's COPYRIGHT file, verbatim from the musl release named
// by muslVersion.
//
//go:embed musl-COPYRIGHT
var muslCopyright string

// mitTerms is the standard MIT license text without a copyright line, written
// for modules whose README names the MIT license but that ship no license file.
const mitTerms = `Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.`

// licenseFileName matches the license and notice files collected from a
// module's root directory and from the directories of its compiled packages.
var licenseFileName = regexp.MustCompile(`(?i)^(licen[cs]e|copying|notice|patents|unlicense|copyright)([._-].*)?$`)

// noticeModuleLine matches a list entry in NOTICE that names a module path.
var noticeModuleLine = regexp.MustCompile(`(?m)^[ \t]*-[ \t]+([a-z0-9.-]+\.[a-z]{2,}/[^\s]+)`)

// dockerFrom matches the image reference of a Dockerfile FROM line.
var dockerFrom = regexp.MustCompile(`(?im)^[ \t]*FROM[ \t]+(?:--platform=[^\s]+[ \t]+)?([^\s]+)`)

// alpineRelease matches the Alpine release in an image reference such as
// golang:1.26-alpine3.24 or alpine:3.24.
var alpineRelease = regexp.MustCompile(`alpine:?(\d+\.\d+)`)

type platform struct{ goos, goarch string }

// releasePlatforms returns the release targets built by cd.yml.
func releasePlatforms() []platform {
	return []platform{
		{"linux", "amd64"}, {"linux", "arm64"},
		{"darwin", "amd64"}, {"darwin", "arm64"},
		{"windows", "amd64"}, {"windows", "arm64"},
	}
}

// readmeLicense describes a module that ships no license file but states its
// license in a file of its root directory. The statement is checked verbatim,
// so a changed README fails generation until it is reviewed again.
type readmeLicense struct {
	file    string
	excerpt string
	terms   string
}

func readmeLicenses() map[string]readmeLicense {
	return map[string]readmeLicense{
		"github.com/mattn/go-localereader": {
			file:    "README.md",
			excerpt: "## License\n\nMIT\n\n## Author\n\nYasuhiro Matsumoto (a.k.a. mattn)",
			terms:   mitTerms,
		},
	}
}

// module is a third-party module compiled into at least one release binary.
type module struct {
	path      string
	dir       string
	pkgDirs   map[string]struct{}
	platforms map[platform]struct{}
}

// licenseText is one file (or statement) reproduced in a section.
type licenseText struct {
	name string
	text string
}

// section is one component of THIRD_PARTY_LICENSES.
type section struct {
	title    string
	included string
	texts    []licenseText
}

func main() {
	check := flag.Bool("check", false, "check that "+outputFile+" and "+noticeFile+" are up to date instead of writing "+outputFile)
	root := flag.String("C", ".", "repository root")
	flag.Parse()
	if flag.NArg() != 0 {
		_, _ = fmt.Fprintln(os.Stderr, "licenses: unexpected arguments:", strings.Join(flag.Args(), " "))
		os.Exit(2)
	}
	os.Exit(run(*root, *check, os.Stdout, os.Stderr))
}

func run(root string, check bool, stdout, stderr io.Writer) int {
	ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
	defer cancel()

	mods, err := listModules(ctx, root, releasePlatforms())
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "licenses:", err)
		return 1
	}
	goroot, err := goEnv(ctx, root, "GOROOT")
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "licenses:", err)
		return 1
	}
	content, err := generate(root, goroot, mods)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "licenses:", err)
		return 1
	}
	return finish(root, check, content, mods, stdout, stderr)
}

// finish writes or checks THIRD_PARTY_LICENSES and checks NOTICE.
func finish(root string, check bool, content string, mods []*module, stdout, stderr io.Writer) int {
	status := 0
	outPath := filepath.Join(root, outputFile)
	if check {
		current, err := readRepoFile(outPath)
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "licenses: %v\n", err)
			return 1
		}
		// A Windows checkout may have CRLF line endings; compare the text.
		if line, differs := firstDifference(strings.ReplaceAll(current, "\r\n", "\n"), content); differs {
			_, _ = fmt.Fprintf(stderr, "licenses: %s is out of date (first difference at line %d); run `go run ./tools/licenses` and commit the result\n", outputFile, line)
			status = 1
		}
	} else {
		// A tracked source file shipped in every release archive, so it is
		// world-readable like the rest of the checkout.
		if err := os.WriteFile(outPath, []byte(content), 0o644); err != nil { // #nosec G306 -- repository file meant to be world-readable, see above
			_, _ = fmt.Fprintf(stderr, "licenses: write %s: %v\n", outputFile, err)
			return 1
		}
		_, _ = fmt.Fprintf(stdout, "wrote %s (%d third-party modules)\n", outputFile, len(mods))
	}

	notice, err := readRepoFile(filepath.Join(root, noticeFile))
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "licenses: %v\n", err)
		return 1
	}
	for _, problem := range noticeProblems(notice, mods) {
		_, _ = fmt.Fprintf(stderr, "licenses: %s: %s\n", noticeFile, problem)
		status = 1
	}
	return status
}

// listModules returns the third-party modules of the packages compiled into
// the main package for each platform, sorted by module path.
func listModules(ctx context.Context, root string, targets []platform) ([]*module, error) {
	byPath := map[string]*module{}
	for _, target := range targets {
		// The module set must not depend on the caller's workspace or flags.
		env := []string{"GOOS=" + target.goos, "GOARCH=" + target.goarch, "CGO_ENABLED=1", "GOWORK=off", "GOFLAGS=-mod=readonly"}
		tagSets := []string{""}
		if target.goos == "linux" {
			tagSets = append(tagSets, linuxReleaseTags)
		}
		for _, tags := range tagSets {
			args := []string{"list", "-deps", "-json=Dir,Standard,Module"}
			if tags != "" {
				args = append(args, "-tags="+tags)
			}
			out, err := goCommand(ctx, root, env, append(args, ".")...)
			if err != nil {
				return nil, fmt.Errorf("list packages for %s/%s: %w", target.goos, target.goarch, err)
			}
			if err := addPackages(byPath, bytes.NewReader(out), target); err != nil {
				return nil, fmt.Errorf("list packages for %s/%s: %w", target.goos, target.goarch, err)
			}
		}
	}
	mods := make([]*module, 0, len(byPath))
	for _, m := range byPath {
		mods = append(mods, m)
	}
	sort.Slice(mods, func(i, j int) bool { return mods[i].path < mods[j].path })
	return mods, nil
}

// listedPackage holds the `go list -json` fields used here.
type listedPackage struct {
	Dir      string
	Standard bool
	Module   *listedModule
}

type listedModule struct {
	Path    string
	Main    bool
	Dir     string
	Replace *listedModule
}

// addPackages records the modules of the packages in a `go list -json` stream.
func addPackages(byPath map[string]*module, r io.Reader, target platform) error {
	dec := json.NewDecoder(r)
	for {
		var pkg listedPackage
		if err := dec.Decode(&pkg); errors.Is(err, io.EOF) {
			return nil
		} else if err != nil {
			return fmt.Errorf("decode go list output: %w", err)
		}
		if pkg.Standard || pkg.Module == nil || pkg.Module.Main {
			continue
		}
		dir := pkg.Module.Dir
		if pkg.Module.Replace != nil && pkg.Module.Replace.Dir != "" {
			dir = pkg.Module.Replace.Dir
		}
		if dir == "" {
			return fmt.Errorf("module %s has no directory; run `go mod download`", pkg.Module.Path)
		}
		m := byPath[pkg.Module.Path]
		if m == nil {
			m = &module{path: pkg.Module.Path, dir: dir, pkgDirs: map[string]struct{}{}, platforms: map[platform]struct{}{}}
			byPath[pkg.Module.Path] = m
		}
		m.platforms[target] = struct{}{}
		if pkg.Dir != "" {
			m.pkgDirs[pkg.Dir] = struct{}{}
		}
	}
}

// goEnv returns the value of a `go env` variable.
func goEnv(ctx context.Context, root, name string) (string, error) {
	out, err := goCommand(ctx, root, nil, "env", name)
	if err != nil {
		return "", fmt.Errorf("go env %s: %w", name, err)
	}
	value := strings.TrimSpace(string(out))
	if value == "" {
		return "", fmt.Errorf("go env %s is empty", name)
	}
	return value, nil
}

// goCommand runs the go command in dir with extra environment variables and
// returns its standard output.
func goCommand(ctx context.Context, dir string, env []string, args ...string) ([]byte, error) {
	// The executable is the fixed "go" from PATH, the same toolchain that
	// runs this program; the arguments are built from constants above.
	cmd := exec.CommandContext(ctx, "go", args...) // #nosec G204 -- fixed executable, constant arguments, see above
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), env...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return nil, fmt.Errorf("go %s: %w: %s", strings.Join(args, " "), err, msg)
		}
		return nil, fmt.Errorf("go %s: %w", strings.Join(args, " "), err)
	}
	return out, nil
}

// generate renders THIRD_PARTY_LICENSES for the given modules.
func generate(root, goroot string, mods []*module) (string, error) {
	if err := checkDockerfile(root); err != nil {
		return "", err
	}
	all := releasePlatforms()

	goTexts, err := readLicenseFiles(goroot, "")
	if err != nil {
		return "", fmt.Errorf("license files of the Go distribution: %w", err)
	}
	if len(goTexts) == 0 {
		return "", fmt.Errorf("no license file found in GOROOT %s", goroot)
	}
	sections := []section{{
		title:    "Go standard library and runtime",
		included: "all binaries",
		texts:    goTexts,
	}}

	var sqlite *module
	for _, m := range mods {
		if m.path == sqliteModule {
			sqlite = m
		}
	}
	if sqlite != nil {
		notice, err := sqliteNotice(sqlite.dir)
		if err != nil {
			return "", err
		}
		sections = append(sections, section{
			title:    "SQLite (compiled into " + sqliteModule + ")",
			included: includedIn(sqlite.platforms, all),
			texts:    []licenseText{{name: sqliteHeader + ", header comment", text: notice}},
		})
	}

	musl, err := normalizeText([]byte(muslCopyright))
	if err != nil {
		return "", fmt.Errorf("musl-COPYRIGHT: %w", err)
	}
	sections = append(sections, section{
		title:    "musl libc " + muslVersion,
		included: "Linux release binaries (statically linked; built on Alpine " + muslAlpineRelease + ")",
		texts:    []licenseText{{name: "COPYRIGHT", text: musl}},
	})

	for _, m := range mods {
		texts, err := moduleTexts(m)
		if err != nil {
			return "", err
		}
		sections = append(sections, section{title: m.path, included: includedIn(m.platforms, all), texts: texts})
	}
	return render(sections), nil
}

// moduleTexts returns the license texts of a module: the license files in its
// root directory and in the directories of its compiled packages, or the
// reviewed README statement for a module that ships no license file.
func moduleTexts(m *module) ([]licenseText, error) {
	texts, err := readLicenseFiles(m.dir, "")
	if err != nil {
		return nil, fmt.Errorf("module %s: %w", m.path, err)
	}
	if len(texts) == 0 {
		rl, ok := readmeLicenses()[m.path]
		if !ok {
			return nil, fmt.Errorf("module %s has no license file; review its license and add it to readmeLicenses", m.path)
		}
		statement, err := readmeStatement(m.dir, rl)
		if err != nil {
			return nil, fmt.Errorf("module %s: %w", m.path, err)
		}
		texts = []licenseText{
			{name: "no license file; " + rl.file + " states", text: statement},
			{name: "license terms reproduced for reference", text: rl.terms + "\n"},
		}
	}

	// Package directories below the module root, sorted by their slash-separated
	// path so that the output is the same on every OS.
	rels := map[string]string{}
	for dir := range m.pkgDirs {
		rel, err := filepath.Rel(m.dir, dir)
		if err != nil || rel == "." || strings.HasPrefix(rel, "..") {
			continue
		}
		rels[filepath.ToSlash(rel)] = dir
	}
	names := make([]string, 0, len(rels))
	for rel := range rels {
		names = append(names, rel)
	}
	sort.Strings(names)
	for _, rel := range names {
		sub, err := readLicenseFiles(rels[rel], rel+"/")
		if err != nil {
			return nil, fmt.Errorf("module %s: %w", m.path, err)
		}
		texts = append(texts, sub...)
	}
	return texts, nil
}

// readLicenseFiles reads the license files directly in dir, sorted by name.
// prefix is prepended to the names shown in the output.
func readLicenseFiles(dir, prefix string) ([]licenseText, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var texts []licenseText
	for _, e := range entries {
		name := e.Name()
		if !e.Type().IsRegular() || !licenseFileName.MatchString(name) || strings.HasSuffix(strings.ToLower(name), ".go") {
			continue
		}
		data, err := readRepoFile(filepath.Join(dir, name))
		if err != nil {
			return nil, err
		}
		text, err := normalizeText([]byte(data))
		if err != nil {
			return nil, fmt.Errorf("%s: %w", filepath.Join(dir, name), err)
		}
		texts = append(texts, licenseText{name: prefix + name, text: text})
	}
	return texts, nil
}

// readmeStatement returns the reviewed license statement of rl, indented,
// after checking that the module's file still contains it verbatim.
func readmeStatement(dir string, rl readmeLicense) (string, error) {
	data, err := readRepoFile(filepath.Join(dir, rl.file))
	if err != nil {
		return "", err
	}
	text, err := normalizeText([]byte(data))
	if err != nil {
		return "", fmt.Errorf("%s: %w", rl.file, err)
	}
	if !strings.Contains(text, rl.excerpt) {
		return "", fmt.Errorf("%s no longer contains the reviewed license statement; review the module's license", rl.file)
	}
	lines := strings.Split(rl.excerpt, "\n")
	for i, line := range lines {
		if line != "" {
			lines[i] = "    " + line
		}
	}
	return strings.Join(lines, "\n") + "\n", nil
}

// sqliteNotice returns the public-domain dedication at the top of the SQLite
// header that go-sqlite3 compiles in, without the comment markers.
func sqliteNotice(moduleDir string) (string, error) {
	data, err := readRepoFile(filepath.Join(moduleDir, sqliteHeader))
	if err != nil {
		return "", fmt.Errorf("SQLite notice: %w", err)
	}
	lines := strings.Split(strings.ReplaceAll(data, "\r\n", "\n"), "\n")
	start, end := -1, -1
	for i, line := range lines {
		if start < 0 && strings.HasPrefix(line, "/*") {
			start = i + 1
			continue
		}
		if start >= 0 && strings.HasPrefix(line, "*****") {
			end = i
			break
		}
	}
	if start < 0 || end < 0 {
		return "", fmt.Errorf("SQLite notice: no header comment in %s", sqliteHeader)
	}
	var b strings.Builder
	for _, line := range lines[start:end] {
		line = strings.TrimPrefix(line, "**")
		line = strings.TrimPrefix(line, " ")
		b.WriteString(line)
		b.WriteByte('\n')
	}
	text, err := normalizeText([]byte(b.String()))
	if err != nil {
		return "", fmt.Errorf("SQLite notice: %w", err)
	}
	if !strings.Contains(text, sqliteDisclaimer) {
		return "", fmt.Errorf("SQLite notice: the header comment of %s no longer contains %q; review the SQLite license", sqliteHeader, sqliteDisclaimer)
	}
	return text, nil
}

// checkDockerfile verifies that every Alpine image in the Dockerfile uses the
// Alpine release whose musl COPYRIGHT is embedded.
func checkDockerfile(root string) error {
	data, err := readRepoFile(filepath.Join(root, dockerfile))
	if err != nil {
		return err
	}
	found := false
	for _, m := range dockerFrom.FindAllStringSubmatch(data, -1) {
		a := alpineRelease.FindStringSubmatch(m[1])
		if a == nil {
			continue
		}
		found = true
		if a[1] != muslAlpineRelease {
			return fmt.Errorf("%s uses Alpine %s (%s), but tools/licenses embeds the musl %s COPYRIGHT for Alpine %s; update musl-COPYRIGHT, muslVersion and muslAlpineRelease",
				dockerfile, a[1], m[1], muslVersion, muslAlpineRelease)
		}
	}
	if !found {
		return fmt.Errorf("%s has no Alpine image; update the musl section of tools/licenses", dockerfile)
	}
	return nil
}

// includedIn describes the platforms a component is compiled for.
func includedIn(set map[platform]struct{}, all []platform) string {
	if len(set) == len(all) {
		return "all binaries"
	}
	names := map[string]string{"linux": "Linux", "darwin": "macOS", "windows": "Windows"}
	var parts []string
	var order []string
	byOS := map[string][]platform{}
	for _, p := range all {
		if _, ok := byOS[p.goos]; !ok {
			order = append(order, p.goos)
		}
		byOS[p.goos] = append(byOS[p.goos], p)
	}
	for _, goos := range order {
		var have []string
		for _, p := range byOS[goos] {
			if _, ok := set[p]; ok {
				have = append(have, p.goos+"/"+p.goarch)
			}
		}
		switch {
		case len(have) == 0:
		case len(have) == len(byOS[goos]):
			parts = append(parts, names[goos]+" binaries")
		default:
			parts = append(parts, have...)
		}
	}
	return strings.Join(parts, ", ")
}

// render writes the sections in a stable plain-text layout.
func render(sections []section) string {
	var b strings.Builder
	b.WriteString(`Third-party software licenses for Munus
=======================================

The Munus binaries (the release archives and the munus binary in the Docker
image) include the third-party software listed below. Each component is
distributed under its own license, reproduced in full in this file; the Munus
license is in LICENSE and the attribution summary in NOTICE. A binary records
the module versions compiled into it (` + "`go version -m <binary>`" + ` lists them).
The packages of the Docker image's Alpine base system are not covered here;
they carry their own licenses.

This file is generated by ` + "`go run ./tools/licenses`" + ` and checked by
` + "`go test ./tools/licenses`" + ` (or ` + "`go run ./tools/licenses -check`" + `); do not edit
it by hand.

Contents:
`)
	for _, s := range sections {
		b.WriteString("  - " + s.title + "\n")
	}
	rule := strings.Repeat("=", ruleWidth)
	for _, s := range sections {
		b.WriteString("\n" + rule + "\n")
		b.WriteString(s.title + "\n")
		b.WriteString("Included in: " + s.included + "\n")
		b.WriteString(rule + "\n")
		for _, t := range s.texts {
			b.WriteString("\n--- " + t.name + " ---\n\n")
			b.WriteString(t.text)
		}
	}
	return b.String()
}

// normalizeText returns text with a single trailing newline, LF line endings,
// no byte order mark, no trailing spaces and no leading or trailing blank lines.
func normalizeText(data []byte) (string, error) {
	if !utf8.Valid(data) {
		return "", errors.New("not valid UTF-8")
	}
	s := strings.TrimPrefix(string(data), "\ufeff")
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " \t")
	}
	s = strings.Trim(strings.Join(lines, "\n"), "\n")
	if strings.TrimSpace(s) == "" {
		return "", errors.New("empty license text")
	}
	return s + "\n", nil
}

// noticeProblems reports modules missing from NOTICE and NOTICE entries for
// modules that are no longer compiled in.
func noticeProblems(notice string, mods []*module) []string {
	notice = strings.ReplaceAll(notice, "\r\n", "\n")
	listed := map[string]struct{}{}
	for _, m := range noticeModuleLine.FindAllStringSubmatch(notice, -1) {
		listed[m[1]] = struct{}{}
	}
	compiled := map[string]struct{}{}
	var problems []string
	for _, m := range mods {
		compiled[m.path] = struct{}{}
		if _, ok := listed[m.path]; !ok {
			problems = append(problems, "does not list "+m.path)
		}
	}
	var extra []string
	for path := range listed {
		if _, ok := compiled[path]; !ok {
			extra = append(extra, path)
		}
	}
	sort.Strings(extra)
	for _, path := range extra {
		problems = append(problems, "lists "+path+", which no release binary includes")
	}
	return problems
}

// firstDifference returns the first line (1-based) at which a and b differ.
func firstDifference(a, b string) (int, bool) {
	if a == b {
		return 0, false
	}
	al, bl := strings.Split(a, "\n"), strings.Split(b, "\n")
	for i := 0; i < len(al) && i < len(bl); i++ {
		if al[i] != bl[i] {
			return i + 1, true
		}
	}
	return min(len(al), len(bl)) + 1, true
}

// readRepoFile reads a file named by this program: a repository file, a file
// in the Go distribution or a file of a module in the module cache.
func readRepoFile(path string) (string, error) {
	data, err := os.ReadFile(path) // #nosec G304 -- paths come from the repository root, GOROOT and `go list`, see above
	if err != nil {
		return "", err
	}
	return string(data), nil
}
