package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// cleanupPacman 声明式清理：降级多余包 → 清理孤儿
func cleanupPacman(root string, expectedPkgs []string) {
	expectedSet := expandPkgGroups(expectedPkgs)

	dbPath := filepath.Join(root, "var/lib/pacman")

	explicitPkgs, err := listExplicitPkgs(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, T("sync.query.failed", err))
	} else {
		for _, pkg := range explicitPkgs {
			if !expectedSet[pkg] {
				fmt.Println(T("sync.demote", pkg))
				runSilent("pacman", "--root", root, "--dbpath", dbPath,
					"-D", "--asdeps", pkg, "--noconfirm")
			}
		}
	}

	for {
		orphans, err := listOrphans(root)
		if err != nil || len(orphans) == 0 {
			break
		}
		fmt.Println(T("sync.orphans", strings.Join(orphans, " ")))

		args := append([]string{
			"--root", root, "--dbpath", dbPath,
			"-Rs", "--noconfirm",
		}, orphans...)
		if err := run("pacman", args...); err != nil {
			fmt.Fprintln(os.Stderr, T("sync.orphans.failed", err))
			break
		}
	}
}

// syncWithPacman 确保目标系统的显式安装包列表与配置一致
func syncWithPacman(root string, installPkgs, expectedPkgs []string) {
	dbPath := filepath.Join(root, "var/lib/pacman")

	cleanupPacman(root, expectedPkgs)

	fmt.Println(T("sync.install.pkgs"))
	args := append([]string{
		"--root", root, "--dbpath", dbPath,
		"--config", "/etc/pacman.conf",
		"-S", "--needed", "--noconfirm",
	}, installPkgs...)
	if err := run("pacman", args...); err != nil {
		fatal(T("pacman.failed", err))
	}
}

// syncPacman 使用 pacman 同步官方仓库软件包
func syncPacman(root string, pkgs, expectedPkgs []string) {
	fmt.Println(T("sync.pacman"))
	syncWithPacman(root, pkgs, expectedPkgs)
}
