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
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	switch cmd {
	case "build":
		cmdBuild(args)
	case "flatten":
		cmdFlatten(args)
	case "init":
		cmdInit(args)
	case "maintain":
		cmdMaintain(args)
	default:
		fmt.Fprintf(os.Stderr, "未知命令: %s\n", cmd)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`StarSleep - 不可变系统分层构建工具

用法:
  starsleep <命令> [选项]

命令:
  build     分层构建系统快照
  flatten   部署快照到引导
  init      初始化工作环境
  maintain  动态维护模式（直接操作当前系统）

通用选项:
  -c,  --config <路径>   指定配置目录
  -cp, --copy <路径>     复制配置到默认路径后运行

build 选项:
  --clean    清理 layers 后从头构建
  --verify   构建后快照前校验展平一致性

flatten 选项:
  --list              列出已部署的引导条目
  --remove <名称>     移除引导条目
  <快照路径或名称>     要部署的快照 (默认: latest)`)
}
