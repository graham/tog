package app

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"github.com/graham/tog/db"
)

// SchemaOutput represents the full schema output for all databases.
type SchemaOutput struct {
	Databases map[string]DatabaseSchema `json:"databases"`
}

// DatabaseSchema represents the schema for a single database.
type DatabaseSchema struct {
	Tables []db.TableInfo `json:"tables"`
}

func cmdSchema(cfg Config, args []string) {
	fs := newFlagSet(cfg.Name, "schema", "Dump database schema as JSON.")
	fs.Parse(args)

	dbm, err := db.NewManagerFromEnvOrFile("")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to connect to database: %v\n", err)
		os.Exit(1)
	}
	defer dbm.Close()

	output := SchemaOutput{
		Databases: make(map[string]DatabaseSchema),
	}

	names := dbm.Names()
	sort.Strings(names)

	for _, name := range names {
		database := dbm.Get(name)
		if database == nil {
			continue
		}

		tables, err := database.SchemaInfo()
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to fetch schema for database %q: %v\n", name, err)
			os.Exit(1)
		}

		output.Databases[name] = DatabaseSchema{
			Tables: tables,
		}
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(output); err != nil {
		fmt.Fprintf(os.Stderr, "failed to encode JSON: %v\n", err)
		os.Exit(1)
	}
}
