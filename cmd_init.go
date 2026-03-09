// cmd_init.go — starsleep init 命令
//
// 初始化 StarSleep 工作环境：创建所需目录结构、检查依赖工具。
// 必须在首次使用 starsleep build 之前运行。
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"starsleep/internal/config"
	"starsleep/internal/i18n"
	"starsleep/internal/util"
)

// cmdInit 执行环境初始化命令
//
// 初始化步骤:
//  1. 检查 root 权限
//  2. 检查所有必要的外部工具是否安装（pacstrap、btrfs 等）
//  3. 检查 overlay 内核模块是否可加载
//  4. 创建所有必要的目录结构
//  5. 可选复制外部配置到默认位置
//
// @param args 命令行参数，支持 -c/--config 指定配置目录
// @throws 缺少依赖时打印列表并退出
func cmdInit(args []string) {
	util.CheckRoot()
	configDir, remaining := config.ParseConfigFlags(defaultConfigDir, args)
	if len(remaining) > 0 {
		util.Fatal(i18n.T("init.unknown.arg", remaining[0]))
	}
	workDir := defaultWorkDir

	// ── 检查必要的外部工具 ──
	requiredCmds := []string{
		"pacstrap", "pacman", "arch-chroot",
		"mount", "umount", "btrfs", "getfattr", "rsync", "setfattr",
	}
	var missing []string
	for _, cmd := range requiredCmds {
		if _, err := exec.LookPath(cmd); err != nil {
			missing = append(missing, cmd)
		}
	}
	// 检查 overlay 内核模块
	if err := util.Run("modprobe", "overlay"); err != nil {
		missing = append(missing, "overlay (kernel module)")
	}
	if len(missing) > 0 {
		fmt.Fprintln(os.Stderr, i18n.T("missing.deps"))
		for _, m := range missing {
			fmt.Fprintf(os.Stderr, "  - %s\n", m)
		}
		os.Exit(1)
	}

	fmt.Println(i18n.T("init.start"))
	fmt.Println(i18n.T("init.workdir", workDir))

	// ── 创建工作目录结构 ──
	dirs := []string{
		filepath.Join(workDir, "layers"),              // 各层 diff 数据
		filepath.Join(workDir, "snapshots"),           // 生产快照
		filepath.Join(workDir, "shared/home"),         // 用户家目录持久化
		filepath.Join(workDir, "shared/pacman-cache"), // pacman 包缓存共享
		filepath.Join(workDir, "config/layers"),       // 层定义 YAML
		filepath.Join(workDir, "work/merged"),         // OverlayFS 合并挂载点
		filepath.Join(workDir, "work/ovl_work"),       // OverlayFS 工作目录
		filepath.Join(workDir, "logs"),                // 构建日志
	}
	for _, d := range dirs {
		os.MkdirAll(d, 0o755)
	}

	// 如果指定了外部配置目录，复制到默认位置
	if configDir != defaultConfigDir {
		fmt.Println(i18n.T("init.copy.config", configDir, defaultConfigDir))
		if err := config.CopyConfig(configDir, filepath.Join(workDir, "config")); err != nil {
			fmt.Fprintln(os.Stderr, i18n.T("init.copy.config.warn", err))
		}
	}

	fmt.Println()
	fmt.Println(i18n.T("init.done"))
	fmt.Println(i18n.T("init.tree.header"))
	fmt.Println(i18n.T("init.tree", workDir))
	fmt.Println()
	fmt.Println(i18n.T("init.next"))
}
