// cmd_maintain.go — starsleep maintain 命令
//
// 动态维护模式：汇总每个 helper 的内容，直接操作当前运行系统。
//
// 与 build 命令不同，maintain 不使用 OverlayFS 分层构建，
// 而是将所有层的包/服务汇总后直接在当前系统上执行:
//  0. 先创建维护前备份快照
//  1. 清理多余软件包（降级为依赖 + 清理孤立包）
//  2. 安装官方仓库包
//  3. 安装 AUR 包
//  4. 启用 systemd 服务
//  5. 创建维护后快照并部署引导条目
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"starsleep/internal/config"
	"starsleep/internal/copyfiles"
	"starsleep/internal/i18n"
	"starsleep/internal/pkgmgr"
	"starsleep/internal/service"
	"starsleep/internal/util"
)

// cmdMaintain 执行动态维护命令
//
// @param args 命令行参数，支持 -c/--config 指定配置目录
// @throws 配置加载失败、包安装失败、快照创建失败等情况下调用 Fatal 退出
func cmdMaintain(args []string) {
	util.CheckRoot()

	configDir, remaining := config.ParseConfigFlags(defaultConfigDir, args)
	if len(remaining) > 0 {
		util.Fatal(i18n.T("maintain.unknown.arg", remaining[0]))
	}

	layers, _, err := config.LoadAllLayers(configDir)
	if err != nil {
		util.Fatal(i18n.T("load.config.failed", err))
	}

	// ── 汇总所有层的包和服务 ──
	// 根据 helper 类型分类到官方包、AUR 包、服务三个列表
	var officialPkgs []string
	var aurPkgs []string
	var services []string
	var copyFileMappings []config.FileMapping

	for _, cfg := range layers {
		switch cfg.Helper {
		case "pacstrap", "pacman":
			officialPkgs = append(officialPkgs, cfg.Packages...)
		case "paru":
			aurPkgs = append(aurPkgs, cfg.Packages...)
		case "enable_service":
			services = append(services, cfg.Services...)
		case "copy_files":
			copyFileMappings = append(copyFileMappings, cfg.Files...)
		}
	}

	// 合并官方包和 AUR 包为全量期望包列表，用于声明式清理
	allPkgs := make([]string, 0, len(officialPkgs)+len(aurPkgs))
	allPkgs = append(allPkgs, officialPkgs...)
	allPkgs = append(allPkgs, aurPkgs...)

	fmt.Println(i18n.T("maintain.separator"))
	fmt.Println(i18n.T("maintain.title"))
	fmt.Println(i18n.T("maintain.config.dir", configDir))
	fmt.Println(i18n.T("maintain.layer.count", len(layers)))
	fmt.Println(i18n.T("maintain.official.pkgs", len(officialPkgs)))
	fmt.Println(i18n.T("maintain.aur.pkgs", len(aurPkgs)))
	fmt.Println(i18n.T("maintain.services.count", len(services)))
	fmt.Println(i18n.T("maintain.separator"))

	root := "/"
	dbPath := filepath.Join(root, "var/lib/pacman")
	currentSnap := detectCurrentSnapshot()
	ts := util.Timestamp()

	// 预备步骤：先保存当前系统状态，确保热更新前可回退
	preSnapName := currentSnap + "-before-maintain-" + ts
	preSnapshotDir := filepath.Join(defaultWorkDir, "snapshots", preSnapName)
	fmt.Println(i18n.T("maintain.pre.snapshot", preSnapName))
	if err := util.Run("btrfs", "subvolume", "snapshot", "/", preSnapshotDir); err != nil {
		util.Fatal(i18n.T("snapshot.failed", err))
	}

	// 1. 清理
	fmt.Println(i18n.T("maintain.step1"))
	maintainCleanup(root, dbPath, allPkgs)

	// 2. 安装官方仓库包
	if len(officialPkgs) > 0 {
		fmt.Println(i18n.T("maintain.step2"))
		args := append([]string{
			"-S", "--needed", "--noconfirm",
		}, officialPkgs...)
		if err := util.Run("pacman", args...); err != nil {
			util.Fatal(i18n.T("pacman.failed", err))
		}
	} else {
		fmt.Println(i18n.T("maintain.step2.skip"))
	}

	// 3. 安装 AUR 包
	if len(aurPkgs) > 0 {
		fmt.Println(i18n.T("maintain.step3"))
		paruArgs := append([]string{
			"-u", "builder", "--",
			"paru", "-S", "--needed", "--noconfirm",
			"--root", root,
		}, aurPkgs...)
		if err := util.Run("runuser", paruArgs...); err != nil {
			fmt.Fprintln(os.Stderr, i18n.T("maintain.paru.warn", err))
		}
	} else {
		fmt.Println(i18n.T("maintain.step3.skip"))
	}

	// 4. 启用服务
	if len(services) > 0 {
		fmt.Println(i18n.T("maintain.step4"))
		service.EnableServiceLive(services)
	} else {
		fmt.Println(i18n.T("maintain.step4.skip"))
	}

	// 4.5. 叠加文件
	if len(copyFileMappings) > 0 {
		fmt.Println(i18n.T("maintain.step.copyfiles"))
		copyfiles.CopyFilesLive(configDir, copyFileMappings)
	} else {
		fmt.Println(i18n.T("maintain.step.copyfiles.skip"))
	}

	// 5. 创建快照并部署引导
	fmt.Println(i18n.T("maintain.step5"))
	newSnapName := currentSnap + "-" + ts
	snapshotDir := filepath.Join(defaultWorkDir, "snapshots", newSnapName)

	// 创建当前系统根的 Btrfs 快照
	fmt.Println(i18n.T("maintain.snapshot.name", newSnapName))
	if err := util.Run("btrfs", "subvolume", "snapshot", "/", snapshotDir); err != nil {
		util.Fatal(i18n.T("snapshot.failed", err))
	}

	// 更新 latest 符号链接指向新快照
	latestLink := filepath.Join(defaultWorkDir, "snapshots/latest")
	os.Remove(latestLink)
	os.Symlink(snapshotDir, latestLink)

	// 部署快照的内核和 initramfs 到引导分区
	deploySnapshot(snapshotDir, newSnapName)

	fmt.Println(i18n.T("maintain.separator"))
	fmt.Println(i18n.T("maintain.done"))
	fmt.Println()
	fmt.Println(i18n.T("deploy.reboot.hint"))
	fmt.Println(i18n.T("deploy.reboot.entry", entryTitle, newSnapName))
}

// detectCurrentSnapshot 从 /proc/cmdline 中解析当前启动的快照名称
//
// 查找内核参数中的 rootflags=subvol=/.../snapshot-XXX 字段，
// 提取子卷路径的最后一部分作为快照名称。
//
// @return 当前快照名称（如 "snapshot-20260307-120000"）
// @throws 无法读取 /proc/cmdline 或找不到 subvol 字段时调用 Fatal 退出
func detectCurrentSnapshot() string {
	data, err := os.ReadFile("/proc/cmdline")
	if err != nil {
		util.Fatal(i18n.T("maintain.detect.failed", err))
	}
	for _, field := range strings.Fields(string(data)) {
		// rootflags=subvol=/starsleep/snapshots/snapshot-YYYYMMDD-HHMMSS
		if idx := strings.Index(field, "subvol="); idx >= 0 {
			subvol := field[idx+len("subvol="):]
			return filepath.Base(subvol)
		}
	}
	util.Fatal(i18n.T("maintain.detect.failed", "subvol not found in /proc/cmdline"))
	return ""
}

// maintainCleanup 声明式清理当前系统中的多余软件包
//
// 将不在期望列表中的显式安装包降级为依赖，
// 然后循环清理孤立依赖包直到无孤立包为止。
//
// @param root 目标根目录（维护模式下为 "/"）
// @param dbPath pacman 数据库路径
// @param expectedPkgs 期望的全量包名列表
func maintainCleanup(root, dbPath string, expectedPkgs []string) {
	// 将包名列表展开组名，得到完整的期望包名集合
	expectedSet := pkgmgr.ExpandPkgGroups(expectedPkgs)

	// 查询当前系统的显式安装包列表
	explicitPkgs, err := pkgmgr.ListExplicitPkgs(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, i18n.T("maintain.query.failed", err))
	} else {
		// 找出所有不在期望列表中的显式包，降级为依赖
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
