// Package pg2sqlite translates PostgreSQL SQL to SQLite3-compatible SQL.
package pg2sqlite

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Translator converts PostgreSQL SQL to SQLite3 SQL.
type Translator struct {
	// SkipPatterns are regex patterns for lines to skip entirely
	SkipPatterns []*regexp.Regexp
}

// NewTranslator creates a new translator with default skip patterns.
func NewTranslator() *Translator {
	return &Translator{
		SkipPatterns: []*regexp.Regexp{
			// Skip PostgreSQL sequence manipulation
			regexp.MustCompile(`(?i)^\s*SELECT\s+setval\s*\(`),
			// Skip ALTER SEQUENCE statements
			regexp.MustCompile(`(?i)^\s*ALTER\s+SEQUENCE\s+`),
		},
	}
}

// TranslateFile reads a PostgreSQL SQL file and returns SQLite3-compatible SQL.
func (t *Translator) TranslateFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	return t.Translate(f)
}

// Translate reads PostgreSQL SQL and returns SQLite3-compatible SQL.
func (t *Translator) Translate(r io.Reader) (string, error) {
	var result strings.Builder
	scanner := bufio.NewScanner(r)

	// Track if we're inside a CREATE TABLE statement for foreign key extraction
	var inCreateTable bool
	var tableName string
	var foreignKeys []string
	var tableLines []string

	// Track if we're inside an INSERT statement for TRUE/FALSE translation
	var inInsert bool

	for scanner.Scan() {
		line := scanner.Text()

		// Check skip patterns
		skip := false
		for _, pattern := range t.SkipPatterns {
			if pattern.MatchString(line) {
				skip = true
				break
			}
		}
		if skip {
			continue
		}

		// Track INSERT statements
		upperLine := strings.ToUpper(line)
		if strings.Contains(upperLine, "INSERT") {
			inInsert = true
		}

		translated := t.translateLine(line, inInsert)

		// End INSERT statement on semicolon
		if inInsert && strings.Contains(line, ";") {
			inInsert = false
		}

		// Track CREATE TABLE statements for foreign key handling
		if strings.Contains(upperLine, "CREATE TABLE") {
			inCreateTable = true
			tableName = extractTableName(line)
			foreignKeys = nil
			tableLines = nil
		}

		if inCreateTable {
			// Extract inline REFERENCES and convert to FOREIGN KEY
			if fk := extractInlineFK(translated, tableName); fk != "" {
				foreignKeys = append(foreignKeys, fk)
				// Remove the REFERENCES clause from the line
				translated = removeInlineReferences(translated)
			}
			tableLines = append(tableLines, translated)

			// Check for end of CREATE TABLE
			if strings.Contains(line, ");") {
				// Insert foreign keys before the closing );
				if len(foreignKeys) > 0 {
					// Find the last line with content before );
					for i := len(tableLines) - 1; i >= 0; i-- {
						if strings.Contains(tableLines[i], ");") {
							// Insert foreign keys
							fkLines := strings.Join(foreignKeys, ",\n")
							// Replace ); with ,\n<fks>\n);
							tableLines[i] = strings.Replace(tableLines[i], ");", ",\n"+fkLines+"\n);", 1)
							break
						}
					}
				}
				// Write all table lines
				for _, tl := range tableLines {
					result.WriteString(tl)
					result.WriteString("\n")
				}
				inCreateTable = false
				continue
			}
		} else {
			result.WriteString(translated)
			result.WriteString("\n")
		}
	}

	if err := scanner.Err(); err != nil {
		return "", err
	}

	return result.String(), nil
}

// translateLine applies all translation rules to a single line.
// inInsert indicates if we're inside an INSERT statement (for TRUE/FALSE translation).
func (t *Translator) translateLine(line string, inInsert bool) string {
	// Preserve goose directives and comments
	trimmed := strings.TrimSpace(line)
	if strings.HasPrefix(trimmed, "--") {
		return line
	}

	result := line

	// SERIAL PRIMARY KEY -> INTEGER PRIMARY KEY
	result = regexp.MustCompile(`(?i)\bSERIAL\s+PRIMARY\s+KEY\b`).ReplaceAllString(result, "INTEGER PRIMARY KEY")

	// SERIAL -> INTEGER (standalone)
	result = regexp.MustCompile(`(?i)\bSERIAL\b`).ReplaceAllString(result, "INTEGER")

	// BIGSERIAL -> INTEGER
	result = regexp.MustCompile(`(?i)\bBIGSERIAL\b`).ReplaceAllString(result, "INTEGER")

	// BOOLEAN DEFAULT TRUE -> INTEGER DEFAULT 1
	result = regexp.MustCompile(`(?i)\bBOOLEAN\s+DEFAULT\s+TRUE\b`).ReplaceAllString(result, "INTEGER DEFAULT 1")

	// BOOLEAN DEFAULT FALSE -> INTEGER DEFAULT 0
	result = regexp.MustCompile(`(?i)\bBOOLEAN\s+DEFAULT\s+FALSE\b`).ReplaceAllString(result, "INTEGER DEFAULT 0")

	// BOOLEAN -> INTEGER (standalone)
	result = regexp.MustCompile(`(?i)\bBOOLEAN\b`).ReplaceAllString(result, "INTEGER")

	// TIMESTAMP WITH TIME ZONE -> TEXT
	result = regexp.MustCompile(`(?i)\bTIMESTAMP\s+WITH\s+TIME\s+ZONE\b`).ReplaceAllString(result, "TEXT")

	// TIMESTAMP WITHOUT TIME ZONE -> TEXT
	result = regexp.MustCompile(`(?i)\bTIMESTAMP\s+WITHOUT\s+TIME\s+ZONE\b`).ReplaceAllString(result, "TEXT")

	// TIMESTAMP -> TEXT
	result = regexp.MustCompile(`(?i)\bTIMESTAMP\b`).ReplaceAllString(result, "TEXT")

	// NUMERIC(x,y) -> REAL
	result = regexp.MustCompile(`(?i)\bNUMERIC\s*\([^)]+\)`).ReplaceAllString(result, "REAL")

	// NUMERIC -> REAL
	result = regexp.MustCompile(`(?i)\bNUMERIC\b`).ReplaceAllString(result, "REAL")

	// DECIMAL(x,y) -> REAL
	result = regexp.MustCompile(`(?i)\bDECIMAL\s*\([^)]+\)`).ReplaceAllString(result, "REAL")

	// DECIMAL -> REAL
	result = regexp.MustCompile(`(?i)\bDECIMAL\b`).ReplaceAllString(result, "REAL")

	// DOUBLE PRECISION -> REAL
	result = regexp.MustCompile(`(?i)\bDOUBLE\s+PRECISION\b`).ReplaceAllString(result, "REAL")

	// EXTRACT(EPOCH FROM NOW())::INTEGER -> (strftime('%s', 'now'))
	result = regexp.MustCompile(`(?i)EXTRACT\s*\(\s*EPOCH\s+FROM\s+NOW\s*\(\s*\)\s*\)\s*::\s*INTEGER`).ReplaceAllString(result, "(strftime('%s', 'now'))")

	// NOW() -> CURRENT_TIMESTAMP (for non-epoch contexts)
	result = regexp.MustCompile(`(?i)\bNOW\s*\(\s*\)`).ReplaceAllString(result, "CURRENT_TIMESTAMP")

	// VARCHAR(n) -> TEXT
	result = regexp.MustCompile(`(?i)\bVARCHAR\s*\([^)]+\)`).ReplaceAllString(result, "TEXT")

	// VARCHAR -> TEXT
	result = regexp.MustCompile(`(?i)\bVARCHAR\b`).ReplaceAllString(result, "TEXT")

	// CHAR(n) -> TEXT
	result = regexp.MustCompile(`(?i)\bCHAR\s*\([^)]+\)`).ReplaceAllString(result, "TEXT")

	// BYTEA -> BLOB
	result = regexp.MustCompile(`(?i)\bBYTEA\b`).ReplaceAllString(result, "BLOB")

	// UUID -> TEXT
	result = regexp.MustCompile(`(?i)\bUUID\b`).ReplaceAllString(result, "TEXT")

	// JSONB -> TEXT
	result = regexp.MustCompile(`(?i)\bJSONB\b`).ReplaceAllString(result, "TEXT")

	// JSON -> TEXT
	result = regexp.MustCompile(`(?i)\bJSON\b`).ReplaceAllString(result, "TEXT")

	// In INSERT statements: TRUE -> 1, FALSE -> 0
	// Only do this for INSERT statements to avoid breaking DEFAULT TRUE/FALSE (already handled above)
	if inInsert {
		// Match TRUE/FALSE as whole words, case insensitive
		result = regexp.MustCompile(`(?i)\bTRUE\b`).ReplaceAllString(result, "1")
		result = regexp.MustCompile(`(?i)\bFALSE\b`).ReplaceAllString(result, "0")
	}

	// DROP TABLE IF EXISTS -> DROP TABLE IF EXISTS (already compatible, but ensure order)
	// Note: SQLite requires dropping in reverse dependency order

	return result
}

// extractTableName extracts the table name from a CREATE TABLE statement.
func extractTableName(line string) string {
	re := regexp.MustCompile(`(?i)CREATE\s+TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?(\w+)`)
	matches := re.FindStringSubmatch(line)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

// extractInlineFK checks if a line has an inline REFERENCES clause and returns a FOREIGN KEY constraint.
func extractInlineFK(line, _ string) string {
	// Match: column_name TYPE ... REFERENCES other_table(column)
	re := regexp.MustCompile(`(?i)(\w+)\s+\w+.*?\bREFERENCES\s+(\w+)\s*\((\w+)\)`)
	matches := re.FindStringSubmatch(line)
	if len(matches) > 3 {
		columnName := matches[1]
		refTable := matches[2]
		refColumn := matches[3]
		return fmt.Sprintf("    FOREIGN KEY (%s) REFERENCES %s(%s)", columnName, refTable, refColumn)
	}
	return ""
}

// removeInlineReferences removes the REFERENCES clause from a column definition.
func removeInlineReferences(line string) string {
	// Remove REFERENCES table(column) from the line
	re := regexp.MustCompile(`(?i)\s*REFERENCES\s+\w+\s*\(\w+\)`)
	return re.ReplaceAllString(line, "")
}

// TranslateDirectory translates all .sql files from srcDir to dstDir.
func TranslateDirectory(srcDir, dstDir string) error {
	t := NewTranslator()

	// Ensure destination directory exists
	if err := os.MkdirAll(dstDir, 0755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	// Find all .sql files in source directory
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return fmt.Errorf("failed to read source directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}

		srcPath := filepath.Join(srcDir, entry.Name())
		dstPath := filepath.Join(dstDir, entry.Name())

		translated, err := t.TranslateFile(srcPath)
		if err != nil {
			return fmt.Errorf("failed to translate %s: %w", srcPath, err)
		}

		if err := os.WriteFile(dstPath, []byte(translated), 0644); err != nil {
			return fmt.Errorf("failed to write %s: %w", dstPath, err)
		}

		fmt.Printf("Translated: %s -> %s\n", srcPath, dstPath)
	}

	return nil
}
