// pacman — 基于 pacman 的包同步和声明式清理
package helper

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"starsleep/internal/i18n"
	"starsleep/internal/pkgmgr"
	"starsleep/internal/util"
)

// CleanupPacman 声明式清理：降级多余显式包为依赖 → 循环清理孤立包
func CleanupPacman(root string, expectedPkgs []string) {
	expectedSet := pkgmgr.ExpandPkgGroups(expectedPkgs)
	dbPath := filepath.Join(root, "var/lib/pacman")
	explicitPkgs, err := pkgmgr.ListExplicitPkgs(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, i18n.T("sync.query.failed", err))
	} else {
		for _, pkg := range explicitPkgs {
			if !expectedSet[pkg] {
				fmt.Println(i18n.T("sync.demote", pkg))
				util.RunSilent("pacman", "--root", root, "--dbpath", dbPath,
					"-D", "--asdeps", pkg, "--noconfirm")
			}
		}
	}
	for {
		orphans, err := pkgmgr.ListOrphans(root)
		if err != nil || len(orphans) == 0 {
			break
		}
		fmt.Println(i18n.T("sync.orphans", strings.Join(orphans, " ")))
		args := append([]string{
			"--root", root, "--dbpath", dbPath,
			"-Rs", "--noconfirm",
		}, orphans...)
		if err := util.Run("pacman", args...); err != nil {
			fmt.Fprintln(os.Stderr, i18n.T("sync.orphans.failed", err))
			break
		}
	}
}

// SyncWithPacman 确保目标系统的显式安装包列表与配置一致
func SyncWithPacman(root string, installPkgs, expectedPkgs []string) {
	dbPath := filepath.Join(root, "var/lib/pacman")
	CleanupPacman(root, expectedPkgs)
	fmt.Println(i18n.T("sync.install.pkgs"))
	args := append([]string{
		"--root", root, "--dbpath", dbPath,
		"--config", "/etc/pacman.conf",
		"-S", "--needed", "--noconfirm",
	}, installPkgs...)
	if err := util.Run("pacman", args...); err != nil {
		util.Fatal(i18n.T("pacman.failed", err))
	}
}

// SyncPacman 使用 pacman 同步官方仓库软件包
func SyncPacman(root string, pkgs, expectedPkgs []string) {
	fmt.Println(i18n.T("sync.pacman"))
	SyncWithPacman(root, pkgs, expectedPkgs)
}
