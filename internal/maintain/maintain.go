// maintain 包 — 动态维护模式主逻辑
package maintain

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"starsleep/internal/build/helper"
	"starsleep/internal/compare"
	"starsleep/internal/config"
	"starsleep/internal/deploy"
	"starsleep/internal/i18n"
	"starsleep/internal/pkgmgr"
	"starsleep/internal/util"
)

// Run 执行动态维护命令
func Run(args []string) {
	util.CheckRoot()

	configDir, remaining := config.ParseConfigFlags(config.DefaultConfigDir, args)

	disauto := false
	var filtered []string
	for _, a := range remaining {
		if a == "--disauto" {
			disauto = true
		} else {
			filtered = append(filtered, a)
		}
	}
	if len(filtered) > 0 {
		util.Fatal(i18n.T("maintain.unknown.arg", filtered[0]))
	}

	layers, mainCfg, err := config.LoadAllLayers(configDir)
	if err != nil {
		util.Fatal(i18n.T("load.config.failed", err))
	}

	agg := AggregateAll(layers)

	// 统一期望包集合：所有 helper 类型的包合并
	allPkgs := make([]string, 0, len(agg.OfficialPkgs)+len(agg.AurPkgs))
	allPkgs = append(allPkgs, agg.OfficialPkgs...)
	allPkgs = append(allPkgs, agg.AurPkgs...)
	for _, cl := range agg.ChrootLayers {
		allPkgs = append(allPkgs, cl.Packages...)
	}

	fmt.Println(i18n.T("maintain.separator"))
	fmt.Println(i18n.T("maintain.title"))
	fmt.Println(i18n.T("maintain.config.dir", configDir))
	fmt.Println(i18n.T("maintain.layer.count", len(layers)))
	fmt.Println(i18n.T("maintain.total.pkgs", len(allPkgs)))
	fmt.Println(i18n.T("maintain.services.count", len(agg.Services)))
	fmt.Println(i18n.T("maintain.separator"))

	root := "/"
	dbPath := config.ResolveDBPath(mainCfg, config.DefaultDBPath)
	absDBPath := filepath.Join(root, dbPath)
	paruCache := filepath.Join(config.DefaultWorkDir, "shared/paru-cache")
	currentSnap := detectCurrentSnapshot()
	ts := util.Timestamp()

	// 预备步骤：保存当前系统状态
	preSnapName := currentSnap + "-before-maintain-" + ts
	preSnapshotDir := filepath.Join(config.DefaultWorkDir, "snapshots", preSnapName)
	fmt.Println(i18n.T("maintain.pre.snapshot", preSnapName))
	if err := util.Run("btrfs", "subvolume", "snapshot", "/", preSnapshotDir); err != nil {
		util.Fatal(i18n.T("snapshot.failed", err))
	}

	// 步骤 1：降级多余显式包
	fmt.Println(i18n.T("maintain.step1"))
	diff, err := compare.ComputePkgDiff(allPkgs, root, dbPath)
	if err != nil {
		util.Fatal(i18n.T("maintain.query.failed", err))
	}
	maintainDemote(absDBPath, diff.Extra, disauto)

	// 步骤 2：安装缺失包（paru 统一处理官方 + AUR）
	if len(diff.Missing) > 0 {
		fmt.Println(i18n.T("maintain.step2"))
		if disauto {
			fmt.Println(i18n.T("maintain.confirm.install", strings.Join(diff.Missing, " ")))
			if !confirmPrompt() {
				util.Fatal(i18n.T("maintain.aborted"))
			}
		}
		helper.EnsureBuilderUser()
		util.Run("chown", "-R", "builder:builder", paruCache)
		paruInstallArgs := append([]string{
			"-u", "builder", "--",
			"paru", "-S", "--needed", "--noconfirm",
			"--clonedir", paruCache,
		}, diff.Missing...)
		if err := util.Run("runuser", paruInstallArgs...); err != nil {
			fmt.Fprintln(os.Stderr, i18n.T("maintain.paru.warn", err))
		}
	} else {
		fmt.Println(i18n.T("maintain.step2.skip"))
	}

	// 步骤 3：清理孤立依赖
	fmt.Println(i18n.T("maintain.step3"))
	maintainRemoveOrphans(root, dbPath, disauto)

	// 步骤 4：全量更新
	fmt.Println(i18n.T("maintain.step.syu"))
	helper.EnsureBuilderUser()
	util.Run("chown", "-R", "builder:builder", paruCache)
	syuArgs := []string{"-u", "builder", "--", "paru", "-Syu", "--noconfirm", "--clonedir", paruCache}
	if err := util.Run("runuser", syuArgs...); err != nil {
		fmt.Fprintln(os.Stderr, i18n.T("maintain.paru.warn", err))
	}

	// 步骤 5：启用服务
	if len(agg.Services) > 0 {
		fmt.Println(i18n.T("maintain.step4"))
		helper.EnableServiceLive(agg.Services)
	} else {
		fmt.Println(i18n.T("maintain.step4.skip"))
	}

	// 步骤 6：叠加文件
	if len(agg.FileMappings) > 0 {
		fmt.Println(i18n.T("maintain.step.copyfiles"))
		helper.CopyFilesLive(configDir, agg.FileMappings)
	} else {
		fmt.Println(i18n.T("maintain.step.copyfiles.skip"))
	}

	// 步骤 7：执行 chroot-cmd（包安装已在步骤 2 统一处理，仅执行命令）
	hasChrootCmds := false
	for _, cl := range agg.ChrootLayers {
		if cl.Helper == "chroot-cmd" {
			hasChrootCmds = true
			break
		}
	}
	if hasChrootCmds {
		fmt.Println(i18n.T("maintain.step.chroot"))
		for _, cl := range agg.ChrootLayers {
			if cl.Helper == "chroot-cmd" {
				helper.ChrootCmdLive(cl.Env, cl.Commands)
			}
		}
	} else {
		fmt.Println(i18n.T("maintain.step.chroot.skip"))
	}

	// 步骤 8：创建快照并部署引导
	fmt.Println(i18n.T("maintain.step5"))
	newSnapName := currentSnap + "-" + ts
	snapshotDir := filepath.Join(config.DefaultWorkDir, "snapshots", newSnapName)

	fmt.Println(i18n.T("maintain.snapshot.name", newSnapName))
	if err := util.Run("btrfs", "subvolume", "snapshot", "/", snapshotDir); err != nil {
		util.Fatal(i18n.T("snapshot.failed", err))
	}

	latestLink := filepath.Join(config.DefaultWorkDir, "snapshots/latest")
	os.Remove(latestLink)
	os.Symlink(snapshotDir, latestLink)

	deploy.DeploySnapshot(snapshotDir, newSnapName)

	fmt.Println(i18n.T("maintain.separator"))
	fmt.Println(i18n.T("maintain.done"))
	fmt.Println()
	fmt.Println(i18n.T("deploy.reboot.hint"))
	fmt.Println(i18n.T("deploy.reboot.entry", deploy.EntryTitle, newSnapName))
}

func detectCurrentSnapshot() string {
	data, err := os.ReadFile("/proc/cmdline")
	if err != nil {
		util.Fatal(i18n.T("maintain.detect.failed", err))
	}
	for _, field := range strings.Fields(string(data)) {
		if idx := strings.Index(field, "subvol="); idx >= 0 {
			subvol := field[idx+len("subvol="):]
			return filepath.Base(subvol)
		}
	}
	util.Fatal(i18n.T("maintain.detect.failed", "subvol not found in /proc/cmdline"))
	return ""
}

func maintainDemote(absDBPath string, extra []string, disauto bool) {
	if len(extra) > 0 {
		fmt.Println(i18n.T("maintain.demote", len(extra), strings.Join(extra, " ")))
		if disauto {
			fmt.Println(i18n.T("maintain.confirm.demote"))
			if !confirmPrompt() {
				util.Fatal(i18n.T("maintain.aborted"))
			}
		}
		for _, pkg := range extra {
			util.RunSilent("pacman", "--dbpath", absDBPath, "-D", "--asdeps", pkg, "--noconfirm")
		}
	}
}

func maintainRemoveOrphans(root, dbPath string, disauto bool) {
	for {
		orphans, err := pkgmgr.ListOrphans(root, dbPath)
		if err != nil || len(orphans) == 0 {
			break
		}
		fmt.Println(i18n.T("maintain.orphans", strings.Join(orphans, " ")))
		if disauto {
			fmt.Println(i18n.T("maintain.confirm.orphans"))
			if !confirmPrompt() {
				util.Fatal(i18n.T("maintain.aborted"))
			}
		}
		args := append([]string{"-Rs", "--noconfirm"}, orphans...)
		if err := util.Run("pacman", args...); err != nil {
			fmt.Fprintln(os.Stderr, i18n.T("maintain.orphans.failed", err))
			break
		}
	}
}

// confirmPrompt 向终端打印提示并读取用户输入，返回是否确认
func confirmPrompt() bool {
	fmt.Print(i18n.T("maintain.confirm.prompt"))
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	answer := strings.TrimSpace(strings.ToLower(scanner.Text()))
	return answer == "y" || answer == "yes"
}
