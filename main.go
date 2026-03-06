package main

import (
	"fmt"
	"os"
)

const (
	defaultWorkDir   = "/starsleep"
	defaultConfigDir = "/starsleep/config"
)

func main() {
	initI18n()
	args := extractGlobalFlags(os.Args[1:])

	if len(args) < 1 {
		printUsage()
		os.Exit(1)
	}

	cmd := args[0]
	cmdArgs := args[1:]

	switch cmd {
	case "build":
		cmdBuild(cmdArgs)
	case "flatten":
		cmdFlatten(cmdArgs)
	case "init":
		cmdInit(cmdArgs)
	case "maintain":
		cmdMaintain(cmdArgs)
	case "verify":
		cmdVerify(cmdArgs)
	default:
		fmt.Fprintln(os.Stderr, T("unknown.cmd", cmd))
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(T("usage"))
}
