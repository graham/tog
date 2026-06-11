// Example tog application demonstrating the framework usage.
//
// Usage:
//
//	go run . serve          # Run the HTTP server
//	go run . watch          # Run with auto-reload on file changes
//	go run . verify         # Verify all queries against database
//	go run . routes         # List all registered routes
//	go run . routes -md     # Generate markdown documentation
//	go run . testdocs       # Generate test documentation HTML
//	go run . findqueries    # Find unregistered SQL queries
package main

import (
	"github.com/graham/tog/app"
	"github.com/graham/tog/examples/routes"
)

func main() {
	app.Run(app.Config{
		Name:       "examples",
		LoadRoutes: routes.LoadRoutes,
		Testdocs: &app.TestdocsConfig{
			PkgPattern: "github.com/graham/tog/...",
			RootDir:    "..",
		},
	})
}
