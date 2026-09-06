package main

import (
	_ "embed"
	"os"
	"runtime/debug"
)

//go:embed VERSION
var version string

func main() {
	info, _ := debug.ReadBuildInfo()
	os.Exit(runCLI(os.Args[1:], info, os.Stdin, os.Stdout, os.Stderr))
}
