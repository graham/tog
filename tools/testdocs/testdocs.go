// Package testdocs generates HTML documentation from Go test files.
// It parses test files using AST to extract doc comments and source code,
// runs tests to capture results, and generates a self-contained HTML report.
package testdocs

import (
	"fmt"
	"os"
	"path/filepath"
)

// Config configures the test documentation generation.
type Config struct {
	// OutputPath is the path to write the HTML file.
	// Default: "docs/tests.html"
	OutputPath string

	// PkgPattern is the package pattern to test (e.g., "./...").
	// Default: "./..."
	PkgPattern string

	// Title is the project title for the report.
	// Default: directory name
	Title string

	// RootDir is the root directory to scan for test files.
	// Default: current directory
	RootDir string
}

// Run generates the test documentation HTML file.
func Run(cfg Config) error {
	// Set defaults
	if cfg.OutputPath == "" {
		cfg.OutputPath = "docs/tests.html"
	}
	if cfg.PkgPattern == "" {
		cfg.PkgPattern = "./..."
	}
	if cfg.RootDir == "" {
		cfg.RootDir = "."
	}
	if cfg.Title == "" {
		abs, err := filepath.Abs(cfg.RootDir)
		if err != nil {
			cfg.Title = "Project"
		} else {
			cfg.Title = filepath.Base(abs)
		}
	}

	// Parse test files to get test info
	fmt.Println("Parsing test files...")
	tests, err := ParseTestFiles(cfg.RootDir)
	if err != nil {
		return fmt.Errorf("parsing test files: %w", err)
	}
	fmt.Printf("Found %d tests\n", len(tests))

	// Run tests to get results
	fmt.Println("Running tests...")
	results, err := RunTests(cfg.PkgPattern)
	if err != nil {
		return fmt.Errorf("running tests: %w", err)
	}

	// Generate report
	fmt.Println("Generating report...")
	report := GenerateReport(cfg.Title, tests, results)

	// Ensure output directory exists
	outputDir := filepath.Dir(cfg.OutputPath)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("creating output directory: %w", err)
	}

	// Write HTML file
	f, err := os.Create(cfg.OutputPath)
	if err != nil {
		return fmt.Errorf("creating output file: %w", err)
	}
	defer f.Close()

	if err := WriteHTML(f, report); err != nil {
		return fmt.Errorf("writing HTML: %w", err)
	}

	fmt.Printf("Test documentation written to %s\n", cfg.OutputPath)
	fmt.Printf("Summary: %d passed, %d failed, %d skipped\n",
		report.TotalPassed, report.TotalFailed, report.TotalSkipped)

	return nil
}
