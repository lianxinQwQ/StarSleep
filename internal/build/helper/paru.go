// paru — 基于 paru 的 AUR 包安装
package helper

import (
	"fmt"

	"starsleep/internal/i18n"
	"starsleep/internal/util"
)

// SyncParu 使用 paru 安装 AUR 软件包
func SyncParu(root string, pkgs, expectedPkgs []string) {
	fmt.Println(i18n.T("sync.paru"))

	paruArgs := append([]string{
		"-u", "builder", "--",
		"paru", "-S", "--needed", "--noconfirm",
		"--root", root,
	}, pkgs...)
	if err := util.Run("runuser", paruArgs...); err != nil {
		util.Fatal(i18n.T("paru.failed", err))
	}

	CleanupPacman(root, expectedPkgs)
}
