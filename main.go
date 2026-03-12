// main.go — StarSleep 主入口
//
// StarSleep 是一个不可变系统分层构建工具，通过 OverlayFS 逐层构建、
// reflink 展平合并并生成 Btrfs 快照，实现声明式的系统配置管理。
// 本文件负责解析命令行参数并分发到对应的子命令处理函数。
package main

import (
	"fmt"
	"os"

	"starsleep/internal/i18n"
)

const (
	// defaultWorkDir 默认工作目录，存放 layers、snapshots、work 等子目录
	defaultWorkDir = DefaultWorkDir
	// defaultConfigDir 默认配置目录，存放 config.yaml、layers/、files/
	defaultConfigDir = DefaultConfigDir
)

// main 程序入口
//
// 初始化 i18n 国际化模块，解析全局标志（如 --lang），
// 然后根据第一个位置参数分发到对应的子命令。
//
// 支持的子命令:
//   - build:    分层构建系统快照
//   - compare:  对比当前系统与配置差异
//   - flatten:  部署快照到引导分区
//   - init:     初始化工作环境
//   - maintain: 动态维护模式（直接操作当前系统）
//   - verify:   展平一致性校验
func main() {
	// 初始化 i18n 国际化，自动检测系统语言
	i18n.Init()
	// 提取全局标志（--lang 等），返回剩余参数
	args := i18n.ExtractGlobalFlags(os.Args[1:])

	if len(args) < 1 {
		printUsage()
		os.Exit(1)
	}

	cmd := args[0]
	cmdArgs := args[1:]

	// 根据子命令名称分发到对应处理函数
	switch cmd {
	case "build":
		cmdBuild(cmdArgs)
	case "compare":
		cmdCompare(cmdArgs)
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

// printUsage 打印程序使用帮助信息
func printUsage() {
	fmt.Println(i18n.T("usage"))
}
