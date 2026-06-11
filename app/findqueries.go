package app

import (
	"fmt"
	"os"

	"github.com/graham/tog/tools/findqueries"
)

func cmdFindqueries(cfg Config, args []string) {
	fs := newFlagSet(cfg.Name, "findqueries", "Find unregistered SQL queries in Go source files.")
	excludeTests := fs.Bool("exclude-tests", false, "exclude test files")
	showSQL := fs.Bool("show-sql", false, "show SQL in output")
	fs.Parse(args)

	root := "."
	if fs.NArg() > 0 {
		root = fs.Arg(0)
	}

	queriesCfg := findqueries.Config{
		ExcludeTests: *excludeTests,
	}

	queries, err := findqueries.Find(root, queriesCfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "findqueries failed: %v\n", err)
		os.Exit(1)
	}

	if len(queries) == 0 {
		fmt.Println("No unregistered queries found.")
		return
	}

	fmt.Print(findqueries.FormatTable(queries, *showSQL))
	os.Exit(1) // Exit with error if unregistered queries found
}
