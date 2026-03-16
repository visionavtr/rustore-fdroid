package main

import (
	"os"

	"github.com/visionavtr/rustore-fdroid/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
