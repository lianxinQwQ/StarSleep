package main

import "fmt"

// syncParu 使用 paru 安装 AUR 软件包
func syncParu(root string, pkgs, expectedPkgs []string) {
	fmt.Println(T("sync.paru"))

	paruArgs := append([]string{
		"-u", "builder", "--",
		"paru", "-S", "--needed", "--noconfirm",
		"--root", root,
	}, pkgs...)
	if err := run("runuser", paruArgs...); err != nil {
		fatal(T("paru.failed", err))
	}

	cleanupPacman(root, expectedPkgs)
}
