package main

import "fmt"

// syncParu 使用 paru 安装 AUR 软件包
func syncParu(root string, pkgs, expectedPkgs []string) {
	fmt.Println("[Sync] 使用 paru 安装 AUR 软件包...")

	paruArgs := append([]string{
		"-u", "builder", "--",
		"paru", "-S", "--needed", "--noconfirm",
		"--root", root,
	}, pkgs...)
	if err := run("runuser", paruArgs...); err != nil {
		fatal(fmt.Sprintf("paru 安装失败: %v", err))
	}

	cleanupPacman(root, expectedPkgs)
}
