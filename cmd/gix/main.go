package main

import (
	"os"

	"github.com/m-mdy-m/gix/internal/commands"
	"github.com/m-mdy-m/gix/internal/ui"
)

func main() {
	if err := commands.New().Execute(); err != nil {
		ui.Error("%s", err)
		os.Exit(1)
	}
}
