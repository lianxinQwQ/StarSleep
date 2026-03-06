package main

import (
	"fmt"
	"os"

	"starsleep/internal/i18n"
)

const (
	defaultWorkDir   = "/starsleep"
	defaultConfigDir = "/starsleep/config"
)

func main() {
	i18n.Init()
	args := i18n.ExtractGlobalFlags(os.Args[1:])

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
		fmt.Fprintln(os.Stderr, i18n.T("unknown.cmd", cmd))
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(i18n.T("usage"))
}
