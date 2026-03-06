package pkgmgr

import (
	"fmt"
	"os"
	"path/filepath"

	"starsleep/internal/i18n"
	"starsleep/internal/util"
)

// SyncPacstrap 使用 pacstrap 初始化或增量同步基础系统
func SyncPacstrap(root string, pkgs, expectedPkgs []string) {
	fmt.Println(i18n.T("sync.pacstrap"))
	os.MkdirAll(root, 0o755)

	alpmDB := filepath.Join(root, "var/lib/pacman/local/ALPM_DB_VERSION")
	if _, err := os.Stat(alpmDB); err == nil {
		fmt.Println(i18n.T("sync.incremental"))
		SyncWithPacman(root, pkgs, expectedPkgs)
		return
	}

	fmt.Println(i18n.T("sync.fresh"))
	args := append([]string{"-K", "-c", root}, pkgs...)
	if err := util.Run("pacstrap", args...); err != nil {
		util.Fatal(i18n.T("pacstrap.failed", err))
	}
}
