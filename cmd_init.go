// cmd_init.go — starsleep init 命令
//
// 初始化 StarSleep 工作环境：创建所需目录结构、检查依赖工具。
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

func cmdInit(args []string) {
	util.CheckRoot()
	configDir, remaining := config.ParseConfigFlags(defaultConfigDir, args)
	if len(remaining) > 0 {
		util.Fatal(i18n.T("init.unknown.arg", remaining[0]))
	}
	workDir := defaultWorkDir
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
	dirs := []string{
		filepath.Join(workDir, "layers"),
		filepath.Join(workDir, "snapshots"),
		filepath.Join(workDir, "shared/home"),
		filepath.Join(workDir, "shared/pacman-cache"),
		filepath.Join(workDir, "config/layers"),
		filepath.Join(workDir, "work/merged"),
		filepath.Join(workDir, "work/ovl_work"),
		filepath.Join(workDir, "logs"),
	}
	for _, d := range dirs {
		os.MkdirAll(d, 0o755)
	}
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
