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
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/jabbott-iii/Munus/pkg"
)

const (
	databasePathEnv  = "MUNUS_DB_PATH"
	databaseFileName = "munus.db"
	dataDirName      = "munus"
)

// databaseLocation returns where this process keeps its database; see
// resolveDatabaseLocation.
func databaseLocation() internal.DatabaseLocation {
	return resolveDatabaseLocation(os.Getenv, runtime.GOOS, os.UserHomeDir)
}

// resolveDatabaseLocation uses MUNUS_DB_PATH when it is set (the user manages
// that path and its directory) and otherwise munus.db in the per-user data
// directory (see defaultDataDir), which is created when a command first opens
// the database. Munus v3.0.0 and older defaulted to munus.db in the working
// directory; when such a file is found there, the location carries a notice.
// Errors are carried in the location, so help and version output still work.
func resolveDatabaseLocation(getenv func(string) string, goos string, home func() (string, error)) internal.DatabaseLocation {
	if path := getenv(databasePathEnv); path != "" {
		return internal.DatabaseLocation{Path: path}
	}
	dir, err := defaultDataDir(getenv, goos, home)
	if err == nil && !filepath.IsAbs(dir) {
		err = fmt.Errorf("data directory %q is not an absolute path", dir)
	}
	path := filepath.Join(dir, databaseFileName)
	if err == nil && strings.Contains(path, "?") {
		// sqlite would read everything after '?' as connection parameters.
		err = fmt.Errorf("default database path %q contains '?'", path)
	}
	if err != nil {
		return internal.DatabaseLocation{
			Err: fmt.Errorf("locate the default database: %w; set %s to choose a database file", err, databasePathEnv),
		}
	}
	return internal.DatabaseLocation{Path: path, Dir: dir, Notice: legacyDatabaseNotice(path)}
}

// defaultDataDir returns the per-user directory for Munus data on goos:
// %LocalAppData%\munus on Windows (local, not roaming, because it holds a
// SQLite database), ~/Library/Application Support/munus on macOS, and
// $XDG_DATA_HOME/munus (default ~/.local/share/munus) elsewhere. A relative
// XDG_DATA_HOME is ignored, as the XDG Base Directory specification requires.
func defaultDataDir(getenv func(string) string, goos string, home func() (string, error)) (string, error) {
	switch goos {
	case "windows":
		base := getenv("LOCALAPPDATA")
		if base == "" {
			return "", errors.New("%LOCALAPPDATA% is not set")
		}
		return filepath.Join(base, dataDirName), nil
	case "darwin", "ios":
		h, err := home()
		if err != nil {
			return "", err
		}
		return filepath.Join(h, "Library", "Application Support", dataDirName), nil
	default:
		if base := getenv("XDG_DATA_HOME"); base != "" && filepath.IsAbs(base) {
			return filepath.Join(base, dataDirName), nil
		}
		h, err := home()
		if err != nil {
			return "", err
		}
		return filepath.Join(h, ".local", "share", dataDirName), nil
	}
}

// legacyDatabaseNotice returns a notice when the working directory holds a
// munus.db that is not the default database, which is where Munus v3.0.0 and
// older kept tasks; it returns "" otherwise. It only reads file metadata.
func legacyDatabaseNotice(defaultPath string) string {
	legacy, err := os.Stat(databaseFileName)
	if err != nil || !legacy.Mode().IsRegular() {
		return ""
	}
	if current, err := os.Stat(defaultPath); err == nil && os.SameFile(legacy, current) {
		// Run from inside the data directory: it is the default database.
		return ""
	}
	return fmt.Sprintf("munus: note: %s in the current directory is no longer used by default; tasks are now stored in %s\n"+
		"munus: to keep using it, set %s to its path; to move its tasks, see \"Database location\" in the README",
		databaseFileName, defaultPath, databasePathEnv)
}
