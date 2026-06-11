package app

import (
	"fmt"
	"os"
	"os/exec"
)

func cmdWatch(cfg Config, args []string) {
	fs := newFlagSet(cfg.Name, "watch", "Run the server with auto-reload on file changes.\n\nRequires air to be installed:\n  go install github.com/air-verse/air@latest")
	fs.Parse(args)

	// Check if air is installed
	airPath, err := exec.LookPath("air")
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error: 'air' is not installed.")
		fmt.Fprintln(os.Stderr, "\nInstall it with:")
		fmt.Fprintln(os.Stderr, "  go install github.com/air-verse/air@latest")
		os.Exit(1)
	}

	// Run air with the config file
	cmd := exec.Command(airPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		fmt.Fprintf(os.Stderr, "air error: %v\n", err)
		os.Exit(1)
	}
}
