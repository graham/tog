package testdocs

import (
	"html"
	"html/template"
	"io"
	"sort"
	"strings"
	"time"
)

// ReportData contains all data needed to generate the HTML report.
type ReportData struct {
	Title        string
	Generated    time.Time
	Packages     []PackageReport // All packages (for backwards compatibility)
	AppPackages  []PackageReport // App/example tests (shown first)
	CorePackages []PackageReport // Core framework tests (shown second)
	TotalPassed  int
	TotalFailed  int
	TotalSkipped int
}

// PackageReport contains test results for a single package.
type PackageReport struct {
	Name  string
	Tests []TestReport
}

// SubtestReport contains a subtest with its result.
type SubtestReport struct {
	Name     string
	Status   string // "pass", "fail", "skip", "unknown"
	Duration float64
	Output   string
	Line     int
}

// TestReport combines test info with its result.
type TestReport struct {
	Name        string
	Description string
	SourceCode  string
	Status      string // "pass", "fail", "skip", "unknown"
	Duration    float64
	Output      string
	FilePath    string
	Line        int
	Subtests    []SubtestReport
}

// GenerateReport creates a ReportData from test info and results.
func GenerateReport(title string, tests []TestInfo, results []TestResult) ReportData {
	resultMap := ResultsByTest(results)

	// Group tests by package
	packageTests := make(map[string][]TestReport)
	for _, t := range tests {
		result, ok := resultMap[t.Name]
		status := "unknown"
		var duration float64
		var output string
		if ok {
			status = result.Action
			duration = result.Elapsed
			output = result.Output
		}

		// Build subtest reports
		var subtestReports []SubtestReport
		for _, st := range t.Subtests {
			// Subtest result key is "TestName/subtest_name" (Go replaces spaces with underscores)
			subtestKey := t.Name + "/" + strings.ReplaceAll(st.Name, " ", "_")
			stResult, stOk := resultMap[subtestKey]
			stStatus := "unknown"
			var stDuration float64
			var stOutput string
			if stOk {
				stStatus = stResult.Action
				stDuration = stResult.Elapsed
				stOutput = stResult.Output
			}
			subtestReports = append(subtestReports, SubtestReport{
				Name:     st.Name,
				Status:   stStatus,
				Duration: stDuration,
				Output:   stOutput,
				Line:     st.Line,
			})
		}

		report := TestReport{
			Name:        t.Name,
			Description: t.Description,
			SourceCode:  t.SourceCode,
			Status:      status,
			Duration:    duration,
			Output:      output,
			FilePath:    t.FilePath,
			Line:        t.Line,
			Subtests:    subtestReports,
		}
		packageTests[t.Package] = append(packageTests[t.Package], report)
	}

	// Sort packages
	var packageNames []string
	for pkg := range packageTests {
		packageNames = append(packageNames, pkg)
	}
	sort.Strings(packageNames)

	// Build package reports, separating app from core
	var packages []PackageReport
	var appPackages []PackageReport
	var corePackages []PackageReport
	var totalPassed, totalFailed, totalSkipped int
	for _, pkg := range packageNames {
		tests := packageTests[pkg]
		for _, t := range tests {
			switch t.Status {
			case "pass":
				totalPassed++
			case "fail":
				totalFailed++
			case "skip":
				totalSkipped++
			}
		}
		report := PackageReport{
			Name:  pkg,
			Tests: tests,
		}
		packages = append(packages, report)

		// Separate app tests from core tests based on file path
		// Check if any test in this package is from routes/ or examples/routes/
		isAppPackage := false
		for _, t := range tests {
			if strings.Contains(t.FilePath, "/routes/") || strings.Contains(t.FilePath, "examples/routes") {
				isAppPackage = true
				break
			}
		}
		if isAppPackage {
			appPackages = append(appPackages, report)
		} else {
			corePackages = append(corePackages, report)
		}
	}

	return ReportData{
		Title:        title,
		Generated:    time.Now(),
		Packages:     packages,
		AppPackages:  appPackages,
		CorePackages: corePackages,
		TotalPassed:  totalPassed,
		TotalFailed:  totalFailed,
		TotalSkipped: totalSkipped,
	}
}

// WriteHTML writes the HTML report to the given writer.
func WriteHTML(w io.Writer, data ReportData) error {
	return htmlTemplate.Execute(w, data)
}

var htmlTemplate = template.Must(template.New("report").Funcs(template.FuncMap{
	"escapeHTML": html.EscapeString,
}).Parse(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>{{.Title}} - Test Documentation</title>
    <style>
        * {
            box-sizing: border-box;
        }
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, Ubuntu, sans-serif;
            line-height: 1.6;
            max-width: 1200px;
            margin: 0 auto;
            padding: 20px;
            background: #f5f5f5;
            color: #333;
        }
        header {
            background: #fff;
            padding: 20px;
            border-radius: 8px;
            margin-bottom: 20px;
            box-shadow: 0 1px 3px rgba(0,0,0,0.1);
        }
        h1 {
            margin: 0 0 10px 0;
            color: #2c3e50;
        }
        .summary {
            display: flex;
            gap: 20px;
            flex-wrap: wrap;
        }
        .stat {
            padding: 8px 16px;
            border-radius: 4px;
            font-weight: 500;
        }
        .stat.passed {
            background: #d4edda;
            color: #155724;
        }
        .stat.failed {
            background: #f8d7da;
            color: #721c24;
        }
        .stat.skipped {
            background: #fff3cd;
            color: #856404;
        }
        .generated {
            color: #6c757d;
            font-size: 0.9em;
            margin-top: 10px;
        }
        .package {
            background: #fff;
            border-radius: 8px;
            margin-bottom: 20px;
            box-shadow: 0 1px 3px rgba(0,0,0,0.1);
            overflow: hidden;
        }
        .package-header {
            background: #e9ecef;
            padding: 12px 20px;
            font-weight: 600;
            color: #495057;
        }
        .test {
            border-bottom: 1px solid #e9ecef;
            padding: 16px 20px;
        }
        .test:last-child {
            border-bottom: none;
        }
        .test-header {
            display: flex;
            align-items: center;
            gap: 12px;
            margin-bottom: 8px;
        }
        .test-name {
            font-weight: 600;
            font-size: 1.1em;
            color: #2c3e50;
        }
        .badge {
            padding: 2px 8px;
            border-radius: 4px;
            font-size: 0.8em;
            font-weight: 500;
            text-transform: uppercase;
        }
        .badge.pass {
            background: #28a745;
            color: #fff;
        }
        .badge.fail {
            background: #dc3545;
            color: #fff;
        }
        .badge.skip {
            background: #ffc107;
            color: #212529;
        }
        .badge.unknown {
            background: #6c757d;
            color: #fff;
        }
        .duration {
            color: #6c757d;
            font-size: 0.9em;
        }
        .description {
            margin: 8px 0;
            color: #495057;
        }
        .no-description {
            color: #adb5bd;
            font-style: italic;
        }
        .file-location {
            font-size: 0.85em;
            color: #6c757d;
            margin-top: 4px;
        }
        details {
            margin-top: 12px;
        }
        summary {
            cursor: pointer;
            color: #007bff;
            font-size: 0.9em;
            padding: 4px 0;
        }
        summary:hover {
            text-decoration: underline;
        }
        pre {
            background: #f8f9fa;
            border: 1px solid #e9ecef;
            border-radius: 4px;
            padding: 12px;
            overflow-x: auto;
            font-size: 0.85em;
            line-height: 1.5;
            margin: 8px 0 0 0;
        }
        .output {
            background: #fff3cd;
            border-color: #ffc107;
        }
        .output.fail {
            background: #f8d7da;
            border-color: #dc3545;
        }
        .toc {
            background: #fff;
            padding: 16px 20px;
            border-radius: 8px;
            margin-bottom: 20px;
            box-shadow: 0 1px 3px rgba(0,0,0,0.1);
        }
        .toc h2 {
            margin: 0 0 12px 0;
            font-size: 1em;
            color: #495057;
        }
        .toc ul {
            margin: 0;
            padding: 0;
            list-style: none;
        }
        .toc li {
            margin: 4px 0;
        }
        .toc a {
            color: #007bff;
            text-decoration: none;
        }
        .toc a:hover {
            text-decoration: underline;
        }
        .search-box {
            background: #fff;
            padding: 16px 20px;
            border-radius: 8px;
            margin-bottom: 20px;
            box-shadow: 0 1px 3px rgba(0,0,0,0.1);
        }
        .search-box input {
            width: 100%;
            padding: 10px 14px;
            font-size: 1em;
            border: 1px solid #ced4da;
            border-radius: 4px;
            outline: none;
        }
        .search-box input:focus {
            border-color: #007bff;
            box-shadow: 0 0 0 2px rgba(0,123,255,0.25);
        }
        .search-box .search-info {
            margin-top: 8px;
            font-size: 0.85em;
            color: #6c757d;
        }
        .hidden {
            display: none !important;
        }
        .subtests {
            margin-top: 12px;
            padding: 12px;
            background: #f8f9fa;
            border-radius: 4px;
        }
        .subtests-header {
            font-weight: 500;
            margin-bottom: 8px;
            color: #495057;
        }
        .subtest {
            display: flex;
            align-items: center;
            gap: 8px;
            padding: 6px 0;
            border-bottom: 1px solid #e9ecef;
        }
        .subtest:last-child {
            border-bottom: none;
        }
        .subtest-name {
            font-family: monospace;
            color: #495057;
        }
        .subtest details {
            margin-left: auto;
        }
        .section-header {
            background: #fff;
            padding: 16px 20px;
            border-radius: 8px;
            margin: 30px 0 20px 0;
            box-shadow: 0 1px 3px rgba(0,0,0,0.1);
            border-left: 4px solid #007bff;
        }
        .section-header h2 {
            margin: 0 0 4px 0;
            color: #2c3e50;
            font-size: 1.3em;
        }
        .section-header p {
            margin: 0;
            color: #6c757d;
            font-size: 0.95em;
        }
    </style>
</head>
<body>
    <header>
        <h1>{{.Title}} - Test Documentation</h1>
        <div class="summary">
            <span class="stat passed">{{.TotalPassed}} Passed</span>
            {{if gt .TotalFailed 0}}<span class="stat failed">{{.TotalFailed}} Failed</span>{{end}}
            {{if gt .TotalSkipped 0}}<span class="stat skipped">{{.TotalSkipped}} Skipped</span>{{end}}
        </div>
        <div class="generated">Generated: {{.Generated.Format "2006-01-02 15:04:05"}}</div>
    </header>

    <div class="search-box">
        <input type="text" id="test-filter" placeholder="Filter tests by name..." autocomplete="off">
        <div class="search-info" id="search-info"></div>
    </div>

    <nav class="toc">
        {{if .AppPackages}}
        <h2>Application Tests</h2>
        <ul>
            {{range .AppPackages}}
            <li><a href="#pkg-{{.Name}}">{{.Name}}</a> ({{len .Tests}} tests)</li>
            {{end}}
        </ul>
        {{end}}
        {{if .CorePackages}}
        <h2>Core Framework Tests</h2>
        <ul>
            {{range .CorePackages}}
            <li><a href="#pkg-{{.Name}}">{{.Name}}</a> ({{len .Tests}} tests)</li>
            {{end}}
        </ul>
        {{end}}
    </nav>

    {{if .AppPackages}}
    <div class="section-header">
        <h2>Application Tests</h2>
        <p>Tests for the example application built on tog.</p>
    </div>
    {{range .AppPackages}}
    <section class="package" id="pkg-{{.Name}}">
        <div class="package-header">{{.Name}}</div>
        {{range .Tests}}
        <div class="test">
            <div class="test-header">
                <span class="badge {{.Status}}">{{.Status}}</span>
                <span class="test-name">{{.Name}}</span>
                {{if gt .Duration 0.0}}
                <span class="duration">{{printf "%.3fs" .Duration}}</span>
                {{end}}
            </div>
            {{if .Description}}
            <div class="description">{{.Description}}</div>
            {{else}}
            <div class="description no-description">No description provided</div>
            {{end}}
            <div class="file-location">{{.FilePath}}:{{.Line}}</div>
            {{if .Subtests}}
            <div class="subtests">
                <div class="subtests-header">Subtests:</div>
                {{range .Subtests}}
                <div class="subtest">
                    <span class="badge {{.Status}}">{{.Status}}</span>
                    <span class="subtest-name">{{.Name}}</span>
                    {{if gt .Duration 0.0}}
                    <span class="duration">{{printf "%.3fs" .Duration}}</span>
                    {{end}}
                    {{if and .Output (eq .Status "fail")}}
                    <details>
                        <summary>Output</summary>
                        <pre class="output fail">{{.Output}}</pre>
                    </details>
                    {{end}}
                </div>
                {{end}}
            </div>
            {{end}}
            {{if .SourceCode}}
            <details>
                <summary>View source code</summary>
                <pre>{{.SourceCode}}</pre>
            </details>
            {{end}}
            {{if and .Output (eq .Status "fail")}}
            <details open>
                <summary>Test output</summary>
                <pre class="output fail">{{.Output}}</pre>
            </details>
            {{end}}
        </div>
        {{end}}
    </section>
    {{end}}
    {{end}}

    {{if .CorePackages}}
    <div class="section-header">
        <h2>Core Framework Tests</h2>
        <p>Tests for the tog framework internals: database, web, and authentication.</p>
    </div>
    {{range .CorePackages}}
    <section class="package" id="pkg-{{.Name}}">
        <div class="package-header">{{.Name}}</div>
        {{range .Tests}}
        <div class="test">
            <div class="test-header">
                <span class="badge {{.Status}}">{{.Status}}</span>
                <span class="test-name">{{.Name}}</span>
                {{if gt .Duration 0.0}}
                <span class="duration">{{printf "%.3fs" .Duration}}</span>
                {{end}}
            </div>
            {{if .Description}}
            <div class="description">{{.Description}}</div>
            {{else}}
            <div class="description no-description">No description provided</div>
            {{end}}
            <div class="file-location">{{.FilePath}}:{{.Line}}</div>
            {{if .Subtests}}
            <div class="subtests">
                <div class="subtests-header">Subtests:</div>
                {{range .Subtests}}
                <div class="subtest">
                    <span class="badge {{.Status}}">{{.Status}}</span>
                    <span class="subtest-name">{{.Name}}</span>
                    {{if gt .Duration 0.0}}
                    <span class="duration">{{printf "%.3fs" .Duration}}</span>
                    {{end}}
                    {{if and .Output (eq .Status "fail")}}
                    <details>
                        <summary>Output</summary>
                        <pre class="output fail">{{.Output}}</pre>
                    </details>
                    {{end}}
                </div>
                {{end}}
            </div>
            {{end}}
            {{if .SourceCode}}
            <details>
                <summary>View source code</summary>
                <pre>{{.SourceCode}}</pre>
            </details>
            {{end}}
            {{if and .Output (eq .Status "fail")}}
            <details open>
                <summary>Test output</summary>
                <pre class="output fail">{{.Output}}</pre>
            </details>
            {{end}}
        </div>
        {{end}}
    </section>
    {{end}}
    {{end}}

    <script>
    (function() {
        const filterInput = document.getElementById('test-filter');
        const searchInfo = document.getElementById('search-info');
        const packages = document.querySelectorAll('.package');
        const allTests = document.querySelectorAll('.test');
        const totalTests = allTests.length;

        filterInput.addEventListener('input', function() {
            const query = this.value.toLowerCase().trim();
            let visibleCount = 0;

            packages.forEach(function(pkg) {
                const tests = pkg.querySelectorAll('.test');
                let pkgHasVisible = false;

                tests.forEach(function(test) {
                    const testName = test.querySelector('.test-name').textContent.toLowerCase();
                    const matches = query === '' || testName.includes(query);

                    if (matches) {
                        test.classList.remove('hidden');
                        pkgHasVisible = true;
                        visibleCount++;
                    } else {
                        test.classList.add('hidden');
                    }
                });

                if (pkgHasVisible) {
                    pkg.classList.remove('hidden');
                } else {
                    pkg.classList.add('hidden');
                }
            });

            if (query === '') {
                searchInfo.textContent = '';
            } else {
                searchInfo.textContent = 'Showing ' + visibleCount + ' of ' + totalTests + ' tests';
            }
        });
    })();
    </script>
</body>
</html>
`))
