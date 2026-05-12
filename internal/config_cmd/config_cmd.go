// config_cmd 包实现 starsleep config 子命令。
//
// 提供三个操作：
//   - --collect            将 config.yaml inherit 列表中的宿主机文件收集到 configDir/inherit/
//   - --export <file>      将整个配置目录打包为 .tar.gz 压缩包
//   - --import <file>      从 .tar.gz 压缩包还原配置目录
package config_cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"starsleep/internal/config"
	"starsleep/internal/i18n"
	"starsleep/internal/util"
)

// Run 执行 config 子命令
func Run(args []string) {
	util.CheckRoot()
	configDir, remaining := config.ParseConfigFlags(config.DefaultConfigDir, args)

	if len(remaining) == 0 {
		util.Fatal(i18n.T("config.usage"))
	}

	switch remaining[0] {
	case "--collect":
		runCollect(configDir)
	case "--export":
		if len(remaining) < 2 {
			util.Fatal(i18n.T("config.export.usage"))
		}
		runExport(configDir, remaining[1])
	case "--import":
		if len(remaining) < 2 {
			util.Fatal(i18n.T("config.import.usage"))
		}
		runImport(configDir, remaining[1])
	default:
		util.Fatal(i18n.T("config.unknown.arg", remaining[0]))
	}
}

// runCollect 将 inherit 列表中的文件从宿主机复制到 configDir/inherit/，保留绝对路径结构
func runCollect(configDir string) {
	mc, err := config.LoadMainConfig(configDir)
	if err != nil {
		util.Fatal(i18n.T("load.config.failed", err))
	}
	paths := config.LoadInheritList(mc)
	if len(paths) == 0 {
		fmt.Println(i18n.T("inherit.not.found"))
		return
	}

	storeDir := filepath.Join(configDir, config.InheritDir)
	os.MkdirAll(storeDir, 0o755)

	fmt.Println(i18n.T("config.collect.start", len(paths)))
	for _, p := range paths {
		fi, err := os.Stat(p)
		if err != nil {
			fmt.Println(i18n.T("inherit.path.missing", p))
			continue
		}

		dst := filepath.Join(storeDir, p)
		os.MkdirAll(filepath.Dir(dst), 0o755)

		if fi.IsDir() {
			if err := util.Run("cp", "-ax", "--reflink=auto", p, filepath.Dir(dst)+"/"); err != nil {
				fmt.Println(i18n.T("config.collect.copy.failed", p, err))
			} else {
				fmt.Println(i18n.T("config.collect.copied", p))
			}
		} else {
			if err := util.Run("cp", "-a", "--reflink=auto", p, dst); err != nil {
				fmt.Println(i18n.T("config.collect.copy.failed", p, err))
			} else {
				fmt.Println(i18n.T("config.collect.copied", p))
			}
		}
	}
	fmt.Println(i18n.T("config.collect.done"))
}

// runExport 将整个 configDir 打包为 .tar.gz 文件
func runExport(configDir string, output string) {
	// 使用绝对路径避免歧义
	absOutput, err := filepath.Abs(output)
	if err != nil {
		util.Fatal(i18n.T("config.export.failed", err))
	}

	fmt.Println(i18n.T("config.export.start", absOutput))
	parent := filepath.Dir(filepath.Clean(configDir))
	base := filepath.Base(filepath.Clean(configDir))
	if err := util.Run("tar", "-czf", absOutput, "-C", parent, base); err != nil {
		util.Fatal(i18n.T("config.export.failed", err))
	}
	fmt.Println(i18n.T("config.export.done", absOutput))
}

// runImport 将 .tar.gz 压缩包解压到 configDir 所在的父目录
func runImport(configDir string, input string) {
	absInput, err := filepath.Abs(input)
	if err != nil {
		util.Fatal(i18n.T("config.import.failed", err))
	}

	fmt.Println(i18n.T("config.import.start", absInput))
	parent := filepath.Dir(filepath.Clean(configDir))
	os.MkdirAll(parent, 0o755)
	if err := util.Run("tar", "-xzf", absInput, "-C", parent); err != nil {
		util.Fatal(i18n.T("config.import.failed", err))
	}
	fmt.Println(i18n.T("config.import.done"))
}
