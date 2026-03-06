// maintain.go — starsleep maintain 命令
//
// 动态维护模式：汇总每个 helper 的内容，直接操作当前运行系统。
// 读取所有层配置，按 helper 类型汇总软件包和服务，
// 然后使用 pacman/paru/systemctl 直接在当前系统上执行同步。
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func cmdMaintain(args []string) {
	checkRoot()

	configDir, remaining := parseConfigFlags(args)
	if len(remaining) > 0 {
		fatal(T("maintain.unknown.arg", remaining[0]))
	}

	layers, _, err := loadAllLayers(configDir)
	if err != nil {
		fatal(T("load.config.failed", err))
	}

	// 按 helper 类型汇总
	var officialPkgs []string // pacstrap + pacman 层的包
	var aurPkgs []string      // paru 层的包
	var services []string     // enable_service 层的服务

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

	// 所有包的合集（用于清理判断）
	allPkgs := make([]string, 0, len(officialPkgs)+len(aurPkgs))
	allPkgs = append(allPkgs, officialPkgs...)
	allPkgs = append(allPkgs, aurPkgs...)

	fmt.Println(T("maintain.separator"))
	fmt.Println(T("maintain.title"))
	fmt.Println(T("maintain.config.dir", configDir))
	fmt.Println(T("maintain.layer.count", len(layers)))
	fmt.Println(T("maintain.official.pkgs", len(officialPkgs)))
	fmt.Println(T("maintain.aur.pkgs", len(aurPkgs)))
	fmt.Println(T("maintain.services.count", len(services)))
	fmt.Println(T("maintain.separator"))

	root := "/"
	dbPath := filepath.Join(root, "var/lib/pacman")

	// 1. 清理：降级不在配置中的显式包，移除孤儿
	fmt.Println(T("maintain.step1"))
	maintainCleanup(root, dbPath, allPkgs)

	// 2. 安装官方仓库包
	if len(officialPkgs) > 0 {
		fmt.Println(T("maintain.step2"))
		args := append([]string{
			"-S", "--needed", "--noconfirm",
		}, officialPkgs...)
		if err := run("pacman", args...); err != nil {
			fatal(T("pacman.failed", err))
		}
	} else {
		fmt.Println(T("maintain.step2.skip"))
	}

	// 3. 安装 AUR 包
	if len(aurPkgs) > 0 {
		fmt.Println(T("maintain.step3"))
		paruArgs := append([]string{
			"-S", "--needed", "--noconfirm",
		}, aurPkgs...)
		if err := run("paru", paruArgs...); err != nil {
			fmt.Fprintln(os.Stderr, T("maintain.paru.warn", err))
		}
	} else {
		fmt.Println(T("maintain.step3.skip"))
	}

	// 4. 启用服务
	if len(services) > 0 {
		fmt.Println(T("maintain.step4"))
		enableServiceLive(services)
	} else {
		fmt.Println(T("maintain.step4.skip"))
	}

	fmt.Println(T("maintain.separator"))
	fmt.Println(T("maintain.done"))
}

// maintainCleanup 在当前系统上执行声明式清理
func maintainCleanup(root, dbPath string, expectedPkgs []string) {
	expectedSet := expandPkgGroups(expectedPkgs)

	// 降级多余的显式安装包为依赖
	explicitPkgs, err := listExplicitPkgs(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, T("maintain.query.failed", err))
	} else {
		var demote []string
		for _, pkg := range explicitPkgs {
			if !expectedSet[pkg] {
				demote = append(demote, pkg)
			}
		}
		if len(demote) > 0 {
			fmt.Println(T("maintain.demote", len(demote), strings.Join(demote, " ")))
			for _, pkg := range demote {
				runSilent("pacman", "--dbpath", dbPath, "-D", "--asdeps", pkg, "--noconfirm")
			}
		}
	}

	// 循环清理孤立依赖
	for {
		orphans, err := listOrphans(root)
		if err != nil || len(orphans) == 0 {
			break
		}
		fmt.Println(T("maintain.orphans", strings.Join(orphans, " ")))
		args := append([]string{"-Rs", "--noconfirm"}, orphans...)
		if err := run("pacman", args...); err != nil {
			fmt.Fprintln(os.Stderr, T("maintain.orphans.failed", err))
			break
		}
	}
}
