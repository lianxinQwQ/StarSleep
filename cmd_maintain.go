// cmd_maintain.go — starsleep maintain 命令
//
// 动态维护模式：汇总每个 helper 的内容，直接操作当前运行系统。
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"starsleep/internal/config"
	"starsleep/internal/i18n"
	"starsleep/internal/pkgmgr"
	"starsleep/internal/service"
	"starsleep/internal/util"
)

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

	var officialPkgs []string
	var aurPkgs []string
	var services []string

	for _, cfg := range layers {
		switch cfg.Helper {
		case "pacstrap", "pacman":
			officialPkgs = append(officialPkgs, cfg.Packages...)
		case "paru":
			aurPkgs = append(aurPkgs, cfg.Packages...)
		case "enable_service":
			services = append(services, cfg.Services...)
		}
	}

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
			"-S", "--needed", "--noconfirm",
		}, aurPkgs...)
		if err := util.Run("paru", paruArgs...); err != nil {
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

	fmt.Println(i18n.T("maintain.separator"))
	fmt.Println(i18n.T("maintain.done"))
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
