// pacstrap — 基于 pacstrap 的基础系统初始化
package helper

import (
	"fmt"
	"os"
	"path/filepath"

	"starsleep/internal/i18n"
	"starsleep/internal/util"
)

// SyncPacstrap 使用 pacstrap 初始化或增量同步基础系统
func SyncPacstrap(root, dbPath string, pkgs, expectedPkgs []string) {
	fmt.Println(i18n.T("sync.pacstrap"))
	os.MkdirAll(root, 0o755)

	alpmDB := filepath.Join(root, dbPath, "local/ALPM_DB_VERSION")
	if _, err := os.Stat(alpmDB); err == nil {
		fmt.Println(i18n.T("sync.incremental"))
		SyncWithPacman(root, dbPath, pkgs, expectedPkgs)
		return
	}

	fmt.Println(i18n.T("sync.fresh"))
	args := append([]string{"-K", "-c", root}, pkgs...)
	if err := util.Run("pacstrap", args...); err != nil {
		util.Fatal(i18n.T("pacstrap.failed", err))
	}
	// pacstrap 始终将数据库写入默认位置；若配置了自定义路径，则移至目标位置
	defaultDB := filepath.Join(root, "var/lib/pacman")
	customDB := filepath.Join(root, dbPath)
	if defaultDB != customDB {
		if err := os.Rename(defaultDB, customDB); err != nil {
			util.Fatal(i18n.T("pacstrap.failed", err))
		}
	}
}
