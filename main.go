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
	"context"
	"fmt"
	"os"

	"github.com/jabbott-iii/Munus/internal"
)

// version is stamped at release time with -ldflags "-X main.version=<tag>";
// it is only written by the linker.
var version = "dev"

func main() {
	// The sqlite database is opened lazily by the root command, so help and
	// version output never create a database file.
	db := internal.NewDeferredDatabase(databasePathFromEnv())

	rootCmd := internal.NewRootCmd(db)
	rootCmd.Version = version
	// main is the program's top-level boundary. Ctrl+C keeps its default
	// behaviour (terminate the process), which is safe because every write is
	// a single SQLite transaction; trapping it would leave prompts hanging.
	err := rootCmd.ExecuteContext(context.Background())
	if cerr := db.Close(); cerr != nil {
		_, _ = fmt.Fprintf(os.Stderr, "munus: close database: %v\n", cerr)
		err = cerr
	}
	if err != nil {
		os.Exit(1)
	}
}
