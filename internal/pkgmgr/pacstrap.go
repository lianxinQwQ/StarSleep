// pacstrap.go — 基于 pacstrap 的基础系统初始化
package pkgmgr

import (
	"fmt"
	"os"
	"path/filepath"

	"starsleep/internal/i18n"
	"starsleep/internal/util"
)

// SyncPacstrap 使用 pacstrap 初始化或增量同步基础系统
//
// 逻辑:
//   - 如果目标根已存在 ALPM 数据库，切换到增量同步模式（SyncWithPacman）
//   - 否则使用 pacstrap -K -c 全新引导
//
// @param root 目标根目录路径
// @param pkgs 当前层要安装的包列表
// @param expectedPkgs 累积包列表（用于增量同步时的声明式清理）
// @throws pacstrap 失败时调用 Fatal 退出
func SyncPacstrap(root string, pkgs, expectedPkgs []string) {
	fmt.Println(i18n.T("sync.pacstrap"))
	os.MkdirAll(root, 0o755)

	// 检查 ALPM 数据库是否已存在，判断是全新引导还是增量同步
	alpmDB := filepath.Join(root, "var/lib/pacman/local/ALPM_DB_VERSION")
	if _, err := os.Stat(alpmDB); err == nil {
		// 已有系统，使用 pacman 增量同步
		fmt.Println(i18n.T("sync.incremental"))
		SyncWithPacman(root, pkgs, expectedPkgs)
		return
	}

	// 全新引导：使用 pacstrap -K -c 初始化
	// -K: 初始化空 keyring  -c: 使用宿主机包缓存
	fmt.Println(i18n.T("sync.fresh"))
	args := append([]string{"-K", "-c", root}, pkgs...)
	if err := util.Run("pacstrap", args...); err != nil {
		util.Fatal(i18n.T("pacstrap.failed", err))
	}
}
