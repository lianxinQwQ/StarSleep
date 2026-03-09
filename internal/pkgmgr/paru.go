// paru.go — 基于 paru 的 AUR 包安装
package pkgmgr

import (
	"fmt"

	"starsleep/internal/i18n"
	"starsleep/internal/util"
)

// SyncParu 使用 paru 安装 AUR 软件包
//
// 通过 runuser -u builder 以非 root 用户身份运行 paru，
// 然后调用 CleanupPacman 声明式清理多余包。
//
// @param root 目标根目录路径
// @param pkgs AUR 包名列表
// @param expectedPkgs 累积包列表（用于后续清理）
// @throws paru 安装失败时调用 Fatal 退出
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
