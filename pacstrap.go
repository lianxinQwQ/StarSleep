package main

import (
	"fmt"
	"os"
	"path/filepath"
)

func syncPacstrap(root string, pkgs, expectedPkgs []string) {
	fmt.Println("[Sync] 使用 pacstrap 初始化基础根文件系统...")
	os.MkdirAll(root, 0o755)

	alpmDB := filepath.Join(root, "var/lib/pacman/local/ALPM_DB_VERSION")
	if _, err := os.Stat(alpmDB); err == nil {
		fmt.Println("[Sync] 检测到已有系统，执行增量同步...")
		syncWithPacman(root, pkgs, expectedPkgs)
		return
	}

	fmt.Println("[Sync] 全新引导...")
	args := append([]string{"-K", "-c", root}, pkgs...)
	if err := run("pacstrap", args...); err != nil {
		fatal(fmt.Sprintf("pacstrap 失败: %v", err))
	}
}
