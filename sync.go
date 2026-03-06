package main

import "fmt"

// syncLayer 根据配置的 helper 类型分发同步操作
func syncLayer(root string, cfg *LayerConfig, expectedPkgs, expectedSvcs []string) {
	printSyncHeader(cfg, root)

	switch cfg.Helper {
	case "pacstrap":
		syncPacstrap(root, cfg.Packages, expectedPkgs)
	case "pacman":
		syncPacman(root, cfg.Packages, expectedPkgs)
	case "paru":
		syncParu(root, cfg.Packages, expectedPkgs)
	case "enable_service":
		syncEnableService(root, cfg.Services, expectedSvcs)
	default:
		fatal(fmt.Sprintf("未知的工具: %s\n支持的工具: pacstrap, pacman, paru, enable_service", cfg.Helper))
	}

	fmt.Printf("[Sync] ✓ 阶段 %s 同步完成\n", cfg.Name)
}

func printSyncHeader(cfg *LayerConfig, root string) {
	fmt.Println("[Sync] ─────────────────────────────────────────────")
	fmt.Printf("[Sync] 阶段: %s\n", cfg.Name)
	fmt.Printf("[Sync] 工具: %s\n", cfg.Helper)
	fmt.Printf("[Sync] 目标: %s\n", root)
	switch cfg.Helper {
	case "pacstrap", "pacman", "paru":
		fmt.Printf("[Sync] 软件包: %d 个\n", len(cfg.Packages))
	case "enable_service":
		fmt.Printf("[Sync] 服务: %d 个\n", len(cfg.Services))
	}
	fmt.Println("[Sync] ─────────────────────────────────────────────")
}
