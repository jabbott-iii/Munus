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
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/jabbott-iii/Munus/internal"
)

// fakeEnv returns a getenv function that sees only vars.
func fakeEnv(vars map[string]string) func(string) string {
	return func(key string) string { return vars[key] }
}

// fakeHome returns a home-directory function with a fixed result.
func fakeHome(dir string, err error) func() (string, error) {
	return func() (string, error) { return dir, err }
}

// chdirEmpty runs the rest of the test in a new empty directory, so a
// munus.db in the real working directory cannot affect it.
func chdirEmpty(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Chdir(dir)
	return dir
}

func writeFile(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
		t.Fatalf("setup failed: %v", err)
	}
}

func TestResolveDatabaseLocationUsesConfiguredPath(t *testing.T) {
	work := chdirEmpty(t)
	writeFile(t, filepath.Join(work, databaseFileName)) // no notice when the path is configured
	want := filepath.Join(t.TempDir(), "tasks.db")

	loc := resolveDatabaseLocation(fakeEnv(map[string]string{databasePathEnv: want}), "linux", fakeHome("", errors.New("unused")))
	if loc != (internal.DatabaseLocation{Path: want}) {
		t.Fatalf("location = %+v, want only Path %q", loc, want)
	}
}

func TestResolveDatabaseLocationDefaultsToDataDir(t *testing.T) {
	chdirEmpty(t)
	data := t.TempDir()
	dir := filepath.Join(data, dataDirName)

	loc := resolveDatabaseLocation(fakeEnv(map[string]string{"XDG_DATA_HOME": data}), "linux", fakeHome("", errors.New("unused")))
	if loc.Err != nil {
		t.Fatalf("unexpected error: %v", loc.Err)
	}
	if loc.Path != filepath.Join(dir, databaseFileName) || loc.Dir != dir || loc.Notice != "" {
		t.Fatalf("location = %+v, want %s in %s without a notice", loc, databaseFileName, dir)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("resolving must not create the directory, stat err=%v", err)
	}
}

func TestDefaultDataDir(t *testing.T) {
	home := t.TempDir()
	base := t.TempDir()
	homeErr := errors.New("no home directory")
	cases := []struct {
		name    string
		goos    string
		env     map[string]string
		homeErr error
		want    string
		wantErr string
	}{
		{name: "linux XDG_DATA_HOME", goos: "linux", env: map[string]string{"XDG_DATA_HOME": base}, want: filepath.Join(base, "munus")},
		{name: "linux default", goos: "linux", want: filepath.Join(home, ".local", "share", "munus")},
		{name: "linux relative XDG_DATA_HOME is ignored", goos: "linux", env: map[string]string{"XDG_DATA_HOME": "rel/data"}, want: filepath.Join(home, ".local", "share", "munus")},
		{name: "freebsd default", goos: "freebsd", want: filepath.Join(home, ".local", "share", "munus")},
		{name: "linux without home", goos: "linux", homeErr: homeErr, wantErr: homeErr.Error()},
		{name: "darwin ignores XDG_DATA_HOME", goos: "darwin", env: map[string]string{"XDG_DATA_HOME": base}, want: filepath.Join(home, "Library", "Application Support", "munus")},
		{name: "darwin without home", goos: "darwin", homeErr: homeErr, wantErr: homeErr.Error()},
		{name: "windows LocalAppData", goos: "windows", env: map[string]string{"LOCALAPPDATA": base, "XDG_DATA_HOME": home}, want: filepath.Join(base, "munus")},
		{name: "windows without LOCALAPPDATA", goos: "windows", wantErr: "LOCALAPPDATA"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := defaultDataDir(fakeEnv(tc.env), tc.goos, fakeHome(home, tc.homeErr))
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("error = %v, want one containing %q", err, tc.wantErr)
				}
				return
			}
			if err != nil || got != tc.want {
				t.Fatalf("defaultDataDir = %q, %v; want %q", got, err, tc.want)
			}
		})
	}
}

// Without a usable default location the error is carried to the first open
// and tells the user about MUNUS_DB_PATH.
func TestResolveDatabaseLocationErrors(t *testing.T) {
	chdirEmpty(t)
	cases := map[string]struct {
		env  map[string]string
		home func() (string, error)
	}{
		"no home directory": {home: fakeHome("", errors.New("$HOME is not defined"))},
		"relative home":     {home: fakeHome(filepath.Join("relative", "home"), nil)},
		"question mark":     {env: map[string]string{"XDG_DATA_HOME": filepath.Join(t.TempDir(), "a?b")}, home: fakeHome("", errors.New("unused"))},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			loc := resolveDatabaseLocation(fakeEnv(tc.env), "linux", tc.home)
			if loc.Err == nil || !strings.Contains(loc.Err.Error(), databasePathEnv) {
				t.Fatalf("expected an error mentioning %s, got %+v", databasePathEnv, loc)
			}
			if loc.Path != "" || loc.Dir != "" {
				t.Fatalf("expected no path on error, got %+v", loc)
			}
		})
	}
}

func TestLegacyDatabaseNotice(t *testing.T) {
	t.Run("old database in the working directory", func(t *testing.T) {
		work := chdirEmpty(t)
		writeFile(t, filepath.Join(work, databaseFileName))
		defaultPath := filepath.Join(t.TempDir(), dataDirName, databaseFileName)
		notice := legacyDatabaseNotice(defaultPath)
		for _, want := range []string{"munus.db in the current directory", defaultPath, databasePathEnv, "Database location"} {
			if !strings.Contains(notice, want) {
				t.Errorf("notice %q does not contain %q", notice, want)
			}
		}
	})
	t.Run("no database in the working directory", func(t *testing.T) {
		chdirEmpty(t)
		if notice := legacyDatabaseNotice(filepath.Join(t.TempDir(), databaseFileName)); notice != "" {
			t.Fatalf("unexpected notice %q", notice)
		}
	})
	t.Run("munus.db is a directory", func(t *testing.T) {
		work := chdirEmpty(t)
		if err := os.Mkdir(filepath.Join(work, databaseFileName), 0o700); err != nil {
			t.Fatalf("setup failed: %v", err)
		}
		if notice := legacyDatabaseNotice(filepath.Join(t.TempDir(), databaseFileName)); notice != "" {
			t.Fatalf("unexpected notice %q", notice)
		}
	})
	t.Run("working directory is the data directory", func(t *testing.T) {
		work := chdirEmpty(t)
		path := filepath.Join(work, databaseFileName)
		writeFile(t, path)
		if notice := legacyDatabaseNotice(path); notice != "" {
			t.Fatalf("unexpected notice %q", notice)
		}
	})
}

// End to end with the real commands: help creates nothing; the first command
// creates the data directory and database there, shows the notice about the
// old ./munus.db and leaves that file and its tasks alone.
func TestDefaultLocationWithLegacyDatabase(t *testing.T) {
	work := chdirEmpty(t)
	legacy := internal.NewDeferredDatabase(filepath.Join(work, databaseFileName))
	run(t, legacy, "add", "-t", "legacy task", "-d", "from v3")
	if err := legacy.Close(); err != nil {
		t.Fatalf("close legacy database: %v", err)
	}
	legacyInfo, err := os.Stat(databaseFileName)
	if err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	data := t.TempDir()
	dir := filepath.Join(data, dataDirName)
	newLocation := func() internal.DatabaseLocation {
		return resolveDatabaseLocation(fakeEnv(map[string]string{"XDG_DATA_HOME": data}), "linux", fakeHome("", errors.New("unused")))
	}

	help := internal.NewDeferredDatabaseAt(newLocation())
	if _, stderr := run(t, help, "--help"); strings.Contains(stderr, "no longer used") {
		t.Fatalf("help showed the notice: %q", stderr)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("help created the data directory, stat err=%v", err)
	}

	db := internal.NewDeferredDatabaseAt(newLocation())
	t.Cleanup(func() { _ = db.Close() })
	stdout, stderr := run(t, db, "list")
	if strings.Contains(stdout, "legacy task") {
		t.Fatalf("the old database was used: %q", stdout)
	}
	if !strings.Contains(stderr, "no longer used by default") || !strings.Contains(stderr, filepath.Join(dir, databaseFileName)) {
		t.Fatalf("expected the notice naming the new location, got %q", stderr)
	}
	dirInfo, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("expected data directory: %v", err)
	}
	fileInfo, err := os.Stat(filepath.Join(dir, databaseFileName))
	if err != nil {
		t.Fatalf("expected database in the data directory: %v", err)
	}
	if after, err := os.Stat(databaseFileName); err != nil || after.Size() != legacyInfo.Size() || !after.ModTime().Equal(legacyInfo.ModTime()) {
		t.Fatalf("the old database changed: %v (err %v)", after, err)
	}
	if runtime.GOOS != "windows" {
		if got := dirInfo.Mode().Perm(); got != 0o700 {
			t.Errorf("data directory mode = %o, want 700", got)
		}
		if got := fileInfo.Mode().Perm(); got != 0o600 {
			t.Errorf("database file mode = %o, want 600", got)
		}
	}
}

// run executes the root command with args and returns its output.
func run(t *testing.T, db *internal.Database, args ...string) (stdout, stderr string) {
	t.Helper()
	root := internal.NewRootCmd(db)
	root.Version = "test"
	var out, errOut bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&errOut)
	root.SetArgs(args)
	if err := root.Execute(); err != nil {
		t.Fatalf("munus %v failed: %v\n%s", args, err, errOut.String())
	}
	return out.String(), errOut.String()
}

func TestDatabaseLocationReadsEnvironment(t *testing.T) {
	want := filepath.Join(t.TempDir(), "tasks.db")
	t.Setenv(databasePathEnv, want)
	if loc := databaseLocation(); loc.Path != want || loc.Dir != "" || loc.Err != nil {
		t.Fatalf("databaseLocation() = %+v, want Path %q", loc, want)
	}
}

// The real lookup for the platform the tests run on (CI runs them on Linux,
// macOS and Windows), with every input pointed at temporary directories.
func TestDatabaseLocationDefaultForThisPlatform(t *testing.T) {
	chdirEmpty(t)
	home, local, xdg := t.TempDir(), t.TempDir(), t.TempDir()
	t.Setenv(databasePathEnv, "")
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("LOCALAPPDATA", local)
	t.Setenv("XDG_DATA_HOME", xdg)
	var dir string
	switch runtime.GOOS {
	case "windows":
		dir = filepath.Join(local, dataDirName)
	case "darwin", "ios":
		dir = filepath.Join(home, "Library", "Application Support", dataDirName)
	default:
		dir = filepath.Join(xdg, dataDirName)
	}

	loc := databaseLocation()
	if loc.Err != nil || loc.Dir != dir || loc.Path != filepath.Join(dir, databaseFileName) || loc.Notice != "" {
		t.Fatalf("databaseLocation() = %+v, want %s in %s", loc, databaseFileName, dir)
	}
	db := internal.NewDeferredDatabaseAt(loc)
	t.Cleanup(func() { _ = db.Close() })
	run(t, db, "add", "-t", "here", "-d", "x")
	if _, err := os.Stat(loc.Path); err != nil {
		t.Fatalf("expected the database at %s: %v", loc.Path, err)
	}
}
