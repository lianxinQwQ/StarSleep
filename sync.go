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
