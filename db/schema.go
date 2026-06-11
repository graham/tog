package db

import (
	"fmt"

	"github.com/lib/pq"
)

// TableInfo represents schema information for a database table.
type TableInfo struct {
	Name        string       `json:"name"`
	Columns     []ColumnInfo `json:"columns"`
	ForeignKeys []ForeignKey `json:"foreign_keys,omitempty"`
	Indexes     []IndexInfo  `json:"indexes,omitempty"`
}

// ColumnInfo represents schema information for a column.
type ColumnInfo struct {
	Name       string  `json:"name"`
	Type       string  `json:"type"`
	Nullable   bool    `json:"nullable"`
	PrimaryKey bool    `json:"primary_key"`
	Default    *string `json:"default"`
}

// ForeignKey represents a foreign key relationship.
type ForeignKey struct {
	Column           string `json:"column"`
	ReferencesTable  string `json:"references_table"`
	ReferencesColumn string `json:"references_column"`
	OnDelete         string `json:"on_delete,omitempty"`
	OnUpdate         string `json:"on_update,omitempty"`
}

// IndexInfo represents an index on a table.
type IndexInfo struct {
	Name    string   `json:"name"`
	Columns []string `json:"columns"`
	Unique  bool     `json:"unique"`
}

// SchemaInfo returns schema information for all tables in the database.
// Supports SQLite and PostgreSQL databases.
func (db *DB) SchemaInfo() ([]TableInfo, error) {
	tables, err := db.listTables()
	if err != nil {
		return nil, err
	}

	var result []TableInfo
	for _, tableName := range tables {
		tableInfo, err := db.TableSchema(tableName)
		if err != nil {
			return nil, fmt.Errorf("table %q: %w", tableName, err)
		}
		if tableInfo != nil {
			result = append(result, *tableInfo)
		}
	}

	return result, nil
}

// TableSchema returns schema information for a specific table.
// Returns nil if the table is not found.
func (db *DB) TableSchema(tableName string) (*TableInfo, error) {
	driver := db.DriverName()

	switch driver {
	case "sqlite3":
		return db.sqliteTableSchema(tableName)
	case "postgres", "pgx":
		return db.postgresTableSchema(tableName)
	default:
		return nil, fmt.Errorf("unsupported driver: %s", driver)
	}
}

// listTables returns a list of all table names in the database.
func (db *DB) listTables() ([]string, error) {
	driver := db.DriverName()

	switch driver {
	case "sqlite3":
		return db.sqliteListTables()
	case "postgres", "pgx":
		return db.postgresListTables()
	default:
		return nil, fmt.Errorf("unsupported driver: %s", driver)
	}
}

// sqliteListTables returns all table names from SQLite.
func (db *DB) sqliteListTables() ([]string, error) {
	query := `SELECT name FROM sqlite_master
	          WHERE type='table' AND name NOT LIKE 'sqlite_%'
	          ORDER BY name`

	var tables []string
	rows, err := db.Queryx(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		tables = append(tables, name)
	}
	return tables, rows.Err()
}

// postgresListTables returns all table names from PostgreSQL.
func (db *DB) postgresListTables() ([]string, error) {
	query := `SELECT table_name FROM information_schema.tables
	          WHERE table_schema = 'public'
	          ORDER BY table_name`

	var tables []string
	rows, err := db.Queryx(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		tables = append(tables, name)
	}
	return tables, rows.Err()
}

// sqliteTableSchema returns schema for a SQLite table.
func (db *DB) sqliteTableSchema(tableName string) (*TableInfo, error) {
	// PRAGMA table_info returns: cid, name, type, notnull, dflt_value, pk
	query := fmt.Sprintf("PRAGMA table_info('%s')", tableName)

	rows, err := db.Queryx(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var columns []ColumnInfo
	for rows.Next() {
		var cid int
		var name, colType string
		var notnull, pk int
		var dfltValue *string

		if err := rows.Scan(&cid, &name, &colType, &notnull, &dfltValue, &pk); err != nil {
			return nil, err
		}

		columns = append(columns, ColumnInfo{
			Name:       name,
			Type:       colType,
			Nullable:   notnull == 0,
			PrimaryKey: pk == 1,
			Default:    dfltValue,
		})
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Empty result means table doesn't exist
	if len(columns) == 0 {
		return nil, nil
	}

	// Get foreign keys
	foreignKeys, err := db.sqliteForeignKeys(tableName)
	if err != nil {
		return nil, fmt.Errorf("foreign keys: %w", err)
	}

	// Get indexes
	indexes, err := db.sqliteIndexes(tableName)
	if err != nil {
		return nil, fmt.Errorf("indexes: %w", err)
	}

	return &TableInfo{
		Name:        tableName,
		Columns:     columns,
		ForeignKeys: foreignKeys,
		Indexes:     indexes,
	}, nil
}

// sqliteForeignKeys returns foreign key relationships for a SQLite table.
func (db *DB) sqliteForeignKeys(tableName string) ([]ForeignKey, error) {
	// PRAGMA foreign_key_list returns: id, seq, table, from, to, on_update, on_delete, match
	query := fmt.Sprintf("PRAGMA foreign_key_list('%s')", tableName)

	rows, err := db.Queryx(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var fks []ForeignKey
	for rows.Next() {
		var id, seq int
		var table, from, to, onUpdate, onDelete, match string

		if err := rows.Scan(&id, &seq, &table, &from, &to, &onUpdate, &onDelete, &match); err != nil {
			return nil, err
		}

		fks = append(fks, ForeignKey{
			Column:           from,
			ReferencesTable:  table,
			ReferencesColumn: to,
			OnDelete:         onDelete,
			OnUpdate:         onUpdate,
		})
	}

	return fks, rows.Err()
}

// sqliteIndexes returns indexes for a SQLite table.
func (db *DB) sqliteIndexes(tableName string) ([]IndexInfo, error) {
	// PRAGMA index_list returns: seq, name, unique, origin, partial
	query := fmt.Sprintf("PRAGMA index_list('%s')", tableName)

	rows, err := db.Queryx(query)
	if err != nil {
		return nil, err
	}

	// Collect index metadata first, then close rows before making more queries
	type indexMeta struct {
		name   string
		unique bool
	}
	var metas []indexMeta

	for rows.Next() {
		var seq int
		var name, origin string
		var unique, partial int

		if err := rows.Scan(&seq, &name, &unique, &origin, &partial); err != nil {
			rows.Close()
			return nil, err
		}

		// Skip auto-generated indexes for primary keys and unique constraints
		if origin == "pk" {
			continue
		}

		metas = append(metas, indexMeta{name: name, unique: unique == 1})
	}

	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()

	// Now query for columns with rows closed
	var indexes []IndexInfo
	for _, meta := range metas {
		columns, err := db.sqliteIndexColumns(meta.name)
		if err != nil {
			return nil, fmt.Errorf("index %s columns: %w", meta.name, err)
		}

		indexes = append(indexes, IndexInfo{
			Name:    meta.name,
			Columns: columns,
			Unique:  meta.unique,
		})
	}

	return indexes, nil
}

// sqliteIndexColumns returns column names for a SQLite index.
func (db *DB) sqliteIndexColumns(indexName string) ([]string, error) {
	query := fmt.Sprintf("PRAGMA index_info('%s')", indexName)

	rows, err := db.Queryx(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var columns []string
	for rows.Next() {
		var seqno, cid int
		var name string

		if err := rows.Scan(&seqno, &cid, &name); err != nil {
			return nil, err
		}
		columns = append(columns, name)
	}

	return columns, rows.Err()
}

// postgresTableSchema returns schema for a PostgreSQL table.
func (db *DB) postgresTableSchema(tableName string) (*TableInfo, error) {
	// First check if table exists and get primary key columns
	pkQuery := `SELECT a.attname
	            FROM pg_index i
	            JOIN pg_attribute a ON a.attrelid = i.indrelid AND a.attnum = ANY(i.indkey)
	            WHERE i.indrelid = $1::regclass AND i.indisprimary`

	pkSet := make(map[string]bool)
	pkRows, err := db.Queryx(pkQuery, tableName)
	if err == nil {
		defer pkRows.Close()
		for pkRows.Next() {
			var colName string
			if err := pkRows.Scan(&colName); err == nil {
				pkSet[colName] = true
			}
		}
	}

	// Get column information
	query := `SELECT column_name, data_type, is_nullable, column_default
	          FROM information_schema.columns
	          WHERE table_schema = 'public' AND table_name = $1
	          ORDER BY ordinal_position`

	rows, err := db.Queryx(query, tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var columns []ColumnInfo
	for rows.Next() {
		var name, dataType, isNullable string
		var dfltValue *string

		if err := rows.Scan(&name, &dataType, &isNullable, &dfltValue); err != nil {
			return nil, err
		}

		columns = append(columns, ColumnInfo{
			Name:       name,
			Type:       dataType,
			Nullable:   isNullable == "YES",
			PrimaryKey: pkSet[name],
			Default:    dfltValue,
		})
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Empty result means table doesn't exist
	if len(columns) == 0 {
		return nil, nil
	}

	// Get foreign keys
	foreignKeys, err := db.postgresForeignKeys(tableName)
	if err != nil {
		return nil, fmt.Errorf("foreign keys: %w", err)
	}

	// Get indexes
	indexes, err := db.postgresIndexes(tableName)
	if err != nil {
		return nil, fmt.Errorf("indexes: %w", err)
	}

	return &TableInfo{
		Name:        tableName,
		Columns:     columns,
		ForeignKeys: foreignKeys,
		Indexes:     indexes,
	}, nil
}

// postgresForeignKeys returns foreign key relationships for a PostgreSQL table.
func (db *DB) postgresForeignKeys(tableName string) ([]ForeignKey, error) {
	query := `
		SELECT
			kcu.column_name,
			ccu.table_name AS references_table,
			ccu.column_name AS references_column,
			rc.delete_rule,
			rc.update_rule
		FROM information_schema.table_constraints tc
		JOIN information_schema.key_column_usage kcu
			ON tc.constraint_name = kcu.constraint_name
			AND tc.table_schema = kcu.table_schema
		JOIN information_schema.referential_constraints rc
			ON tc.constraint_name = rc.constraint_name
			AND tc.table_schema = rc.constraint_schema
		JOIN information_schema.constraint_column_usage ccu
			ON rc.unique_constraint_name = ccu.constraint_name
			AND rc.unique_constraint_schema = ccu.constraint_schema
		WHERE tc.constraint_type = 'FOREIGN KEY'
			AND tc.table_schema = 'public'
			AND tc.table_name = $1`

	rows, err := db.Queryx(query, tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var fks []ForeignKey
	for rows.Next() {
		var column, refTable, refColumn, onDelete, onUpdate string

		if err := rows.Scan(&column, &refTable, &refColumn, &onDelete, &onUpdate); err != nil {
			return nil, err
		}

		fks = append(fks, ForeignKey{
			Column:           column,
			ReferencesTable:  refTable,
			ReferencesColumn: refColumn,
			OnDelete:         onDelete,
			OnUpdate:         onUpdate,
		})
	}

	return fks, rows.Err()
}

// postgresIndexes returns indexes for a PostgreSQL table.
func (db *DB) postgresIndexes(tableName string) ([]IndexInfo, error) {
	query := `
		SELECT
			i.relname AS index_name,
			ix.indisunique AS is_unique,
			array_agg(a.attname ORDER BY array_position(ix.indkey, a.attnum)) AS columns
		FROM pg_class t
		JOIN pg_index ix ON t.oid = ix.indrelid
		JOIN pg_class i ON i.oid = ix.indexrelid
		JOIN pg_attribute a ON a.attrelid = t.oid AND a.attnum = ANY(ix.indkey)
		WHERE t.relkind = 'r'
			AND t.relname = $1
			AND NOT ix.indisprimary
		GROUP BY i.relname, ix.indisunique
		ORDER BY i.relname`

	rows, err := db.Queryx(query, tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var indexes []IndexInfo
	for rows.Next() {
		var name string
		var unique bool
		var columns []string

		if err := rows.Scan(&name, &unique, pq.Array(&columns)); err != nil {
			return nil, err
		}

		indexes = append(indexes, IndexInfo{
			Name:    name,
			Columns: columns,
			Unique:  unique,
		})
	}

	return indexes, rows.Err()
}
