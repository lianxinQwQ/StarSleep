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
		fatal(T("sync.unknown.tool", cfg.Helper))
	}

	fmt.Println(T("sync.stage.done", cfg.Name))
}

func printSyncHeader(cfg *LayerConfig, root string) {
	fmt.Println(T("sync.separator"))
	fmt.Println(T("sync.stage", cfg.Name))
	fmt.Println(T("sync.tool", cfg.Helper))
	fmt.Println(T("sync.target", root))
	switch cfg.Helper {
	case "pacstrap", "pacman", "paru":
		fmt.Println(T("sync.packages", len(cfg.Packages)))
	case "enable_service":
		fmt.Println(T("sync.services", len(cfg.Services)))
	}
	fmt.Println(T("sync.separator"))
}
