// Command pg2sqlite translates PostgreSQL migrations to SQLite3.
//
// Usage:
//
//	pg2sqlite <src-dir> <dst-dir>
//
// Example:
//
//	pg2sqlite migrations/postgres migrations/sqlite3
package main

import (
	"fmt"
	"os"

	"github.com/graham/tog/tools/pg2sqlite"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintf(os.Stderr, "Usage: %s <src-dir> <dst-dir>\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "\nTranslates PostgreSQL SQL files to SQLite3-compatible SQL.\n")
		fmt.Fprintf(os.Stderr, "\nExample:\n")
		fmt.Fprintf(os.Stderr, "  %s migrations/postgres migrations/sqlite3\n", os.Args[0])
		os.Exit(1)
	}

	srcDir := os.Args[1]
	dstDir := os.Args[2]

	if err := pg2sqlite.TranslateDirectory(srcDir, dstDir); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Translation complete.")
}
