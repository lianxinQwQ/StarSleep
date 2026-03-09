// sync.go — 层同步分发器
//
// 根据配置的 helper 类型将同步操作分发到对应的包管理器或服务管理模块。
package main

import (
	"fmt"

	"starsleep/internal/config"
	"starsleep/internal/i18n"
	"starsleep/internal/pkgmgr"
	"starsleep/internal/service"
	"starsleep/internal/util"
)

// syncLayer 根据配置的 helper 类型分发同步操作
//
// 支持的 helper 类型:
//   - pacstrap: 使用 pacstrap 初始化基础系统
//   - pacman: 使用 pacman 同步官方仓库包
//   - paru: 使用 paru 安装 AUR 包
//   - enable_service: 启用 systemd 服务
//
// @param root 目标根目录路径
// @param cfg 当前层配置
// @param expectedPkgs 到当前层为止的累积包列表（用于声明式清理）
// @param expectedSvcs 到当前层为止的累积服务列表（用于声明式清理）
// @throws 未知 helper 类型时调用 Fatal 退出
func syncLayer(root string, cfg *config.LayerConfig, expectedPkgs, expectedSvcs []string) {
	printSyncHeader(cfg, root)

	switch cfg.Helper {
	case "pacstrap":
		pkgmgr.SyncPacstrap(root, cfg.Packages, expectedPkgs)
	case "pacman":
		pkgmgr.SyncPacman(root, cfg.Packages, expectedPkgs)
	case "paru":
		pkgmgr.SyncParu(root, cfg.Packages, expectedPkgs)
	case "enable_service":
		service.SyncEnableService(root, cfg.Services, expectedSvcs)
	default:
		util.Fatal(i18n.T("sync.unknown.tool", cfg.Helper))
	}

	fmt.Println(i18n.T("sync.stage.done", cfg.Name))
}

// printSyncHeader 打印同步阶段的信息摘要头
//
// 显示当前阶段名称、使用的工具、目标路径以及包/服务数量。
//
// @param cfg 当前层配置
// @param root 目标根目录路径
func printSyncHeader(cfg *config.LayerConfig, root string) {
	fmt.Println(i18n.T("sync.separator"))
	fmt.Println(i18n.T("sync.stage", cfg.Name))
	fmt.Println(i18n.T("sync.tool", cfg.Helper))
	fmt.Println(i18n.T("sync.target", root))
	switch cfg.Helper {
	case "pacstrap", "pacman", "paru":
		fmt.Println(i18n.T("sync.packages", len(cfg.Packages)))
	case "enable_service":
		fmt.Println(i18n.T("sync.services", len(cfg.Services)))
	}
	fmt.Println(i18n.T("sync.separator"))
}
