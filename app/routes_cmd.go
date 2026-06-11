package app

import (
	"fmt"
	"os"

	"github.com/graham/tog/tools/routes"
)

func cmdRoutes(cfg Config, args []string) {
	fs := newFlagSet(cfg.Name, "routes", "List all registered HTTP routes.")
	showAll := fs.Bool("all", false, "show all routes including middleware internals")
	markdown := fs.Bool("md", false, "output as markdown")
	fs.Parse(args)

	router, dbm, err := createRouter(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create router: %v\n", err)
		os.Exit(1)
	}
	defer dbm.Close()

	routesCfg := routes.Config{
		ShowAll: *showAll,
	}

	routeInfos := routes.CollectRoutes(router.Mux, routesCfg)

	if *markdown {
		content := routes.FormatMarkdown(routeInfos)
		if err := os.WriteFile("api.md", []byte(content), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "failed to write api.md: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Generated api.md")
	} else {
		fmt.Print(routes.FormatTable(routeInfos))
	}
}
