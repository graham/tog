package app

import (
	"fmt"
	"os"

	"github.com/graham/tog/tools/testdocs"
)

func cmdTestdocs(cfg Config, args []string) {
	// Determine defaults based on config
	defaultOutput := "docs/tests.html"
	defaultPkg := ""
	defaultRoot := "."
	defaultTitle := cfg.Name

	if cfg.Testdocs != nil {
		if cfg.Testdocs.OutputPath != "" {
			defaultOutput = cfg.Testdocs.OutputPath
		}
		if cfg.Testdocs.PkgPattern != "" {
			defaultPkg = cfg.Testdocs.PkgPattern
		}
		if cfg.Testdocs.RootDir != "" {
			defaultRoot = cfg.Testdocs.RootDir
		}
	}

	fs := newFlagSet(cfg.Name, "testdocs", "Generate HTML documentation from test files.")
	outputPath := fs.String("o", defaultOutput, "output HTML file path")
	pkgPattern := fs.String("pkg", defaultPkg, "package pattern to test (e.g., github.com/user/repo/...)")
	rootDir := fs.String("root", defaultRoot, "root directory to scan for test files")
	title := fs.String("title", defaultTitle, "project title")
	fs.Parse(args)

	// If no package pattern provided, try to infer from go.mod
	pkg := *pkgPattern
	if pkg == "" {
		fmt.Fprintf(os.Stderr, "error: -pkg flag is required (e.g., github.com/user/repo/...)\n")
		os.Exit(1)
	}

	err := testdocs.Run(testdocs.Config{
		OutputPath: *outputPath,
		PkgPattern: pkg,
		RootDir:    *rootDir,
		Title:      *title,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "testdocs failed: %v\n", err)
		os.Exit(1)
	}
}
