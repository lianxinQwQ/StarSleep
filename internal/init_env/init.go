// init_env 包 — starsleep init 命令
//
// 初始化 StarSleep 工作环境：创建所需目录结构、检查依赖工具。
package init_env

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"starsleep/internal/config"
	"starsleep/internal/i18n"
	"starsleep/internal/util"
)

// Run 执行环境初始化命令
func Run(args []string) {
	util.CheckRoot()
	configDir, remaining := config.ParseConfigFlags(config.DefaultConfigDir, args)
	if len(remaining) > 0 {
		util.Fatal(i18n.T("init.unknown.arg", remaining[0]))
	}
	workDir := config.DefaultWorkDir

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
		filepath.Join(workDir, "layers"),
		filepath.Join(workDir, "snapshots"),
		filepath.Join(workDir, "shared"),
		filepath.Join(workDir, "config/layers"),
		filepath.Join(workDir, "config/files"),
		filepath.Join(workDir, "work/merged"),
		filepath.Join(workDir, "work/ovl_work"),
		filepath.Join(workDir, "logs"),
	}
	for _, d := range dirs {
		os.MkdirAll(d, 0o755)
	}

	// 共享缓存目录创建为 Btrfs 子卷（不存在时）
	for _, subvol := range []string{
		filepath.Join(workDir, "shared/pacman-cache"),
		filepath.Join(workDir, "shared/paru-cache"),
	} {
		if _, err := os.Stat(subvol); err == nil {
			continue
		}
		if err := util.Run("btrfs", "subvolume", "create", subvol); err != nil {
			util.Fatal(i18n.T("create.subvol.failed", subvol, err))
		}
	}

	// 如果指定了外部配置目录，复制到默认位置
	if configDir != config.DefaultConfigDir {
		fmt.Println(i18n.T("init.copy.config", configDir, config.DefaultConfigDir))
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
