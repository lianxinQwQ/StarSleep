package main

import (
	"fmt"
	"os"
	"path/filepath"
)

func syncPacstrap(root string, pkgs, expectedPkgs []string) {
	fmt.Println(T("sync.pacstrap"))
	os.MkdirAll(root, 0o755)

	alpmDB := filepath.Join(root, "var/lib/pacman/local/ALPM_DB_VERSION")
	if _, err := os.Stat(alpmDB); err == nil {
		fmt.Println(T("sync.incremental"))
		syncWithPacman(root, pkgs, expectedPkgs)
		return
	}

	fmt.Println(T("sync.fresh"))
	args := append([]string{"-K", "-c", root}, pkgs...)
	if err := run("pacstrap", args...); err != nil {
		fatal(T("pacstrap.failed", err))
	}
}
