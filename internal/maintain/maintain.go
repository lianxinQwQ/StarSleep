// maintain 包 — 动态维护模式主逻辑
package maintain

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"starsleep/internal/build/helper"
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
	if len(remaining) > 0 {
		util.Fatal(i18n.T("maintain.unknown.arg", remaining[0]))
	}

	layers, _, err := config.LoadAllLayers(configDir)
	if err != nil {
		util.Fatal(i18n.T("load.config.failed", err))
	}

	agg := AggregateAll(layers)

	allPkgs := make([]string, 0, len(agg.OfficialPkgs)+len(agg.AurPkgs))
	allPkgs = append(allPkgs, agg.OfficialPkgs...)
	allPkgs = append(allPkgs, agg.AurPkgs...)

	fmt.Println(i18n.T("maintain.separator"))
	fmt.Println(i18n.T("maintain.title"))
	fmt.Println(i18n.T("maintain.config.dir", configDir))
	fmt.Println(i18n.T("maintain.layer.count", len(layers)))
	fmt.Println(i18n.T("maintain.official.pkgs", len(agg.OfficialPkgs)))
	fmt.Println(i18n.T("maintain.aur.pkgs", len(agg.AurPkgs)))
	fmt.Println(i18n.T("maintain.services.count", len(agg.Services)))
	fmt.Println(i18n.T("maintain.separator"))

	root := "/"
	dbPath := filepath.Join(root, "var/lib/pacman")
	currentSnap := detectCurrentSnapshot()
	ts := util.Timestamp()

	// 预备步骤：保存当前系统状态
	preSnapName := currentSnap + "-before-maintain-" + ts
	preSnapshotDir := filepath.Join(config.DefaultWorkDir, "snapshots", preSnapName)
	fmt.Println(i18n.T("maintain.pre.snapshot", preSnapName))
	if err := util.Run("btrfs", "subvolume", "snapshot", "/", preSnapshotDir); err != nil {
		util.Fatal(i18n.T("snapshot.failed", err))
	}

	// 1. 清理
	fmt.Println(i18n.T("maintain.step1"))
	maintainCleanup(root, dbPath, allPkgs)

	// 2. 安装官方仓库包
	if len(agg.OfficialPkgs) > 0 {
		fmt.Println(i18n.T("maintain.step2"))
		args := append([]string{
			"-S", "--needed", "--noconfirm",
		}, agg.OfficialPkgs...)
		if err := util.Run("pacman", args...); err != nil {
			util.Fatal(i18n.T("pacman.failed", err))
		}
	} else {
		fmt.Println(i18n.T("maintain.step2.skip"))
	}

	// 3. 安装 AUR 包
	if len(agg.AurPkgs) > 0 {
		fmt.Println(i18n.T("maintain.step3"))
		paruArgs := append([]string{
			"-u", "builder", "--",
			"paru", "-S", "--needed", "--noconfirm",
			"--root", root,
		}, agg.AurPkgs...)
		if err := util.Run("runuser", paruArgs...); err != nil {
			fmt.Fprintln(os.Stderr, i18n.T("maintain.paru.warn", err))
		}
	} else {
		fmt.Println(i18n.T("maintain.step3.skip"))
	}

	// 4. 启用服务
	if len(agg.Services) > 0 {
		fmt.Println(i18n.T("maintain.step4"))
		helper.EnableServiceLive(agg.Services)
	} else {
		fmt.Println(i18n.T("maintain.step4.skip"))
	}

	// 4.5. 叠加文件
	if len(agg.FileMappings) > 0 {
		fmt.Println(i18n.T("maintain.step.copyfiles"))
		helper.CopyFilesLive(configDir, agg.FileMappings)
	} else {
		fmt.Println(i18n.T("maintain.step.copyfiles.skip"))
	}

	// 4.6. 执行 chroot 层
	if len(agg.ChrootLayers) > 0 {
		fmt.Println(i18n.T("maintain.step.chroot"))
		for _, cl := range agg.ChrootLayers {
			switch cl.Helper {
			case "chroot-cmd":
				helper.ChrootCmdLive(cl.Env, cl.Commands)
			case "chroot-pacman":
				helper.ChrootPacmanLive(cl.Env, cl.Packages)
			}
		}
	} else {
		fmt.Println(i18n.T("maintain.step.chroot.skip"))
	}

	// 5. 创建快照并部署引导
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

func maintainCleanup(root, dbPath string, expectedPkgs []string) {
	expectedSet := pkgmgr.ExpandPkgGroups(expectedPkgs)

	explicitPkgs, err := pkgmgr.ListExplicitPkgs(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, i18n.T("maintain.query.failed", err))
	} else {
		var demote []string
		for _, pkg := range explicitPkgs {
			if !expectedSet[pkg] {
				demote = append(demote, pkg)
			}
		}
		if len(demote) > 0 {
			fmt.Println(i18n.T("maintain.demote", len(demote), strings.Join(demote, " ")))
			for _, pkg := range demote {
				util.RunSilent("pacman", "--dbpath", dbPath, "-D", "--asdeps", pkg, "--noconfirm")
			}
		}
	}

	for {
		orphans, err := pkgmgr.ListOrphans(root)
		if err != nil || len(orphans) == 0 {
			break
		}
		fmt.Println(i18n.T("maintain.orphans", strings.Join(orphans, " ")))
		args := append([]string{"-Rs", "--noconfirm"}, orphans...)
		if err := util.Run("pacman", args...); err != nil {
			fmt.Fprintln(os.Stderr, i18n.T("maintain.orphans.failed", err))
			break
		}
	}
}
