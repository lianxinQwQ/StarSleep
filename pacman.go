package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// cleanupPacman 声明式清理：降级多余包 → 清理孤儿
func cleanupPacman(root string, expectedPkgs []string) {
	expectedSet := make(map[string]bool, len(expectedPkgs))
	for _, pkg := range expectedPkgs {
		expectedSet[pkg] = true
	}

	dbPath := filepath.Join(root, "var/lib/pacman")

	explicitPkgs, err := listExplicitPkgs(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[Sync] 警告: 查询显式包列表失败: %v，跳过降级步骤\n", err)
	} else {
		for _, pkg := range explicitPkgs {
			if !expectedSet[pkg] {
				fmt.Printf("[Sync] 降级为依赖: %s\n", pkg)
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
		fmt.Printf("[Sync] 清理孤立依赖: %s\n", strings.Join(orphans, " "))

		args := append([]string{
			"--root", root, "--dbpath", dbPath,
			"-Rs", "--noconfirm",
		}, orphans...)
		if err := run("pacman", args...); err != nil {
			fmt.Fprintf(os.Stderr, "[Sync] 警告: 清理孤立依赖失败: %v\n", err)
			break
		}
	}
}

// syncWithPacman 确保目标系统的显式安装包列表与配置一致
func syncWithPacman(root string, installPkgs, expectedPkgs []string) {
	dbPath := filepath.Join(root, "var/lib/pacman")

	cleanupPacman(root, expectedPkgs)

	fmt.Println("[Sync] 安装/更新软件包...")
	args := append([]string{
		"--root", root, "--dbpath", dbPath,
		"--config", "/etc/pacman.conf",
		"-S", "--needed", "--noconfirm",
	}, installPkgs...)
	if err := run("pacman", args...); err != nil {
		fatal(fmt.Sprintf("pacman 安装失败: %v", err))
	}
}

// syncPacman 使用 pacman 同步官方仓库软件包
func syncPacman(root string, pkgs, expectedPkgs []string) {
	fmt.Println("[Sync] 使用 pacman 同步官方仓库软件包...")
	syncWithPacman(root, pkgs, expectedPkgs)
}
