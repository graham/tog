package testdocs

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

// SubtestInfo contains information about a subtest (t.Run).
type SubtestInfo struct {
	Name string
	Line int
}

// TestInfo contains information about a single test function.
type TestInfo struct {
	Package     string
	Name        string
	Description string
	SourceCode  string
	FilePath    string
	Line        int
	Subtests    []SubtestInfo
}

// ParseTestFiles finds and parses all test files in the given directory tree.
func ParseTestFiles(root string) ([]TestInfo, error) {
	var tests []TestInfo

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, "_test.go") {
			return nil
		}

		fileTests, err := parseTestFile(path)
		if err != nil {
			return err
		}
		tests = append(tests, fileTests...)
		return nil
	})

	return tests, err
}

func parseTestFile(path string) ([]TestInfo, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, src, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	var tests []TestInfo
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		if !strings.HasPrefix(fn.Name.Name, "Test") {
			continue
		}
		// Skip helper functions (must have *testing.T or *testing.B as first param)
		if fn.Type.Params == nil || len(fn.Type.Params.List) == 0 {
			continue
		}

		info := TestInfo{
			Package:  file.Name.Name,
			Name:     fn.Name.Name,
			FilePath: path,
			Line:     fset.Position(fn.Pos()).Line,
		}

		// Extract doc comment
		if fn.Doc != nil {
			info.Description = strings.TrimSpace(fn.Doc.Text())
		}

		// Extract source code
		start := fset.Position(fn.Pos()).Offset
		end := fset.Position(fn.End()).Offset
		if end <= len(src) {
			info.SourceCode = string(src[start:end])
		}

		// Extract subtests (t.Run calls)
		info.Subtests = extractSubtests(fn.Body, fset)

		tests = append(tests, info)
	}

	return tests, nil
}

// extractSubtests finds all t.Run("name", ...) calls in a function body.
func extractSubtests(body *ast.BlockStmt, fset *token.FileSet) []SubtestInfo {
	if body == nil {
		return nil
	}

	var subtests []SubtestInfo

	ast.Inspect(body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		// Check if it's a method call (x.Run)
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "Run" {
			return true
		}

		// Check if first argument is a string literal (subtest name)
		if len(call.Args) < 1 {
			return true
		}

		lit, ok := call.Args[0].(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return true
		}

		// Remove quotes from string literal
		name := strings.Trim(lit.Value, `"'` + "`")
		subtests = append(subtests, SubtestInfo{
			Name: name,
			Line: fset.Position(call.Pos()).Line,
		})

		return true
	})

	return subtests
}

// TestInfoByPackage groups tests by their package name.
func TestInfoByPackage(tests []TestInfo) map[string][]TestInfo {
	grouped := make(map[string][]TestInfo)
	for _, t := range tests {
		grouped[t.Package] = append(grouped[t.Package], t)
	}
	return grouped
}
