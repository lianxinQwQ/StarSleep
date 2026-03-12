// cmd_compare.go — starsleep compare 命令
//
// 对比当前系统与声明式配置之间的差异，支持两种对比模式:
//
//   - 包列表对比（--packages）:
//     仅对比主动安装的软件包列表与配置定义的期望包列表，
//     不实际构建系统，轻量快速。
//
//   - 文件对比（--files <目录>）:
//     基于最近一次构建快照，应用继承列表后与指定目录进行
//     文件级别的差异对比，无需重新构建。
//
// 用法:
//
//	starsleep compare --packages [-c <配置目录>] [--verbose/-v]
//	starsleep compare --files <目标目录> [-c <配置目录>]
package main

import (
	"starsleep/internal/compare"
	"starsleep/internal/config"
	"starsleep/internal/i18n"
	"starsleep/internal/util"
)

// compareMode 对比模式枚举
type compareMode int

const (
	// compareModeNone 未指定对比模式
	compareModeNone compareMode = iota
	// compareModePackages 包列表对比模式
	compareModePackages
	// compareModeFiles 文件对比模式
	compareModeFiles
)

// cmdCompare 执行对比命令
//
// 解析命令行参数，选择对比模式并分发到 internal/compare 包:
//   - --packages: 包列表对比，不实际构建
//   - --files <目录>: 基于快照的文件对比
//   - --verbose / -v: 详细输出（显示包组状态、无差异提示等）
//
// @param args 命令行参数列表
// @throws 参数不合法、配置加载失败等情况下调用 Fatal 退出
func cmdCompare(args []string) {
	util.CheckRoot()

	// 解析 -c/--config 标志，提取配置目录路径
	configDir, remaining := config.ParseConfigFlags(defaultConfigDir, args)

	// ── 解析 compare 专用参数 ──
	mode := compareModeNone
	verbose := false  // 详细模式标志
	filesTarget := "" // --files 模式的目标目录
	for i := 0; i < len(remaining); i++ {
		switch remaining[i] {
		case "--packages":
			mode = compareModePackages
		case "--files":
			mode = compareModeFiles
			i++
			if i < len(remaining) {
				filesTarget = remaining[i]
			} else {
				util.Fatal(i18n.T("compare.files.need.dir"))
			}
		case "--verbose", "-v":
			verbose = true
		default:
			util.Fatal(i18n.T("compare.unknown.arg", remaining[i]))
		}
	}

	// 未指定模式时打印用法并退出
	if mode == compareModeNone {
		util.Fatal(i18n.T("compare.usage"))
	}

	// 加载配置
	layers, mainCfg, err := config.LoadAllLayers(configDir)
	if err != nil {
		util.Fatal(i18n.T("load.config.failed", err))
	}

	// 根据模式分发到 internal/compare 包
	switch mode {
	case compareModePackages:
		compare.Packages(layers, configDir, verbose)
	case compareModeFiles:
		compare.Files(filesTarget, defaultWorkDir, func(snapshotDir string) {
			applyInheritList(mainCfg, snapshotDir)
		})
	}
}
