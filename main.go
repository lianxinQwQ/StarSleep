// main.go — StarSleep 主入口
package main

import (
	"fmt"
	"os"

	"starsleep/internal/i18n"
)

func main() {
	i18n.Init()
	args := i18n.ExtractGlobalFlags(os.Args[1:])

	if len(args) < 1 {
		printUsage()
		os.Exit(1)
	}

	if !dispatch(args[0], args[1:]) {
		fmt.Fprintln(os.Stderr, i18n.T("unknown.cmd", args[0]))
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(i18n.T("usage"))
}
