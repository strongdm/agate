package main

import (
	"os"

	"github.com/strongdm/agate/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		// If no command set a specific exit code, this is a Cobra-level
		// error (unknown command, bad flags, etc.). Print and exit 2.
		if cmd.GetExitCode() == 0 {
			cmd.PrintError("%v", err)
			os.Exit(2)
		}
	}
	os.Exit(cmd.GetExitCode())
}
