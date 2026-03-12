// helper 包提供分层构建的同步分发器和所有 helper 实现。
//
// 每个 helper 对应一种层类型，负责在目标根目录上执行具体操作。
package helper

import (
	"fmt"

	"starsleep/internal/config"
	"starsleep/internal/i18n"
	"starsleep/internal/util"
)

// Dispatch 根据配置的 helper 类型分发同步操作
func Dispatch(root, configDir string, cfg *config.LayerConfig, expectedPkgs, expectedSvcs []string) {
	printSyncHeader(cfg, root)
	switch cfg.Helper {
	case "pacstrap":
		SyncPacstrap(root, cfg.Packages, expectedPkgs)
	case "pacman":
		SyncPacman(root, cfg.Packages, expectedPkgs)
	case "paru":
		SyncParu(root, cfg.Packages, expectedPkgs)
	case "enable_service":
		SyncEnableService(root, cfg.Services, expectedSvcs)
	case "copy_files":
		SyncCopyFiles(root, configDir, cfg.Files)
	case "chroot-cmd":
		SyncChrootCmd(root, cfg.Env, cfg.Commands)
	case "chroot-pacman":
		SyncChrootPacman(root, cfg.Env, cfg.Packages)
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
	case "pacstrap", "pacman", "paru", "chroot-pacman":
		fmt.Println(i18n.T("sync.packages", len(cfg.Packages)))
	case "enable_service":
		fmt.Println(i18n.T("sync.services", len(cfg.Services)))
	case "copy_files":
		fmt.Println(i18n.T("sync.files", len(cfg.Files)))
	case "chroot-cmd":
		fmt.Println(i18n.T("sync.commands", len(cfg.Commands)))
	}
	fmt.Println(i18n.T("sync.separator"))
}
