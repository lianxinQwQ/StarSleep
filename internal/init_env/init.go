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

type Options struct {
	WorkDir string
}

type commandRunner func(name string, args ...string) error

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

	if err := InitializeWorkspace(Options{WorkDir: workDir}); err != nil {
		util.Fatal(err.Error())
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

func InitializeWorkspace(opts Options) error {
	workDir := opts.WorkDir
	if workDir == "" {
		workDir = config.DefaultWorkDir
	}
	for _, d := range WorkspaceDirs(workDir) {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return err
		}
	}
	for _, subvol := range WorkspaceSubvolumes(workDir) {
		if err := ensureBtrfsSubvolume(subvol, util.Run); err != nil {
			return err
		}
	}
	if err := os.MkdirAll(filepath.Join(workDir, "shared/home/builder/.cache/paru/clone"), 0o755); err != nil {
		return err
	}
	if err := os.Chmod(filepath.Join(workDir, "shared/root"), 0o700); err != nil {
		return err
	}
	return nil
}

func WorkspaceDirs(workDir string) []string {
	return []string{
		workDir,
		filepath.Join(workDir, "layers"),
		filepath.Join(workDir, "snapshots"),
		filepath.Join(workDir, "shared"),
		filepath.Join(workDir, "config/layers"),
		filepath.Join(workDir, "config/files"),
		filepath.Join(workDir, "config", config.InheritDir),
		filepath.Join(workDir, "work/merged"),
		filepath.Join(workDir, "work/ovl_work"),
		filepath.Join(workDir, "logs"),
		filepath.Join(workDir, "var/log"),
		filepath.Join(workDir, "var/cache"),
	}
}

func WorkspaceSubvolumes(workDir string) []string {
	return []string{
		filepath.Join(workDir, "shared/home"),
		filepath.Join(workDir, "shared/root"),
		filepath.Join(workDir, "shared/pacman-cache"),
		filepath.Join(workDir, "shared/paru-cache"),
	}
}

func ensureBtrfsSubvolume(path string, run commandRunner) error {
	if _, err := os.Stat(path); err == nil {
		if err := run("btrfs", "subvolume", "show", path); err == nil {
			return nil
		}
		return fmt.Errorf("路径已存在但不是 Btrfs 子卷: %s", path)
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := run("btrfs", "subvolume", "create", path); err != nil {
		return fmt.Errorf("%s", i18n.T("create.subvol.failed", path, err))
	}
	return nil
}
