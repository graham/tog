package testdocs

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os/exec"
	"strings"
)

// TestResult contains the result of running a single test.
type TestResult struct {
	Package string
	Test    string
	Action  string  // "pass", "fail", "skip"
	Elapsed float64 // seconds
	Output  string  // combined output
}

// testEvent represents a single JSON event from go test -json.
type testEvent struct {
	Action  string  `json:"Action"`
	Package string  `json:"Package"`
	Test    string  `json:"Test"`
	Elapsed float64 `json:"Elapsed"`
	Output  string  `json:"Output"`
}

// RunTests executes go test with JSON output and parses the results.
func RunTests(pkgPattern string) ([]TestResult, error) {
	cmd := exec.Command("go", "test", "-json", pkgPattern)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	// Run the command (ignore error since failing tests return non-zero)
	_ = cmd.Run()

	return parseTestOutput(&stdout)
}

func parseTestOutput(output *bytes.Buffer) ([]TestResult, error) {
	// Map to accumulate output per test
	testOutputs := make(map[string]*strings.Builder)
	results := make(map[string]*TestResult)

	scanner := bufio.NewScanner(output)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var event testEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			continue // Skip malformed lines
		}

		// Skip package-level events (no Test field) except for output
		if event.Test == "" {
			continue
		}

		key := event.Package + "/" + event.Test

		// Initialize result if new
		if results[key] == nil {
			results[key] = &TestResult{
				Package: event.Package,
				Test:    event.Test,
			}
			testOutputs[key] = &strings.Builder{}
		}

		switch event.Action {
		case "output":
			testOutputs[key].WriteString(event.Output)
		case "pass", "fail", "skip":
			results[key].Action = event.Action
			results[key].Elapsed = event.Elapsed
		}
	}

	// Convert map to slice and attach outputs
	var resultList []TestResult
	for key, result := range results {
		result.Output = strings.TrimSpace(testOutputs[key].String())
		resultList = append(resultList, *result)
	}

	return resultList, scanner.Err()
}

// ResultsByTest creates a map from test name to result for easy lookup.
func ResultsByTest(results []TestResult) map[string]TestResult {
	m := make(map[string]TestResult)
	for _, r := range results {
		m[r.Test] = r
	}
	return m
}

// ResultsByPackageAndTest creates a nested map: package -> test name -> result.
func ResultsByPackageAndTest(results []TestResult) map[string]map[string]TestResult {
	m := make(map[string]map[string]TestResult)
	for _, r := range results {
		if m[r.Package] == nil {
			m[r.Package] = make(map[string]TestResult)
		}
		m[r.Package][r.Test] = r
	}
	return m
}
