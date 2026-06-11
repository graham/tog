package testkit

import (
	"context"
	"database/sql"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/graham/tog/tools/pg2sqlite"
	"github.com/pressly/goose/v3"
)

// RunMigrations applies all migrations from the given directory to the database.
// The migrationsDir should be a path relative to the calling test file,
// e.g., "../../migrations" when called from internal/routes/.
// Migrations are read from the postgres/ subdirectory and translated to SQLite on-the-fly.
func RunMigrations(sqlDB *sql.DB, migrationsDir string) error {
	return RunMigrationsWithDialect(sqlDB, migrationsDir, "sqlite3")
}

// RunMigrationsWithDialect applies all migrations using the specified dialect.
// Dialect should be "sqlite3" or "postgres".
// Migrations are always stored in migrations/postgres/ and translated at runtime for SQLite.
func RunMigrationsWithDialect(sqlDB *sql.DB, migrationsDir string, dialect string) error {
	var store goose.Dialect
	var fsys fs.FS

	// Always read from postgres directory
	postgresDir := filepath.Join(migrationsDir, "postgres")

	switch dialect {
	case "postgres":
		store = goose.DialectPostgres
		fsys = os.DirFS(postgresDir)
	default:
		store = goose.DialectSQLite3
		// Use translating filesystem for SQLite
		fsys = &translatingFS{
			base:       os.DirFS(postgresDir),
			translator: pg2sqlite.NewTranslator(),
		}
	}

	provider, err := goose.NewProvider(store, sqlDB, fsys)
	if err != nil {
		return err
	}

	_, err = provider.Up(context.Background())
	return err
}

// RunMigrationsForDriver applies migrations using the driver name to select dialect.
// This is useful when you have a *sql.DB and know its driver name.
// driverName should be "sqlite3" or "postgres".
func RunMigrationsForDriver(sqlDB *sql.DB, migrationsDir string, driverName string) error {
	dialect := "sqlite3"
	if driverName == "postgres" || driverName == "pgx" {
		dialect = "postgres"
	}
	return RunMigrationsWithDialect(sqlDB, migrationsDir, dialect)
}

// translatingFS wraps a base filesystem and translates PostgreSQL SQL to SQLite on read.
type translatingFS struct {
	base       fs.FS
	translator *pg2sqlite.Translator
}

func (t *translatingFS) Open(name string) (fs.File, error) {
	f, err := t.base.Open(name)
	if err != nil {
		return nil, err
	}

	// Only translate .sql files
	if !strings.HasSuffix(name, ".sql") {
		return f, nil
	}

	// Read and translate the content
	stat, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, err
	}

	if stat.IsDir() {
		return f, nil
	}

	// Translate the content
	translated, err := t.translator.Translate(f)
	f.Close()
	if err != nil {
		return nil, err
	}

	return &translatedFile{
		name:    name,
		content: translated,
		info:    stat,
	}, nil
}

func (t *translatingFS) ReadDir(name string) ([]fs.DirEntry, error) {
	if rd, ok := t.base.(fs.ReadDirFS); ok {
		return rd.ReadDir(name)
	}
	// Fallback: open and read directory
	f, err := t.base.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	if d, ok := f.(fs.ReadDirFile); ok {
		return d.ReadDir(-1)
	}
	return nil, &fs.PathError{Op: "readdir", Path: name, Err: fs.ErrInvalid}
}

// translatedFile is an in-memory file with translated content.
type translatedFile struct {
	name    string
	content string
	info    fs.FileInfo
	offset  int
}

func (f *translatedFile) Stat() (fs.FileInfo, error) {
	return &translatedFileInfo{
		name: f.name,
		size: int64(len(f.content)),
		mode: f.info.Mode(),
	}, nil
}

func (f *translatedFile) Read(b []byte) (int, error) {
	if f.offset >= len(f.content) {
		return 0, io.EOF
	}
	n := copy(b, f.content[f.offset:])
	f.offset += n
	return n, nil
}

func (f *translatedFile) Close() error {
	return nil
}

// translatedFileInfo implements fs.FileInfo for translated files.
type translatedFileInfo struct {
	name string
	size int64
	mode fs.FileMode
}

func (fi *translatedFileInfo) Name() string       { return filepath.Base(fi.name) }
func (fi *translatedFileInfo) Size() int64        { return fi.size }
func (fi *translatedFileInfo) Mode() fs.FileMode  { return fi.mode }
func (fi *translatedFileInfo) ModTime() time.Time { return time.Time{} }
func (fi *translatedFileInfo) IsDir() bool        { return false }
func (fi *translatedFileInfo) Sys() any           { return nil }
