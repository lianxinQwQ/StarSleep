// paru — 基于 paru 的 AUR 包安装
package helper

import (
	"fmt"
	"os"
	"path/filepath"

	"starsleep/internal/i18n"
	"starsleep/internal/util"
)

// SyncParu 使用 paru 安装 AUR 软件包
func SyncParu(root, dbPath, cloneDir string, pkgs, expectedPkgs []string) {
	fmt.Println(i18n.T("sync.paru"))

	ensureBuilderLive(cloneDir)

	// 清除宿主机和目标根的残留 pacman 锁文件，防止 libalpm 初始化失败
	os.Remove(filepath.Join("/", dbPath, "db.lck"))
	os.Remove(filepath.Join(root, dbPath, "db.lck"))

	// 确保 paru 缓存目录及其中所有克隆仓库归属 builder，
	// 防止 git 的 safe.directory 检查因目录所有者与运行用户不同而失败
	util.Run("chown", "-R", "builder:builder", cloneDir)
	absDBPath := filepath.Join(root, dbPath)
	paruArgs := append([]string{
		"-u", "builder", "--",
		"paru", "-Sy", "--needed", "--noconfirm",
		"--clonedir", cloneDir,
		"--root", root,
		"--dbpath", absDBPath,
	}, pkgs...)
	if err := util.Run("runuser", paruArgs...); err != nil {
		util.Fatal(i18n.T("paru.failed", err))
	}

	CleanupPacman(root, dbPath, expectedPkgs)
}
