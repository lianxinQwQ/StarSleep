// init_cmd.go — starsleep init 命令
//
// 创建工作目录结构、复制配置模板
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func cmdInit(args []string) {
	checkRoot()

	configDir, remaining := parseConfigFlags(args)
	if len(remaining) > 0 {
		fatal(T("init.unknown.arg", remaining[0]))
	}

	workDir := defaultWorkDir

	// 依赖检查
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
	if err := run("modprobe", "overlay"); err != nil {
		missing = append(missing, "overlay (kernel module)")
	}
	if len(missing) > 0 {
		fmt.Fprintln(os.Stderr, T("missing.deps"))
		for _, m := range missing {
			fmt.Fprintf(os.Stderr, "  - %s\n", m)
		}
		os.Exit(1)
	}

	fmt.Println(T("init.start"))
	fmt.Println(T("init.workdir", workDir))

	// 创建目录结构
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

	// 复制配置
	if configDir != defaultConfigDir {
		fmt.Println(T("init.copy.config", configDir, defaultConfigDir))
		if err := copyConfig(configDir, filepath.Join(workDir, "config")); err != nil {
			fmt.Fprintln(os.Stderr, T("init.copy.config.warn", err))
		}
	}

	// 部署 starsleep-verify（仍为 bash 脚本）
	selfDir, _ := os.Executable()
	if selfDir != "" {
		selfDir = filepath.Dir(selfDir)
	}
	verifyScript := filepath.Join(selfDir, "starsleep-verify")
	if _, err := os.Stat(verifyScript); err == nil {
		data, err := os.ReadFile(verifyScript)
		if err == nil {
			dst := filepath.Join(workDir, "starsleep-verify")
			os.WriteFile(dst, data, 0o755)
			fmt.Println(T("init.deployed", dst))
		}
	}

	fmt.Println()
	fmt.Println(T("init.done"))
	fmt.Println(T("init.tree.header"))
	fmt.Println(T("init.tree", workDir))
	fmt.Println()
	fmt.Println(T("init.next"))
}
