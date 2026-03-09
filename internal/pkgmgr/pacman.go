// pacman.go — 基于 pacman 的包同步和声明式清理
package pkgmgr

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"starsleep/internal/i18n"
	"starsleep/internal/util"
)

// CleanupPacman 声明式清理：降级多余显式包为依赖 → 循环清理孤立包
//
// 核心逻辑:
//  1. 展开组名得到完整的期望包集合
//  2. 查询当前显式安装的包，将不在期望集合中的降级为依赖
//  3. 循环调用 pacman -Rs 清理孤立依赖
//
// @param root 目标根目录路径
// @param expectedPkgs 期望的全量包名列表（包含组名）
func CleanupPacman(root string, expectedPkgs []string) {
	expectedSet := ExpandPkgGroups(expectedPkgs)
	dbPath := filepath.Join(root, "var/lib/pacman")
	explicitPkgs, err := ListExplicitPkgs(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, i18n.T("sync.query.failed", err))
	} else {
		for _, pkg := range explicitPkgs {
			if !expectedSet[pkg] {
				fmt.Println(i18n.T("sync.demote", pkg))
				util.RunSilent("pacman", "--root", root, "--dbpath", dbPath,
					"-D", "--asdeps", pkg, "--noconfirm")
			}
		}
	}
	for {
		orphans, err := ListOrphans(root)
		if err != nil || len(orphans) == 0 {
			break
		}
		fmt.Println(i18n.T("sync.orphans", strings.Join(orphans, " ")))
		args := append([]string{
			"--root", root, "--dbpath", dbPath,
			"-Rs", "--noconfirm",
		}, orphans...)
		if err := util.Run("pacman", args...); err != nil {
			fmt.Fprintln(os.Stderr, i18n.T("sync.orphans.failed", err))
			break
		}
	}
}

// SyncWithPacman 确保目标系统的显式安装包列表与配置一致
//
// 先执行声明式清理，然后用 pacman -S --needed 安装缺少的包。
//
// @param root 目标根目录路径
// @param installPkgs 当前层要安装的包列表
// @param expectedPkgs 到当前层为止的累积包列表（用于清理）
// @throws pacman 安装失败时调用 Fatal 退出
func SyncWithPacman(root string, installPkgs, expectedPkgs []string) {
	dbPath := filepath.Join(root, "var/lib/pacman")
	CleanupPacman(root, expectedPkgs)
	fmt.Println(i18n.T("sync.install.pkgs"))
	args := append([]string{
		"--root", root, "--dbpath", dbPath,
		"--config", "/etc/pacman.conf",
		"-S", "--needed", "--noconfirm",
	}, installPkgs...)
	if err := util.Run("pacman", args...); err != nil {
		util.Fatal(i18n.T("pacman.failed", err))
	}
}

// SyncPacman 使用 pacman 同步官方仓库软件包
//
// 封装 SyncWithPacman，添加同步开始日志。
//
// @param root 目标根目录路径
// @param pkgs 当前层要安装的包列表
// @param expectedPkgs 累积包列表
func SyncPacman(root string, pkgs, expectedPkgs []string) {
	fmt.Println(i18n.T("sync.pacman"))
	SyncWithPacman(root, pkgs, expectedPkgs)
}
