package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/leo-ramos/als/internal/cli"
	"github.com/leo-ramos/als/internal/config"
	"github.com/leo-ramos/als/internal/exec"
	"github.com/leo-ramos/als/internal/store"
)

func main() {
	base := filepath.Base(os.Args[0])

	// If invoked directly as "als" or "main" (e.g. go run)
	if base == "als" || base == "main" || strings.HasPrefix(base, "als-") {
		cli.Execute()
		return
	}

	// Invoked as a Shim via symlink (e.g. "connectServerProd")
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "als error: %v\n", err)
		os.Exit(1)
	}

	st := store.New(cfg)
	runner := exec.NewRunner(st)

	if err := runner.RunShim(base, os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "als: %v\n", err)
		os.Exit(1)
	}
}
