// cmd.go — 子命令分发
package main

import (
	"starsleep/internal/build"
	"starsleep/internal/compare"
	"starsleep/internal/deploy"
	"starsleep/internal/init_env"
	"starsleep/internal/maintain"
	"starsleep/internal/verify"
)

// dispatch 将子命令分发到对应业务包
func dispatch(cmd string, args []string) bool {
	switch cmd {
	case "build":
		build.Run(args)
	case "compare":
		compare.Run(args)
	case "flatten":
		deploy.Run(args)
	case "init":
		init_env.Run(args)
	case "maintain":
		maintain.Run(args)
	case "verify":
		verify.Run(args)
	default:
		return false
	}
	return true
}
